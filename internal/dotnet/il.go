package dotnet

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// method body flags
const (
	corILMethodTinyFormat = 0x2
	corILMethodFatFormat  = 0x3
	corILMethodMoreSects  = 0x8
	corILMethodInitLocals = 0x10
)

type methodBody struct {
	MaxStack    uint16
	CodeSize    uint32
	LocalVarSig uint32
	InitLocals  bool
	Code        []byte
	CodeRVA     uint32
}

func (a *Assembly) readMethodBody(rva uint32) (*methodBody, error) {
	off := a.pe.rva(rva)
	if off < 0 || off >= len(a.pe.Data) {
		return nil, fmt.Errorf("method RVA 0x%x not in image", rva)
	}
	data := a.pe.Data
	first := data[off]
	format := first & 0x3
	mb := &methodBody{CodeRVA: rva}

	if format == corILMethodTinyFormat {
		// bits 2-7 = code size
		mb.CodeSize = uint32(first >> 2)
		mb.MaxStack = 8
		codeOff := off + 1
		end := codeOff + int(mb.CodeSize)
		if end > len(data) {
			return nil, fmt.Errorf("tiny method body truncated")
		}
		mb.Code = data[codeOff:end]
		return mb, nil
	}
	if format != corILMethodFatFormat && (first&0x3) != 0x3 {
		// fat format: low 2 bits must be 3
		// some writers set flags differently; require flags&3==3 for fat
		if first&0x3 != 0x3 {
			return nil, fmt.Errorf("unknown method header format 0x%02x", first)
		}
	}
	// Fat header: 12 bytes
	// Flags/size: u16 (flags in low 12 bits, size in high 4 = header dwords)
	// MaxStack u16, CodeSize u32, LocalVarSigTok u32
	if off+12 > len(data) {
		return nil, fmt.Errorf("fat method header truncated")
	}
	flagsSize := binary.LittleEndian.Uint16(data[off:])
	flags := flagsSize & 0x0FFF
	// headerSizeDwords := flagsSize >> 12  // normally 3
	mb.MaxStack = binary.LittleEndian.Uint16(data[off+2:])
	mb.CodeSize = binary.LittleEndian.Uint32(data[off+4:])
	mb.LocalVarSig = binary.LittleEndian.Uint32(data[off+8:])
	mb.InitLocals = flags&corILMethodInitLocals != 0
	codeOff := off + 12
	end := codeOff + int(mb.CodeSize)
	if end > len(data) {
		return nil, fmt.Errorf("fat method body truncated (need %d)", end)
	}
	mb.Code = data[codeOff:end]
	return mb, nil
}

func (a *Assembly) disassemble(m Method) (string, error) {
	mb, err := a.readMethodBody(m.RVA)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "// %s::%s  RVA=0x%X  code=%d  maxstack=%d\n",
		m.TypeName, m.Name, m.RVA, mb.CodeSize, mb.MaxStack)
	code := mb.Code
	ip := 0
	for ip < len(code) {
		start := ip
		line, next, err := a.decodeOne(code, ip)
		if err != nil {
			fmt.Fprintf(&b, "IL_%04X: // decode error at +%d: %v\n", start, start, err)
			break
		}
		fmt.Fprintf(&b, "IL_%04X: %s\n", start, line)
		ip = next
	}
	return b.String(), nil
}

// operand kinds
const (
	opNone = iota
	opInt8
	opUInt8
	opInt16
	opInt32
	opInt64
	opFloat32
	opFloat64
	opToken
	opTarget32 // branch relative int32
	opTarget8  // branch relative int8
	opSwitch
	opVarU8   // short local/arg index
	opVarU16  // fat local/arg index
)

type opcodeInfo struct {
	Name string
	Kind int
	Size int // opcode size in bytes (1 or 2)
}

// single-byte opcodes (index = opcode byte)
var opcodes [256]opcodeInfo

// two-byte opcodes after 0xFE prefix
var opcodesFE [256]opcodeInfo

func init() {
	// defaults
	for i := range opcodes {
		opcodes[i] = opcodeInfo{Name: fmt.Sprintf("unknown_0x%02X", i), Kind: opNone, Size: 1}
	}
	for i := range opcodesFE {
		opcodesFE[i] = opcodeInfo{Name: fmt.Sprintf("unknown_fe_%02X", i), Kind: opNone, Size: 2}
	}

	def := func(op byte, name string, kind int) {
		opcodes[op] = opcodeInfo{Name: name, Kind: kind, Size: 1}
	}
	defFE := func(op byte, name string, kind int) {
		opcodesFE[op] = opcodeInfo{Name: name, Kind: kind, Size: 2}
	}

	// constants / loads
	def(0x00, "nop", opNone)
	def(0x01, "break", opNone)
	def(0x02, "ldarg.0", opNone)
	def(0x03, "ldarg.1", opNone)
	def(0x04, "ldarg.2", opNone)
	def(0x05, "ldarg.3", opNone)
	def(0x06, "ldloc.0", opNone)
	def(0x07, "ldloc.1", opNone)
	def(0x08, "ldloc.2", opNone)
	def(0x09, "ldloc.3", opNone)
	def(0x0A, "stloc.0", opNone)
	def(0x0B, "stloc.1", opNone)
	def(0x0C, "stloc.2", opNone)
	def(0x0D, "stloc.3", opNone)
	def(0x0E, "ldarg.s", opVarU8)
	def(0x0F, "ldarga.s", opVarU8)
	def(0x10, "starg.s", opVarU8)
	def(0x11, "ldloc.s", opVarU8)
	def(0x12, "ldloca.s", opVarU8)
	def(0x13, "stloc.s", opVarU8)
	def(0x14, "ldnull", opNone)
	def(0x15, "ldc.i4.m1", opNone)
	def(0x16, "ldc.i4.0", opNone)
	def(0x17, "ldc.i4.1", opNone)
	def(0x18, "ldc.i4.2", opNone)
	def(0x19, "ldc.i4.3", opNone)
	def(0x1A, "ldc.i4.4", opNone)
	def(0x1B, "ldc.i4.5", opNone)
	def(0x1C, "ldc.i4.6", opNone)
	def(0x1D, "ldc.i4.7", opNone)
	def(0x1E, "ldc.i4.8", opNone)
	def(0x1F, "ldc.i4.s", opInt8)
	def(0x20, "ldc.i4", opInt32)
	def(0x21, "ldc.i8", opInt64)
	def(0x22, "ldc.r4", opFloat32)
	def(0x23, "ldc.r8", opFloat64)
	def(0x25, "dup", opNone)
	def(0x26, "pop", opNone)
	def(0x27, "jmp", opToken)
	def(0x28, "call", opToken)
	def(0x29, "calli", opToken)
	def(0x2A, "ret", opNone)
	def(0x2B, "br.s", opTarget8)
	def(0x2C, "brfalse.s", opTarget8)
	def(0x2D, "brtrue.s", opTarget8)
	def(0x2E, "beq.s", opTarget8)
	def(0x2F, "bge.s", opTarget8)
	def(0x30, "bgt.s", opTarget8)
	def(0x31, "ble.s", opTarget8)
	def(0x32, "blt.s", opTarget8)
	def(0x33, "bne.un.s", opTarget8)
	def(0x34, "bge.un.s", opTarget8)
	def(0x35, "bgt.un.s", opTarget8)
	def(0x36, "ble.un.s", opTarget8)
	def(0x37, "blt.un.s", opTarget8)
	def(0x38, "br", opTarget32)
	def(0x39, "brfalse", opTarget32)
	def(0x3A, "brtrue", opTarget32)
	def(0x3B, "beq", opTarget32)
	def(0x3C, "bge", opTarget32)
	def(0x3D, "bgt", opTarget32)
	def(0x3E, "ble", opTarget32)
	def(0x3F, "blt", opTarget32)
	def(0x40, "bne.un", opTarget32)
	def(0x41, "bge.un", opTarget32)
	def(0x42, "bgt.un", opTarget32)
	def(0x43, "ble.un", opTarget32)
	def(0x44, "blt.un", opTarget32)
	def(0x45, "switch", opSwitch)
	def(0x46, "ldind.i1", opNone)
	def(0x47, "ldind.u1", opNone)
	def(0x48, "ldind.i2", opNone)
	def(0x49, "ldind.u2", opNone)
	def(0x4A, "ldind.i4", opNone)
	def(0x4B, "ldind.u4", opNone)
	def(0x4C, "ldind.i8", opNone)
	def(0x4D, "ldind.i", opNone)
	def(0x4E, "ldind.r4", opNone)
	def(0x4F, "ldind.r8", opNone)
	def(0x50, "ldind.ref", opNone)
	def(0x51, "stind.ref", opNone)
	def(0x52, "stind.i1", opNone)
	def(0x53, "stind.i2", opNone)
	def(0x54, "stind.i4", opNone)
	def(0x55, "stind.i8", opNone)
	def(0x56, "stind.r4", opNone)
	def(0x57, "stind.r8", opNone)
	def(0x58, "add", opNone)
	def(0x59, "sub", opNone)
	def(0x5A, "mul", opNone)
	def(0x5B, "div", opNone)
	def(0x5C, "div.un", opNone)
	def(0x5D, "rem", opNone)
	def(0x5E, "rem.un", opNone)
	def(0x5F, "and", opNone)
	def(0x60, "or", opNone)
	def(0x61, "xor", opNone)
	def(0x62, "shl", opNone)
	def(0x63, "shr", opNone)
	def(0x64, "shr.un", opNone)
	def(0x65, "neg", opNone)
	def(0x66, "not", opNone)
	def(0x67, "conv.i1", opNone)
	def(0x68, "conv.i2", opNone)
	def(0x69, "conv.i4", opNone)
	def(0x6A, "conv.i8", opNone)
	def(0x6B, "conv.r4", opNone)
	def(0x6C, "conv.r8", opNone)
	def(0x6D, "conv.u4", opNone)
	def(0x6E, "conv.u8", opNone)
	def(0x6F, "callvirt", opToken)
	def(0x70, "cpobj", opToken)
	def(0x71, "ldobj", opToken)
	def(0x72, "ldstr", opToken)
	def(0x73, "newobj", opToken)
	def(0x74, "castclass", opToken)
	def(0x75, "isinst", opToken)
	def(0x76, "conv.r.un", opNone)
	def(0x79, "unbox", opToken)
	def(0x7A, "throw", opNone)
	def(0x7B, "ldfld", opToken)
	def(0x7C, "ldflda", opToken)
	def(0x7D, "stfld", opToken)
	def(0x7E, "ldsfld", opToken)
	def(0x7F, "ldsflda", opToken)
	def(0x80, "stsfld", opToken)
	def(0x81, "stobj", opToken)
	def(0x82, "conv.ovf.i1.un", opNone)
	def(0x83, "conv.ovf.i2.un", opNone)
	def(0x84, "conv.ovf.i4.un", opNone)
	def(0x85, "conv.ovf.i8.un", opNone)
	def(0x86, "conv.ovf.u1.un", opNone)
	def(0x87, "conv.ovf.u2.un", opNone)
	def(0x88, "conv.ovf.u4.un", opNone)
	def(0x89, "conv.ovf.u8.un", opNone)
	def(0x8A, "conv.ovf.i.un", opNone)
	def(0x8B, "conv.ovf.u.un", opNone)
	def(0x8C, "box", opToken)
	def(0x8D, "newarr", opToken)
	def(0x8E, "ldlen", opNone)
	def(0x8F, "ldelema", opToken)
	def(0x90, "ldelem.i1", opNone)
	def(0x91, "ldelem.u1", opNone)
	def(0x92, "ldelem.i2", opNone)
	def(0x93, "ldelem.u2", opNone)
	def(0x94, "ldelem.i4", opNone)
	def(0x95, "ldelem.u4", opNone)
	def(0x96, "ldelem.i8", opNone)
	def(0x97, "ldelem.i", opNone)
	def(0x98, "ldelem.r4", opNone)
	def(0x99, "ldelem.r8", opNone)
	def(0x9A, "ldelem.ref", opNone)
	def(0x9B, "stelem.i", opNone)
	def(0x9C, "stelem.i1", opNone)
	def(0x9D, "stelem.i2", opNone)
	def(0x9E, "stelem.i4", opNone)
	def(0x9F, "stelem.i8", opNone)
	def(0xA0, "stelem.r4", opNone)
	def(0xA1, "stelem.r8", opNone)
	def(0xA2, "stelem.ref", opNone)
	def(0xA3, "ldelem", opToken)
	def(0xA4, "stelem", opToken)
	def(0xA5, "unbox.any", opToken)
	def(0xB3, "conv.ovf.i1", opNone)
	def(0xB4, "conv.ovf.u1", opNone)
	def(0xB5, "conv.ovf.i2", opNone)
	def(0xB6, "conv.ovf.u2", opNone)
	def(0xB7, "conv.ovf.i4", opNone)
	def(0xB8, "conv.ovf.u4", opNone)
	def(0xB9, "conv.ovf.i8", opNone)
	def(0xBA, "conv.ovf.u8", opNone)
	def(0xC2, "refanyval", opToken)
	def(0xC3, "ckfinite", opNone)
	def(0xC6, "mkrefany", opToken)
	def(0xD0, "ldtoken", opToken)
	def(0xD1, "conv.u2", opNone)
	def(0xD2, "conv.u1", opNone)
	def(0xD3, "conv.i", opNone)
	def(0xD4, "conv.ovf.i", opNone)
	def(0xD5, "conv.ovf.u", opNone)
	def(0xD6, "add.ovf", opNone)
	def(0xD7, "add.ovf.un", opNone)
	def(0xD8, "mul.ovf", opNone)
	def(0xD9, "mul.ovf.un", opNone)
	def(0xDA, "sub.ovf", opNone)
	def(0xDB, "sub.ovf.un", opNone)
	def(0xDC, "endfinally", opNone)
	def(0xDD, "leave", opTarget32)
	def(0xDE, "leave.s", opTarget8)
	def(0xDF, "stind.i", opNone)
	def(0xE0, "conv.u", opNone)

	// 0xFE prefix
	defFE(0x00, "arglist", opNone)
	defFE(0x01, "ceq", opNone)
	defFE(0x02, "cgt", opNone)
	defFE(0x03, "cgt.un", opNone)
	defFE(0x04, "clt", opNone)
	defFE(0x05, "clt.un", opNone)
	defFE(0x06, "ldftn", opToken)
	defFE(0x07, "ldvirtftn", opToken)
	defFE(0x09, "ldarg", opVarU16)
	defFE(0x0A, "ldarga", opVarU16)
	defFE(0x0B, "starg", opVarU16)
	defFE(0x0C, "ldloc", opVarU16)
	defFE(0x0D, "ldloca", opVarU16)
	defFE(0x0E, "stloc", opVarU16)
	defFE(0x0F, "localloc", opNone)
	defFE(0x11, "endfilter", opNone)
	defFE(0x12, "unaligned.", opUInt8) // prefix
	defFE(0x13, "volatile.", opNone)   // prefix
	defFE(0x14, "tail.", opNone)       // prefix
	defFE(0x15, "initobj", opToken)
	defFE(0x16, "constrained.", opToken) // prefix
	defFE(0x17, "cpblk", opNone)
	defFE(0x18, "initblk", opNone)
	defFE(0x1A, "rethrow", opNone)
	defFE(0x1C, "sizeof", opToken)
	defFE(0x1D, "refanytype", opNone)
	defFE(0x1E, "readonly.", opNone) // prefix
}

func (a *Assembly) decodeOne(code []byte, ip int) (line string, next int, err error) {
	if ip >= len(code) {
		return "", ip, fmt.Errorf("EOF")
	}
	var info opcodeInfo
	opStart := ip
	if code[ip] == 0xFE {
		if ip+1 >= len(code) {
			return "", ip, fmt.Errorf("truncated FE prefix")
		}
		info = opcodesFE[code[ip+1]]
		ip += 2
	} else {
		info = opcodes[code[ip]]
		ip++
	}

	switch info.Kind {
	case opNone:
		return info.Name, ip, nil
	case opInt8:
		if ip >= len(code) {
			return "", opStart, fmt.Errorf("truncated int8")
		}
		v := int8(code[ip])
		ip++
		return fmt.Sprintf("%s %d", info.Name, v), ip, nil
	case opUInt8:
		if ip >= len(code) {
			return "", opStart, fmt.Errorf("truncated uint8")
		}
		v := code[ip]
		ip++
		return fmt.Sprintf("%s %d", info.Name, v), ip, nil
	case opInt16:
		if ip+2 > len(code) {
			return "", opStart, fmt.Errorf("truncated int16")
		}
		v := int16(binary.LittleEndian.Uint16(code[ip:]))
		ip += 2
		return fmt.Sprintf("%s %d", info.Name, v), ip, nil
	case opInt32:
		if ip+4 > len(code) {
			return "", opStart, fmt.Errorf("truncated int32")
		}
		v := int32(binary.LittleEndian.Uint32(code[ip:]))
		ip += 4
		return fmt.Sprintf("%s %d", info.Name, v), ip, nil
	case opInt64:
		if ip+8 > len(code) {
			return "", opStart, fmt.Errorf("truncated int64")
		}
		v := int64(binary.LittleEndian.Uint64(code[ip:]))
		ip += 8
		return fmt.Sprintf("%s %d", info.Name, v), ip, nil
	case opFloat32:
		if ip+4 > len(code) {
			return "", opStart, fmt.Errorf("truncated r4")
		}
		bits := binary.LittleEndian.Uint32(code[ip:])
		ip += 4
		return fmt.Sprintf("%s 0x%08X", info.Name, bits), ip, nil
	case opFloat64:
		if ip+8 > len(code) {
			return "", opStart, fmt.Errorf("truncated r8")
		}
		bits := binary.LittleEndian.Uint64(code[ip:])
		ip += 8
		return fmt.Sprintf("%s 0x%016X", info.Name, bits), ip, nil
	case opToken:
		if ip+4 > len(code) {
			return "", opStart, fmt.Errorf("truncated token")
		}
		tok := binary.LittleEndian.Uint32(code[ip:])
		ip += 4
		resolved := a.ResolveToken(tok)
		// ildasm-ish: ldstr "foo"  or call Class::Method
		if info.Name == "ldstr" {
			// ResolveToken already quotes user strings
			return fmt.Sprintf("%s %s", info.Name, resolved), ip, nil
		}
		return fmt.Sprintf("%s %s /* 0x%08X */", info.Name, resolved, tok), ip, nil
	case opTarget8:
		if ip >= len(code) {
			return "", opStart, fmt.Errorf("truncated br.s")
		}
		rel := int8(code[ip])
		ip++
		target := ip + int(rel)
		return fmt.Sprintf("%s IL_%04X", info.Name, target), ip, nil
	case opTarget32:
		if ip+4 > len(code) {
			return "", opStart, fmt.Errorf("truncated br")
		}
		rel := int32(binary.LittleEndian.Uint32(code[ip:]))
		ip += 4
		target := ip + int(rel)
		return fmt.Sprintf("%s IL_%04X", info.Name, target), ip, nil
	case opSwitch:
		if ip+4 > len(code) {
			return "", opStart, fmt.Errorf("truncated switch count")
		}
		n := binary.LittleEndian.Uint32(code[ip:])
		ip += 4
		base := ip + int(n)*4
		var parts []string
		for i := uint32(0); i < n; i++ {
			if ip+4 > len(code) {
				return "", opStart, fmt.Errorf("truncated switch target")
			}
			rel := int32(binary.LittleEndian.Uint32(code[ip:]))
			ip += 4
			parts = append(parts, fmt.Sprintf("IL_%04X", base+int(rel)))
		}
		return fmt.Sprintf("%s (%s)", info.Name, strings.Join(parts, ", ")), ip, nil
	case opVarU8:
		if ip >= len(code) {
			return "", opStart, fmt.Errorf("truncated var.u8")
		}
		v := code[ip]
		ip++
		return fmt.Sprintf("%s V_%d", info.Name, v), ip, nil
	case opVarU16:
		if ip+2 > len(code) {
			return "", opStart, fmt.Errorf("truncated var.u16")
		}
		v := binary.LittleEndian.Uint16(code[ip:])
		ip += 2
		return fmt.Sprintf("%s V_%d", info.Name, v), ip, nil
	default:
		return info.Name, ip, nil
	}
}
