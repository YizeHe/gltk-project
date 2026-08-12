package native

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"groklang/gltk/internal/vm"
)

func moduleNet() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"dial":    netDial,
		"resolve": netResolve,
		"listen":  netListen,
		"accept":  netAccept,
		// low-level on id (also available as handle methods)
		"read":    netRead,
		"write":   netWrite,
		"close":   netClose,
		"set_deadline": netSetDeadline,
	})
}

type netConn struct {
	c       net.Conn
	network string
	addr    string
	closed  bool
}

type netListener struct {
	ln   net.Listener
	addr string
}

var (
	netMu   sync.Mutex
	netID   int64
	netCs   = map[int64]*netConn{}
	netLns  = map[int64]*netListener{}
)

func allocConn(c net.Conn, network, addr string) int64 {
	netMu.Lock()
	defer netMu.Unlock()
	netID++
	id := netID
	netCs[id] = &netConn{c: c, network: network, addr: addr}
	return id
}

func getConn(id int64) (*netConn, error) {
	netMu.Lock()
	defer netMu.Unlock()
	c, ok := netCs[id]
	if !ok || c == nil || c.closed {
		return nil, fmt.Errorf("net: invalid conn id %d", id)
	}
	return c, nil
}

func freeConn(id int64) {
	netMu.Lock()
	defer netMu.Unlock()
	if c, ok := netCs[id]; ok && c != nil {
		_ = c.c.Close()
		c.closed = true
		delete(netCs, id)
	}
}

// net.dial(network, address, timeout_sec?)
// network: "tcp", "tcp4", "tcp6", "udp", "unix"
func netDial(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("net.dial(network, address, timeout_sec?)")
	}
	network := args[0].AsStr()
	address := args[1].AsStr()
	timeout := 30 * time.Second
	if len(args) >= 3 {
		if t, err := args[2].AsInt(); err == nil && t > 0 {
			timeout = time.Duration(t) * time.Second
		}
	}
	d := net.Dialer{Timeout: timeout}
	c, err := d.Dial(network, address)
	if err != nil {
		return vm.MapVal(map[string]vm.Value{
			"ok":    vm.Bool(false),
			"error": vm.Str(err.Error()),
		}), nil
	}
	id := allocConn(c, network, address)
	return makeNetHandle(id, network, address, c.LocalAddr().String(), c.RemoteAddr().String()), nil
}

func makeNetHandle(id int64, network, addr, local, remote string) vm.Value {
	m := map[string]vm.Value{
		"ok":       vm.Bool(true),
		"id":       vm.Int(id),
		"_nid":     vm.Int(id),
		"network":  vm.Str(network),
		"address":  vm.Str(addr),
		"local":    vm.Str(local),
		"remote":   vm.Str(remote),
		"closed":   vm.Bool(false),
		"error":    vm.Str(""),
	}
	m["read"] = vm.Native("conn.read", func(_ *vm.VM, args []vm.Value) (vm.Value, error) {
		n := int64(4096)
		if len(args) >= 1 {
			if x, err := args[0].AsInt(); err == nil && x > 0 {
				n = x
			}
		}
		return netReadID(id, n)
	})
	m["write"] = vm.Native("conn.write", func(_ *vm.VM, args []vm.Value) (vm.Value, error) {
		if len(args) < 1 {
			return vm.Null(), errf("conn.write(data)")
		}
		return netWriteID(id, args[0])
	})
	m["close"] = vm.Native("conn.close", func(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
		freeConn(id)
		m["closed"] = vm.Bool(true)
		return vm.Bool(true), nil
	})
	m["set_deadline"] = vm.Native("conn.set_deadline", func(_ *vm.VM, args []vm.Value) (vm.Value, error) {
		sec := int64(30)
		if len(args) >= 1 {
			if x, err := args[0].AsInt(); err == nil {
				sec = x
			}
		}
		return netDeadlineID(id, sec)
	})
	return vm.MapVal(m)
}

func netResolve(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("net.resolve(host)")
	}
	host := args[0].AsStr()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return vm.MapVal(map[string]vm.Value{
			"ok":    vm.Bool(false),
			"error": vm.Str(err.Error()),
			"addrs": vm.Array(nil),
		}), nil
	}
	arr := make([]vm.Value, 0, len(ips))
	for _, ip := range ips {
		arr = append(arr, vm.Str(ip.IP.String()))
	}
	return vm.MapVal(map[string]vm.Value{
		"ok":    vm.Bool(true),
		"error": vm.Str(""),
		"addrs": vm.Array(arr),
	}), nil
}

func netListen(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("net.listen(network, address)")
	}
	network := args[0].AsStr()
	address := args[1].AsStr()
	ln, err := net.Listen(network, address)
	if err != nil {
		return vm.MapVal(map[string]vm.Value{
			"ok":    vm.Bool(false),
			"error": vm.Str(err.Error()),
		}), nil
	}
	netMu.Lock()
	netID++
	id := netID
	netLns[id] = &netListener{ln: ln, addr: ln.Addr().String()}
	netMu.Unlock()
	m := map[string]vm.Value{
		"ok":      vm.Bool(true),
		"id":      vm.Int(id),
		"_lid":    vm.Int(id),
		"address": vm.Str(ln.Addr().String()),
		"error":   vm.Str(""),
	}
	m["accept"] = vm.Native("listener.accept", func(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
		return netAcceptID(id)
	})
	m["close"] = vm.Native("listener.close", func(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
		netMu.Lock()
		defer netMu.Unlock()
		if l, ok := netLns[id]; ok {
			_ = l.ln.Close()
			delete(netLns, id)
		}
		return vm.Bool(true), nil
	})
	return vm.MapVal(m), nil
}

func netAccept(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("net.accept(listener_id_or_handle)")
	}
	id, err := netIDFrom(args[0], "_lid")
	if err != nil {
		return vm.Null(), err
	}
	return netAcceptID(id)
}

func netAcceptID(lid int64) (vm.Value, error) {
	netMu.Lock()
	l, ok := netLns[lid]
	netMu.Unlock()
	if !ok || l == nil {
		return vm.MapVal(map[string]vm.Value{"ok": vm.Bool(false), "error": vm.Str("bad listener")}), nil
	}
	c, err := l.ln.Accept()
	if err != nil {
		return vm.MapVal(map[string]vm.Value{"ok": vm.Bool(false), "error": vm.Str(err.Error())}), nil
	}
	id := allocConn(c, c.RemoteAddr().Network(), c.RemoteAddr().String())
	return makeNetHandle(id, c.RemoteAddr().Network(), c.RemoteAddr().String(), c.LocalAddr().String(), c.RemoteAddr().String()), nil
}

func netRead(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("net.read(conn, n?)")
	}
	id, err := netIDFrom(args[0], "_nid")
	if err != nil {
		return vm.Null(), err
	}
	n := int64(4096)
	if len(args) >= 2 {
		if x, err := args[1].AsInt(); err == nil && x > 0 {
			n = x
		}
	}
	return netReadID(id, n)
}

func netReadID(id, n int64) (vm.Value, error) {
	c, err := getConn(id)
	if err != nil {
		return vm.MapVal(map[string]vm.Value{"ok": vm.Bool(false), "error": vm.Str(err.Error()), "data": vm.Bytes(nil)}), nil
	}
	if n <= 0 {
		n = 4096
	}
	if n > 16<<20 {
		n = 16 << 20
	}
	buf := make([]byte, n)
	nr, err := c.c.Read(buf)
	if err != nil && err != io.EOF {
		return vm.MapVal(map[string]vm.Value{
			"ok":    vm.Bool(false),
			"error": vm.Str(err.Error()),
			"data":  vm.Bytes(nil),
			"n":     vm.Int(0),
			"eof":   vm.Bool(false),
		}), nil
	}
	return vm.MapVal(map[string]vm.Value{
		"ok":    vm.Bool(true),
		"error": vm.Str(""),
		"data":  vm.Bytes(buf[:nr]),
		"n":     vm.Int(int64(nr)),
		"eof":   vm.Bool(err == io.EOF || nr == 0),
	}), nil
}

func netWrite(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("net.write(conn, data)")
	}
	id, err := netIDFrom(args[0], "_nid")
	if err != nil {
		return vm.Null(), err
	}
	return netWriteID(id, args[1])
}

func netWriteID(id int64, data vm.Value) (vm.Value, error) {
	c, err := getConn(id)
	if err != nil {
		return vm.MapVal(map[string]vm.Value{"ok": vm.Bool(false), "error": vm.Str(err.Error()), "n": vm.Int(0)}), nil
	}
	var b []byte
	if bs, err := data.AsBytes(); err == nil {
		b = bs
	} else {
		b = []byte(data.AsStr())
	}
	nw, err := c.c.Write(b)
	if err != nil {
		return vm.MapVal(map[string]vm.Value{"ok": vm.Bool(false), "error": vm.Str(err.Error()), "n": vm.Int(int64(nw))}), nil
	}
	return vm.MapVal(map[string]vm.Value{"ok": vm.Bool(true), "error": vm.Str(""), "n": vm.Int(int64(nw))}), nil
}

func netClose(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Bool(false), nil
	}
	id, err := netIDFrom(args[0], "_nid")
	if err != nil {
		// try listener
		if lid, e2 := netIDFrom(args[0], "_lid"); e2 == nil {
			netMu.Lock()
			if l, ok := netLns[lid]; ok {
				_ = l.ln.Close()
				delete(netLns, lid)
			}
			netMu.Unlock()
			return vm.Bool(true), nil
		}
		return vm.Bool(false), nil
	}
	freeConn(id)
	return vm.Bool(true), nil
}

func netSetDeadline(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("net.set_deadline(conn, sec?)")
	}
	id, err := netIDFrom(args[0], "_nid")
	if err != nil {
		return vm.Null(), err
	}
	sec := int64(30)
	if len(args) >= 2 {
		if x, err := args[1].AsInt(); err == nil {
			sec = x
		}
	}
	return netDeadlineID(id, sec)
}

func netDeadlineID(id, sec int64) (vm.Value, error) {
	c, err := getConn(id)
	if err != nil {
		return vm.Bool(false), nil
	}
	if sec <= 0 {
		_ = c.c.SetDeadline(time.Time{})
	} else {
		_ = c.c.SetDeadline(time.Now().Add(time.Duration(sec) * time.Second))
	}
	return vm.Bool(true), nil
}

func netIDFrom(v vm.Value, key string) (int64, error) {
	if v.Typ == vm.TypeInt {
		return v.AsInt()
	}
	if v.Typ == vm.TypeMap && v.Map != nil {
		if x, ok := (*v.Map)[key]; ok {
			return x.AsInt()
		}
		if x, ok := (*v.Map)["id"]; ok {
			return x.AsInt()
		}
	}
	return 0, fmt.Errorf("net: expected conn handle or id")
}
