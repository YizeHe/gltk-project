package bytecode

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// Encode serializes chunk to GLKB binary format.
//
//	magic[4] "GLKB"
//	version u16 LE
//	source_name: u32 len + bytes
//	nconst u32
//	  each: kind u8 + payload
//	nproto u32
//	  each: name, numregs u8, numparams u8, numupvals u8, ncode u32, code[]u32 LE, nlines u32, lines[]u32
//	main_index u32
func Encode(c *Chunk) ([]byte, error) {
	var b []byte
	b = append(b, []byte(Magic)...)
	b = appendU16(b, c.Version)
	b = appendString(b, c.SourceName)
	b = appendU32(b, uint32(len(c.Consts)))
	for _, k := range c.Consts {
		b = append(b, byte(k.Kind))
		switch k.Kind {
		case ConstNull:
		case ConstBool:
			if k.Bool {
				b = append(b, 1)
			} else {
				b = append(b, 0)
			}
		case ConstInt:
			b = appendI64(b, k.Int)
		case ConstFloat:
			b = appendU64(b, math.Float64bits(k.Float))
		case ConstStr:
			b = appendString(b, k.Str)
		case ConstBytes:
			b = appendU32(b, uint32(len(k.Bytes)))
			b = append(b, k.Bytes...)
		default:
			return nil, fmt.Errorf("unknown const kind %d", k.Kind)
		}
	}
	b = appendU32(b, uint32(len(c.Protos)))
	for _, p := range c.Protos {
		if p == nil {
			return nil, fmt.Errorf("nil proto")
		}
		b = appendString(b, p.Name)
		b = append(b, p.NumRegs, p.NumParams, p.NumUpvals)
		b = appendU32(b, uint32(len(p.Code)))
		for _, ins := range p.Code {
			tmp := make([]byte, 4)
			binary.LittleEndian.PutUint32(tmp, ins)
			b = append(b, tmp...)
		}
		b = appendU32(b, uint32(len(p.Lines)))
		for _, ln := range p.Lines {
			b = appendU32(b, ln)
		}
	}
	b = appendU32(b, uint32(c.MainIndex))
	return b, nil
}

// Decode reads a GLKB chunk from bytes.
func Decode(data []byte) (*Chunk, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("glkb: too short")
	}
	if string(data[:4]) != Magic {
		return nil, fmt.Errorf("glkb: bad magic %q", data[:4])
	}
	r := &reader{b: data, i: 4}
	ver, err := r.u16()
	if err != nil {
		return nil, err
	}
	src, err := r.str()
	if err != nil {
		return nil, err
	}
	nconst, err := r.u32()
	if err != nil {
		return nil, err
	}
	c := &Chunk{Version: ver, SourceName: src}
	for i := uint32(0); i < nconst; i++ {
		kind, err := r.u8()
		if err != nil {
			return nil, err
		}
		k := Constant{Kind: ConstKind(kind)}
		switch ConstKind(kind) {
		case ConstNull:
		case ConstBool:
			v, err := r.u8()
			if err != nil {
				return nil, err
			}
			k.Bool = v != 0
		case ConstInt:
			v, err := r.i64()
			if err != nil {
				return nil, err
			}
			k.Int = v
		case ConstFloat:
			bits, err := r.u64()
			if err != nil {
				return nil, err
			}
			k.Float = math.Float64frombits(bits)
		case ConstStr:
			s, err := r.str()
			if err != nil {
				return nil, err
			}
			k.Str = s
		case ConstBytes:
			n, err := r.u32()
			if err != nil {
				return nil, err
			}
			bs, err := r.bytes(int(n))
			if err != nil {
				return nil, err
			}
			k.Bytes = bs
		default:
			return nil, fmt.Errorf("glkb: unknown const kind %d", kind)
		}
		c.Consts = append(c.Consts, k)
	}
	nproto, err := r.u32()
	if err != nil {
		return nil, err
	}
	for i := uint32(0); i < nproto; i++ {
		name, err := r.str()
		if err != nil {
			return nil, err
		}
		nr, err := r.u8()
		if err != nil {
			return nil, err
		}
		np, err := r.u8()
		if err != nil {
			return nil, err
		}
		nu, err := r.u8()
		if err != nil {
			return nil, err
		}
		ncode, err := r.u32()
		if err != nil {
			return nil, err
		}
		code := make([]uint32, ncode)
		for j := uint32(0); j < ncode; j++ {
			ins, err := r.u32()
			if err != nil {
				return nil, err
			}
			code[j] = ins
		}
		nlines, err := r.u32()
		if err != nil {
			return nil, err
		}
		var lines []uint32
		for j := uint32(0); j < nlines; j++ {
			ln, err := r.u32()
			if err != nil {
				return nil, err
			}
			lines = append(lines, ln)
		}
		c.Protos = append(c.Protos, &Proto{
			Name: name, NumRegs: nr, NumParams: np, NumUpvals: nu,
			Code: code, Lines: lines,
		})
	}
	main, err := r.u32()
	if err != nil {
		return nil, err
	}
	c.MainIndex = int(main)
	return c, nil
}

// Write encodes to w.
func Write(w io.Writer, c *Chunk) error {
	data, err := Encode(c)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// Read decodes from r (reads all).
func Read(r io.Reader) (*Chunk, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return Decode(data)
}

type reader struct {
	b []byte
	i int
}

func (r *reader) need(n int) error {
	if r.i+n > len(r.b) {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (r *reader) u8() (uint8, error) {
	if err := r.need(1); err != nil {
		return 0, err
	}
	v := r.b[r.i]
	r.i++
	return v, nil
}
func (r *reader) u16() (uint16, error) {
	if err := r.need(2); err != nil {
		return 0, err
	}
	v := binary.LittleEndian.Uint16(r.b[r.i:])
	r.i += 2
	return v, nil
}
func (r *reader) u32() (uint32, error) {
	if err := r.need(4); err != nil {
		return 0, err
	}
	v := binary.LittleEndian.Uint32(r.b[r.i:])
	r.i += 4
	return v, nil
}
func (r *reader) u64() (uint64, error) {
	if err := r.need(8); err != nil {
		return 0, err
	}
	v := binary.LittleEndian.Uint64(r.b[r.i:])
	r.i += 8
	return v, nil
}
func (r *reader) i64() (int64, error) {
	v, err := r.u64()
	return int64(v), err
}
func (r *reader) bytes(n int) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("negative length")
	}
	if err := r.need(n); err != nil {
		return nil, err
	}
	out := make([]byte, n)
	copy(out, r.b[r.i:r.i+n])
	r.i += n
	return out, nil
}
func (r *reader) str() (string, error) {
	n, err := r.u32()
	if err != nil {
		return "", err
	}
	bs, err := r.bytes(int(n))
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

func appendU16(b []byte, v uint16) []byte {
	tmp := make([]byte, 2)
	binary.LittleEndian.PutUint16(tmp, v)
	return append(b, tmp...)
}
func appendU32(b []byte, v uint32) []byte {
	tmp := make([]byte, 4)
	binary.LittleEndian.PutUint32(tmp, v)
	return append(b, tmp...)
}
func appendI64(b []byte, v int64) []byte {
	tmp := make([]byte, 8)
	binary.LittleEndian.PutUint64(tmp, uint64(v))
	return append(b, tmp...)
}
func appendU64(b []byte, v uint64) []byte {
	tmp := make([]byte, 8)
	binary.LittleEndian.PutUint64(tmp, v)
	return append(b, tmp...)
}
func appendString(b []byte, s string) []byte {
	b = appendU32(b, uint32(len(s)))
	return append(b, s...)
}
