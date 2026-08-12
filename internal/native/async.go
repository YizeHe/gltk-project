package native

import (
	"sync"
	"sync/atomic"
	"time"

	"groklang/gltk/internal/vm"
)

func moduleAsync() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"spawn":     asyncSpawn,
		"await":     asyncAwait,
		"await_all": asyncAwaitAll,
		"ready":     asyncReady,
		"sleep":     asyncSleep,
		"channel":   asyncChannel,
		"send":      asyncSend,
		"recv":      asyncRecv,
		"close":     asyncClose,
		"mutex":     asyncMutex,
		"lock":      asyncLock,
		"unlock":    asyncUnlock,
		"waitgroup": asyncWaitGroup,
		"wg_add":    asyncWGAdd,
		"wg_done":   asyncWGDone,
		"wg_wait":   asyncWGWait,
		"parallel":  asyncParallel,
		"drop":      asyncDrop, // release handle early (future/ch/mutex/wg)
	})
}

type handleKind int

const (
	hkFuture handleKind = iota + 1
	hkChannel
	hkMutex
	hkWG
)

type handleRec struct {
	kind handleKind
	fut  *vm.Future
	ch   *vm.Channel
	mu   *vm.Mutex
	wg   *vm.WaitGroup
}

var (
	handleSeq int64
	handles   = map[int64]*handleRec{}
	handlesMu sync.Mutex
)

func putHandle(r *handleRec) int64 {
	id := atomic.AddInt64(&handleSeq, 1)
	handlesMu.Lock()
	handles[id] = r
	handlesMu.Unlock()
	return id
}

func getHandle(id int64) *handleRec {
	handlesMu.Lock()
	defer handlesMu.Unlock()
	return handles[id]
}

// delHandle removes a handle so Go GC can reclaim the underlying Future/Channel/etc.
func delHandle(id int64) {
	handlesMu.Lock()
	delete(handles, id)
	handlesMu.Unlock()
}

// LiveAsyncHandles returns the size of the global async handle table (tests/diagnostics).
func LiveAsyncHandles() int {
	handlesMu.Lock()
	defer handlesMu.Unlock()
	return len(handles)
}

func handleMap(kind string, id int64) vm.Value {
	return vm.MapVal(map[string]vm.Value{
		"type": vm.Str(kind),
		"id":   vm.Int(id),
	})
}

func handleID(v vm.Value) (int64, error) {
	if v.Typ != vm.TypeMap || v.Map == nil {
		return 0, errf("async: expected handle map")
	}
	idv, ok := (*v.Map)["id"]
	if !ok {
		return 0, errf("async: handle missing id")
	}
	return idv.AsInt()
}

// async.spawn(fn, args_array?) -> future handle
func asyncSpawn(v *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 || args[0].Typ != vm.TypeFunc || args[0].Fn == nil {
		return vm.Null(), errf("async.spawn(fn, args?)")
	}
	var callArgs []vm.Value
	if len(args) >= 2 && args[1].Typ == vm.TypeArray && args[1].Arr != nil {
		callArgs = append(callArgs, (*args[1].Arr)...)
	}
	fut := v.SpawnClosure(args[0].Fn, callArgs)
	id := putHandle(&handleRec{kind: hkFuture, fut: fut})
	return handleMap("future", id), nil
}

func asyncAwait(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("async.await(future)")
	}
	id, err := handleID(args[0])
	if err != nil {
		return vm.Null(), err
	}
	h := getHandle(id)
	if h == nil || h.kind != hkFuture || h.fut == nil {
		return vm.Null(), errf("async.await: not a future")
	}
	res, err := h.fut.Wait()
	// Future is done — drop handle so GC can reclaim (result is returned to script).
	delHandle(id)
	return res, err
}

func asyncAwaitAll(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 || args[0].Typ != vm.TypeArray || args[0].Arr == nil {
		return vm.Null(), errf("async.await_all([futures])")
	}
	var out []vm.Value
	for _, fv := range *args[0].Arr {
		id, err := handleID(fv)
		if err != nil {
			return vm.Null(), err
		}
		h := getHandle(id)
		if h == nil || h.fut == nil {
			return vm.Null(), errf("async.await_all: bad future")
		}
		res, err := h.fut.Wait()
		delHandle(id)
		if err != nil {
			return vm.Null(), err
		}
		out = append(out, res)
	}
	return vm.Array(out), nil
}

// async.drop(handle) — explicit release of future/channel/mutex/waitgroup handle.
func asyncDrop(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Bool(false), nil
	}
	id, err := handleID(args[0])
	if err != nil {
		return vm.Bool(false), nil
	}
	h := getHandle(id)
	if h == nil {
		return vm.Bool(false), nil
	}
	if h.ch != nil {
		h.ch.Close()
	}
	delHandle(id)
	return vm.Bool(true), nil
}

func asyncReady(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Bool(false), nil
	}
	id, err := handleID(args[0])
	if err != nil {
		return vm.Bool(false), nil
	}
	h := getHandle(id)
	if h == nil || h.fut == nil {
		return vm.Bool(false), nil
	}
	return vm.Bool(h.fut.Ready()), nil
}

func asyncSleep(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	ms := int64(0)
	if len(args) >= 1 {
		var err error
		ms, err = args[0].AsInt()
		if err != nil {
			return vm.Null(), err
		}
	}
	if ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
	return vm.Bool(true), nil
}

func asyncChannel(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	n := 0
	if len(args) >= 1 {
		if i, err := args[0].AsInt(); err == nil {
			n = int(i)
		}
	}
	ch := vm.NewChannel(n)
	id := putHandle(&handleRec{kind: hkChannel, ch: ch})
	return handleMap("channel", id), nil
}

func asyncSend(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("async.send(ch, value, timeout_ms?=-1)")
	}
	id, err := handleID(args[0])
	if err != nil {
		return vm.Null(), err
	}
	h := getHandle(id)
	if h == nil || h.ch == nil {
		return vm.Null(), errf("async.send: not a channel")
	}
	timeout := int64(-1)
	if len(args) >= 3 {
		timeout, _ = args[2].AsInt()
	}
	if err := h.ch.Send(args[1], timeout); err != nil {
		return vm.MapVal(map[string]vm.Value{"ok": vm.Bool(false), "error": vm.Str(err.Error())}), nil
	}
	return vm.MapVal(map[string]vm.Value{"ok": vm.Bool(true)}), nil
}

func asyncRecv(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("async.recv(ch, timeout_ms?=-1)")
	}
	id, err := handleID(args[0])
	if err != nil {
		return vm.Null(), err
	}
	h := getHandle(id)
	if h == nil || h.ch == nil {
		return vm.Null(), errf("async.recv: not a channel")
	}
	timeout := int64(-1)
	if len(args) >= 2 {
		timeout, _ = args[1].AsInt()
	}
	v, ok, err := h.ch.Recv(timeout)
	if err != nil {
		return vm.MapVal(map[string]vm.Value{"ok": vm.Bool(false), "error": vm.Str(err.Error())}), nil
	}
	return vm.MapVal(map[string]vm.Value{
		"ok":     vm.Bool(ok),
		"value":  v,
		"closed": vm.Bool(!ok),
	}), nil
}

func asyncClose(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("async.close(ch)")
	}
	id, err := handleID(args[0])
	if err != nil {
		return vm.Null(), err
	}
	h := getHandle(id)
	if h == nil || h.ch == nil {
		return vm.Null(), errf("async.close: not a channel")
	}
	h.ch.Close()
	// Keep handle until drop so late recv can still observe closed state; do not del here.
	return vm.Bool(true), nil
}

func asyncMutex(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
	id := putHandle(&handleRec{kind: hkMutex, mu: &vm.Mutex{}})
	return handleMap("mutex", id), nil
}

func asyncLock(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	id, err := handleID(args[0])
	if err != nil {
		return vm.Null(), err
	}
	h := getHandle(id)
	if h == nil || h.mu == nil {
		return vm.Null(), errf("async.lock: not a mutex")
	}
	h.mu.Lock()
	return vm.Bool(true), nil
}

func asyncUnlock(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	id, err := handleID(args[0])
	if err != nil {
		return vm.Null(), err
	}
	h := getHandle(id)
	if h == nil || h.mu == nil {
		return vm.Null(), errf("async.unlock: not a mutex")
	}
	h.mu.Unlock()
	return vm.Bool(true), nil
}

func asyncWaitGroup(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
	id := putHandle(&handleRec{kind: hkWG, wg: &vm.WaitGroup{}})
	return handleMap("waitgroup", id), nil
}

func asyncWGAdd(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	id, err := handleID(args[0])
	if err != nil {
		return vm.Null(), err
	}
	h := getHandle(id)
	if h == nil || h.wg == nil {
		return vm.Null(), errf("async.wg_add: not a waitgroup")
	}
	n := 1
	if len(args) >= 2 {
		if i, e := args[1].AsInt(); e == nil {
			n = int(i)
		}
	}
	h.wg.Add(n)
	return vm.Bool(true), nil
}

func asyncWGDone(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	id, err := handleID(args[0])
	if err != nil {
		return vm.Null(), err
	}
	h := getHandle(id)
	if h == nil || h.wg == nil {
		return vm.Null(), errf("async.wg_done: not a waitgroup")
	}
	h.wg.Done()
	return vm.Bool(true), nil
}

func asyncWGWait(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	id, err := handleID(args[0])
	if err != nil {
		return vm.Null(), err
	}
	h := getHandle(id)
	if h == nil || h.wg == nil {
		return vm.Null(), errf("async.wg_wait: not a waitgroup")
	}
	h.wg.Wait()
	return vm.Bool(true), nil
}

// async.parallel(fn, items_array) -> array of results
// Spawns one task per item: fn(item), awaits all (order preserved).
func asyncParallel(v *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 || args[0].Typ != vm.TypeFunc || args[0].Fn == nil {
		return vm.Null(), errf("async.parallel(fn, items)")
	}
	if args[1].Typ != vm.TypeArray || args[1].Arr == nil {
		return vm.Null(), errf("async.parallel: items must be array")
	}
	items := *args[1].Arr
	cl := args[0].Fn
	futs := make([]*vm.Future, len(items))
	for i, it := range items {
		// SpawnClosure already unregisters from VM map on settle; no global handle table used here.
		futs[i] = v.SpawnClosure(cl, []vm.Value{it})
	}
	out := make([]vm.Value, len(futs))
	for i, f := range futs {
		res, err := f.Wait()
		if err != nil {
			return vm.Null(), err
		}
		out[i] = res
		futs[i] = nil // allow GC of future while remaining wait
	}
	return vm.Array(out), nil
}
