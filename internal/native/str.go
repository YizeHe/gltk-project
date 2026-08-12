package native

import (
	"net/url"
	"strings"
	"unicode/utf16"

	"groklang/gltk/internal/vm"
)

func moduleStr() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"scan":         strScan,
		"contains":     strContains,
		"lower":        strLower,
		"upper":        strUpper,
		"from_utf8":    strFromUTF8,
		"from_utf16le": strFromUTF16LE,
		"len":          strLen,
		"split":        strSplit,
		"replace":      strReplace,
		"trim":         strTrim,
		"index_of":     strIndexOf,
		"url_encode":   strURLEncode,
		"starts_with":  strStartsWith,
		"ends_with":    strEndsWith,
		"slice":        strSlice,
		"substr":       strSlice, // alias
		"repeat":       strRepeat,
		"join":         strJoin,
		"has_prefix":   strStartsWith,
		"has_suffix":   strEndsWith,
	})
}

// str.slice(s, start, end?) — rune-aware; end exclusive, negative from end
func strSlice(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("str.slice(s, start, end?)")
	}
	rs := []rune(args[0].AsStr())
	n := int64(len(rs))
	start, err := args[1].AsInt()
	if err != nil {
		return vm.Str(""), nil
	}
	end := n
	if len(args) >= 3 {
		if e, err := args[2].AsInt(); err == nil {
			end = e
		}
	}
	if start < 0 {
		start = n + start
	}
	if end < 0 {
		end = n + end
	}
	if start < 0 {
		start = 0
	}
	if end > n {
		end = n
	}
	if start > end {
		start = end
	}
	return vm.Str(string(rs[start:end])), nil
}

func strRepeat(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Str(""), nil
	}
	s := args[0].AsStr()
	n, err := args[1].AsInt()
	if err != nil || n <= 0 {
		return vm.Str(""), nil
	}
	if n > 100000 {
		n = 100000
	}
	return vm.Str(strings.Repeat(s, int(n))), nil
}

func strJoin(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	// str.join(array, sep)
	if len(args) < 1 || args[0].Typ != vm.TypeArray || args[0].Arr == nil {
		return vm.Str(""), nil
	}
	sep := ""
	if len(args) >= 2 {
		sep = args[1].AsStr()
	}
	arr := *args[0].Arr
	parts := make([]string, len(arr))
	for i, v := range arr {
		parts[i] = v.AsStr()
	}
	return vm.Str(strings.Join(parts, sep)), nil
}

func strURLEncode(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Str(""), nil
	}
	return vm.Str(url.QueryEscape(args[0].AsStr())), nil
}

func strStartsWith(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Bool(false), nil
	}
	return vm.Bool(strings.HasPrefix(args[0].AsStr(), args[1].AsStr())), nil
}

func strEndsWith(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Bool(false), nil
	}
	return vm.Bool(strings.HasSuffix(args[0].AsStr(), args[1].AsStr())), nil
}

func strContains(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Bool(false), nil
	}
	return vm.Bool(strings.Contains(args[0].AsStr(), args[1].AsStr())), nil
}

func strLower(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Str(""), nil
	}
	return vm.Str(strings.ToLower(args[0].AsStr())), nil
}

func strUpper(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Str(""), nil
	}
	return vm.Str(strings.ToUpper(args[0].AsStr())), nil
}

func strFromUTF8(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Str(""), nil
	}
	bs, err := args[0].AsBytes()
	if err != nil {
		return vm.Str(args[0].AsStr()), nil
	}
	return vm.Str(string(bs)), nil
}

func strFromUTF16LE(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Str(""), nil
	}
	bs, err := args[0].AsBytes()
	if err != nil {
		return vm.Null(), err
	}
	if len(bs)%2 != 0 {
		bs = bs[:len(bs)-len(bs)%2]
	}
	u16 := make([]uint16, len(bs)/2)
	for i := 0; i < len(u16); i++ {
		u16[i] = uint16(bs[i*2]) | uint16(bs[i*2+1])<<8
	}
	// strip BOM
	if len(u16) > 0 && u16[0] == 0xFEFF {
		u16 = u16[1:]
	}
	return vm.Str(string(utf16.Decode(u16))), nil
}

func strLen(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Int(0), nil
	}
	return vm.Int(int64(len(args[0].AsStr()))), nil
}

func strSplit(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Array(nil), nil
	}
	parts := strings.Split(args[0].AsStr(), args[1].AsStr())
	arr := make([]vm.Value, len(parts))
	for i, p := range parts {
		arr[i] = vm.Str(p)
	}
	return vm.Array(arr), nil
}

func strReplace(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 3 {
		return vm.Null(), errf("str.replace(s, old, new)")
	}
	return vm.Str(strings.ReplaceAll(args[0].AsStr(), args[1].AsStr(), args[2].AsStr())), nil
}

func strTrim(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Str(""), nil
	}
	return vm.Str(strings.TrimSpace(args[0].AsStr())), nil
}

func strIndexOf(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Int(-1), nil
	}
	return vm.Int(int64(strings.Index(args[0].AsStr(), args[1].AsStr()))), nil
}

// strScan extracts ASCII and UTF-16LE strings from bytes with min length.
// args: (bytes, minLen?=4, maxHits?=0 unlimited)
// returns array of maps {enc, off, text}
func strScan(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Array(nil), nil
	}
	bs, err := args[0].AsBytes()
	if err != nil {
		return vm.Null(), err
	}
	minLen := 4
	if len(args) > 1 {
		if n, e := args[1].AsInt(); e == nil && n > 0 {
			minLen = int(n)
		}
	}
	maxHits := 0
	if len(args) > 2 {
		if n, e := args[2].AsInt(); e == nil && n > 0 {
			maxHits = int(n)
		}
	}
	var out []vm.Value
	add := func(m map[string]vm.Value) bool {
		out = append(out, vm.MapVal(m))
		return maxHits > 0 && len(out) >= maxHits
	}

	// ASCII runs
	i := 0
	for i < len(bs) {
		if bs[i] >= 32 && bs[i] < 127 {
			j := i
			for j < len(bs) && bs[j] >= 32 && bs[j] < 127 {
				j++
			}
			if j-i >= minLen {
				if add(map[string]vm.Value{
					"enc":  vm.Str("ascii"),
					"off":  vm.Int(int64(i)),
					"text": vm.Str(string(bs[i:j])),
				}) {
					return vm.Array(out), nil
				}
			}
			i = j
			continue
		}
		i++
	}

	// UTF-16LE runs (printable)
	i = 0
	for i+1 < len(bs) {
		lo, hi := bs[i], bs[i+1]
		if hi == 0 && lo >= 32 && lo < 127 {
			j := i
			var run []byte
			for j+1 < len(bs) && bs[j+1] == 0 && bs[j] >= 32 && bs[j] < 127 {
				run = append(run, bs[j])
				j += 2
			}
			if len(run) >= minLen {
				if add(map[string]vm.Value{
					"enc":  vm.Str("utf16le"),
					"off":  vm.Int(int64(i)),
					"text": vm.Str(string(run)),
				}) {
					return vm.Array(out), nil
				}
			}
			if j <= i {
				i += 2
			} else {
				i = j
			}
			continue
		}
		i++
	}
	return vm.Array(out), nil
}

// scanInteresting finds RE-relevant strings (used by pe.summary).
func scanInteresting(bs []byte, minLen, maxHits int) []vm.Value {
	keys := []string{
		"http://", "https://", "github.com", ".exe", ".dll", ".apk",
		"Nullsoft", "Inno", "Electron", "chrome", "app.asar",
		"AutoHotkey", "AutoIt", "bat2exe", "KeePass", "WinRAR",
		"Install", "Setup", "Password", "License", "Copyright",
		"CreateFile", "VirtualAlloc", "LoadLibrary", "GetProcAddress",
		"RegSetValue", "ShellExecute", "powershell", "cmd.exe",
		"WiX", "MSI", "ProductName", "CompanyName", "FileVersion",
	}
	// build from strScan capped then filter
	raw, _ := strScan(nil, []vm.Value{vm.Bytes(bs), vm.Int(int64(minLen)), vm.Int(5000)})
	var out []vm.Value
	if raw.Arr == nil {
		return out
	}
	for _, h := range *raw.Arr {
		if h.Map == nil {
			continue
		}
		t := (*h.Map)["text"].AsStr()
		lt := strings.ToLower(t)
		keep := false
		for _, k := range keys {
			if strings.Contains(lt, strings.ToLower(k)) {
				keep = true
				break
			}
		}
		if keep {
			out = append(out, h)
			if maxHits > 0 && len(out) >= maxHits {
				break
			}
		}
	}
	return out
}
