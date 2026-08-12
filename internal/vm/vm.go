package vm

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"sync"

	"groklang/gltk/internal/bytecode"
)

// tryFrame is one active try/catch handler on a call frame.
type tryFrame struct {
	catchIP int
	errReg  int
}

// Frame is a call frame with registers.
type Frame struct {
	proto    *bytecode.Proto
	protoIx  int
	regs     []Value
	ip       int
	upvals   []Value
	retReg   int // destination in caller for return value
	base     int // unused reserved
	tryStack []tryFrame
}

// VM is the GrokLang register-based virtual machine.
type VM struct {
	chunk   *bytecode.Chunk
	frames  []*Frame
	globals map[string]Value
	modules map[string]Value // native modules
	Ops     uint64           // executed instruction counter
	MaxOps  uint64           // 0 = unlimited
	Out     func(string)     // print sink (default stdout via Set)

	// Open file handles for I/O builtins (open/read/write/close).
	filesMu    sync.Mutex
	files      map[int64]*os.File
	nextFileID int64

	// Async runtime (futures bookkeeping); created lazily.
	async *AsyncRuntime
}

// New creates a VM for a chunk with optional native modules.
func New(chunk *bytecode.Chunk, modules map[string]Value) *VM {
	if modules == nil {
		modules = map[string]Value{}
	}
	return &VM{
		chunk:      chunk,
		globals:    map[string]Value{},
		modules:    modules,
		Out:        func(s string) { fmt.Print(s) },
		files:      map[int64]*os.File{},
		nextFileID: 1,
	}
}

// AllocFile stores f and returns a unique file id.
func (vm *VM) AllocFile(f *os.File) int64 {
	vm.filesMu.Lock()
	defer vm.filesMu.Unlock()
	id := vm.nextFileID
	vm.nextFileID++
	vm.files[id] = f
	return id
}

// GetFile returns the open file for id, or nil.
func (vm *VM) GetFile(id int64) *os.File {
	vm.filesMu.Lock()
	defer vm.filesMu.Unlock()
	return vm.files[id]
}

// CloseFile closes and removes the file for id. Returns false if unknown.
func (vm *VM) CloseFile(id int64) error {
	vm.filesMu.Lock()
	f, ok := vm.files[id]
	if ok {
		delete(vm.files, id)
	}
	vm.filesMu.Unlock()
	if !ok || f == nil {
		return nil
	}
	return f.Close()
}

// SetGlobal registers a global value.
func (vm *VM) SetGlobal(name string, v Value) {
	vm.globals[name] = v
}

// RegisterModule adds a native module map.
func (vm *VM) RegisterModule(name string, m Value) {
	vm.modules[name] = m
	vm.globals[name] = m
}

// Run executes the main proto with args (array placed in R0 if params>=1).
func (vm *VM) Run(args []Value) (Value, error) {
	if vm.chunk == nil || len(vm.chunk.Protos) == 0 {
		return Null(), fmt.Errorf("vm: empty chunk")
	}
	mi := vm.chunk.MainIndex
	if mi < 0 || mi >= len(vm.chunk.Protos) {
		return Null(), fmt.Errorf("vm: bad main index %d", mi)
	}
	// seed module globals
	for k, v := range vm.modules {
		vm.globals[k] = v
	}
	// bind all protos as global functions by name
	for i, p := range vm.chunk.Protos {
		if p.Name == "" || p.Name == "<main>" {
			continue
		}
		cl := &Closure{Proto: p, ProtoIx: i}
		vm.globals[p.Name] = Func(cl)
	}
	proto := vm.chunk.Protos[mi]
	fr := vm.newFrame(proto, mi, nil, 0)
	// args: if main has params, R0 = args array
	if proto.NumParams >= 1 {
		fr.regs[0] = Array(args)
	}
	vm.frames = []*Frame{fr}
	return vm.execute()
}

// CallClosure invokes a closure (for tests / natives / GUI callbacks).
//
// The caller's frame stack is saved and restored so nested invocation from a
// blocking native (e.g. gui.run message loop) cannot resume the outer function
// mid-execute. Without this, OpRET after a GUI click would continue main past
// gui.run() while the message loop is still active.
func (vm *VM) CallClosure(cl *Closure, args []Value) (Value, error) {
	if cl == nil || cl.Proto == nil {
		return Null(), fmt.Errorf("CallClosure: nil closure")
	}
	saved := vm.frames
	vm.frames = nil
	defer func() { vm.frames = saved }()

	fr := vm.newFrame(cl.Proto, cl.ProtoIx, cl.Upvals, 0)
	for i := 0; i < int(cl.Proto.NumParams) && i < len(args); i++ {
		fr.regs[i] = args[i]
	}
	vm.frames = []*Frame{fr}
	return vm.execute()
}

func (vm *VM) newFrame(p *bytecode.Proto, ix int, up []Value, retReg int) *Frame {
	n := int(p.NumRegs)
	if n < 16 {
		n = 16
	}
	if n < int(p.NumParams)+16 {
		n = int(p.NumParams) + 32
	}
	regs := make([]Value, n)
	for i := range regs {
		regs[i] = Null()
	}
	return &Frame{
		proto: p, protoIx: ix, regs: regs, ip: 0, upvals: up, retReg: retReg,
	}
}

// MaxRegisters caps per-frame register growth (runaway bytecode / bad codegen).
// Keeps a single call frame from pinning unbounded memory before Go GC can help.
const MaxRegisters = 65536

// ensureReg grows the current frame register file to include index r.
// Returns false if r is out of the allowed range (caller should abort the op).
func (vm *VM) ensureReg(fr *Frame, r int) bool {
	if r < 0 || r >= MaxRegisters {
		return false
	}
	for len(fr.regs) <= r {
		fr.regs = append(fr.regs, Null())
	}
	return true
}

// releaseFrame clears references held by a finished call frame so the Go GC can
// reclaim arrays/maps/strings that were only reachable from that frame's regs.
func releaseFrame(fr *Frame) {
	if fr == nil {
		return
	}
	for i := range fr.regs {
		fr.regs[i] = Null()
	}
	fr.regs = nil
	for i := range fr.upvals {
		fr.upvals[i] = Null()
	}
	fr.upvals = nil
	fr.tryStack = nil
	fr.proto = nil
}

// Release drops VM-owned resources that Go GC cannot free (open files) and
// clears stacks/globals bookkeeping that may pin large graphs. Safe to call
// after Run/CallClosure completes. Does not free the *bytecode.Chunk (shared).
func (vm *VM) Release() {
	if vm == nil {
		return
	}
	// Close any leaked open files.
	vm.filesMu.Lock()
	for id, f := range vm.files {
		if f != nil {
			_ = f.Close()
		}
		delete(vm.files, id)
	}
	vm.filesMu.Unlock()

	for i := range vm.frames {
		releaseFrame(vm.frames[i])
	}
	vm.frames = nil

	if vm.async != nil {
		vm.async.mu.Lock()
		vm.async.futures = map[int64]*Future{}
		vm.async.mu.Unlock()
	}
}

// LiveFutures returns in-flight async futures registered on this VM (diagnostics).
func (vm *VM) LiveFutures() int {
	if vm == nil || vm.async == nil {
		return 0
	}
	return vm.async.LiveFutures()
}

func (vm *VM) cur() *Frame {
	return vm.frames[len(vm.frames)-1]
}

func (vm *VM) execute() (Value, error) {
	for {
		if len(vm.frames) == 0 {
			return Null(), nil
		}
		fr := vm.cur()
		if fr.ip < 0 || fr.ip >= len(fr.proto.Code) {
			// implicit return null
			return vm.doReturn(Null())
		}
		ins := fr.proto.Code[fr.ip]
		fr.ip++
		vm.Ops++
		if vm.MaxOps > 0 && vm.Ops > vm.MaxOps {
			return Null(), fmt.Errorf("vm: max ops exceeded (%d)", vm.MaxOps)
		}
		op := bytecode.GetOp(ins)
		a := int(bytecode.GetA(ins))
		b := int(bytecode.GetB(ins))
		c := int(bytecode.GetC(ins))
		bx := bytecode.GetBx(ins)
		sbx := bytecode.GetsBx(ins)
		// ensure common register indices exist (register machine may need growth)
		if !vm.ensureReg(fr, a) {
			return Null(), vm.errf("register index out of range: A=%d (max %d)", a, MaxRegisters-1)
		}
		if b < 256 && !vm.ensureReg(fr, b) {
			return Null(), vm.errf("register index out of range: B=%d (max %d)", b, MaxRegisters-1)
		}
		if c < 256 && !vm.ensureReg(fr, c) {
			return Null(), vm.errf("register index out of range: C=%d (max %d)", c, MaxRegisters-1)
		}
		// CALL / NEWARR / BSLICE may touch base+n
		if op == bytecode.OpCALL {
			if !vm.ensureReg(fr, b+c+1) {
				return Null(), vm.errf("register index out of range on CALL (base+n)")
			}
		}
		if op == bytecode.OpNEWARR || op == bytecode.OpLIST {
			if c > 0 && !vm.ensureReg(fr, b+c-1) {
				return Null(), vm.errf("register index out of range on NEWARR/LIST")
			}
		}
		if op == bytecode.OpBSLICE {
			if !vm.ensureReg(fr, c+1) {
				return Null(), vm.errf("register index out of range on BSLICE")
			}
		}

		switch op {
		case bytecode.OpHALT:
			return Null(), nil
		case bytecode.OpNOP:
			// nothing
		case bytecode.OpMOVE, bytecode.OpDUP:
			fr.regs[a] = fr.regs[b]
		case bytecode.OpLOADK, bytecode.OpLOADF:
			if int(bx) >= len(vm.chunk.Consts) {
				return Null(), vm.errf("LOADK bad const %d", bx)
			}
			fr.regs[a] = ConstToValue(vm.chunk.Consts[bx])
		case bytecode.OpLOADI:
			fr.regs[a] = Int(int64(sbx))
		case bytecode.OpLOADN:
			fr.regs[a] = Null()
		case bytecode.OpLOADB:
			fr.regs[a] = Bool(b != 0)
		case bytecode.OpADD:
			v, err := arith(fr.regs[b], fr.regs[c], '+')
			if err != nil {
				if vm.handleTry(vm.wrap(err)) {
					continue
				}
				return Null(), vm.wrap(err)
			}
			fr.regs[a] = v
		case bytecode.OpSUB:
			v, err := arith(fr.regs[b], fr.regs[c], '-')
			if err != nil {
				if vm.handleTry(vm.wrap(err)) {
					continue
				}
				return Null(), vm.wrap(err)
			}
			fr.regs[a] = v
		case bytecode.OpMUL:
			v, err := arith(fr.regs[b], fr.regs[c], '*')
			if err != nil {
				if vm.handleTry(vm.wrap(err)) {
					continue
				}
				return Null(), vm.wrap(err)
			}
			fr.regs[a] = v
		case bytecode.OpDIV:
			v, err := arith(fr.regs[b], fr.regs[c], '/')
			if err != nil {
				if vm.handleTry(vm.wrap(err)) {
					continue
				}
				return Null(), vm.wrap(err)
			}
			fr.regs[a] = v
		case bytecode.OpMOD:
			v, err := arith(fr.regs[b], fr.regs[c], '%')
			if err != nil {
				if vm.handleTry(vm.wrap(err)) {
					continue
				}
				return Null(), vm.wrap(err)
			}
			fr.regs[a] = v
		case bytecode.OpAND:
			ib, err1 := fr.regs[b].AsInt()
			ic, err2 := fr.regs[c].AsInt()
			if err1 != nil || err2 != nil {
				err := vm.errf("AND type error")
				if vm.handleTry(err) {
					continue
				}
				return Null(), err
			}
			fr.regs[a] = Int(ib & ic)
		case bytecode.OpOR:
			ib, err1 := fr.regs[b].AsInt()
			ic, err2 := fr.regs[c].AsInt()
			if err1 != nil || err2 != nil {
				err := vm.errf("OR type error")
				if vm.handleTry(err) {
					continue
				}
				return Null(), err
			}
			fr.regs[a] = Int(ib | ic)
		case bytecode.OpXOR:
			ib, err1 := fr.regs[b].AsInt()
			ic, err2 := fr.regs[c].AsInt()
			if err1 != nil || err2 != nil {
				err := vm.errf("XOR type error")
				if vm.handleTry(err) {
					continue
				}
				return Null(), err
			}
			fr.regs[a] = Int(ib ^ ic)
		case bytecode.OpSHL:
			ib, err1 := fr.regs[b].AsInt()
			ic, err2 := fr.regs[c].AsInt()
			if err1 != nil || err2 != nil {
				err := vm.errf("SHL type error")
				if vm.handleTry(err) {
					continue
				}
				return Null(), err
			}
			fr.regs[a] = Int(ib << uint(ic&63))
		case bytecode.OpSHR:
			ib, err1 := fr.regs[b].AsInt()
			ic, err2 := fr.regs[c].AsInt()
			if err1 != nil || err2 != nil {
				err := vm.errf("SHR type error")
				if vm.handleTry(err) {
					continue
				}
				return Null(), err
			}
			fr.regs[a] = Int(int64(uint64(ib) >> uint(ic&63)))
		case bytecode.OpROL:
			ib, err1 := fr.regs[b].AsInt()
			ic, err2 := fr.regs[c].AsInt()
			if err1 != nil || err2 != nil {
				err := vm.errf("ROL type error")
				if vm.handleTry(err) {
					continue
				}
				return Null(), err
			}
			fr.regs[a] = Int(int64(rol32(uint32(ib), uint(ic))))
		case bytecode.OpROR:
			ib, err1 := fr.regs[b].AsInt()
			ic, err2 := fr.regs[c].AsInt()
			if err1 != nil || err2 != nil {
				err := vm.errf("ROR type error")
				if vm.handleTry(err) {
					continue
				}
				return Null(), err
			}
			fr.regs[a] = Int(int64(ror32(uint32(ib), uint(ic))))
		case bytecode.OpNOT:
			ib, err := fr.regs[b].AsInt()
			if err != nil {
				if vm.handleTry(vm.wrap(err)) {
					continue
				}
				return Null(), vm.wrap(err)
			}
			fr.regs[a] = Int(^ib)
		case bytecode.OpNEG:
			if fr.regs[b].Typ == TypeFloat {
				fr.regs[a] = Float(-fr.regs[b].F)
			} else {
				ib, err := fr.regs[b].AsInt()
				if err != nil {
					if vm.handleTry(vm.wrap(err)) {
						continue
					}
					return Null(), vm.wrap(err)
				}
				fr.regs[a] = Int(-ib)
			}
		case bytecode.OpLNOT:
			fr.regs[a] = Bool(!fr.regs[b].Truthy())
		case bytecode.OpEQ:
			fr.regs[a] = Bool(fr.regs[b].Equal(fr.regs[c]))
		case bytecode.OpNE:
			fr.regs[a] = Bool(!fr.regs[b].Equal(fr.regs[c]))
		case bytecode.OpLT, bytecode.OpLE, bytecode.OpGT, bytecode.OpGE:
			cmp, err := fr.regs[b].Compare(fr.regs[c])
			if err != nil {
				if vm.handleTry(vm.wrap(err)) {
					continue
				}
				return Null(), vm.wrap(err)
			}
			var ok bool
			switch op {
			case bytecode.OpLT:
				ok = cmp < 0
			case bytecode.OpLE:
				ok = cmp <= 0
			case bytecode.OpGT:
				ok = cmp > 0
			case bytecode.OpGE:
				ok = cmp >= 0
			}
			fr.regs[a] = Bool(ok)
		case bytecode.OpJMP:
			fr.ip += int(sbx)
		case bytecode.OpJT:
			if fr.regs[a].Truthy() {
				fr.ip += int(sbx)
			}
		case bytecode.OpJF:
			if !fr.regs[a].Truthy() {
				fr.ip += int(sbx)
			}
		case bytecode.OpNEWARR, bytecode.OpLIST:
			n := int(c)
			base := int(b)
			arr := make([]Value, n)
			for i := 0; i < n; i++ {
				arr[i] = fr.regs[base+i]
			}
			fr.regs[a] = Array(arr)
		case bytecode.OpNEWMAP:
			m := map[string]Value{}
			fr.regs[a] = MapVal(m)
		case bytecode.OpGETI:
			v, err := fr.regs[b].GetIndex(fr.regs[c])
			if err != nil {
				if vm.handleTry(vm.wrap(err)) {
					continue
				}
				return Null(), vm.wrap(err)
			}
			fr.regs[a] = v
		case bytecode.OpSETI:
			if err := fr.regs[a].SetIndex(fr.regs[b], fr.regs[c]); err != nil {
				if vm.handleTry(vm.wrap(err)) {
					continue
				}
				return Null(), vm.wrap(err)
			}
		case bytecode.OpGETK:
			if int(c) >= len(vm.chunk.Consts) {
				return Null(), vm.errf("GETK bad const")
			}
			key := ConstToValue(vm.chunk.Consts[c]).AsStr()
			obj := fr.regs[b]
			if obj.Typ != TypeMap || obj.Map == nil {
				// also allow native module maps
				fr.regs[a] = Null()
			} else {
				fr.regs[a] = (*obj.Map)[key]
			}
		case bytecode.OpSETK:
			if int(b) >= len(vm.chunk.Consts) {
				return Null(), vm.errf("SETK bad const")
			}
			key := ConstToValue(vm.chunk.Consts[b]).AsStr()
			obj := fr.regs[a]
			if obj.Typ != TypeMap {
				err := vm.errf("SETK on non-map")
				if vm.handleTry(err) {
					continue
				}
				return Null(), err
			}
			if obj.Map == nil {
				m := map[string]Value{}
				obj.Map = &m
				fr.regs[a] = obj
			}
			(*obj.Map)[key] = fr.regs[c]
		case bytecode.OpLEN:
			n, err := fr.regs[b].Len()
			if err != nil {
				if vm.handleTry(vm.wrap(err)) {
					continue
				}
				return Null(), vm.wrap(err)
			}
			fr.regs[a] = Int(n)
		case bytecode.OpCONCAT:
			v, err := concat(fr.regs[b], fr.regs[c])
			if err != nil {
				if vm.handleTry(vm.wrap(err)) {
					continue
				}
				return Null(), vm.wrap(err)
			}
			fr.regs[a] = v
		case bytecode.OpBGET8:
			bs, off, err := bytesOff(fr.regs[b], fr.regs[c])
			if err != nil {
				if vm.handleTry(vm.wrap(err)) {
					continue
				}
				return Null(), vm.wrap(err)
			}
			if off < 0 || off >= len(bs) {
				err := vm.errf("BGET8 OOB")
				if vm.handleTry(err) {
					continue
				}
				return Null(), err
			}
			fr.regs[a] = Int(int64(bs[off]))
		case bytecode.OpBGET16:
			bs, off, err := bytesOff(fr.regs[b], fr.regs[c])
			if err != nil {
				if vm.handleTry(vm.wrap(err)) {
					continue
				}
				return Null(), vm.wrap(err)
			}
			if off < 0 || off+2 > len(bs) {
				err := vm.errf("BGET16 OOB")
				if vm.handleTry(err) {
					continue
				}
				return Null(), err
			}
			fr.regs[a] = Int(int64(binary.LittleEndian.Uint16(bs[off:])))
		case bytecode.OpBGET32:
			bs, off, err := bytesOff(fr.regs[b], fr.regs[c])
			if err != nil {
				if vm.handleTry(vm.wrap(err)) {
					continue
				}
				return Null(), vm.wrap(err)
			}
			if off < 0 || off+4 > len(bs) {
				err := vm.errf("BGET32 OOB")
				if vm.handleTry(err) {
					continue
				}
				return Null(), err
			}
			fr.regs[a] = Int(int64(binary.LittleEndian.Uint32(bs[off:])))
		case bytecode.OpBGET64:
			bs, off, err := bytesOff(fr.regs[b], fr.regs[c])
			if err != nil {
				if vm.handleTry(vm.wrap(err)) {
					continue
				}
				return Null(), vm.wrap(err)
			}
			if off < 0 || off+8 > len(bs) {
				err := vm.errf("BGET64 OOB")
				if vm.handleTry(err) {
					continue
				}
				return Null(), err
			}
			fr.regs[a] = Int(int64(binary.LittleEndian.Uint64(bs[off:])))
		case bytecode.OpBSLICE:
			// R[A] = slice(R[B], R[C], R[C+1])
			src := fr.regs[b]
			start, err1 := fr.regs[c].AsInt()
			end, err2 := fr.regs[c+1].AsInt()
			if err1 != nil || err2 != nil {
				err := vm.errf("BSLICE bounds type")
				if vm.handleTry(err) {
					continue
				}
				return Null(), err
			}
			switch src.Typ {
			case TypeBytes:
				if start < 0 {
					start = 0
				}
				if end > int64(len(src.Bytes)) {
					end = int64(len(src.Bytes))
				}
				if start > end {
					start = end
				}
				fr.regs[a] = Bytes(src.Bytes[start:end]) // zero-copy subslice
			case TypeStr:
				if start < 0 {
					start = 0
				}
				if end > int64(len(src.S)) {
					end = int64(len(src.S))
				}
				if start > end {
					start = end
				}
				fr.regs[a] = Str(src.S[start:end])
			case TypeArray:
				if src.Arr == nil {
					fr.regs[a] = Array(nil)
					break
				}
				arr := *src.Arr
				if start < 0 {
					start = 0
				}
				if end > int64(len(arr)) {
					end = int64(len(arr))
				}
				if start > end {
					start = end
				}
				sub := make([]Value, end-start)
				copy(sub, arr[start:end])
				fr.regs[a] = Array(sub)
			default:
				err := vm.errf("BSLICE on %s", src.TypeName())
				if vm.handleTry(err) {
					continue
				}
				return Null(), err
			}
		case bytecode.OpBSET8:
			if fr.regs[a].Typ != TypeBytes {
				err := vm.errf("BSET8 non-bytes")
				if vm.handleTry(err) {
					continue
				}
				return Null(), err
			}
			off, err := fr.regs[b].AsInt()
			if err != nil {
				if vm.handleTry(vm.wrap(err)) {
					continue
				}
				return Null(), vm.wrap(err)
			}
			val, err := fr.regs[c].AsInt()
			if err != nil {
				if vm.handleTry(vm.wrap(err)) {
					continue
				}
				return Null(), vm.wrap(err)
			}
			if off < 0 || int(off) >= len(fr.regs[a].Bytes) {
				err := vm.errf("BSET8 OOB")
				if vm.handleTry(err) {
					continue
				}
				return Null(), err
			}
			fr.regs[a].Bytes[off] = byte(val)
		case bytecode.OpCALL:
			if err := vm.doCall(int(a), int(b), int(c)); err != nil {
				if vm.handleTry(err) {
					continue
				}
				return Null(), err
			}
		case bytecode.OpRET:
			ret, err := vm.doReturn(fr.regs[a])
			if err != nil {
				return Null(), err
			}
			if len(vm.frames) == 0 {
				return ret, nil
			}
		case bytecode.OpRETN:
			ret, err := vm.doReturn(Null())
			if err != nil {
				return Null(), err
			}
			if len(vm.frames) == 0 {
				return ret, nil
			}
		case bytecode.OpMAKEFN:
			if int(bx) >= len(vm.chunk.Protos) {
				return Null(), vm.errf("MAKEFN bad proto %d", bx)
			}
			p := vm.chunk.Protos[bx]
			fr.regs[a] = Func(&Closure{Proto: p, ProtoIx: int(bx)})
		case bytecode.OpCLOSURE:
			// R[A] = closure(proto B) with C upvalues; next C instructions ignored —
			// we use simpler form: upvalues already in regs[A+1..]
			if int(b) >= len(vm.chunk.Protos) {
				return Null(), vm.errf("CLOSURE bad proto")
			}
			p := vm.chunk.Protos[b]
			nup := int(c)
			ups := make([]Value, nup)
			for i := 0; i < nup; i++ {
				ups[i] = fr.regs[int(a)+1+i]
			}
			fr.regs[a] = Func(&Closure{Proto: p, ProtoIx: int(b), Upvals: ups})
		case bytecode.OpGETUPV:
			if int(b) >= len(fr.upvals) {
				return Null(), vm.errf("GETUPV OOB")
			}
			fr.regs[a] = fr.upvals[b]
		case bytecode.OpSETUPV:
			if int(a) >= len(fr.upvals) {
				return Null(), vm.errf("SETUPV OOB")
			}
			fr.upvals[a] = fr.regs[b]
		case bytecode.OpTOSTR:
			fr.regs[a] = Str(fr.regs[b].AsStr())
		case bytecode.OpTOINT:
			i, err := fr.regs[b].AsInt()
			if err != nil {
				if vm.handleTry(vm.wrap(err)) {
					continue
				}
				return Null(), vm.wrap(err)
			}
			fr.regs[a] = Int(i)
		case bytecode.OpTYPEOF:
			fr.regs[a] = Str(fr.regs[b].TypeName())
		case bytecode.OpISNULL:
			fr.regs[a] = Bool(fr.regs[b].Typ == TypeNull)
		case bytecode.OpIN:
			// R[A] = key R[B] in container R[C]
			cont := fr.regs[c]
			key := fr.regs[b]
			found := false
			switch cont.Typ {
			case TypeMap:
				if cont.Map != nil {
					_, found = (*cont.Map)[key.AsStr()]
				}
			case TypeArray:
				if cont.Arr != nil {
					for _, e := range *cont.Arr {
						if e.Equal(key) {
							found = true
							break
						}
					}
				}
			case TypeStr:
				found = strings.Contains(cont.S, key.AsStr())
			}
			fr.regs[a] = Bool(found)
		case bytecode.OpARRPUSH:
			if fr.regs[a].Typ != TypeArray || fr.regs[a].Arr == nil {
				err := vm.errf("ARRPUSH non-array")
				if vm.handleTry(err) {
					continue
				}
				return Null(), err
			}
			*fr.regs[a].Arr = append(*fr.regs[a].Arr, fr.regs[c])
		case bytecode.OpKEYS:
			if fr.regs[b].Typ != TypeMap || fr.regs[b].Map == nil {
				fr.regs[a] = Array(nil)
			} else {
				keys := make([]Value, 0, len(*fr.regs[b].Map))
				for k := range *fr.regs[b].Map {
					keys = append(keys, Str(k))
				}
				fr.regs[a] = Array(keys)
			}
		case bytecode.OpASSERT:
			if !fr.regs[a].Truthy() {
				msg := "assertion failed"
				if int(b) < len(vm.chunk.Consts) {
					msg = ConstToValue(vm.chunk.Consts[b]).AsStr()
				}
				return Null(), vm.errf("%s", msg)
			}
		case bytecode.OpLOADG:
			if int(bx) >= len(vm.chunk.Consts) {
				return Null(), vm.errf("LOADG bad const")
			}
			name := ConstToValue(vm.chunk.Consts[bx]).AsStr()
			if v, ok := vm.globals[name]; ok {
				fr.regs[a] = v
			} else {
				fr.regs[a] = Null()
			}
		case bytecode.OpSTOREG:
			if int(bx) >= len(vm.chunk.Consts) {
				return Null(), vm.errf("STOREG bad const")
			}
			name := ConstToValue(vm.chunk.Consts[bx]).AsStr()
			vm.globals[name] = fr.regs[a]
		case bytecode.OpIMPORT:
			if int(bx) >= len(vm.chunk.Consts) {
				return Null(), vm.errf("IMPORT bad const")
			}
			name := ConstToValue(vm.chunk.Consts[bx]).AsStr()
			if m, ok := vm.modules[name]; ok {
				fr.regs[a] = m
				vm.globals[name] = m
			} else {
				return Null(), vm.errf("unknown module %q", name)
			}
		case bytecode.OpSWAP:
			fr.regs[a], fr.regs[b] = fr.regs[b], fr.regs[a]
		case bytecode.OpTRY:
			// push try: catch at ip+sBx (ip already advanced past this instr)
			fr.tryStack = append(fr.tryStack, tryFrame{
				catchIP: fr.ip + int(sbx),
				errReg:  a,
			})
		case bytecode.OpENDTRY:
			if len(fr.tryStack) > 0 {
				fr.tryStack = fr.tryStack[:len(fr.tryStack)-1]
			}
		case bytecode.OpTHROW:
			msg := fr.regs[a].AsStr()
			err := fmt.Errorf("%s", msg)
			if vm.handleTry(err) {
				continue
			}
			return Null(), vm.wrap(err)
		default:
			return Null(), vm.errf("unknown opcode %d (%s)", op, op.Name())
		}
	}
}

// handleTry routes an error to the nearest try handler (unwinding frames if needed).
// Returns true if the error was caught.
func (vm *VM) handleTry(err error) bool {
	// strip multi-line stack traces for catch binding: first line only
	msg := err.Error()
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		msg = msg[:i]
	}
	for fi := len(vm.frames) - 1; fi >= 0; fi-- {
		fr := vm.frames[fi]
		if len(fr.tryStack) == 0 {
			continue
		}
		// drop frames above the handler frame
		vm.frames = vm.frames[:fi+1]
		t := fr.tryStack[len(fr.tryStack)-1]
		fr.tryStack = fr.tryStack[:len(fr.tryStack)-1]
		if !vm.ensureReg(fr, t.errReg) {
			// cannot deliver error into out-of-range register — leave message for outer handlers
			continue
		}
		fr.regs[t.errReg] = Str(msg)
		fr.ip = t.catchIP
		return true
	}
	return false
}

func (vm *VM) doCall(retReg, fnReg, nargs int) error {
	fr := vm.cur()
	// grow regs if needed (capped)
	need := fnReg + 1 + nargs
	if need > MaxRegisters || retReg >= MaxRegisters {
		return fmt.Errorf("register index out of range on call (need=%d ret=%d max=%d)", need, retReg, MaxRegisters)
	}
	for len(fr.regs) < need {
		fr.regs = append(fr.regs, Null())
	}
	// ensure dest reg
	for len(fr.regs) <= retReg {
		fr.regs = append(fr.regs, Null())
	}
	fn := fr.regs[fnReg]
	args := make([]Value, nargs)
	for i := 0; i < nargs; i++ {
		args[i] = fr.regs[fnReg+1+i]
	}
	switch fn.Typ {
	case TypeNative:
		if fn.Nat == nil {
			return vm.errf("nil native")
		}
		res, err := fn.Nat(vm, args)
		if err != nil {
			return vm.wrap(err)
		}
		fr.regs[retReg] = res
		return nil
	case TypeFunc:
		if fn.Fn == nil || fn.Fn.Proto == nil {
			return vm.errf("nil func")
		}
		child := vm.newFrame(fn.Fn.Proto, fn.Fn.ProtoIx, fn.Fn.Upvals, retReg)
		for i := 0; i < int(fn.Fn.Proto.NumParams) && i < len(args); i++ {
			child.regs[i] = args[i]
		}
		vm.frames = append(vm.frames, child)
		return nil
	case TypeMap:
		// calling a map is error — unless it's a mistake
		return vm.errf("cannot call map")
	default:
		return vm.errf("cannot call %s", fn.TypeName())
	}
}

func (vm *VM) doReturn(v Value) (Value, error) {
	if len(vm.frames) == 0 {
		return v, nil
	}
	fr := vm.frames[len(vm.frames)-1]
	retReg := fr.retReg
	vm.frames = vm.frames[:len(vm.frames)-1]
	// Drop refs from the finished frame promptly (Go GC will reclaim heap objects).
	releaseFrame(fr)
	if len(vm.frames) == 0 {
		return v, nil
	}
	caller := vm.cur()
	for len(caller.regs) <= retReg {
		caller.regs = append(caller.regs, Null())
	}
	caller.regs[retReg] = v
	return v, nil
}

func (vm *VM) errf(format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s\n%s", msg, vm.stackTrace())
}

func (vm *VM) wrap(err error) error {
	return fmt.Errorf("%v\n%s", err, vm.stackTrace())
}

func (vm *VM) stackTrace() string {
	var b strings.Builder
	b.WriteString("stack traceback:")
	for i := len(vm.frames) - 1; i >= 0; i-- {
		fr := vm.frames[i]
		name := fr.proto.Name
		if name == "" {
			name = "<anon>"
		}
		line := 0
		ip := fr.ip - 1
		if ip >= 0 && ip < len(fr.proto.Lines) {
			line = int(fr.proto.Lines[ip])
		}
		fmt.Fprintf(&b, "\n  %s ip=%d line=%d", name, ip, line)
	}
	return b.String()
}

func rol32(x uint32, n uint) uint32 {
	n &= 31
	return (x << n) | (x >> (32 - n))
}
func ror32(x uint32, n uint) uint32 {
	n &= 31
	return (x >> n) | (x << (32 - n))
}

func arith(l, r Value, op rune) (Value, error) {
	// array concat if + and both arrays
	if op == '+' && l.Typ == TypeArray && r.Typ == TypeArray {
		var left, right []Value
		if l.Arr != nil {
			left = *l.Arr
		}
		if r.Arr != nil {
			right = *r.Arr
		}
		out := make([]Value, 0, len(left)+len(right))
		out = append(out, left...)
		out = append(out, right...)
		return Array(out), nil
	}
	// string concat if + and either is str
	if op == '+' && (l.Typ == TypeStr || r.Typ == TypeStr) {
		return Str(l.AsStr() + r.AsStr()), nil
	}
	// float if either float
	if l.Typ == TypeFloat || r.Typ == TypeFloat {
		lf, err1 := l.AsFloat()
		rf, err2 := r.AsFloat()
		if err1 != nil || err2 != nil {
			return Null(), fmt.Errorf("arith type error")
		}
		switch op {
		case '+':
			return Float(lf + rf), nil
		case '-':
			return Float(lf - rf), nil
		case '*':
			return Float(lf * rf), nil
		case '/':
			if rf == 0 {
				return Null(), fmt.Errorf("division by zero")
			}
			return Float(lf / rf), nil
		case '%':
			return Null(), fmt.Errorf("mod on float")
		}
	}
	li, err1 := l.AsInt()
	ri, err2 := r.AsInt()
	if err1 != nil || err2 != nil {
		return Null(), fmt.Errorf("arith type error")
	}
	switch op {
	case '+':
		return Int(li + ri), nil
	case '-':
		return Int(li - ri), nil
	case '*':
		return Int(li * ri), nil
	case '/':
		if ri == 0 {
			return Null(), fmt.Errorf("division by zero")
		}
		return Int(li / ri), nil
	case '%':
		if ri == 0 {
			return Null(), fmt.Errorf("mod by zero")
		}
		return Int(li % ri), nil
	}
	return Null(), fmt.Errorf("bad arith op")
}

func concat(l, r Value) (Value, error) {
	if l.Typ == TypeBytes || r.Typ == TypeBytes {
		lb, err1 := l.AsBytes()
		rb, err2 := r.AsBytes()
		if err1 != nil || err2 != nil {
			return Null(), fmt.Errorf("concat bytes error")
		}
		out := make([]byte, len(lb)+len(rb))
		copy(out, lb)
		copy(out[len(lb):], rb)
		return Bytes(out), nil
	}
	return Str(l.AsStr() + r.AsStr()), nil
}

func bytesOff(src, offV Value) ([]byte, int, error) {
	bs, err := src.AsBytes()
	if err != nil {
		return nil, 0, err
	}
	off, err := offV.AsInt()
	if err != nil {
		return nil, 0, err
	}
	return bs, int(off), nil
}
