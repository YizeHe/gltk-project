package vm

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Future is an async result from async.spawn (goroutine + VM clone).
type Future struct {
	id   int64
	done chan struct{}
	once sync.Once
	res  Value
	err  error
}

func (f *Future) settle(v Value, err error) {
	f.once.Do(func() {
		f.res = v
		f.err = err
		close(f.done)
	})
}

func (f *Future) Wait() (Value, error) {
	<-f.done
	return f.res, f.err
}

func (f *Future) Ready() bool {
	select {
	case <-f.done:
		return true
	default:
		return false
	}
}

func (f *Future) ID() int64 { return f.id }

// Channel is a thread-safe buffered channel of Values.
type Channel struct {
	mu     sync.Mutex
	ch     chan Value
	closed bool
	cap    int
}

func NewChannel(n int) *Channel {
	if n < 0 {
		n = 0
	}
	return &Channel{ch: make(chan Value, n), cap: n}
}

func (c *Channel) Send(v Value, timeoutMs int64) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("send on closed channel")
	}
	c.mu.Unlock()
	if timeoutMs < 0 {
		c.ch <- v
		return nil
	}
	if timeoutMs == 0 {
		select {
		case c.ch <- v:
			return nil
		default:
			return fmt.Errorf("channel send would block")
		}
	}
	select {
	case c.ch <- v:
		return nil
	case <-time.After(time.Duration(timeoutMs) * time.Millisecond):
		return fmt.Errorf("channel send timeout")
	}
}

func (c *Channel) Recv(timeoutMs int64) (Value, bool, error) {
	if timeoutMs < 0 {
		v, ok := <-c.ch
		return v, ok, nil
	}
	if timeoutMs == 0 {
		select {
		case v, ok := <-c.ch:
			return v, ok, nil
		default:
			return Null(), false, fmt.Errorf("channel recv would block")
		}
	}
	select {
	case v, ok := <-c.ch:
		return v, ok, nil
	case <-time.After(time.Duration(timeoutMs) * time.Millisecond):
		return Null(), false, fmt.Errorf("channel recv timeout")
	}
}

func (c *Channel) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.ch)
	}
}

func (c *Channel) Len() int { return len(c.ch) }
func (c *Channel) Cap() int { return c.cap }

// Mutex for script-level critical sections.
type Mutex struct{ mu sync.Mutex }

func (m *Mutex) Lock()   { m.mu.Lock() }
func (m *Mutex) Unlock() { m.mu.Unlock() }

// WaitGroup for batching tasks.
type WaitGroup struct{ wg sync.WaitGroup }

func (w *WaitGroup) Add(n int) { w.wg.Add(n) }
func (w *WaitGroup) Done()     { w.wg.Done() }
func (w *WaitGroup) Wait()     { w.wg.Wait() }

// AsyncRuntime optional bookkeeping for in-flight futures.
// Settled futures are removed from the map so the Go GC can reclaim them.
type AsyncRuntime struct {
	mu      sync.Mutex
	futures map[int64]*Future
}

func newAsyncRuntime() *AsyncRuntime {
	return &AsyncRuntime{futures: map[int64]*Future{}}
}

// LiveFutures returns the number of futures still registered (for tests / diagnostics).
func (a *AsyncRuntime) LiveFutures() int {
	if a == nil {
		return 0
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.futures)
}

func (a *AsyncRuntime) forget(id int64) {
	if a == nil {
		return
	}
	a.mu.Lock()
	delete(a.futures, id)
	a.mu.Unlock()
}

var futureSeq int64

func nextFutureID() int64 { return atomic.AddInt64(&futureSeq, 1) }

// CloneForAsync builds a VM that can run closures in parallel safely.
// Chunk and modules are shared (treated read-only); globals are shallow-copied.
// File tables are not shared (each clone has its own open-file map).
func (vm *VM) CloneForAsync() *VM {
	g := make(map[string]Value, len(vm.globals))
	for k, v := range vm.globals {
		g[k] = v
	}
	out := vm.Out
	if out == nil {
		out = func(s string) { fmt.Print(s) }
	}
	return &VM{
		chunk:      vm.chunk,
		globals:    g,
		modules:    vm.modules,
		Out:        out,
		files:      map[int64]*os.File{},
		nextFileID: 1,
		async:      newAsyncRuntime(),
	}
}

// SpawnClosure runs cl(args) on a clone in a new goroutine and returns a Future.
// After the future settles it is dropped from the parent VM's bookkeeping map so
// the result is only retained by whoever holds the *Future (native handle table).
func (vm *VM) SpawnClosure(cl *Closure, args []Value) *Future {
	if vm.async == nil {
		vm.async = newAsyncRuntime()
	}
	id := nextFutureID()
	f := &Future{id: id, done: make(chan struct{})}
	vm.async.mu.Lock()
	vm.async.futures[id] = f
	vm.async.mu.Unlock()

	parent := vm
	go func() {
		child := parent.CloneForAsync()
		res, err := child.CallClosure(cl, args)
		// Release child resources before settling so large temporary graphs can GC.
		child.Release()
		f.settle(res, err)
		// Unregister from parent map; native handles still hold *Future until drop/await.
		parent.async.forget(id)
	}()
	return f
}
