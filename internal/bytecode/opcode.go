// Package bytecode defines GrokLang opcodes, chunks, and serialization.
package bytecode

// Opcode is a single GLVM instruction operation code.
type Opcode uint8

const (
	OpHALT Opcode = iota // stop VM
	OpMOVE               // R[A] = R[B]
	OpLOADK              // R[A] = K[Bx]
	OpLOADI              // R[A] = sBx (signed immediate int)
	OpLOADN              // R[A] = null
	OpLOADB              // R[A] = (B != 0)  bool
	OpLOADF              // R[A] = K[Bx] as float (const)
	OpADD                // R[A] = R[B] + R[C]
	OpSUB                // R[A] = R[B] - R[C]
	OpMUL                // R[A] = R[B] * R[C]
	OpDIV                // R[A] = R[B] / R[C]
	OpMOD                // R[A] = R[B] % R[C]
	OpAND                // R[A] = R[B] & R[C] (int)
	OpOR                 // R[A] = R[B] | R[C]
	OpXOR                // R[A] = R[B] ^ R[C]
	OpSHL                // R[A] = R[B] << R[C]
	OpSHR                // R[A] = R[B] >> R[C] (logical for uint-like)
	OpROL                // R[A] = rol32(R[B], R[C])
	OpROR                // R[A] = ror32(R[B], R[C])
	OpNOT                // R[A] = ~R[B] (bitwise)
	OpNEG                // R[A] = -R[B]
	OpLNOT               // R[A] = !truthy(R[B])
	OpEQ                 // R[A] = R[B] == R[C]
	OpNE                 // R[A] = R[B] != R[C]
	OpLT                 // R[A] = R[B] <  R[C]
	OpLE                 // R[A] = R[B] <= R[C]
	OpGT                 // R[A] = R[B] >  R[C]
	OpGE                 // R[A] = R[B] >= R[C]
	OpJMP                // ip += sBx
	OpJT                 // if truthy(R[A]) ip += sBx
	OpJF                 // if !truthy(R[A]) ip += sBx
	OpNEWARR             // R[A] = array from R[B]..R[B+C-1]
	OpNEWMAP             // R[A] = empty map (or filled via SETK)
	OpGETI               // R[A] = R[B][R[C]]  (array/map/str/bytes)
	OpSETI               // R[A][R[B]] = R[C]
	OpGETK               // R[A] = R[B][K[C]]  map string key const
	OpSETK               // R[A][K[B]] = R[C]
	OpLEN                // R[A] = len(R[B])
	OpCONCAT             // R[A] = concat(R[B], R[C]) str/bytes
	OpBGET8              // R[A] = u8  R[B][R[C]]
	OpBGET16             // R[A] = u16le R[B][R[C]]
	OpBGET32             // R[A] = u32le R[B][R[C]]
	OpBGET64             // R[A] = u64le R[B][R[C]] as int64
	OpBSLICE             // R[A] = slice(R[B], R[C], R[C+1])
	OpBSET8              // R[A][R[B]] = u8(R[C])
	OpCALL               // R[A] = call R[B] with C args at R[B+1]..
	OpRET                // return R[A]
	OpRETN               // return null
	OpMAKEFN             // R[A] = closure(proto Bx)
	OpCLOSURE            // R[A] = closure(proto B) with C upvalues following as MOVE/GETUPVAL pairs
	OpGETUPV             // R[A] = upvals[B]
	OpSETUPV             // upvals[A] = R[B]
	OpNOP                // no-op
	OpTOSTR              // R[A] = string(R[B])
	OpTOINT              // R[A] = int(R[B])
	OpTYPEOF             // R[A] = type name string of R[B]
	OpISNULL             // R[A] = R[B] is null
	OpIN                 // R[A] = R[B] in R[C] (key/index exists)
	OpBAND               // logical and result already via jumps; keep bitwise AND as OpAND
	OpARRPUSH            // append R[C] onto array R[A] (mut)
	OpKEYS               // R[A] = keys of map R[B] as array
	OpASSERT             // if !R[A] runtime error with K[B]
	OpFORPREP            // internal: for-range prep (optional)
	OpFORLOOP            // internal
	OpLOADG              // R[A] = globals[K[Bx]]
	OpSTOREG             // globals[K[Bx]] = R[A]
	OpDUP                // R[A] = R[B] (alias MOVE)
	OpSWAP               // swap R[A], R[B]
	OpIMPORT             // R[A] = native_module(K[Bx])
	OpLIST               // synonym NEWARR
	// --- append-only exception opcodes (do not reorder above) ---
	OpTRY                // push try: catch at ip+sBx, error into R[A]
	OpENDTRY             // pop try frame
	OpTHROW              // raise R[A] as error
	OpMAX                // sentinel
)

// Name returns the mnemonic for an opcode.
func (op Opcode) Name() string {
	if int(op) < len(opNames) {
		return opNames[op]
	}
	return "?"
}

var opNames = []string{
	"HALT", "MOVE", "LOADK", "LOADI", "LOADN", "LOADB", "LOADF",
	"ADD", "SUB", "MUL", "DIV", "MOD",
	"AND", "OR", "XOR", "SHL", "SHR", "ROL", "ROR",
	"NOT", "NEG", "LNOT",
	"EQ", "NE", "LT", "LE", "GT", "GE",
	"JMP", "JT", "JF",
	"NEWARR", "NEWMAP", "GETI", "SETI", "GETK", "SETK",
	"LEN", "CONCAT",
	"BGET8", "BGET16", "BGET32", "BGET64", "BSLICE", "BSET8",
	"CALL", "RET", "RETN", "MAKEFN", "CLOSURE", "GETUPV", "SETUPV",
	"NOP", "TOSTR", "TOINT", "TYPEOF", "ISNULL", "IN",
	"BAND", "ARRPUSH", "KEYS", "ASSERT", "FORPREP", "FORLOOP",
	"LOADG", "STOREG", "DUP", "SWAP", "IMPORT", "LIST",
	"TRY", "ENDTRY", "THROW",
}

// Instruction packing: 32-bit
// bits [0:8)  opcode
// bits [8:16) A
// bits [16:24) B
// bits [24:32) C
// Bx = B<<8|C as uint16; sBx = int16(Bx) - 0x7fff for jumps? We use signed int16 directly in Bx.

// MakeABC packs opcode and A,B,C registers/operands.
func MakeABC(op Opcode, a, b, c uint8) uint32 {
	return uint32(op) | uint32(a)<<8 | uint32(b)<<16 | uint32(c)<<24
}

// MakeABx packs opcode, A, and unsigned 16-bit Bx.
func MakeABx(op Opcode, a uint8, bx uint16) uint32 {
	return uint32(op) | uint32(a)<<8 | uint32(bx)<<16
}

// MakeAsBx packs opcode, A, and signed 16-bit sBx (for jumps relative).
func MakeAsBx(op Opcode, a uint8, sbx int16) uint32 {
	return uint32(op) | uint32(a)<<8 | uint32(uint16(sbx))<<16
}

// Decode fields from instruction.
func GetOp(i uint32) Opcode { return Opcode(i & 0xff) }
func GetA(i uint32) uint8   { return uint8((i >> 8) & 0xff) }
func GetB(i uint32) uint8   { return uint8((i >> 16) & 0xff) }
func GetC(i uint32) uint8   { return uint8((i >> 24) & 0xff) }
func GetBx(i uint32) uint16 { return uint16((i >> 16) & 0xffff) }
func GetsBx(i uint32) int16 { return int16(uint16((i >> 16) & 0xffff)) }
