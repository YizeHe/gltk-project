// Package compiler lowers GrokLang AST to GLKB bytecode Chunks.
package compiler

import (
	"fmt"

	"groklang/gltk/internal/ast"
	"groklang/gltk/internal/bytecode"
)

// PathImport describes a compiled .glk library to bind as a map global.
type PathImport struct {
	Alias   string
	Exports map[string]int // export name -> proto index in shared Chunk
}

// CompileOptions controls single- or multi-file compilation into a Chunk.
type CompileOptions struct {
	Filename    string          // source name for diagnostics / chunk
	Chunk       *bytecode.Chunk // shared chunk; nil creates a new one
	NamePrefix  string          // prefix for proto names (e.g. "lib#helpers#")
	IsLibrary   bool            // library: no main required; skip top-level stmts
	PathImports []PathImport    // emit map setups at start of main (entry only)
}

// Result is the outcome of Compile.
type Result struct {
	Chunk   *bytecode.Chunk
	Exports map[string]int // for libraries: public name -> proto index
}

// Compiler state for a whole program unit.
type Compiler struct {
	chunk  *bytecode.Chunk
	errors []string
	funcs  map[string]int // local unit name -> proto index
	anonN  int            // nested/anon function counter
}

// Compile transforms a Program into bytecode on opts.Chunk (or a new Chunk).
func Compile(prog *ast.Program, opts CompileOptions) (*Result, error) {
	chunk := opts.Chunk
	if chunk == nil {
		chunk = bytecode.NewChunk(opts.Filename)
	} else if chunk.SourceName == "" {
		chunk.SourceName = opts.Filename
	}
	c := &Compiler{
		chunk: chunk,
		funcs: map[string]int{},
	}

	type pending struct {
		fn       *ast.FuncDecl
		proto    *bytecode.Proto
		index    int
		origName string
	}
	var funcs []pending
	mainIdx := -1
	exports := map[string]int{}

	for _, fn := range prog.Funcs {
		if opts.IsLibrary && fn.Name == "main" {
			continue
		}
		name := opts.NamePrefix + fn.Name
		p := &bytecode.Proto{
			Name:      name,
			NumParams: uint8(len(fn.Params)),
		}
		ix := c.chunk.AddProto(p)
		funcs = append(funcs, pending{fn: fn, proto: p, index: ix, origName: fn.Name})
		c.funcs[fn.Name] = ix
		if fn.Name == "main" {
			mainIdx = ix
		}
		if opts.IsLibrary {
			exports[fn.Name] = ix
		} else if fn.Name != "main" {
			exports[fn.Name] = ix
		}
	}

	if !opts.IsLibrary && mainIdx < 0 {
		p := &bytecode.Proto{Name: "main", NumParams: 0}
		mainIdx = c.chunk.AddProto(p)
		fc := newFuncCompiler(c, p, mainIdx, nil, nil)
		emitPathImports(fc, opts.PathImports)
		emitBareImports(fc, prog)
		fc.fileLevel = true
		for _, s := range prog.Stmts {
			fc.compileStmt(s)
		}
		fc.fileLevel = false
		fc.emit(bytecode.MakeABC(bytecode.OpRETN, 0, 0, 0), 0)
		fc.finish()
	}

	for _, pe := range funcs {
		fc := newFuncCompiler(c, pe.proto, pe.index, pe.fn.Params, nil)
		if !opts.IsLibrary && pe.fn.Name == "main" {
			emitPathImports(fc, opts.PathImports)
			emitBareImports(fc, prog)
			fc.fileLevel = true
			for _, s := range prog.Stmts {
				fc.compileStmt(s)
			}
			fc.fileLevel = false
			// File-level temps must not be reused for main params (regs 0..n-1).
			fc.freeRegs = nil
		}
		for i, name := range pe.fn.Params {
			fc.locals[name] = uint8(i)
			if int(i)+1 > fc.nextReg {
				fc.nextReg = int(i) + 1
			}
		}
		fc.compileBlock(pe.fn.Body)
		fc.emit(bytecode.MakeABC(bytecode.OpRETN, 0, 0, 0), 0)
		fc.finish()
	}

	if opts.IsLibrary {
		if len(prog.Stmts) > 0 {
			_ = prog.Stmts
		}
		if len(c.errors) > 0 {
			return &Result{Chunk: c.chunk, Exports: exports}, fmt.Errorf("compile: %s", c.errors[0])
		}
		return &Result{Chunk: c.chunk, Exports: exports}, nil
	}

	if mainIdx < 0 && len(c.chunk.Protos) > 0 {
		mainIdx = 0
	}
	c.chunk.MainIndex = mainIdx
	if mainIdx < 0 {
		return nil, fmt.Errorf("compile: no main function")
	}
	if len(c.errors) > 0 {
		return &Result{Chunk: c.chunk, Exports: exports}, fmt.Errorf("compile: %s", c.errors[0])
	}
	return &Result{Chunk: c.chunk, Exports: exports}, nil
}

// CompileSource is a convenience for single-file programs (no path libs).
func CompileSource(prog *ast.Program, sourceName string) (*bytecode.Chunk, error) {
	res, err := Compile(prog, CompileOptions{Filename: sourceName})
	if res == nil {
		return nil, err
	}
	return res.Chunk, err
}

func emitBareImports(fc *funcCompiler, prog *ast.Program) {
	for _, imp := range prog.Imports {
		if imp.Path != "" {
			continue
		}
		for _, name := range imp.Names {
			ki := fc.parent.chunk.AddStrConst(name)
			r := fc.alloc()
			fc.emit(bytecode.MakeABx(bytecode.OpIMPORT, r, uint16(ki)), imp.Line)
			fc.emit(bytecode.MakeABx(bytecode.OpSTOREG, r, uint16(ki)), imp.Line)
			fc.free(r)
		}
	}
}

func emitPathImports(fc *funcCompiler, imports []PathImport) {
	for _, pi := range imports {
		if pi.Alias == "" {
			continue
		}
		mreg := fc.alloc()
		fc.emit(bytecode.MakeABC(bytecode.OpNEWMAP, mreg, 0, 0), 0)
		for name, pidx := range pi.Exports {
			freg := fc.alloc()
			if pidx > 0xFFFF {
				fc.parent.errors = append(fc.parent.errors, "proto index too large for MAKEFN")
			}
			fc.emit(bytecode.MakeABx(bytecode.OpMAKEFN, freg, uint16(pidx)), 0)
			ki := fc.parent.chunk.AddStrConst(name)
			if ki > 255 {
				kreg := fc.alloc()
				fc.emit(bytecode.MakeABx(bytecode.OpLOADK, kreg, uint16(ki)), 0)
				fc.emit(bytecode.MakeABC(bytecode.OpSETI, mreg, kreg, freg), 0)
				fc.free(kreg)
			} else {
				fc.emit(bytecode.MakeABC(bytecode.OpSETK, mreg, uint8(ki), freg), 0)
			}
			fc.free(freg)
		}
		aki := fc.parent.chunk.AddStrConst(pi.Alias)
		fc.emit(bytecode.MakeABx(bytecode.OpSTOREG, mreg, uint16(aki)), 0)
		fc.free(mreg)
	}
}

// loopFrame tracks break/continue patch points for a loop.
type loopFrame struct {
	breakPatches   []int
	continueTarget int
}

// capture describes a free variable for nested functions (capture-by-copy).
type capture struct {
	name      string
	fromLocal bool  // true: outer local reg; false: outer upval index
	index     uint8 // local reg or upval slot in outer
}

type funcCompiler struct {
	parent     *Compiler
	outer      *funcCompiler
	proto      *bytecode.Proto
	protoIx    int
	locals     map[string]uint8
	nextReg    int
	maxRegs    int
	freeRegs   []int
	loops      []loopFrame
	captures   []capture
	captureIdx map[string]uint8
	fileLevel  bool // top-level stmts compiled into main → true globals via STOREG
}

func newFuncCompiler(parent *Compiler, p *bytecode.Proto, ix int, params []string, outer *funcCompiler) *funcCompiler {
	fc := &funcCompiler{
		parent:     parent,
		outer:      outer,
		proto:      p,
		protoIx:    ix,
		locals:     map[string]uint8{},
		nextReg:    0,
		maxRegs:    0,
		captureIdx: map[string]uint8{},
	}
	for i, name := range params {
		fc.locals[name] = uint8(i)
		fc.nextReg = i + 1
	}
	fc.noteRegs()
	return fc
}

func (fc *funcCompiler) noteRegs() {
	if fc.nextReg > fc.maxRegs {
		fc.maxRegs = fc.nextReg
	}
}

func (fc *funcCompiler) emit(ins uint32, line int) int {
	ip := len(fc.proto.Code)
	fc.proto.Code = append(fc.proto.Code, ins)
	fc.proto.Lines = append(fc.proto.Lines, uint32(line))
	return ip
}

func (fc *funcCompiler) patchSbx(ip int, sbx int16) {
	ins := fc.proto.Code[ip]
	op := bytecode.GetOp(ins)
	a := bytecode.GetA(ins)
	fc.proto.Code[ip] = bytecode.MakeAsBx(op, a, sbx)
}

func (fc *funcCompiler) alloc() uint8 {
	if len(fc.freeRegs) > 0 {
		r := fc.freeRegs[len(fc.freeRegs)-1]
		fc.freeRegs = fc.freeRegs[:len(fc.freeRegs)-1]
		return uint8(r)
	}
	// OpABC/AsBx use 8-bit A; keep headroom under 255
	if fc.nextReg >= 254 {
		line := 0
		if n := len(fc.proto.Lines); n > 0 {
			line = int(fc.proto.Lines[n-1])
		}
		if line > 0 {
			fc.parent.errors = append(fc.parent.errors,
				fmt.Sprintf("too many registers (line %d); split function or use push", line))
		} else {
			fc.parent.errors = append(fc.parent.errors,
				"too many registers; split function or use push")
		}
		return 0
	}
	r := fc.nextReg
	fc.nextReg++
	fc.noteRegs()
	return uint8(r)
}

func (fc *funcCompiler) free(r uint8) {
	for _, loc := range fc.locals {
		if loc == r {
			return
		}
	}
	fc.freeRegs = append(fc.freeRegs, int(r))
}

func (fc *funcCompiler) finish() {
	n := fc.maxRegs
	if fc.nextReg > n {
		n = fc.nextReg
	}
	n += 16
	if n < int(fc.proto.NumParams)+8 {
		n = int(fc.proto.NumParams) + 16
	}
	if n > 255 {
		n = 255
	}
	if n < 16 {
		n = 16
	}
	fc.proto.NumRegs = uint8(n)
	fc.proto.NumUpvals = uint8(len(fc.captures))
}

func (fc *funcCompiler) pushLoop(continueTarget int) {
	fc.loops = append(fc.loops, loopFrame{continueTarget: continueTarget})
}

func (fc *funcCompiler) popLoop(endIP int) {
	if len(fc.loops) == 0 {
		return
	}
	lf := fc.loops[len(fc.loops)-1]
	fc.loops = fc.loops[:len(fc.loops)-1]
	for _, ip := range lf.breakPatches {
		fc.patchSbx(ip, int16(endIP-ip-1))
	}
}

func (fc *funcCompiler) curLoop() *loopFrame {
	if len(fc.loops) == 0 {
		return nil
	}
	return &fc.loops[len(fc.loops)-1]
}

func (fc *funcCompiler) compileBlock(b *ast.BlockStmt) {
	if b == nil {
		return
	}
	saved := copyMap(fc.locals)
	savedNext := fc.nextReg
	for _, s := range b.Stmts {
		fc.compileStmt(s)
	}
	fc.locals = saved
	fc.nextReg = savedNext
	fc.freeRegs = nil
}

func copyMap(m map[string]uint8) map[string]uint8 {
	n := make(map[string]uint8, len(m))
	for k, v := range m {
		n[k] = v
	}
	return n
}

func (fc *funcCompiler) compileStmt(s ast.Stmt) {
	switch st := s.(type) {
	case *ast.BlockStmt:
		fc.compileBlock(st)
	case *ast.LetStmt:
		r := fc.alloc()
		fc.compileExpr(st.Value, r)
		if fc.fileLevel {
			// File-level let is a true global (visible to all functions).
			ki := fc.parent.chunk.AddStrConst(st.Name)
			fc.emit(bytecode.MakeABx(bytecode.OpSTOREG, r, uint16(ki)), st.Line)
			fc.free(r)
		} else {
			fc.locals[st.Name] = r
		}
	case *ast.AssignStmt:
		fc.compileAssign(st)
	case *ast.ExprStmt:
		r := fc.alloc()
		fc.compileExpr(st.X, r)
		fc.free(r)
	case *ast.ReturnStmt:
		if st.Value == nil {
			fc.emit(bytecode.MakeABC(bytecode.OpRETN, 0, 0, 0), st.Line)
		} else {
			r := fc.alloc()
			fc.compileExpr(st.Value, r)
			fc.emit(bytecode.MakeABC(bytecode.OpRET, r, 0, 0), st.Line)
			fc.free(r)
		}
	case *ast.IfStmt:
		fc.compileIf(st)
	case *ast.WhileStmt:
		fc.compileWhile(st)
	case *ast.ForInStmt:
		fc.compileForIn(st)
	case *ast.BreakStmt:
		lf := fc.curLoop()
		if lf == nil {
			fc.parent.errors = append(fc.parent.errors, "break outside loop")
			return
		}
		ip := fc.emit(bytecode.MakeAsBx(bytecode.OpJMP, 0, 0), st.Line)
		lf.breakPatches = append(lf.breakPatches, ip)
	case *ast.ContinueStmt:
		lf := fc.curLoop()
		if lf == nil {
			fc.parent.errors = append(fc.parent.errors, "continue outside loop")
			return
		}
		back := lf.continueTarget - (len(fc.proto.Code) + 1)
		fc.emit(bytecode.MakeAsBx(bytecode.OpJMP, 0, int16(back)), st.Line)
	case *ast.SwitchStmt:
		fc.compileSwitch(st)
	case *ast.TryStmt:
		fc.compileTry(st)
	case *ast.ThrowStmt:
		r := fc.alloc()
		fc.compileExpr(st.Value, r)
		fc.emit(bytecode.MakeABC(bytecode.OpTHROW, r, 0, 0), st.Line)
		fc.free(r)
	default:
		fc.parent.errors = append(fc.parent.errors, "unknown stmt")
	}
}

func (fc *funcCompiler) compileAssign(st *ast.AssignStmt) {
	switch t := st.Target.(type) {
	case *ast.Ident:
		if reg, ok := fc.locals[t.Name]; ok {
			fc.compileExpr(st.Value, reg)
			return
		}
		if ui, ok := fc.captureIdx[t.Name]; ok {
			r := fc.alloc()
			fc.compileExpr(st.Value, r)
			fc.emit(bytecode.MakeABC(bytecode.OpSETUPV, ui, r, 0), st.Line)
			fc.free(r)
			return
		}
		r := fc.alloc()
		fc.compileExpr(st.Value, r)
		ki := fc.parent.chunk.AddStrConst(t.Name)
		fc.emit(bytecode.MakeABx(bytecode.OpSTOREG, r, uint16(ki)), st.Line)
		fc.free(r)
	case *ast.IndexExpr:
		obj := fc.alloc()
		fc.compileExpr(t.X, obj)
		idx := fc.alloc()
		fc.compileExpr(t.Index, idx)
		val := fc.alloc()
		fc.compileExpr(st.Value, val)
		fc.emit(bytecode.MakeABC(bytecode.OpSETI, obj, idx, val), st.Line)
		fc.free(obj)
		fc.free(idx)
		fc.free(val)
	case *ast.FieldExpr:
		obj := fc.alloc()
		fc.compileExpr(t.X, obj)
		val := fc.alloc()
		fc.compileExpr(st.Value, val)
		ki := fc.parent.chunk.AddStrConst(t.Name)
		// SETK only has 8-bit const index; fall back to LOADK+SETI for large pools.
		if ki <= 255 {
			fc.emit(bytecode.MakeABC(bytecode.OpSETK, obj, uint8(ki), val), st.Line)
		} else {
			kreg := fc.alloc()
			fc.emit(bytecode.MakeABx(bytecode.OpLOADK, kreg, uint16(ki)), st.Line)
			fc.emit(bytecode.MakeABC(bytecode.OpSETI, obj, kreg, val), st.Line)
			fc.free(kreg)
		}
		fc.free(obj)
		fc.free(val)
	default:
		fc.parent.errors = append(fc.parent.errors, "invalid assignment target")
	}
}

func (fc *funcCompiler) compileIf(st *ast.IfStmt) {
	cond := fc.alloc()
	fc.compileExpr(st.Cond, cond)
	jf := fc.emit(bytecode.MakeAsBx(bytecode.OpJF, cond, 0), st.Line)
	fc.free(cond)
	fc.compileBlock(st.Then)
	if st.Else != nil {
		jmp := fc.emit(bytecode.MakeAsBx(bytecode.OpJMP, 0, 0), st.Line)
		elseIP := len(fc.proto.Code)
		fc.patchSbx(jf, int16(elseIP-jf-1))
		switch e := st.Else.(type) {
		case *ast.BlockStmt:
			fc.compileBlock(e)
		case *ast.IfStmt:
			fc.compileIf(e)
		}
		end := len(fc.proto.Code)
		fc.patchSbx(jmp, int16(end-jmp-1))
	} else {
		end := len(fc.proto.Code)
		fc.patchSbx(jf, int16(end-jf-1))
	}
}

func (fc *funcCompiler) compileWhile(st *ast.WhileStmt) {
	// continue jumps to condition re-eval
	loopStart := len(fc.proto.Code)
	fc.pushLoop(loopStart)
	cond := fc.alloc()
	fc.compileExpr(st.Cond, cond)
	jf := fc.emit(bytecode.MakeAsBx(bytecode.OpJF, cond, 0), st.Line)
	fc.free(cond)
	fc.compileBlock(st.Body)
	back := loopStart - (len(fc.proto.Code) + 1)
	fc.emit(bytecode.MakeAsBx(bytecode.OpJMP, 0, int16(back)), st.Line)
	end := len(fc.proto.Code)
	fc.patchSbx(jf, int16(end-jf-1))
	fc.popLoop(end)
}

func (fc *funcCompiler) compileForIn(st *ast.ForInStmt) {
	// Map-aware for-in (iterate keys for maps):
	//   it = expr
	//   if typeof(it) == "map" { it = keys(it) }
	//   i = -1; n = len(it)
	//   cont: i = i + 1
	//   if !(i < n) goto end
	//   name = it[i]; body; jmp cont
	//   end:
	it := fc.alloc()
	fc.compileExpr(st.Iter, it)

	typeReg := fc.alloc()
	fc.emit(bytecode.MakeABC(bytecode.OpTYPEOF, typeReg, it, 0), st.Line)
	mapLit := fc.alloc()
	ki := fc.parent.chunk.AddStrConst("map")
	fc.emit(bytecode.MakeABx(bytecode.OpLOADK, mapLit, uint16(ki)), st.Line)
	eq := fc.alloc()
	fc.emit(bytecode.MakeABC(bytecode.OpEQ, eq, typeReg, mapLit), st.Line)
	jfMap := fc.emit(bytecode.MakeAsBx(bytecode.OpJF, eq, 0), st.Line)
	keysReg := fc.alloc()
	fc.emit(bytecode.MakeABC(bytecode.OpKEYS, keysReg, it, 0), st.Line)
	fc.emit(bytecode.MakeABC(bytecode.OpMOVE, it, keysReg, 0), st.Line)
	fc.free(keysReg)
	afterKeys := len(fc.proto.Code)
	fc.patchSbx(jfMap, int16(afterKeys-jfMap-1))
	fc.free(typeReg)
	fc.free(mapLit)
	fc.free(eq)

	iReg := fc.alloc()
	fc.emit(bytecode.MakeAsBx(bytecode.OpLOADI, iReg, -1), st.Line)
	nReg := fc.alloc()
	fc.emit(bytecode.MakeABC(bytecode.OpLEN, nReg, it, 0), st.Line)

	// continue target known before body
	contIP := len(fc.proto.Code)
	fc.pushLoop(contIP)
	// i = i + 1
	one := fc.alloc()
	fc.emit(bytecode.MakeAsBx(bytecode.OpLOADI, one, 1), st.Line)
	fc.emit(bytecode.MakeABC(bytecode.OpADD, iReg, iReg, one), st.Line)
	fc.free(one)
	// if !(i < n) break
	cmp := fc.alloc()
	fc.emit(bytecode.MakeABC(bytecode.OpLT, cmp, iReg, nReg), st.Line)
	jf := fc.emit(bytecode.MakeAsBx(bytecode.OpJF, cmp, 0), st.Line)
	fc.free(cmp)

	elem := fc.alloc()
	fc.emit(bytecode.MakeABC(bytecode.OpGETI, elem, it, iReg), st.Line)
	prev, had := fc.locals[st.Name]
	fc.locals[st.Name] = elem
	fc.compileBlock(st.Body)
	if had {
		fc.locals[st.Name] = prev
	} else {
		delete(fc.locals, st.Name)
	}
	fc.free(elem)

	back := contIP - (len(fc.proto.Code) + 1)
	fc.emit(bytecode.MakeAsBx(bytecode.OpJMP, 0, int16(back)), st.Line)
	end := len(fc.proto.Code)
	fc.patchSbx(jf, int16(end-jf-1))
	fc.popLoop(end)

	fc.free(it)
	fc.free(iReg)
	fc.free(nReg)
}


// ---- Switch / Try / Ternary / FuncExpr / Expr ----

func (fc *funcCompiler) compileSwitch(st *ast.SwitchStmt) {
	// tag = expr
	// for each case: if tag==v0 || tag==v1 ... { body; jmp end }
	// default body
	tag := fc.alloc()
	fc.compileExpr(st.Tag, tag)
	var endJmps []int
	for _, cs := range st.Cases {
		// build match: or of equals
		match := fc.alloc()
		fc.emit(bytecode.MakeABC(bytecode.OpLOADB, match, 0, 0), st.Line) // false
		for _, v := range cs.Values {
			vr := fc.alloc()
			fc.compileExpr(v, vr)
			eq := fc.alloc()
			fc.emit(bytecode.MakeABC(bytecode.OpEQ, eq, tag, vr), st.Line)
			// match = match || eq
			// short-circuit style: if match skip; match = eq
			jt := fc.emit(bytecode.MakeAsBx(bytecode.OpJT, match, 0), st.Line)
			fc.emit(bytecode.MakeABC(bytecode.OpMOVE, match, eq, 0), st.Line)
			after := len(fc.proto.Code)
			fc.patchSbx(jt, int16(after-jt-1))
			fc.free(vr)
			fc.free(eq)
		}
		jf := fc.emit(bytecode.MakeAsBx(bytecode.OpJF, match, 0), st.Line)
		fc.free(match)
		fc.compileBlock(cs.Body)
		endJmps = append(endJmps, fc.emit(bytecode.MakeAsBx(bytecode.OpJMP, 0, 0), st.Line))
		afterCase := len(fc.proto.Code)
		fc.patchSbx(jf, int16(afterCase-jf-1))
	}
	if st.Default != nil {
		fc.compileBlock(st.Default)
	}
	end := len(fc.proto.Code)
	for _, ip := range endJmps {
		fc.patchSbx(ip, int16(end-ip-1))
	}
	fc.free(tag)
}

func (fc *funcCompiler) compileTry(st *ast.TryStmt) {
	// errReg holds caught error
	errReg := fc.alloc()
	// OpTRY A sBx — catch at ip+sBx
	tryIP := fc.emit(bytecode.MakeAsBx(bytecode.OpTRY, errReg, 0), st.Line)
	fc.compileBlock(st.Body)
	fc.emit(bytecode.MakeABC(bytecode.OpENDTRY, 0, 0, 0), st.Line)
	jmpEnd := fc.emit(bytecode.MakeAsBx(bytecode.OpJMP, 0, 0), st.Line)
	// catch
	catchIP := len(fc.proto.Code)
	fc.patchSbx(tryIP, int16(catchIP-tryIP-1))
	// bind err name
	prev, had := fc.locals[st.ErrName]
	fc.locals[st.ErrName] = errReg
	fc.compileBlock(st.Catch)
	if had {
		fc.locals[st.ErrName] = prev
	} else {
		delete(fc.locals, st.ErrName)
	}
	end := len(fc.proto.Code)
	fc.patchSbx(jmpEnd, int16(end-jmpEnd-1))
	// keep errReg live through catch (it's a local binding during catch)
	// free after
	fc.free(errReg)
}

func (fc *funcCompiler) compileExpr(e ast.Expr, dest uint8) {
	if e == nil {
		fc.emit(bytecode.MakeABC(bytecode.OpLOADN, dest, 0, 0), 0)
		return
	}
	switch ex := e.(type) {
	case *ast.Literal:
		fc.compileLiteral(ex, dest)
	case *ast.Ident:
		fc.compileIdent(ex, dest)
	case *ast.BinaryExpr:
		fc.compileBinary(ex, dest)
	case *ast.UnaryExpr:
		fc.compileUnary(ex, dest)
	case *ast.TernaryExpr:
		fc.compileTernary(ex, dest)
	case *ast.FuncExpr:
		fc.compileFuncExpr(ex, dest)
	case *ast.CallExpr:
		fc.compileCall(ex, dest)
	case *ast.IndexExpr:
		obj := fc.alloc()
		fc.compileExpr(ex.X, obj)
		idx := fc.alloc()
		fc.compileExpr(ex.Index, idx)
		fc.emit(bytecode.MakeABC(bytecode.OpGETI, dest, obj, idx), ex.Line)
		fc.free(obj)
		fc.free(idx)
	case *ast.FieldExpr:
		obj := fc.alloc()
		fc.compileExpr(ex.X, obj)
		ki := fc.parent.chunk.AddStrConst(ex.Name)
		if ki <= 255 {
			fc.emit(bytecode.MakeABC(bytecode.OpGETK, dest, obj, uint8(ki)), ex.Line)
		} else {
			kreg := fc.alloc()
			fc.emit(bytecode.MakeABx(bytecode.OpLOADK, kreg, uint16(ki)), ex.Line)
			fc.emit(bytecode.MakeABC(bytecode.OpGETI, dest, obj, kreg), ex.Line)
			fc.free(kreg)
		}
		fc.free(obj)
	case *ast.ArrayExpr:
		n := len(ex.Elts)
		if n == 0 {
			fc.emit(bytecode.MakeABC(bytecode.OpNEWARR, dest, 0, 0), ex.Line)
			return
		}
		base := fc.alloc()
		regs := make([]uint8, n)
		regs[0] = base
		for i := 1; i < n; i++ {
			regs[i] = fc.alloc()
			if regs[i] != regs[0]+uint8(i) {
				for j := 0; j < i; j++ {
					fc.free(regs[j])
				}
				start := fc.nextReg
				for fc.nextReg < start+n {
					fc.nextReg++
				}
				for j := 0; j < n; j++ {
					regs[j] = uint8(start + j)
				}
				for j := 0; j < n; j++ {
					fc.compileExpr(ex.Elts[j], regs[j])
				}
				fc.emit(bytecode.MakeABC(bytecode.OpNEWARR, dest, regs[0], uint8(n)), ex.Line)
				for j := 0; j < n; j++ {
					fc.free(regs[j])
				}
				return
			}
		}
		for i := 0; i < n; i++ {
			fc.compileExpr(ex.Elts[i], regs[i])
		}
		fc.emit(bytecode.MakeABC(bytecode.OpNEWARR, dest, regs[0], uint8(n)), ex.Line)
		for i := 0; i < n; i++ {
			fc.free(regs[i])
		}
	case *ast.MapExpr:
		fc.emit(bytecode.MakeABC(bytecode.OpNEWMAP, dest, 0, 0), ex.Line)
		for i := range ex.Keys {
			var ki int
			if lit, ok := ex.Keys[i].(*ast.Literal); ok && lit.Kind == ast.LitStr {
				ki = fc.parent.chunk.AddStrConst(lit.Str)
			} else {
				kreg := fc.alloc()
				fc.compileExpr(ex.Keys[i], kreg)
				vreg := fc.alloc()
				fc.compileExpr(ex.Vals[i], vreg)
				fc.emit(bytecode.MakeABC(bytecode.OpSETI, dest, kreg, vreg), ex.Line)
				fc.free(kreg)
				fc.free(vreg)
				continue
			}
			vreg := fc.alloc()
			fc.compileExpr(ex.Vals[i], vreg)
			if ki <= 255 {
				fc.emit(bytecode.MakeABC(bytecode.OpSETK, dest, uint8(ki), vreg), ex.Line)
			} else {
				kreg := fc.alloc()
				fc.emit(bytecode.MakeABx(bytecode.OpLOADK, kreg, uint16(ki)), ex.Line)
				fc.emit(bytecode.MakeABC(bytecode.OpSETI, dest, kreg, vreg), ex.Line)
				fc.free(kreg)
			}
			fc.free(vreg)
		}
	default:
		fc.parent.errors = append(fc.parent.errors, "unknown expr")
		fc.emit(bytecode.MakeABC(bytecode.OpLOADN, dest, 0, 0), 0)
	}
}

func (fc *funcCompiler) compileTernary(ex *ast.TernaryExpr, dest uint8) {
	// cond; JF else; then; JMP end; else; end
	cond := fc.alloc()
	fc.compileExpr(ex.Cond, cond)
	jf := fc.emit(bytecode.MakeAsBx(bytecode.OpJF, cond, 0), ex.Line)
	fc.free(cond)
	fc.compileExpr(ex.Then, dest)
	jmp := fc.emit(bytecode.MakeAsBx(bytecode.OpJMP, 0, 0), ex.Line)
	elseIP := len(fc.proto.Code)
	fc.patchSbx(jf, int16(elseIP-jf-1))
	fc.compileExpr(ex.Else, dest)
	end := len(fc.proto.Code)
	fc.patchSbx(jmp, int16(end-jmp-1))
}

func (fc *funcCompiler) compileFuncExpr(ex *ast.FuncExpr, dest uint8) {
	// Create nested proto, compile with free-var analysis, emit CLOSURE or MAKEFN
	fc.parent.anonN++
	name := fmt.Sprintf("<anon:%d>", fc.parent.anonN)
	p := &bytecode.Proto{
		Name:      name,
		NumParams: uint8(len(ex.Params)),
	}
	ix := fc.parent.chunk.AddProto(p)
	child := newFuncCompiler(fc.parent, p, ix, ex.Params, fc)
	for i, name := range ex.Params {
		child.locals[name] = uint8(i)
		if int(i)+1 > child.nextReg {
			child.nextReg = int(i) + 1
		}
	}
	child.compileBlock(ex.Body)
	child.emit(bytecode.MakeABC(bytecode.OpRETN, 0, 0, 0), ex.Line)
	child.finish()

	nup := len(child.captures)
	if nup == 0 {
		if ix > 0xFFFF {
			fc.parent.errors = append(fc.parent.errors, "proto index too large")
		}
		fc.emit(bytecode.MakeABx(bytecode.OpMAKEFN, dest, uint16(ix)), ex.Line)
		return
	}
	// CLOSURE uses 8-bit proto index in B
	if ix > 255 {
		fc.parent.errors = append(fc.parent.errors, "proto index too large for CLOSURE (max 255)")
	}
	// Place upvalue sources in dest+1 .. dest+nup (need consecutive free regs)
	// Ensure dest and following nup regs are available
	// Allocate block: use nextReg for base if dest might overlap
	// Load captures into R[dest+1+i]
	start := int(dest) + 1
	// grow nextReg if needed
	needEnd := start + nup
	if fc.nextReg < needEnd {
		fc.nextReg = needEnd
		fc.noteRegs()
	}
	for i, cap := range child.captures {
		r := uint8(start + i)
		if cap.fromLocal {
			if r != cap.index {
				fc.emit(bytecode.MakeABC(bytecode.OpMOVE, r, cap.index, 0), ex.Line)
			}
		} else {
			// from outer upval
			fc.emit(bytecode.MakeABC(bytecode.OpGETUPV, r, cap.index, 0), ex.Line)
		}
	}
	fc.emit(bytecode.MakeABC(bytecode.OpCLOSURE, dest, uint8(ix), uint8(nup)), ex.Line)
}

// addCapture records a free variable from outer scopes; returns upval index.
func (fc *funcCompiler) addCapture(name string) (uint8, bool) {
	if ui, ok := fc.captureIdx[name]; ok {
		return ui, true
	}
	if fc.outer == nil {
		return 0, false
	}
	// local in outer?
	if reg, ok := fc.outer.locals[name]; ok {
		ui := uint8(len(fc.captures))
		fc.captures = append(fc.captures, capture{name: name, fromLocal: true, index: reg})
		fc.captureIdx[name] = ui
		return ui, true
	}
	// upval in outer?
	if uiOuter, ok := fc.outer.captureIdx[name]; ok {
		ui := uint8(len(fc.captures))
		fc.captures = append(fc.captures, capture{name: name, fromLocal: false, index: uiOuter})
		fc.captureIdx[name] = ui
		return ui, true
	}
	// recurse: force outer to capture it, then capture from outer's upval
	if uiOuter, ok := fc.outer.addCapture(name); ok {
		ui := uint8(len(fc.captures))
		fc.captures = append(fc.captures, capture{name: name, fromLocal: false, index: uiOuter})
		fc.captureIdx[name] = ui
		return ui, true
	}
	return 0, false
}

func (fc *funcCompiler) compileLiteral(lit *ast.Literal, dest uint8) {
	switch lit.Kind {
	case ast.LitNull:
		fc.emit(bytecode.MakeABC(bytecode.OpLOADN, dest, 0, 0), lit.Line)
	case ast.LitBool:
		var b uint8
		if lit.Bool {
			b = 1
		}
		fc.emit(bytecode.MakeABC(bytecode.OpLOADB, dest, b, 0), lit.Line)
	case ast.LitInt:
		if lit.Int >= -32768 && lit.Int <= 32767 {
			fc.emit(bytecode.MakeAsBx(bytecode.OpLOADI, dest, int16(lit.Int)), lit.Line)
		} else {
			ki := fc.parent.chunk.AddIntConst(lit.Int)
			fc.emit(bytecode.MakeABx(bytecode.OpLOADK, dest, uint16(ki)), lit.Line)
		}
	case ast.LitFloat:
		ki := fc.parent.chunk.AddFloatConst(lit.Float)
		fc.emit(bytecode.MakeABx(bytecode.OpLOADK, dest, uint16(ki)), lit.Line)
	case ast.LitStr:
		ki := fc.parent.chunk.AddStrConst(lit.Str)
		fc.emit(bytecode.MakeABx(bytecode.OpLOADK, dest, uint16(ki)), lit.Line)
	}
}

func (fc *funcCompiler) compileIdent(id *ast.Ident, dest uint8) {
	if reg, ok := fc.locals[id.Name]; ok {
		if reg != dest {
			fc.emit(bytecode.MakeABC(bytecode.OpMOVE, dest, reg, 0), id.Line)
		}
		return
	}
	if ui, ok := fc.captureIdx[id.Name]; ok {
		fc.emit(bytecode.MakeABC(bytecode.OpGETUPV, dest, ui, 0), id.Line)
		return
	}
	if ui, ok := fc.addCapture(id.Name); ok {
		fc.emit(bytecode.MakeABC(bytecode.OpGETUPV, dest, ui, 0), id.Line)
		return
	}
	// same compilation unit function → MAKEFN
	if ix, ok := fc.parent.funcs[id.Name]; ok {
		fc.emit(bytecode.MakeABx(bytecode.OpMAKEFN, dest, uint16(ix)), id.Line)
		return
	}
	// global / module
	ki := fc.parent.chunk.AddStrConst(id.Name)
	fc.emit(bytecode.MakeABx(bytecode.OpLOADG, dest, uint16(ki)), id.Line)
}

func (fc *funcCompiler) compileUnary(ex *ast.UnaryExpr, dest uint8) {
	tmp := fc.alloc()
	fc.compileExpr(ex.X, tmp)
	switch ex.Op {
	case "!":
		fc.emit(bytecode.MakeABC(bytecode.OpLNOT, dest, tmp, 0), ex.Line)
	case "-":
		fc.emit(bytecode.MakeABC(bytecode.OpNEG, dest, tmp, 0), ex.Line)
	case "~":
		fc.emit(bytecode.MakeABC(bytecode.OpNOT, dest, tmp, 0), ex.Line)
	default:
		fc.parent.errors = append(fc.parent.errors, "bad unary "+ex.Op)
	}
	fc.free(tmp)
}

func (fc *funcCompiler) compileBinary(ex *ast.BinaryExpr, dest uint8) {
	if ex.Op == "&&" {
		fc.compileExpr(ex.Left, dest)
		jf := fc.emit(bytecode.MakeAsBx(bytecode.OpJF, dest, 0), ex.Line)
		fc.compileExpr(ex.Right, dest)
		end := len(fc.proto.Code)
		fc.patchSbx(jf, int16(end-jf-1))
		return
	}
	if ex.Op == "||" {
		fc.compileExpr(ex.Left, dest)
		jt := fc.emit(bytecode.MakeAsBx(bytecode.OpJT, dest, 0), ex.Line)
		fc.compileExpr(ex.Right, dest)
		end := len(fc.proto.Code)
		fc.patchSbx(jt, int16(end-jt-1))
		return
	}
	l := fc.alloc()
	r := fc.alloc()
	fc.compileExpr(ex.Left, l)
	fc.compileExpr(ex.Right, r)
	var op bytecode.Opcode
	switch ex.Op {
	case "+":
		op = bytecode.OpADD
	case "-":
		op = bytecode.OpSUB
	case "*":
		op = bytecode.OpMUL
	case "/":
		op = bytecode.OpDIV
	case "%":
		op = bytecode.OpMOD
	case "&":
		op = bytecode.OpAND
	case "|":
		op = bytecode.OpOR
	case "^":
		op = bytecode.OpXOR
	case "<<":
		op = bytecode.OpSHL
	case ">>":
		op = bytecode.OpSHR
	case "==":
		op = bytecode.OpEQ
	case "!=":
		op = bytecode.OpNE
	case "<":
		op = bytecode.OpLT
	case "<=":
		op = bytecode.OpLE
	case ">":
		op = bytecode.OpGT
	case ">=":
		op = bytecode.OpGE
	default:
		fc.parent.errors = append(fc.parent.errors, "bad binary "+ex.Op)
		op = bytecode.OpADD
	}
	fc.emit(bytecode.MakeABC(op, dest, l, r), ex.Line)
	fc.free(l)
	fc.free(r)
}

func (fc *funcCompiler) compileCall(ex *ast.CallExpr, dest uint8) {
	n := len(ex.Args)
	start := fc.nextReg
	need := n + 1
	for fc.nextReg < start+need {
		fc.nextReg++
	}
	fc.noteRegs()
	base := uint8(start)
	fc.compileExpr(ex.Fun, base)
	for i, arg := range ex.Args {
		fc.compileExpr(arg, base+1+uint8(i))
	}
	fc.emit(bytecode.MakeABC(bytecode.OpCALL, dest, base, uint8(n)), ex.Line)
	for i := 0; i < need; i++ {
		fc.free(base + uint8(i))
	}
}
