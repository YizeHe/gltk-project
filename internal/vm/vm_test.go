package vm

import (
	"testing"

	"groklang/gltk/internal/bytecode"
)

func handChunkArith() *bytecode.Chunk {
	// fn main: R0 = 10; R1 = 32; R2 = R0+R1; RET R2  => 42
	c := bytecode.NewChunk("arith.glk")
	p := &bytecode.Proto{Name: "main", NumRegs: 8, NumParams: 0}
	p.Code = []uint32{
		bytecode.MakeAsBx(bytecode.OpLOADI, 0, 10),
		bytecode.MakeAsBx(bytecode.OpLOADI, 1, 32),
		bytecode.MakeABC(bytecode.OpADD, 2, 0, 1),
		bytecode.MakeABC(bytecode.OpRET, 2, 0, 0),
	}
	c.MainIndex = c.AddProto(p)
	return c
}

func TestVMArith(t *testing.T) {
	v := New(handChunkArith(), nil)
	res, err := v.Run(nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Typ != TypeInt || res.I != 42 {
		t.Fatalf("got %v want 42", res)
	}
	if v.Ops == 0 {
		t.Fatal("ops should be counted")
	}
}

func TestVMCall(t *testing.T) {
	// proto 0: add(a,b) = a+b
	// proto 1 main: call add(20,22)
	c := bytecode.NewChunk("call.glk")
	add := &bytecode.Proto{Name: "add", NumRegs: 4, NumParams: 2}
	add.Code = []uint32{
		bytecode.MakeABC(bytecode.OpADD, 2, 0, 1),
		bytecode.MakeABC(bytecode.OpRET, 2, 0, 0),
	}
	c.AddProto(add)

	main := &bytecode.Proto{Name: "main", NumRegs: 8, NumParams: 0}
	// R0 = MAKEFN proto0; R1=20; R2=22; but CALL needs fn at R0, args at R1,R2
	// Actually CALL A B C: result A, fn at B, nargs C, args at B+1..
	main.Code = []uint32{
		bytecode.MakeABx(bytecode.OpMAKEFN, 0, 0),
		bytecode.MakeAsBx(bytecode.OpLOADI, 1, 20),
		bytecode.MakeAsBx(bytecode.OpLOADI, 2, 22),
		bytecode.MakeABC(bytecode.OpCALL, 3, 0, 2),
		bytecode.MakeABC(bytecode.OpRET, 3, 0, 0),
	}
	c.MainIndex = c.AddProto(main)

	v := New(c, nil)
	res, err := v.Run(nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Typ != TypeInt || res.I != 42 {
		t.Fatalf("got %v want 42", res)
	}
}

func TestVMBytesBget(t *testing.T) {
	// const bytes {01 02 03 04} LE u32 = 0x04030201
	c := bytecode.NewChunk("bget.glk")
	ki := c.AddConst(bytecode.Constant{Kind: bytecode.ConstBytes, Bytes: []byte{1, 2, 3, 4, 5, 6, 7, 8}})
	p := &bytecode.Proto{Name: "main", NumRegs: 8, NumParams: 0}
	p.Code = []uint32{
		bytecode.MakeABx(bytecode.OpLOADK, 0, uint16(ki)),
		bytecode.MakeAsBx(bytecode.OpLOADI, 1, 0),
		bytecode.MakeABC(bytecode.OpBGET32, 2, 0, 1),
		bytecode.MakeABC(bytecode.OpRET, 2, 0, 0),
	}
	c.MainIndex = c.AddProto(p)
	v := New(c, nil)
	res, err := v.Run(nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Typ != TypeInt || res.I != 0x04030201 {
		t.Fatalf("got %#x want 0x04030201", res.I)
	}
}

func TestVMNative(t *testing.T) {
	c := bytecode.NewChunk("nat.glk")
	// R0 = LOADG "dbl"; R1=21; CALL R2,R0,1; RET R2
	k := c.AddStrConst("dbl")
	p := &bytecode.Proto{Name: "main", NumRegs: 8, NumParams: 0}
	p.Code = []uint32{
		bytecode.MakeABx(bytecode.OpLOADG, 0, uint16(k)),
		bytecode.MakeAsBx(bytecode.OpLOADI, 1, 21),
		bytecode.MakeABC(bytecode.OpCALL, 2, 0, 1),
		bytecode.MakeABC(bytecode.OpRET, 2, 0, 0),
	}
	c.MainIndex = c.AddProto(p)
	v := New(c, nil)
	v.SetGlobal("dbl", Native("dbl", func(_ *VM, args []Value) (Value, error) {
		return Int(args[0].I * 2), nil
	}))
	res, err := v.Run(nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.I != 42 {
		t.Fatalf("got %v", res)
	}
}

func TestChunkRoundTrip(t *testing.T) {
	c := handChunkArith()
	data, err := bytecode.Encode(c)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := bytecode.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	v := New(c2, nil)
	res, err := v.Run(nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.I != 42 {
		t.Fatalf("got %v", res)
	}
}

func TestArrayConcatArith(t *testing.T) {
	a := Array([]Value{Int(1), Int(2)})
	b := Array([]Value{Int(3)})
	v, err := arith(a, b, '+')
	if err != nil {
		t.Fatal(err)
	}
	if v.Typ != TypeArray || v.Arr == nil || len(*v.Arr) != 3 {
		t.Fatalf("got %v", v)
	}
	if (*v.Arr)[0].I != 1 || (*v.Arr)[2].I != 3 {
		t.Fatalf("concat content %v", v)
	}
	// originals unchanged
	if len(*a.Arr) != 2 || len(*b.Arr) != 1 {
		t.Fatal("concat should not mutate inputs")
	}
}

func TestHandleTryGETI(t *testing.T) {
	// main: try { GETI empty arr[0] } catch → RET 1
	// Simpler: hand-rolled TRY + GETI OOB + catch loads 1
	c := bytecode.NewChunk("try.glk")
	p := &bytecode.Proto{Name: "main", NumRegs: 8, NumParams: 0}
	// R0 = NEWARR empty; R1 = 0; TRY catch; GETI R2=R0[R1]; LOADI R3=0; JMP end; catch: LOADI R3=1; end: RET R3
	// Use OpTRY with sBx relative to next instr
	p.Code = []uint32{
		bytecode.MakeABC(bytecode.OpNEWARR, 0, 0, 0), // empty arr at R0
		bytecode.MakeAsBx(bytecode.OpLOADI, 1, 0),     // idx 0
		bytecode.MakeAsBx(bytecode.OpTRY, 4, 3),       // catch at ip+3, err in R4; after TRY ip points to GETI
		bytecode.MakeABC(bytecode.OpGETI, 2, 0, 1),    // OOB
		bytecode.MakeAsBx(bytecode.OpLOADI, 3, 0),     // success path (shouldn't run)
		bytecode.MakeAsBx(bytecode.OpJMP, 0, 1),       // skip catch
		bytecode.MakeAsBx(bytecode.OpLOADI, 3, 1),     // catch: R3=1
		bytecode.MakeABC(bytecode.OpENDTRY, 0, 0, 0),
		bytecode.MakeABC(bytecode.OpRET, 3, 0, 0),
	}
	// Fix TRY sBx: catch should land on LOADI R3=1 which is at index 6.
	// After OpTRY at index 2, ip becomes 3. catchIP = 3 + sbx => need 6, so sbx=3. OK.
	c.MainIndex = c.AddProto(p)
	v := New(c, nil)
	res, err := v.Run(nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Typ != TypeInt || res.I != 1 {
		t.Fatalf("want 1 (caught) got %v", res)
	}
}

// --- Memory / GC hygiene (Go GC + frame/resource cleanup) ---

func TestReleaseFrameOnReturn(t *testing.T) {
	// nested call: after return, frames slice empty and no dangling frame regs
	c := bytecode.NewChunk("ret-clean.glk")
	inner := &bytecode.Proto{Name: "inner", NumRegs: 4, NumParams: 0}
	inner.Code = []uint32{
		bytecode.MakeAsBx(bytecode.OpLOADI, 0, 7),
		bytecode.MakeABC(bytecode.OpRET, 0, 0, 0),
	}
	c.AddProto(inner)
	main := &bytecode.Proto{Name: "main", NumRegs: 8, NumParams: 0}
	main.Code = []uint32{
		bytecode.MakeABx(bytecode.OpMAKEFN, 0, 0),
		bytecode.MakeABC(bytecode.OpCALL, 1, 0, 0),
		bytecode.MakeABC(bytecode.OpRET, 1, 0, 0),
	}
	c.MainIndex = c.AddProto(main)
	v := New(c, nil)
	res, err := v.Run(nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.I != 7 {
		t.Fatalf("got %v", res)
	}
	if len(v.frames) != 0 {
		t.Fatalf("frames should be empty after Run, got %d", len(v.frames))
	}
	v.Release()
	if len(v.files) != 0 {
		t.Fatalf("files map should be empty after Release")
	}
}

func TestMaxRegistersGuard(t *testing.T) {
	fr := &Frame{regs: make([]Value, 4)}
	v := New(bytecode.NewChunk("x"), nil)
	if v.ensureReg(fr, MaxRegisters) {
		t.Fatal("ensureReg(MaxRegisters) should fail")
	}
	if !v.ensureReg(fr, 10) {
		t.Fatal("ensureReg(10) should succeed")
	}
	if len(fr.regs) <= 10 {
		t.Fatalf("regs len %d", len(fr.regs))
	}
}

func TestSpawnClosureForgetsFuture(t *testing.T) {
	c := bytecode.NewChunk("spawn.glk")
	// trivial proto for closure body
	body := &bytecode.Proto{Name: "job", NumRegs: 4, NumParams: 0}
	body.Code = []uint32{
		bytecode.MakeAsBx(bytecode.OpLOADI, 0, 1),
		bytecode.MakeABC(bytecode.OpRET, 0, 0, 0),
	}
	ix := c.AddProto(body)
	main := &bytecode.Proto{Name: "main", NumRegs: 4, NumParams: 0}
	main.Code = []uint32{bytecode.MakeABC(bytecode.OpRET, 0, 0, 0)}
	c.MainIndex = c.AddProto(main)

	v := New(c, nil)
	cl := &Closure{Proto: body, ProtoIx: ix}
	f := v.SpawnClosure(cl, nil)
	res, err := f.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if res.I != 1 {
		t.Fatalf("got %v", res)
	}
	// allow forget() after settle
	for i := 0; i < 50 && v.LiveFutures() > 0; i++ {
		// tiny spin: forget runs in goroutine after settle
	}
	// After wait, forget should have run (same goroutine path settles then forgets)
	// Give scheduler a moment if needed
	if v.LiveFutures() > 0 {
		// one more wait for the go func to finish forget after settle
		for i := 0; i < 100 && v.LiveFutures() > 0; i++ {
			_ = i
		}
	}
	if v.LiveFutures() != 0 {
		t.Fatalf("expected 0 live futures after settle+forget, got %d", v.LiveFutures())
	}
}
