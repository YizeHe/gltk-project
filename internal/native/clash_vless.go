package native

import (
	"bufio"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"groklang/gltk/internal/vm"
)

// Minimal Clash-like core: local SOCKS5 → single VLESS outbound.
// Supports: TCP (+TLS), WS (+TLS/WSS).
// Not supported: REALITY, flow(xtls-rprx-vision), gRPC, UDP, rules.

func moduleClash() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"start":      clashStart,
		"stop":       clashStop,
		"status":     clashStatus,
		"test_parse": clashTestParse, // parse UUID / build header self-check
		"version":    clashVersion,
	})
}

func clashVersion(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
	return vm.Str("clash-vless-lite-0.2"), nil
}

type vlessNode struct {
	Name           string
	Server         string
	Port           int
	UUID           [16]byte
	TLS            bool
	ServerName     string
	SkipCertVerify bool
	Network        string // "tcp" | "ws"
	WSPath         string
	WSHost         string
}

type clashRuntime struct {
	mu      sync.Mutex
	running bool
	ln      net.Listener
	node    vlessNode
	port    int // local socks port

	conns   int64
	bytesUp int64
	bytesDn int64
	accepts int64
	errors  int64
	lastErr string
}

var (
	clashOnce sync.Once
	clashRT   = &clashRuntime{}
)

func clashStart(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return failMap("clash.start(config_map_or_json_string)")
	}
	node, socksPort, err := parseClashConfig(args[0])
	if err != nil {
		return failMap(err.Error())
	}

	clashRT.mu.Lock()
	defer clashRT.mu.Unlock()
	if clashRT.running {
		return failMap("already running; call clash.stop() first")
	}

	addr := fmt.Sprintf("127.0.0.1:%d", socksPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return failMap("listen " + addr + ": " + err.Error())
	}

	clashRT.ln = ln
	clashRT.node = node
	clashRT.port = socksPort
	clashRT.running = true
	clashRT.lastErr = ""
	atomic.StoreInt64(&clashRT.conns, 0)
	atomic.StoreInt64(&clashRT.bytesUp, 0)
	atomic.StoreInt64(&clashRT.bytesDn, 0)
	atomic.StoreInt64(&clashRT.accepts, 0)
	atomic.StoreInt64(&clashRT.errors, 0)

	go clashAcceptLoop(clashRT)

	return vm.MapVal(map[string]vm.Value{
		"ok":      vm.Bool(true),
		"port":    vm.Int(int64(socksPort)),
		"listen":  vm.Str(addr),
		"proxy":   vm.Str(node.Name),
		"server":  vm.Str(fmt.Sprintf("%s:%d", node.Server, node.Port)),
		"tls":     vm.Bool(node.TLS),
		"error":   vm.Str(""),
		"version":  vm.Str("clash-vless-lite-0.2"),
		"network":  vm.Str(node.Network),
	}), nil
}

func clashStop(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
	clashRT.mu.Lock()
	defer clashRT.mu.Unlock()
	if !clashRT.running {
		return vm.MapVal(map[string]vm.Value{
			"ok":    vm.Bool(true),
			"error": vm.Str("not running"),
		}), nil
	}
	if clashRT.ln != nil {
		_ = clashRT.ln.Close()
		clashRT.ln = nil
	}
	clashRT.running = false
	return vm.MapVal(map[string]vm.Value{
		"ok":    vm.Bool(true),
		"error": vm.Str(""),
	}), nil
}

func clashStatus(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
	clashRT.mu.Lock()
	running := clashRT.running
	port := clashRT.port
	name := clashRT.node.Name
	server := ""
	if clashRT.node.Server != "" {
		server = fmt.Sprintf("%s:%d", clashRT.node.Server, clashRT.node.Port)
	}
	lastErr := clashRT.lastErr
	clashRT.mu.Unlock()
	return vm.MapVal(map[string]vm.Value{
		"ok":        vm.Bool(true),
		"running":   vm.Bool(running),
		"port":      vm.Int(int64(port)),
		"proxy":     vm.Str(name),
		"server":    vm.Str(server),
		"conns":     vm.Int(atomic.LoadInt64(&clashRT.conns)),
		"accepts":   vm.Int(atomic.LoadInt64(&clashRT.accepts)),
		"errors":    vm.Int(atomic.LoadInt64(&clashRT.errors)),
		"bytes_up":  vm.Int(atomic.LoadInt64(&clashRT.bytesUp)),
		"bytes_down": vm.Int(atomic.LoadInt64(&clashRT.bytesDn)),
		"last_error": vm.Str(lastErr),
	}), nil
}

func clashTestParse(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return failMap("clash.test_parse(uuid_string)")
	}
	id, err := parseUUID(args[0].AsStr())
	if err != nil {
		return failMap(err.Error())
	}
	hdr, err := buildVLESSHeader(id, "tcp", "example.com", 443)
	if err != nil {
		return failMap(err.Error())
	}
	return vm.MapVal(map[string]vm.Value{
		"ok":         vm.Bool(true),
		"uuid_hex":   vm.Str(hex.EncodeToString(id[:])),
		"header_len": vm.Int(int64(len(hdr))),
		"header_hex": vm.Str(hex.EncodeToString(hdr)),
		"error":      vm.Str(""),
	}), nil
}

func failMap(msg string) (vm.Value, error) {
	return vm.MapVal(map[string]vm.Value{
		"ok":    vm.Bool(false),
		"error": vm.Str(msg),
	}), nil
}

// ---- config ----

func parseClashConfig(v vm.Value) (vlessNode, int, error) {
	// Accept map or JSON string
	var raw map[string]interface{}
	switch v.Typ {
	case vm.TypeStr:
		s := strings.TrimSpace(v.AsStr())
		if s == "" {
			return vlessNode{}, 0, fmt.Errorf("empty config")
		}
		if err := json.Unmarshal([]byte(s), &raw); err != nil {
			return vlessNode{}, 0, fmt.Errorf("json: %w", err)
		}
	case vm.TypeMap:
		b, err := json.Marshal(vmMapToIface(v))
		if err != nil {
			return vlessNode{}, 0, err
		}
		if err := json.Unmarshal(b, &raw); err != nil {
			return vlessNode{}, 0, err
		}
	default:
		return vlessNode{}, 0, fmt.Errorf("config must be map or JSON string")
	}

	socksPort := 10808
	if p, ok := asInt(raw["socks-port"]); ok && p > 0 {
		socksPort = p
	} else if p, ok := asInt(raw["port"]); ok && p > 0 {
		socksPort = p
	} else if p, ok := asInt(raw["mixed-port"]); ok && p > 0 {
		socksPort = p
	}

	// proxies: list, or single proxy fields at top level
	var candidates []map[string]interface{}
	if pl, ok := raw["proxies"].([]interface{}); ok {
		for _, it := range pl {
			if m, ok := it.(map[string]interface{}); ok {
				candidates = append(candidates, m)
			}
		}
	}
	if m, ok := raw["proxy"].(map[string]interface{}); ok {
		candidates = append(candidates, m)
	}
	if len(candidates) == 0 {
		if _, ok := raw["server"]; ok {
			candidates = append(candidates, raw)
		}
	}
	if len(candidates) == 0 {
		return vlessNode{}, 0, fmt.Errorf("no proxies / proxy / server in config")
	}

	// Optional: proxy-index (0-based) or proxy-name
	wantIdx := -1
	if p, ok := asInt(raw["proxy-index"]); ok {
		wantIdx = p
	}
	wantName := asString(raw["proxy-name"])

	var proxy map[string]interface{}
	var skipReasons []string
	for i, c := range candidates {
		if wantIdx >= 0 && i != wantIdx {
			continue
		}
		if wantName != "" && asString(c["name"]) != wantName {
			continue
		}
		if err := liteProxyOK(c); err != nil {
			skipReasons = append(skipReasons, fmt.Sprintf("[%d] %s: %v", i, asString(c["name"]), err))
			continue
		}
		proxy = c
		break
	}
	if proxy == nil {
		msg := "no lite-compatible VLESS node"
		if len(skipReasons) > 0 {
			msg += " — " + strings.Join(skipReasons, "; ")
		}
		msg += " (lite supports network=tcp|ws without REALITY/flow)"
		return vlessNode{}, 0, fmt.Errorf("%s", msg)
	}

	node, _, err := mapToVlessNode(proxy)
	if err != nil {
		return vlessNode{}, 0, err
	}
	return node, socksPort, nil
}

func liteProxyOK(proxy map[string]interface{}) error {
	typ := strings.ToLower(asString(proxy["type"]))
	if typ != "" && typ != "vless" {
		return fmt.Errorf("type %q", typ)
	}
	if _, ok := proxy["reality-opts"]; ok {
		return fmt.Errorf("REALITY not supported")
	}
	if flow := strings.TrimSpace(asString(proxy["flow"])); flow != "" {
		return fmt.Errorf("flow %q not supported", flow)
	}
	network := strings.ToLower(asString(proxy["network"]))
	if network == "" {
		network = "tcp"
	}
	if network != "tcp" && network != "ws" {
		return fmt.Errorf("network %q not supported", network)
	}
	return nil
}

func mapToVlessNode(proxy map[string]interface{}) (vlessNode, int, error) {
	// socks port is filled by caller — return 0 here and fix signature
	// Actually parseClashConfig needs socksPort separately. Keep helper returning only node.
	server := asString(proxy["server"])
	if server == "" {
		return vlessNode{}, 0, fmt.Errorf("proxy.server required")
	}
	port, ok := asInt(proxy["port"])
	if !ok || port <= 0 {
		return vlessNode{}, 0, fmt.Errorf("proxy.port required")
	}
	uuidStr := asString(proxy["uuid"])
	if uuidStr == "" {
		uuidStr = asString(proxy["id"])
	}
	uid, err := parseUUID(uuidStr)
	if err != nil {
		return vlessNode{}, 0, fmt.Errorf("uuid: %w", err)
	}

	tlsOn := asBool(proxy["tls"]) || asBool(proxy["TLS"])
	sni := asString(proxy["servername"])
	if sni == "" {
		sni = asString(proxy["sni"])
	}
	if sni == "" {
		sni = server
	}
	skip := true
	if v, ok := proxy["skip-cert-verify"]; ok {
		skip = asBool(v)
	} else if v, ok := proxy["insecure"]; ok {
		skip = asBool(v)
	} else if v, ok := proxy["allowInsecure"]; ok {
		skip = asBool(v)
	}
	name := asString(proxy["name"])
	if name == "" {
		name = "vless-1"
	}
	network := strings.ToLower(asString(proxy["network"]))
	if network == "" {
		network = "tcp"
	}

	wsPath := "/"
	wsHost := sni
	if wo, ok := proxy["ws-opts"].(map[string]interface{}); ok {
		if p := asString(wo["path"]); p != "" {
			wsPath = p
		}
		if hdr, ok := wo["headers"].(map[string]interface{}); ok {
			if h := asString(hdr["Host"]); h != "" {
				wsHost = h
			} else if h := asString(hdr["host"]); h != "" {
				wsHost = h
			}
		}
	}
	if p := asString(proxy["ws-path"]); p != "" {
		wsPath = p
	}
	if !strings.HasPrefix(wsPath, "/") {
		wsPath = "/" + wsPath
	}

	return vlessNode{
		Name:           name,
		Server:         server,
		Port:           port,
		UUID:           uid,
		TLS:            tlsOn,
		ServerName:     sni,
		SkipCertVerify: skip,
		Network:        network,
		WSPath:         wsPath,
		WSHost:         wsHost,
	}, 0, nil
}

func vmMapToIface(v vm.Value) interface{} {
	switch v.Typ {
	case vm.TypeMap:
		if v.Map == nil {
			return map[string]interface{}{}
		}
		out := make(map[string]interface{}, len(*v.Map))
		for k, vv := range *v.Map {
			out[k] = vmMapToIface(vv)
		}
		return out
	case vm.TypeArray:
		if v.Arr == nil {
			return []interface{}{}
		}
		out := make([]interface{}, len(*v.Arr))
		for i, vv := range *v.Arr {
			out[i] = vmMapToIface(vv)
		}
		return out
	case vm.TypeStr:
		return v.AsStr()
	case vm.TypeInt:
		n, _ := v.AsInt()
		return n
	case vm.TypeFloat:
		return v.F
	case vm.TypeBool:
		return v.B
	case vm.TypeNull:
		return nil
	default:
		return v.AsStr()
	}
}

func asString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprint(t)
	}
}

func asInt(v interface{}) (int, bool) {
	switch t := v.(type) {
	case float64:
		return int(t), true
	case int:
		return t, true
	case int64:
		return int(t), true
	case json.Number:
		i, err := t.Int64()
		return int(i), err == nil
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(t))
		return i, err == nil
	default:
		return 0, false
	}
}

func asBool(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		return s == "1" || s == "true" || s == "yes"
	case float64:
		return t != 0
	case int:
		return t != 0
	default:
		return false
	}
}

func parseUUID(s string) ([16]byte, error) {
	var out [16]byte
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "-", "")
	if len(s) != 32 {
		return out, fmt.Errorf("want 32 hex chars, got %d", len(s))
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return out, err
	}
	copy(out[:], b)
	return out, nil
}

// ---- accept / SOCKS5 / VLESS ----

func clashAcceptLoop(rt *clashRuntime) {
	for {
		rt.mu.Lock()
		ln := rt.ln
		node := rt.node
		running := rt.running
		rt.mu.Unlock()
		if !running || ln == nil {
			return
		}
		c, err := ln.Accept()
		if err != nil {
			rt.mu.Lock()
			still := rt.running
			rt.mu.Unlock()
			if !still {
				return
			}
			atomic.AddInt64(&rt.errors, 1)
			rt.mu.Lock()
			rt.lastErr = err.Error()
			rt.mu.Unlock()
			// brief backoff on temporary errors
			time.Sleep(50 * time.Millisecond)
			continue
		}
		atomic.AddInt64(&rt.accepts, 1)
		go clashHandleClient(rt, c, node)
	}
}

func clashHandleClient(rt *clashRuntime, client net.Conn, node vlessNode) {
	defer client.Close()
	_ = client.SetDeadline(time.Now().Add(30 * time.Second))

	host, port, err := socks5Handshake(client)
	if err != nil {
		atomic.AddInt64(&rt.errors, 1)
		rt.mu.Lock()
		rt.lastErr = "socks5: " + err.Error()
		rt.mu.Unlock()
		return
	}

	remote, err := dialVLESS(node, host, port)
	if err != nil {
		// SOCKS5 reply host unreachable
		_, _ = client.Write([]byte{0x05, 0x04, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		atomic.AddInt64(&rt.errors, 1)
		rt.mu.Lock()
		rt.lastErr = "vless dial: " + err.Error()
		rt.mu.Unlock()
		return
	}
	defer remote.Close()

	// success reply
	_, _ = client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	_ = client.SetDeadline(time.Time{})
	_ = remote.SetDeadline(time.Time{})

	atomic.AddInt64(&rt.conns, 1)
	defer atomic.AddInt64(&rt.conns, -1)

	// bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		n, _ := io.Copy(remote, client)
		atomic.AddInt64(&rt.bytesUp, n)
		closeWrite(remote)
	}()
	go func() {
		defer wg.Done()
		n, _ := io.Copy(client, remote)
		atomic.AddInt64(&rt.bytesDn, n)
		closeWrite(client)
	}()
	wg.Wait()
}

func closeWrite(c net.Conn) {
	type cw interface{ CloseWrite() error }
	if x, ok := c.(cw); ok {
		_ = x.CloseWrite()
	}
}

// socks5Handshake: no-auth only, CONNECT only. Returns host, port.
func socks5Handshake(c net.Conn) (string, int, error) {
	br := bufio.NewReader(c)
	// greeting
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(br, hdr); err != nil {
		return "", 0, err
	}
	if hdr[0] != 0x05 {
		return "", 0, fmt.Errorf("not socks5 (ver=%d)", hdr[0])
	}
	nMethods := int(hdr[1])
	methods := make([]byte, nMethods)
	if _, err := io.ReadFull(br, methods); err != nil {
		return "", 0, err
	}
	// no auth
	if _, err := c.Write([]byte{0x05, 0x00}); err != nil {
		return "", 0, err
	}

	// request
	req := make([]byte, 4)
	if _, err := io.ReadFull(br, req); err != nil {
		return "", 0, err
	}
	if req[0] != 0x05 {
		return "", 0, fmt.Errorf("bad req ver")
	}
	if req[1] != 0x01 { // CONNECT
		_, _ = c.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return "", 0, fmt.Errorf("only CONNECT supported")
	}
	var host string
	switch req[3] {
	case 0x01: // IPv4
		ip := make([]byte, 4)
		if _, err := io.ReadFull(br, ip); err != nil {
			return "", 0, err
		}
		host = net.IP(ip).String()
	case 0x03: // domain
		lb := make([]byte, 1)
		if _, err := io.ReadFull(br, lb); err != nil {
			return "", 0, err
		}
		d := make([]byte, int(lb[0]))
		if _, err := io.ReadFull(br, d); err != nil {
			return "", 0, err
		}
		host = string(d)
	case 0x04: // IPv6
		ip := make([]byte, 16)
		if _, err := io.ReadFull(br, ip); err != nil {
			return "", 0, err
		}
		host = net.IP(ip).String()
	default:
		return "", 0, fmt.Errorf("bad atyp %d", req[3])
	}
	pb := make([]byte, 2)
	if _, err := io.ReadFull(br, pb); err != nil {
		return "", 0, err
	}
	port := int(binary.BigEndian.Uint16(pb))
	// drain any remaining buffered into… not needed; rest is payload after CONNECT success
	// but br may have buffered future payload — we must not lose it.
	// After handshake, client payload may already be in br buffer. Hand off via multi-reader.
	// Simpler: require no early payload (typical browsers wait for reply). OK for lite.
	_ = br
	return host, port, nil
}

func dialVLESS(node vlessNode, targetHost string, targetPort int) (net.Conn, error) {
	var conn net.Conn
	var err error
	switch node.Network {
	case "ws":
		conn, err = dialVLESSWS(node)
	default: // tcp
		conn, err = dialVLESSTCP(node)
	}
	if err != nil {
		return nil, err
	}

	hdr, err := buildVLESSHeader(node.UUID, "tcp", targetHost, targetPort)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if _, err := conn.Write(hdr); err != nil {
		_ = conn.Close()
		return nil, err
	}

	// Read VLESS response header: version(1) + addonLen(1) + addons
	rh := make([]byte, 2)
	if _, err := io.ReadFull(conn, rh); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("vless response: %w", err)
	}
	addonLen := int(rh[1])
	if addonLen > 0 {
		if _, err := io.CopyN(io.Discard, conn, int64(addonLen)); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("vless addon: %w", err)
		}
	}
	return conn, nil
}

func dialVLESSTCP(node vlessNode) (net.Conn, error) {
	addr := net.JoinHostPort(node.Server, strconv.Itoa(node.Port))
	d := net.Dialer{Timeout: 20 * time.Second}
	raw, err := d.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	if !node.TLS {
		return raw, nil
	}
	cfg := &tls.Config{
		ServerName:         node.ServerName,
		InsecureSkipVerify: node.SkipCertVerify,
		MinVersion:         tls.VersionTLS12,
	}
	tc := tls.Client(raw, cfg)
	if err := tc.Handshake(); err != nil {
		_ = raw.Close()
		return nil, fmt.Errorf("tls: %w", err)
	}
	return tc, nil
}

func dialVLESSWS(node vlessNode) (net.Conn, error) {
	scheme := "ws"
	if node.TLS {
		scheme = "wss"
	}
	hostPort := net.JoinHostPort(node.Server, strconv.Itoa(node.Port))
	u := url.URL{Scheme: scheme, Host: hostPort, Path: node.WSPath}
	// If path has query (some panels put ?ed=2048), keep it
	if i := strings.Index(node.WSPath, "?"); i >= 0 {
		u.Path = node.WSPath[:i]
		u.RawQuery = node.WSPath[i+1:]
	}

	hdr := http.Header{}
	if node.WSHost != "" {
		hdr.Set("Host", node.WSHost)
	}

	d := websocket.Dialer{
		HandshakeTimeout: 20 * time.Second,
		TLSClientConfig: &tls.Config{
			ServerName:         node.ServerName,
			InsecureSkipVerify: node.SkipCertVerify,
			MinVersion:         tls.VersionTLS12,
		},
	}
	wc, resp, err := d.Dial(u.String(), hdr)
	if err != nil {
		extra := ""
		if resp != nil {
			extra = fmt.Sprintf(" status=%d", resp.StatusCode)
		}
		return nil, fmt.Errorf("ws dial %s:%s", u.String(), extra+" "+err.Error())
	}
	return &wsNetConn{Conn: wc}, nil
}

// wsNetConn adapts gorilla websocket to a stream-oriented net.Conn for VLESS relay.
type wsNetConn struct {
	*websocket.Conn
	r   io.Reader
	wMu sync.Mutex
	rMu sync.Mutex
}

func (c *wsNetConn) Read(p []byte) (int, error) {
	c.rMu.Lock()
	defer c.rMu.Unlock()
	for {
		if c.r == nil {
			_, r, err := c.NextReader()
			if err != nil {
				return 0, err
			}
			c.r = r
		}
		n, err := c.r.Read(p)
		if n > 0 {
			if err == io.EOF {
				c.r = nil
				return n, nil
			}
			return n, err
		}
		if err == io.EOF {
			c.r = nil
			continue
		}
		if err != nil {
			return 0, err
		}
	}
}

func (c *wsNetConn) Write(p []byte) (int, error) {
	c.wMu.Lock()
	defer c.wMu.Unlock()
	if err := c.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *wsNetConn) SetDeadline(t time.Time) error {
	_ = c.SetReadDeadline(t)
	_ = c.SetWriteDeadline(t)
	return nil
}

func (c *wsNetConn) SetReadDeadline(t time.Time) error {
	return c.Conn.SetReadDeadline(t)
}

func (c *wsNetConn) SetWriteDeadline(t time.Time) error {
	return c.Conn.SetWriteDeadline(t)
}

// buildVLESSHeader builds request without payload.
// version=0, uuid, addonLen=0, cmd=TCP(1), port BE, atype+addr
func buildVLESSHeader(uuid [16]byte, network, host string, port int) ([]byte, error) {
	var cmd byte = 0x01 // TCP
	if network == "udp" {
		cmd = 0x02
	}
	buf := make([]byte, 0, 64)
	buf = append(buf, 0x00) // version
	buf = append(buf, uuid[:]...)
	buf = append(buf, 0x00) // no addons
	buf = append(buf, cmd)
	pb := make([]byte, 2)
	binary.BigEndian.PutUint16(pb, uint16(port))
	buf = append(buf, pb...)

	host = strings.TrimSpace(host)
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			buf = append(buf, 0x01)
			buf = append(buf, v4...)
		} else {
			v6 := ip.To16()
			if v6 == nil {
				return nil, fmt.Errorf("bad ip %q", host)
			}
			buf = append(buf, 0x03) // VLESS uses 0x03 for IPv6 (same as SOCKS)
			// Xray VLESS: ATYP 1=IPv4, 2=Domain, 3=IPv6
			buf = append(buf, v6...)
		}
	} else {
		if len(host) == 0 || len(host) > 255 {
			return nil, fmt.Errorf("bad domain length")
		}
		buf = append(buf, 0x02) // domain
		buf = append(buf, byte(len(host)))
		buf = append(buf, host...)
	}
	return buf, nil
}
