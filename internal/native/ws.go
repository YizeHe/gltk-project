package native

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"groklang/gltk/internal/vm"
)

func moduleWS() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"connect": wsConnect,
		"send":    wsSend,
		"recv":    wsRecv,
		"close":   wsClose,
	})
}

type wsConn struct {
	c      *websocket.Conn
	closed bool
}

var (
	wsMu  sync.Mutex
	wsID  int64
	wsMap = map[int64]*wsConn{}
)

func allocWS(c *websocket.Conn) int64 {
	wsMu.Lock()
	defer wsMu.Unlock()
	wsID++
	id := wsID
	wsMap[id] = &wsConn{c: c}
	return id
}

func getWS(id int64) (*wsConn, error) {
	wsMu.Lock()
	defer wsMu.Unlock()
	c, ok := wsMap[id]
	if !ok || c == nil || c.closed {
		return nil, fmt.Errorf("ws: invalid id %d", id)
	}
	return c, nil
}

func freeWS(id int64) {
	wsMu.Lock()
	defer wsMu.Unlock()
	if c, ok := wsMap[id]; ok && c != nil {
		_ = c.c.Close()
		c.closed = true
		delete(wsMap, id)
	}
}

// ws.connect(url, opts?)
// opts: headers map, timeout, insecure (via custom dial — use http transport)
func wsConnect(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("ws.connect(url, opts?)")
	}
	urlStr := args[0].AsStr()
	timeout := 30 * time.Second
	hdr := http.Header{}
	if len(args) >= 2 && args[1].Typ == vm.TypeMap && args[1].Map != nil {
		m := *args[1].Map
		if v, ok := m["timeout"]; ok {
			if t, err := v.AsInt(); err == nil && t > 0 {
				timeout = time.Duration(t) * time.Second
			}
		}
		if v, ok := m["headers"]; ok && v.Typ == vm.TypeMap && v.Map != nil {
			for k, hv := range *v.Map {
				hdr.Set(k, hv.AsStr())
			}
		}
		// bare map as headers if no timeout key
		if _, ok := m["timeout"]; !ok {
			if _, ok := m["headers"]; !ok {
				for k, hv := range m {
					hdr.Set(k, hv.AsStr())
				}
			}
		}
	}
	dialer := websocket.Dialer{
		HandshakeTimeout: timeout,
		// RE-friendly: allow any TLS
		// Proxy from environment
		Proxy: http.ProxyFromEnvironment,
	}
	c, resp, err := dialer.Dial(urlStr, hdr)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		return vm.MapVal(map[string]vm.Value{
			"ok":     vm.Bool(false),
			"error":  vm.Str(err.Error()),
			"status": vm.Int(int64(status)),
		}), nil
	}
	id := allocWS(c)
	m := map[string]vm.Value{
		"ok":     vm.Bool(true),
		"id":     vm.Int(id),
		"_wid":   vm.Int(id),
		"url":    vm.Str(urlStr),
		"error":  vm.Str(""),
		"closed": vm.Bool(false),
	}
	m["send"] = vm.Native("ws.send", func(_ *vm.VM, a []vm.Value) (vm.Value, error) {
		if len(a) < 1 {
			return vm.Null(), errf("ws_conn.send(data, binary?)")
		}
		bin := false
		if len(a) >= 2 {
			bin = a[1].Truthy()
		}
		return wsSendID(id, a[0], bin)
	})
	m["recv"] = vm.Native("ws.recv", func(_ *vm.VM, a []vm.Value) (vm.Value, error) {
		sec := int64(30)
		if len(a) >= 1 {
			if t, err := a[0].AsInt(); err == nil {
				sec = t
			}
		}
		return wsRecvID(id, sec)
	})
	m["close"] = vm.Native("ws.close", func(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
		freeWS(id)
		m["closed"] = vm.Bool(true)
		return vm.Bool(true), nil
	})
	return vm.MapVal(m), nil
}

func wsSend(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("ws.send(conn, data, binary?)")
	}
	id, err := wsIDFrom(args[0])
	if err != nil {
		return vm.Null(), err
	}
	bin := false
	if len(args) >= 3 {
		bin = args[2].Truthy()
	}
	return wsSendID(id, args[1], bin)
}

func wsSendID(id int64, data vm.Value, binary bool) (vm.Value, error) {
	c, err := getWS(id)
	if err != nil {
		return vm.MapVal(map[string]vm.Value{"ok": vm.Bool(false), "error": vm.Str(err.Error())}), nil
	}
	var mt int
	var payload []byte
	if binary {
		mt = websocket.BinaryMessage
		if b, err := data.AsBytes(); err == nil {
			payload = b
		} else {
			payload = []byte(data.AsStr())
		}
	} else {
		mt = websocket.TextMessage
		if b, err := data.AsBytes(); err == nil {
			payload = b
		} else {
			payload = []byte(data.AsStr())
		}
	}
	if err := c.c.WriteMessage(mt, payload); err != nil {
		return vm.MapVal(map[string]vm.Value{"ok": vm.Bool(false), "error": vm.Str(err.Error())}), nil
	}
	return vm.MapVal(map[string]vm.Value{"ok": vm.Bool(true), "error": vm.Str("")}), nil
}

func wsRecv(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("ws.recv(conn, timeout_sec?)")
	}
	id, err := wsIDFrom(args[0])
	if err != nil {
		return vm.Null(), err
	}
	sec := int64(30)
	if len(args) >= 2 {
		if t, err := args[1].AsInt(); err == nil {
			sec = t
		}
	}
	return wsRecvID(id, sec)
}

func wsRecvID(id, sec int64) (vm.Value, error) {
	c, err := getWS(id)
	if err != nil {
		return vm.MapVal(map[string]vm.Value{"ok": vm.Bool(false), "error": vm.Str(err.Error())}), nil
	}
	if sec > 0 {
		_ = c.c.SetReadDeadline(time.Now().Add(time.Duration(sec) * time.Second))
	} else {
		_ = c.c.SetReadDeadline(time.Time{})
	}
	mt, data, err := c.c.ReadMessage()
	if err != nil {
		return vm.MapVal(map[string]vm.Value{
			"ok":    vm.Bool(false),
			"error": vm.Str(err.Error()),
			"type":  vm.Str(""),
			"data":  vm.Bytes(nil),
			"text":  vm.Str(""),
		}), nil
	}
	typ := "text"
	if mt == websocket.BinaryMessage {
		typ = "binary"
	} else if mt == websocket.CloseMessage {
		typ = "close"
	} else if mt == websocket.PingMessage {
		typ = "ping"
	} else if mt == websocket.PongMessage {
		typ = "pong"
	}
	return vm.MapVal(map[string]vm.Value{
		"ok":    vm.Bool(true),
		"error": vm.Str(""),
		"type":  vm.Str(typ),
		"data":  vm.Bytes(data),
		"text":  vm.Str(string(data)),
	}), nil
}

func wsClose(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Bool(false), nil
	}
	id, err := wsIDFrom(args[0])
	if err != nil {
		return vm.Bool(false), nil
	}
	freeWS(id)
	return vm.Bool(true), nil
}

func wsIDFrom(v vm.Value) (int64, error) {
	if v.Typ == vm.TypeInt {
		return v.AsInt()
	}
	if v.Typ == vm.TypeMap && v.Map != nil {
		if x, ok := (*v.Map)["_wid"]; ok {
			return x.AsInt()
		}
		if x, ok := (*v.Map)["id"]; ok {
			return x.AsInt()
		}
	}
	return 0, fmt.Errorf("ws: expected conn handle")
}
