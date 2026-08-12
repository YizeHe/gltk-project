package native

import (
	"os"

	"groklang/gltk/internal/vm"
)

const (
	stringsExtractCap   = 50000
	defaultStringMinLen = 4
	defaultEntropyBlock = 4096
	defaultEntropyThr   = 7.0
)

func moduleStringsRe() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"extract":           stringsExtract,
		"extract_utf16":     stringsExtractUTF16,
		"entropy":           stringsEntropy,
		"entropy_all":       stringsEntropyAll,
		"entropy_map":       stringsEntropyMap,
		"find_high_entropy": stringsFindHighEntropy,
	})
}

// stringsBytesArg accepts bytes or a filesystem path string (if file exists).
func stringsBytesArg(v vm.Value) ([]byte, error) {
	if v.Typ == vm.TypeBytes {
		return v.Bytes, nil
	}
	if v.Typ == vm.TypeStr {
		p := v.AsStr()
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return os.ReadFile(p)
		}
		return []byte(p), nil
	}
	return v.AsBytes()
}

func isPrintableASCII(b byte) bool {
	return b >= 0x20 && b <= 0x7e
}

// strings_re.extract(bytes|path, min_len?) -> [{offset, value}, ...]
func stringsExtract(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Array(nil), nil
	}
	bs, err := stringsBytesArg(args[0])
	if err != nil {
		return vm.Null(), err
	}
	minLen := defaultStringMinLen
	if len(args) > 1 {
		if n, e := args[1].AsInt(); e == nil && n > 0 {
			minLen = int(n)
		}
	}
	var out []vm.Value
	i := 0
	for i < len(bs) {
		if !isPrintableASCII(bs[i]) {
			i++
			continue
		}
		j := i + 1
		for j < len(bs) && isPrintableASCII(bs[j]) {
			j++
		}
		if j-i >= minLen {
			out = append(out, vm.MapVal(map[string]vm.Value{
				"offset": vm.Int(int64(i)),
				"value":  vm.Str(string(bs[i:j])),
			}))
			if len(out) >= stringsExtractCap {
				return vm.Array(out), nil
			}
		}
		i = j
	}
	return vm.Array(out), nil
}

// strings_re.extract_utf16(bytes|path, min_len?) -> [{offset, value}, ...]
// Scans UTF-16LE runs of printable ASCII code units (lo printable, hi == 0).
func stringsExtractUTF16(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Array(nil), nil
	}
	bs, err := stringsBytesArg(args[0])
	if err != nil {
		return vm.Null(), err
	}
	minLen := defaultStringMinLen
	if len(args) > 1 {
		if n, e := args[1].AsInt(); e == nil && n > 0 {
			minLen = int(n)
		}
	}
	var out []vm.Value
	i := 0
	for i+1 < len(bs) {
		lo, hi := bs[i], bs[i+1]
		if hi == 0 && isPrintableASCII(lo) {
			start := i
			var run []byte
			j := i
			for j+1 < len(bs) && bs[j+1] == 0 && isPrintableASCII(bs[j]) {
				run = append(run, bs[j])
				j += 2
			}
			if len(run) >= minLen {
				out = append(out, vm.MapVal(map[string]vm.Value{
					"offset": vm.Int(int64(start)),
					"value":  vm.Str(string(run)),
				}))
				if len(out) >= stringsExtractCap {
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

// strings_re.entropy(bytes, offset?, size?) -> float Shannon 0..8
func stringsEntropy(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Float(0), nil
	}
	bs, err := stringsBytesArg(args[0])
	if err != nil {
		return vm.Null(), err
	}
	off := int64(0)
	if len(args) > 1 {
		if n, e := args[1].AsInt(); e == nil {
			off = n
		}
	}
	size := int64(len(bs)) - off
	if len(args) > 2 {
		if n, e := args[2].AsInt(); e == nil {
			size = n
		}
	}
	if off < 0 {
		off = 0
	}
	if size < 0 {
		size = 0
	}
	if off > int64(len(bs)) {
		return vm.Float(0), nil
	}
	end := off + size
	if end > int64(len(bs)) {
		end = int64(len(bs))
	}
	return vm.Float(shannon(bs[off:end])), nil
}

// strings_re.entropy_all(bytes) -> float
func stringsEntropyAll(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Float(0), nil
	}
	bs, err := stringsBytesArg(args[0])
	if err != nil {
		return vm.Null(), err
	}
	return vm.Float(shannon(bs)), nil
}

// strings_re.entropy_map(bytes, block_size?) -> [{offset, entropy, size}, ...]
func stringsEntropyMap(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Array(nil), nil
	}
	bs, err := stringsBytesArg(args[0])
	if err != nil {
		return vm.Null(), err
	}
	block := defaultEntropyBlock
	if len(args) > 1 {
		if n, e := args[1].AsInt(); e == nil && n > 0 {
			block = int(n)
		}
	}
	var out []vm.Value
	for off := 0; off < len(bs); off += block {
		end := off + block
		if end > len(bs) {
			end = len(bs)
		}
		chunk := bs[off:end]
		out = append(out, vm.MapVal(map[string]vm.Value{
			"offset":  vm.Int(int64(off)),
			"entropy": vm.Float(shannon(chunk)),
			"size":    vm.Int(int64(len(chunk))),
		}))
	}
	return vm.Array(out), nil
}

// strings_re.find_high_entropy(bytes, threshold?, block_size?) -> [{offset, size, entropy}, ...]
// Merges consecutive blocks with entropy >= threshold into regions.
func stringsFindHighEntropy(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Array(nil), nil
	}
	bs, err := stringsBytesArg(args[0])
	if err != nil {
		return vm.Null(), err
	}
	threshold := defaultEntropyThr
	if len(args) > 1 {
		if f, e := args[1].AsFloat(); e == nil {
			threshold = f
		}
	}
	block := defaultEntropyBlock
	if len(args) > 2 {
		if n, e := args[2].AsInt(); e == nil && n > 0 {
			block = int(n)
		}
	}

	var out []vm.Value
	regionStart := -1
	regionEnd := -1
	var weightedSum float64
	var totalSize int

	flush := func() {
		if regionStart < 0 {
			return
		}
		avg := 0.0
		if totalSize > 0 {
			avg = weightedSum / float64(totalSize)
		}
		out = append(out, vm.MapVal(map[string]vm.Value{
			"offset":  vm.Int(int64(regionStart)),
			"size":    vm.Int(int64(regionEnd - regionStart)),
			"entropy": vm.Float(avg),
		}))
		regionStart = -1
		regionEnd = -1
		weightedSum = 0
		totalSize = 0
	}

	for off := 0; off < len(bs); off += block {
		end := off + block
		if end > len(bs) {
			end = len(bs)
		}
		chunk := bs[off:end]
		h := shannon(chunk)
		if h >= threshold {
			if regionStart < 0 {
				regionStart = off
			}
			regionEnd = end
			weightedSum += h * float64(len(chunk))
			totalSize += len(chunk)
		} else {
			flush()
		}
	}
	flush()
	return vm.Array(out), nil
}
