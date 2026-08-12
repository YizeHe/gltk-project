package bytecode

import "fmt"

// ConstKind tags constant pool entries.
type ConstKind uint8

const (
	ConstNull ConstKind = iota
	ConstBool
	ConstInt
	ConstFloat
	ConstStr
	ConstBytes
)

// Constant is a pool entry.
type Constant struct {
	Kind  ConstKind
	Bool  bool
	Int   int64
	Float float64
	Str   string
	Bytes []byte
}

// Proto is a function prototype (code + metadata).
type Proto struct {
	Name      string
	NumRegs   uint8
	NumParams uint8
	NumUpvals uint8
	Code      []uint32
	// LineInfo optional: parallel to Code
	Lines []uint32
}

// Chunk is a full compiled unit (.glkb).
type Chunk struct {
	Version    uint16
	Consts     []Constant
	Protos     []*Proto
	MainIndex  int
	SourceName string
}

const (
	Magic   = "GLKB"
	Version = uint16(1)
)

// NewChunk creates an empty chunk at current version.
func NewChunk(source string) *Chunk {
	return &Chunk{
		Version:    Version,
		Consts:     nil,
		Protos:     nil,
		MainIndex:  0,
		SourceName: source,
	}
}

// AddConst appends a constant and returns its index.
func (c *Chunk) AddConst(k Constant) int {
	c.Consts = append(c.Consts, k)
	return len(c.Consts) - 1
}

// AddIntConst adds or reuses an int constant.
func (c *Chunk) AddIntConst(v int64) int {
	for i, k := range c.Consts {
		if k.Kind == ConstInt && k.Int == v {
			return i
		}
	}
	return c.AddConst(Constant{Kind: ConstInt, Int: v})
}

// AddStrConst adds or reuses a string constant.
func (c *Chunk) AddStrConst(s string) int {
	for i, k := range c.Consts {
		if k.Kind == ConstStr && k.Str == s {
			return i
		}
	}
	return c.AddConst(Constant{Kind: ConstStr, Str: s})
}

// AddFloatConst adds a float constant.
func (c *Chunk) AddFloatConst(f float64) int {
	for i, k := range c.Consts {
		if k.Kind == ConstFloat && k.Float == f {
			return i
		}
	}
	return c.AddConst(Constant{Kind: ConstFloat, Float: f})
}

// AddBoolConst adds a bool constant.
func (c *Chunk) AddBoolConst(b bool) int {
	for i, k := range c.Consts {
		if k.Kind == ConstBool && k.Bool == b {
			return i
		}
	}
	return c.AddConst(Constant{Kind: ConstBool, Bool: b})
}

// AddProto appends a proto and returns index.
func (c *Chunk) AddProto(p *Proto) int {
	c.Protos = append(c.Protos, p)
	return len(c.Protos) - 1
}

// String summary.
func (c *Chunk) String() string {
	return fmt.Sprintf("Chunk{ver=%d consts=%d protos=%d main=%d src=%q}",
		c.Version, len(c.Consts), len(c.Protos), c.MainIndex, c.SourceName)
}
