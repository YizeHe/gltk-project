// Package obfus applies strong bytecode-level obfuscation to GLKB chunks.
package obfus

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"groklang/gltk/internal/bytecode"
)

// Options controls obfuscation intensity.
type Options struct {
	NOPDensity    int  // max NOPs before each insn (default 3)
	JunkBlocks    int  // dead unreachable fragments density (default 4)
	ShuffleConsts bool // permute constant pool
	StripMeta     bool // wipe source/proto names and line tables
	BogusConsts   bool // pad pool with fake strings
}

// Default aggressive packer defaults.
func Default() Options {
	return Options{
		NOPDensity:    3,
		JunkBlocks:    4,
		ShuffleConsts: true,
		StripMeta:     true,
		BogusConsts:   true,
	}
}

// Apply returns an obfuscated deep copy of src.
func Apply(src *bytecode.Chunk, opt Options) (*bytecode.Chunk, error) {
	if src == nil {
		return nil, fmt.Errorf("obfus: nil chunk")
	}
	c := cloneChunk(src)
	if opt.StripMeta {
		c.SourceName = randomName("g", 14)
		for i, p := range c.Protos {
			if p == nil {
				continue
			}
			// Empty names: VM only exports named protos as globals; main is by index.
			p.Name = ""
			if i != c.MainIndex {
				p.Name = randomName("f", 12)
			}
			p.Lines = nil
		}
	}
	if opt.BogusConsts {
		padBogusConsts(c, 24+int(randByte()%40))
	}
	if opt.ShuffleConsts {
		if err := shuffleConsts(c); err != nil {
			return nil, err
		}
	}
	for pi, p := range c.Protos {
		if p == nil {
			continue
		}
		np, err := expandProto(p, opt)
		if err != nil {
			return nil, fmt.Errorf("proto %d: %w", pi, err)
		}
		c.Protos[pi] = np
	}
	return c, nil
}

func cloneChunk(src *bytecode.Chunk) *bytecode.Chunk {
	c := &bytecode.Chunk{
		Version:    src.Version,
		SourceName: src.SourceName,
		MainIndex:  src.MainIndex,
		Consts:     make([]bytecode.Constant, len(src.Consts)),
		Protos:     make([]*bytecode.Proto, len(src.Protos)),
	}
	copy(c.Consts, src.Consts)
	for i, p := range src.Protos {
		if p == nil {
			continue
		}
		np := *p
		np.Code = append([]uint32(nil), p.Code...)
		if p.Lines != nil {
			np.Lines = append([]uint32(nil), p.Lines...)
		}
		c.Protos[i] = &np
	}
	return c
}

func padBogusConsts(c *bytecode.Chunk, n int) {
	for i := 0; i < n; i++ {
		c.Consts = append(c.Consts, bytecode.Constant{
			Kind: bytecode.ConstStr,
			Str:  randomName("k", 28),
		})
	}
}

func shuffleConsts(c *bytecode.Chunk) error {
	n := len(c.Consts)
	if n <= 1 {
		return nil
	}
	// Keep GETK/SETK safe: if any const index used in 8-bit field >255 after shuffle, only shuffle within 0..255
	// Simple approach: full shuffle but reject if any GETK/SETK/ASSERT would need >255 (pool can grow with bogus).
	perm := randPerm(n)
	oldToNew := make([]int, n)
	for newI, oldI := range perm {
		oldToNew[oldI] = newI
	}
	// verify 8-bit const ops
	for _, p := range c.Protos {
		if p == nil {
			continue
		}
		for _, ins := range p.Code {
			op := bytecode.GetOp(ins)
			switch op {
			case bytecode.OpGETK:
				cc := int(bytecode.GetC(ins))
				if cc < n && oldToNew[cc] > 255 {
					return nil // skip shuffle if unsafe — still have other obfus
				}
			case bytecode.OpSETK, bytecode.OpASSERT:
				bb := int(bytecode.GetB(ins))
				if bb < n && oldToNew[bb] > 255 {
					return nil
				}
			}
		}
	}
	newConsts := make([]bytecode.Constant, n)
	for newI, oldI := range perm {
		newConsts[newI] = c.Consts[oldI]
	}
	c.Consts = newConsts
	for _, p := range c.Protos {
		if p == nil {
			continue
		}
		for i, ins := range p.Code {
			p.Code[i] = remapConstInstr(ins, oldToNew)
		}
	}
	return nil
}

func remapConstInstr(ins uint32, oldToNew []int) uint32 {
	op := bytecode.GetOp(ins)
	a := bytecode.GetA(ins)
	n := len(oldToNew)
	switch op {
	case bytecode.OpLOADK, bytecode.OpLOADF, bytecode.OpLOADG, bytecode.OpSTOREG, bytecode.OpIMPORT:
		bx := int(bytecode.GetBx(ins))
		if bx >= 0 && bx < n {
			return bytecode.MakeABx(op, a, uint16(oldToNew[bx]))
		}
	case bytecode.OpGETK:
		b, cc := bytecode.GetB(ins), int(bytecode.GetC(ins))
		if cc >= 0 && cc < n {
			ni := oldToNew[cc]
			if ni <= 255 {
				return bytecode.MakeABC(op, a, b, uint8(ni))
			}
		}
	case bytecode.OpSETK:
		bb, c := int(bytecode.GetB(ins)), bytecode.GetC(ins)
		if bb >= 0 && bb < n {
			ni := oldToNew[bb]
			if ni <= 255 {
				return bytecode.MakeABC(op, a, uint8(ni), c)
			}
		}
	case bytecode.OpASSERT:
		bb := int(bytecode.GetB(ins))
		if bb >= 0 && bb < n {
			ni := oldToNew[bb]
			if ni <= 255 {
				return bytecode.MakeABC(op, a, uint8(ni), bytecode.GetC(ins))
			}
		}
	}
	return ins
}

// expandProto inserts NOPs/junk and rewrites relative jumps.
// VM model: fetch increments ip first, then JMP does ip += sBx.
// So from instruction at ip, target = ip+1+sBx, thus sBx = target-ip-1.
func expandProto(p *bytecode.Proto, opt Options) (*bytecode.Proto, error) {
	src := p.Code
	n := len(src)
	if n == 0 {
		out := *p
		out.Lines = nil
		return &out, nil
	}

	// pieces: expanded stream with markers for original IPs
	type piece struct {
		origIP int // >=0 if this slot holds original insn from origIP
		ins    uint32
	}
	var pieces []piece
	for i, ins := range src {
		// leading NOPs
		nd := 0
		if opt.NOPDensity > 0 {
			nd = int(randByte()) % (opt.NOPDensity + 1)
		}
		for j := 0; j < nd; j++ {
			pieces = append(pieces, piece{origIP: -1, ins: bytecode.MakeABC(bytecode.OpNOP, 0, 0, 0)})
		}
		// junk: JMP over dead region
		if opt.JunkBlocks > 0 && int(randByte())%4 == 0 {
			deadN := 1 + int(randByte())%4
			pieces = append(pieces, piece{origIP: -1, ins: bytecode.MakeAsBx(bytecode.OpJMP, 0, int16(deadN))})
			for j := 0; j < deadN; j++ {
				pieces = append(pieces, piece{origIP: -1, ins: junkInsn()})
			}
		}
		pieces = append(pieces, piece{origIP: i, ins: ins})
	}
	// tail padding
	for j := 0; j < 1+int(randByte())%3; j++ {
		pieces = append(pieces, piece{origIP: -1, ins: bytecode.MakeABC(bytecode.OpNOP, 0, 0, 0)})
	}

	oldToNew := make([]int, n)
	for i := range oldToNew {
		oldToNew[i] = -1
	}
	newCode := make([]uint32, len(pieces))
	for ni, pc := range pieces {
		if pc.origIP >= 0 {
			oldToNew[pc.origIP] = ni
		}
		newCode[ni] = pc.ins
	}
	endIP := len(newCode)

	// rewrite original jumps
	for oldIP := 0; oldIP < n; oldIP++ {
		newIP := oldToNew[oldIP]
		if newIP < 0 {
			continue
		}
		ins := src[oldIP]
		op := bytecode.GetOp(ins)
		if op != bytecode.OpJMP && op != bytecode.OpJT && op != bytecode.OpJF {
			newCode[newIP] = ins
			continue
		}
		sbx := bytecode.GetsBx(ins)
		target := oldIP + 1 + int(sbx)
		var newTarget int
		if target < 0 {
			newTarget = 0
		} else if target >= n {
			newTarget = endIP
		} else {
			newTarget = oldToNew[target]
			if newTarget < 0 {
				newTarget = endIP
			}
		}
		newSbx := newTarget - newIP - 1
		if newSbx < -32768 || newSbx > 32767 {
			return nil, fmt.Errorf("jump overflow oldIP=%d", oldIP)
		}
		newCode[newIP] = bytecode.MakeAsBx(op, bytecode.GetA(ins), int16(newSbx))
	}

	// dead tail junk (unreachable if code always returns)
	if opt.JunkBlocks > 0 {
		for j := 0; j < opt.JunkBlocks*2; j++ {
			newCode = append(newCode, junkInsn())
		}
	}

	out := *p
	out.Code = newCode
	out.Lines = nil
	if out.NumRegs < 8 {
		out.NumRegs = 8
	}
	return &out, nil
}

func junkInsn() uint32 {
	// Never emit control-flow in dead regions (would need paired targets).
	switch randByte() % 5 {
	case 0:
		return bytecode.MakeABC(bytecode.OpNOP, 0, 0, 0)
	case 1:
		return bytecode.MakeABC(bytecode.OpLOADN, uint8(randByte()%4), 0, 0)
	case 2:
		return bytecode.MakeABC(bytecode.OpLOADB, uint8(randByte()%4), randByte()&1, 0)
	case 3:
		return bytecode.MakeAsBx(bytecode.OpLOADI, uint8(randByte()%4), int16(randByte()))
	default:
		return bytecode.MakeABC(bytecode.OpMOVE, uint8(randByte()%4), uint8(randByte()%4), 0)
	}
}

func randPerm(n int) []int {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	for i := n - 1; i > 0; i-- {
		jBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		j := 0
		if err == nil {
			j = int(jBig.Int64())
		}
		p[i], p[j] = p[j], p[i]
	}
	return p
}

func randByte() byte {
	var b [1]byte
	_, _ = rand.Read(b[:])
	return b[0]
}

func randomName(prefix string, n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = alphabet[int(randByte())%len(alphabet)]
	}
	return prefix + "_" + string(buf)
}
