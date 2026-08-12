package bytecode

import (
	"fmt"
	"strings"
)

// Disassemble returns a human-readable listing of the chunk.
func Disassemble(c *Chunk) string {
	var b strings.Builder
	fmt.Fprintf(&b, "; GLKB v%d source=%q consts=%d protos=%d main=%d\n",
		c.Version, c.SourceName, len(c.Consts), len(c.Protos), c.MainIndex)
	b.WriteString("; --- constants ---\n")
	for i, k := range c.Consts {
		fmt.Fprintf(&b, "K[%d] = %s\n", i, formatConst(k))
	}
	for pi, p := range c.Protos {
		mark := ""
		if pi == c.MainIndex {
			mark = "  ; <main>"
		}
		fmt.Fprintf(&b, "\n; --- proto %d %q regs=%d params=%d upvals=%d ---%s\n",
			pi, p.Name, p.NumRegs, p.NumParams, p.NumUpvals, mark)
		for ip, ins := range p.Code {
			fmt.Fprintf(&b, "%04d  %s\n", ip, FormatInstr(ins, c))
		}
	}
	return b.String()
}

// FormatInstr formats a single instruction.
func FormatInstr(ins uint32, c *Chunk) string {
	op := GetOp(ins)
	a, bb, cc := GetA(ins), GetB(ins), GetC(ins)
	bx := GetBx(ins)
	sbx := GetsBx(ins)
	name := op.Name()
	switch op {
	case OpMOVE, OpNOT, OpNEG, OpLNOT, OpLEN, OpTOSTR, OpTOINT, OpTYPEOF, OpISNULL,
		OpGETUPV, OpSETUPV, OpKEYS, OpDUP:
		return fmt.Sprintf("%-8s R%d, R%d", name, a, bb)
	case OpLOADK, OpLOADF, OpLOADG, OpSTOREG, OpIMPORT:
		k := ""
		if c != nil && int(bx) < len(c.Consts) {
			k = " ; " + formatConst(c.Consts[bx])
		}
		return fmt.Sprintf("%-8s R%d, K[%d]%s", name, a, bx, k)
	case OpMAKEFN:
		return fmt.Sprintf("%-8s R%d, proto[%d]", name, a, bx)
	case OpLOADI:
		return fmt.Sprintf("%-8s R%d, %d", name, a, sbx)
	case OpLOADB:
		return fmt.Sprintf("%-8s R%d, %v", name, a, bb != 0)
	case OpLOADN:
		return fmt.Sprintf("%-8s R%d", name, a)
	case OpNEWMAP:
		return fmt.Sprintf("%-8s R%d", name, a)
	case OpHALT, OpNOP, OpRETN:
		return name
	case OpJMP:
		return fmt.Sprintf("%-8s %+d", name, sbx)
	case OpJT, OpJF, OpTRY:
		return fmt.Sprintf("%-8s R%d, %+d", name, a, sbx)
	case OpENDTRY:
		return name
	case OpTHROW:
		return fmt.Sprintf("%-8s R%d", name, a)
	case OpCALL:
		return fmt.Sprintf("%-8s R%d, R%d, nargs=%d", name, a, bb, cc)
	case OpRET:
		return fmt.Sprintf("%-8s R%d", name, a)
	case OpNEWARR, OpLIST:
		return fmt.Sprintf("%-8s R%d, base=R%d, n=%d", name, a, bb, cc)
	case OpGETK:
		ks := ""
		if c != nil && int(cc) < len(c.Consts) {
			ks = " ; " + formatConst(c.Consts[cc])
		}
		return fmt.Sprintf("%-8s R%d, R%d, K[%d]%s", name, a, bb, cc, ks)
	case OpSETK:
		ks := ""
		if c != nil && int(bb) < len(c.Consts) {
			ks = " ; " + formatConst(c.Consts[bb])
		}
		return fmt.Sprintf("%-8s R%d, K[%d], R%d%s", name, a, bb, cc, ks)
	case OpBSLICE:
		return fmt.Sprintf("%-8s R%d, R%d, start=R%d end=R%d", name, a, bb, cc, cc+1)
	case OpCLOSURE:
		return fmt.Sprintf("%-8s R%d, proto=%d, nup=%d", name, a, bb, cc)
	case OpASSERT:
		return fmt.Sprintf("%-8s R%d, K[%d]", name, a, bb)
	default:
		return fmt.Sprintf("%-8s R%d, R%d, R%d", name, a, bb, cc)
	}
}

func formatConst(k Constant) string {
	switch k.Kind {
	case ConstNull:
		return "null"
	case ConstBool:
		return fmt.Sprintf("%v", k.Bool)
	case ConstInt:
		return fmt.Sprintf("%d", k.Int)
	case ConstFloat:
		return fmt.Sprintf("%g", k.Float)
	case ConstStr:
		return fmt.Sprintf("%q", k.Str)
	case ConstBytes:
		return fmt.Sprintf("bytes[%d]", len(k.Bytes))
	default:
		return "?"
	}
}
