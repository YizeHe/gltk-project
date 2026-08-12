package native

import (
	"encoding/hex"
	"strings"

	"golang.org/x/arch/arm64/arm64asm"
	"golang.org/x/arch/x86/x86asm"

	"groklang/gltk/internal/vm"
)

const (
	disasmDefaultCount = 100
	disasmMaxCount     = 10000
)

func moduleDisasm() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"disasm":     disasmMany,
		"disasm_one": disasmOne,
	})
}

// disasm.disasm(bytes, arch, offset?, count?) -> [{addr, bytes, mnemonic, op_str}, ...]
func disasmMany(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("disasm.disasm(bytes, arch, offset?, count?)")
	}
	data, err := args[0].AsBytes()
	if err != nil {
		return vm.Null(), err
	}
	arch, mode, err := disasmParseArch(args[1].AsStr())
	if err != nil {
		return vm.Null(), err
	}
	offset := int64(0)
	if len(args) >= 3 {
		offset, err = args[2].AsInt()
		if err != nil {
			return vm.Null(), err
		}
	}
	count := int64(disasmDefaultCount)
	if len(args) >= 4 {
		count, err = args[3].AsInt()
		if err != nil {
			return vm.Null(), err
		}
	}
	if count < 0 {
		count = 0
	}
	if count > disasmMaxCount {
		count = disasmMaxCount
	}
	insns, err := disasmDecode(data, arch, mode, offset, int(count))
	if err != nil {
		return vm.Null(), err
	}
	return vm.Array(insns), nil
}

// disasm.disasm_one(bytes, arch, offset?) -> map or null
func disasmOne(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("disasm.disasm_one(bytes, arch, offset?)")
	}
	data, err := args[0].AsBytes()
	if err != nil {
		return vm.Null(), err
	}
	arch, mode, err := disasmParseArch(args[1].AsStr())
	if err != nil {
		return vm.Null(), err
	}
	offset := int64(0)
	if len(args) >= 3 {
		offset, err = args[2].AsInt()
		if err != nil {
			return vm.Null(), err
		}
	}
	insns, err := disasmDecode(data, arch, mode, offset, 1)
	if err != nil {
		return vm.Null(), err
	}
	if len(insns) == 0 {
		return vm.Null(), nil
	}
	return insns[0], nil
}

// archKind: "x86" | "arm64"; mode is 32/64 for x86, ignored for arm64.
func disasmParseArch(s string) (arch string, mode int, err error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "x86", "i386", "ia32":
		return "x86", 32, nil
	case "x64", "amd64", "x86_64", "x86-64":
		return "x86", 64, nil
	case "arm64", "aarch64", "armv8":
		return "arm64", 64, nil
	default:
		return "", 0, errf("disasm: unknown arch %q (want x86|x64|arm64)", s)
	}
}

func disasmDecode(data []byte, arch string, mode int, baseVA int64, maxCount int) ([]vm.Value, error) {
	out := make([]vm.Value, 0, maxCount)
	pos := 0
	for len(out) < maxCount && pos < len(data) {
		pc := uint64(baseVA) + uint64(pos)
		var (
			size     int
			mnemonic string
			opStr    string
			ok       bool
		)
		switch arch {
		case "x86":
			inst, err := x86asm.Decode(data[pos:], mode)
			if err != nil || inst.Len <= 0 {
				// stop on first undecodable byte
				return out, nil
			}
			size = inst.Len
			full := strings.TrimSpace(x86asm.IntelSyntax(inst, pc, nil))
			mnemonic, opStr = splitAsm(full)
			ok = true
		case "arm64":
			if len(data)-pos < 4 {
				return out, nil
			}
			inst, err := arm64asm.Decode(data[pos:])
			if err != nil {
				return out, nil
			}
			size = 4
			full := strings.TrimSpace(arm64asm.GNUSyntax(inst))
			mnemonic, opStr = splitAsm(full)
			ok = true
		default:
			return nil, errf("disasm: internal unknown arch %q", arch)
		}
		if !ok || size <= 0 {
			return out, nil
		}
		if pos+size > len(data) {
			return out, nil
		}
		enc := data[pos : pos+size]
		m := map[string]vm.Value{
			"addr":     vm.Int(int64(pc)),
			"bytes":    vm.Str(hex.EncodeToString(enc)),
			"mnemonic": vm.Str(mnemonic),
			"op_str":   vm.Str(opStr),
		}
		out = append(out, vm.MapVal(m))
		pos += size
	}
	return out, nil
}

func splitAsm(full string) (mnemonic, opStr string) {
	full = strings.TrimSpace(full)
	if full == "" {
		return "", ""
	}
	// Collapse multi-space separators from some syntax formatters.
	fields := strings.Fields(full)
	if len(fields) == 0 {
		return "", ""
	}
	mnemonic = fields[0]
	if len(fields) == 1 {
		return mnemonic, ""
	}
	// Re-join remaining tokens with a single space; for Intel this preserves commas.
	// Prefer the substring after first space when original had "mnemonic rest".
	if i := strings.IndexByte(full, ' '); i >= 0 {
		opStr = strings.TrimSpace(full[i+1:])
	}
	return mnemonic, opStr
}
