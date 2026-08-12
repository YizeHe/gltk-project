package native

import (
	"encoding/binary"
	"os"
	"unicode/utf16"

	"groklang/gltk/internal/scriptguard"
	"groklang/gltk/internal/vm"
)

func moduleAHK() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"extract_u_dwords":        ahkExtractUDwords,
		"decrypt_scriptguard":     ahkDecryptScriptGuard,
		"decrypt_scriptguard2":    ahkDecryptScriptGuard2,
		"decrypt_scriptguard_auto": ahkDecryptScriptGuardAuto,
		"is_scriptguard2":         ahkIsScriptGuard2,
		"decode_utf16le":          ahkDecodeUTF16LE,
		// high-level: text/bytes/path → plain script text or bytes
		"decrypt_rcdata": ahkDecryptRCDATA,
	})
}

// extract_u_dwords parses ScriptGuard "s.="u123u456..."" style text into array of uint32.
func ahkExtractUDwords(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Array(nil), nil
	}
	text, err := ahkTextArg(args[0])
	if err != nil {
		return vm.Null(), err
	}
	dws := scriptguard.ExtractUDwords(text)
	out := make([]vm.Value, len(dws))
	for i, d := range dws {
		out[i] = vm.Int(int64(d))
	}
	return vm.Array(out), nil
}

// decrypt_scriptguard(key_bytes, dwords_array) -> bytes (SG1 classic)
func ahkDecryptScriptGuard(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("ahk.decrypt_scriptguard(key_bytes, dwords)")
	}
	key, err := args[0].AsBytes()
	if err != nil {
		return vm.Null(), err
	}
	dwords, err := ahkDwordArray(args[1])
	if err != nil {
		return vm.Null(), err
	}
	out, err := scriptguard.DecryptSG1(key, dwords)
	if err != nil {
		return vm.Null(), err
	}
	return vm.Bytes(out), nil
}

// decrypt_scriptguard2(dwords_array | text | bytes) -> map {ok, plain, file_iv, dwords, error}
// Full SG2 pipeline (LCG + xorshift shellcode). No external AHK.exe key.
func ahkDecryptScriptGuard2(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("ahk.decrypt_scriptguard2(dwords|text|bytes)")
	}
	return ahkRunSG2(args[0])
}

// decrypt_scriptguard_auto(text|bytes, key_bytes?) -> map
// Prefer SG2 if markers present; else SG1 if key provided; else try SG2.
func ahkDecryptScriptGuardAuto(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("ahk.decrypt_scriptguard_auto(text|bytes, key?)")
	}
	text, err := ahkTextArg(args[0])
	if err != nil {
		return vm.Null(), err
	}
	if scriptguard.LooksLikeSG2(text) || len(args) < 2 {
		return ahkRunSG2(args[0])
	}
	// SG1 path
	key, err := args[1].AsBytes()
	if err != nil {
		key = []byte(args[1].AsStr())
	}
	dws := scriptguard.ExtractUDwords(text)
	if len(dws) == 0 {
		return vm.MapVal(map[string]vm.Value{
			"ok":    vm.Bool(false),
			"error": vm.Str("no dwords"),
			"algo":  vm.Str("sg1"),
		}), nil
	}
	plain, err := scriptguard.DecryptSG1(key, dws)
	if err != nil {
		return vm.MapVal(map[string]vm.Value{
			"ok":    vm.Bool(false),
			"error": vm.Str(err.Error()),
			"algo":  vm.Str("sg1"),
		}), nil
	}
	return vm.MapVal(map[string]vm.Value{
		"ok":     vm.Bool(true),
		"plain":  vm.Bytes(plain),
		"dwords": vm.Int(int64(len(dws))),
		"algo":   vm.Str("sg1"),
		"error":  vm.Str(""),
	}), nil
}

func ahkIsScriptGuard2(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Bool(false), nil
	}
	text, err := ahkTextArg(args[0])
	if err != nil {
		return vm.Bool(false), nil
	}
	return vm.Bool(scriptguard.LooksLikeSG2(text)), nil
}

// decrypt_rcdata(path|bytes, out_path?) -> map {ok, script, plain, algo, file_iv, ...}
// Convenience: decrypt RCDATA blob and return UTF-16 decoded script text.
func ahkDecryptRCDATA(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("ahk.decrypt_rcdata(path|bytes, out_path?)")
	}
	res, err := ahkRunSG2(args[0])
	if err != nil {
		return vm.Null(), err
	}
	if res.Typ != vm.TypeMap || res.Map == nil {
		return res, nil
	}
	m := *res.Map
	if !m["ok"].Truthy() {
		return res, nil
	}
	plain := m["plain"].Bytes
	script := decodeUTF16LEBytes(plain)
	m["script"] = vm.Str(script)
	m["script_len"] = vm.Int(int64(len(script)))
	if len(args) >= 2 {
		outp := args[1].AsStr()
		if outp != "" {
			if err := os.WriteFile(outp, []byte(script), 0o644); err != nil {
				m["write_error"] = vm.Str(err.Error())
			} else {
				m["out_path"] = vm.Str(outp)
			}
		}
	}
	return vm.MapVal(m), nil
}

func ahkRunSG2(arg vm.Value) (vm.Value, error) {
	// array of dwords
	if arg.Typ == vm.TypeArray && arg.Arr != nil {
		dws, err := ahkDwordArray(arg)
		if err != nil {
			return vm.Null(), err
		}
		plain, iv := scriptguard.DecryptSG2Dwords(dws)
		return vm.MapVal(map[string]vm.Value{
			"ok":      vm.Bool(len(plain) > 0),
			"plain":   vm.Bytes(plain),
			"file_iv": vm.Int(int64(iv)),
			"dwords":  vm.Int(int64(len(dws))),
			"algo":    vm.Str("sg2"),
			"error":   vm.Str(""),
		}), nil
	}
	text, err := ahkTextArg(arg)
	if err != nil {
		return vm.Null(), err
	}
	plain, iv, n, err := scriptguard.DecryptSG2Text(text)
	if err != nil {
		return vm.MapVal(map[string]vm.Value{
			"ok":    vm.Bool(false),
			"error": vm.Str(err.Error()),
			"algo":  vm.Str("sg2"),
		}), nil
	}
	return vm.MapVal(map[string]vm.Value{
		"ok":      vm.Bool(true),
		"plain":   vm.Bytes(plain),
		"file_iv": vm.Int(int64(iv)),
		"dwords":  vm.Int(int64(n)),
		"algo":    vm.Str("sg2"),
		"error":   vm.Str(""),
	}), nil
}

func ahkTextArg(v vm.Value) (string, error) {
	if v.Typ == vm.TypeBytes {
		return string(v.Bytes), nil
	}
	if v.Typ == vm.TypeStr {
		p := v.S
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			b, err := os.ReadFile(p)
			if err != nil {
				return "", err
			}
			return string(b), nil
		}
		return p, nil
	}
	if b, err := v.AsBytes(); err == nil {
		return string(b), nil
	}
	return v.AsStr(), nil
}

func ahkDwordArray(v vm.Value) ([]uint32, error) {
	if v.Typ != vm.TypeArray || v.Arr == nil {
		return nil, errf("dwords must be array")
	}
	arr := *v.Arr
	out := make([]uint32, 0, len(arr))
	for _, dv := range arr {
		n, err := dv.AsInt()
		if err != nil {
			return nil, err
		}
		out = append(out, uint32(n))
	}
	return out, nil
}

func ahkDecodeUTF16LE(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Str(""), nil
	}
	bs, err := args[0].AsBytes()
	if err != nil {
		return vm.Null(), err
	}
	return vm.Str(decodeUTF16LEBytes(bs)), nil
}

func decodeUTF16LEBytes(bs []byte) string {
	for len(bs) > 0 && bs[len(bs)-1] == 0 {
		bs = bs[:len(bs)-1]
	}
	if len(bs)%2 != 0 {
		bs = bs[:len(bs)-1]
	}
	u16 := make([]uint16, len(bs)/2)
	for i := range u16 {
		u16[i] = binary.LittleEndian.Uint16(bs[i*2:])
	}
	if len(u16) > 0 && u16[0] == 0xFEFF {
		u16 = u16[1:]
	}
	return string(utf16.Decode(u16))
}
