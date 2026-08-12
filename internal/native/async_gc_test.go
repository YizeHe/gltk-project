package native

import (
	"testing"
	"time"

	"groklang/gltk/internal/bytecode"
	"groklang/gltk/internal/vm"
)

// Ensures await drops global handle table entries (process-wide leak fix).
func TestAsyncAwaitDropsHandle(t *testing.T) {
	before := LiveAsyncHandles()

	c := bytecode.NewChunk("async-gc.glk")
	body := &bytecode.Proto{Name: "job", NumRegs: 4, NumParams: 0}
	body.Code = []uint32{
		bytecode.MakeAsBx(bytecode.OpLOADI, 0, 42),
		bytecode.MakeABC(bytecode.OpRET, 0, 0, 0),
	}
	ix := c.AddProto(body)
	main := &bytecode.Proto{Name: "main", NumRegs: 4, NumParams: 0}
	main.Code = []uint32{bytecode.MakeABC(bytecode.OpRET, 0, 0, 0)}
	c.MainIndex = c.AddProto(main)

	v := vm.New(c, nil)
	cl := &vm.Closure{Proto: body, ProtoIx: ix}
	fut := v.SpawnClosure(cl, nil)
	id := putHandle(&handleRec{kind: hkFuture, fut: fut})
	if LiveAsyncHandles() != before+1 {
		t.Fatalf("handle not registered: before=%d now=%d", before, LiveAsyncHandles())
	}

	h := getHandle(id)
	if h == nil || h.fut == nil {
		t.Fatal("missing handle")
	}
	res, err := h.fut.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if res.I != 42 {
		t.Fatalf("got %v", res)
	}
	delHandle(id)
	if LiveAsyncHandles() != before {
		t.Fatalf("handle not dropped: before=%d now=%d", before, LiveAsyncHandles())
	}

	// async.await path (same delHandle)
	fut2 := v.SpawnClosure(cl, nil)
	id2 := putHandle(&handleRec{kind: hkFuture, fut: fut2})
	hm := handleMap("future", id2)
	out, err := asyncAwait(v, []vm.Value{hm})
	if err != nil {
		t.Fatal(err)
	}
	if out.I != 42 {
		t.Fatalf("await got %v", out)
	}
	if getHandle(id2) != nil {
		t.Fatal("async.await should delHandle")
	}
	if LiveAsyncHandles() != before {
		t.Fatalf("leaked handles: before=%d now=%d", before, LiveAsyncHandles())
	}

	// brief settle for VM future map forget
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && v.LiveFutures() > 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if v.LiveFutures() != 0 {
		t.Fatalf("VM still tracking futures: %d", v.LiveFutures())
	}
	v.Release()
}
