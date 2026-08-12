package native

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"sync"

	"groklang/gltk/internal/vm"
)

func moduleBin() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"u8":            binU8,
		"u16le":         binU16le,
		"u32le":         binU32le,
		"u64le":         binU64le,
		"slice":         binSlice,
		"rol32":         binRol32,
		"ror32":         binRor32,
		"find_bytes":    binFindBytes,
		"concat":        binConcat,
		"len":           binLen,
		"from_u32le":    binFromU32le,
		"from_u8":       binFromU8,
		"write_at":      binWriteAt,
		"fill":          binFill,
		"nop_fill":      binNopFill,
		"swap16":        binSwap16,
		"swap32":        binSwap32,
		"checksum_sum8": binChecksumSum8,
		"crc32":         binCRC32,
		// Mutable byte buffers (generic; enable pure-GLTK codecs like UPX/LZMA)
		"mbuf":   binMBuf,
		"mget":   binMGet,
		"mset":   binMSet,
		"mlen":   binMLen,
		"mbytes": binMBytes,
		"mfill":  binMFill,
		"zeros":  binZeros,
	})
}

// --- mutable buffers (generic primitives, not UPX-specific) ---

var (
	mbufMu  sync.Mutex
	mbufSeq int64
	mbufs   = map[int64][]byte{}
)

func binMBuf(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	n := int64(0)
	if len(args) >= 1 {
		var err error
		n, err = args[0].AsInt()
		if err != nil {
			return vm.Null(), err
		}
	}
	if n < 0 {
		return vm.Null(), errf("bin.mbuf: negative size")
	}
	if n > 256<<20 {
		return vm.Null(), errf("bin.mbuf: size cap 256MiB")
	}
	mbufMu.Lock()
	mbufSeq++
	id := mbufSeq
	mbufs[id] = make([]byte, int(n))
	mbufMu.Unlock()
	return vm.MapVal(map[string]vm.Value{
		"type": vm.Str("mbuf"),
		"id":   vm.Int(id),
	}), nil
}

func mbufID(v vm.Value) (int64, error) {
	if v.Typ != vm.TypeMap || v.Map == nil {
		return 0, errf("bin.mbuf: expected handle map")
	}
	idv, ok := (*v.Map)["id"]
	if !ok {
		return 0, errf("bin.mbuf: missing id")
	}
	return idv.AsInt()
}

func binMGet(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("bin.mget(mbuf, off)")
	}
	id, err := mbufID(args[0])
	if err != nil {
		return vm.Null(), err
	}
	off, err := args[1].AsInt()
	if err != nil {
		return vm.Null(), err
	}
	mbufMu.Lock()
	bs := mbufs[id]
	mbufMu.Unlock()
	if bs == nil {
		return vm.Null(), errf("bin.mget: bad mbuf")
	}
	if off < 0 || int(off) >= len(bs) {
		return vm.Null(), errf("bin.mget: OOB")
	}
	return vm.Int(int64(bs[off])), nil
}

func binMSet(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 3 {
		return vm.Null(), errf("bin.mset(mbuf, off, byte)")
	}
	id, err := mbufID(args[0])
	if err != nil {
		return vm.Null(), err
	}
	off, err := args[1].AsInt()
	if err != nil {
		return vm.Null(), err
	}
	val, err := args[2].AsInt()
	if err != nil {
		return vm.Null(), err
	}
	mbufMu.Lock()
	defer mbufMu.Unlock()
	bs := mbufs[id]
	if bs == nil {
		return vm.Null(), errf("bin.mset: bad mbuf")
	}
	if off < 0 || int(off) >= len(bs) {
		return vm.Null(), errf("bin.mset: OOB")
	}
	bs[off] = byte(val & 0xff)
	return vm.Bool(true), nil
}

func binMLen(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Int(0), nil
	}
	id, err := mbufID(args[0])
	if err != nil {
		return vm.Null(), err
	}
	mbufMu.Lock()
	bs := mbufs[id]
	mbufMu.Unlock()
	if bs == nil {
		return vm.Int(-1), nil
	}
	return vm.Int(int64(len(bs))), nil
}

func binMBytes(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("bin.mbytes(mbuf)")
	}
	id, err := mbufID(args[0])
	if err != nil {
		return vm.Null(), err
	}
	mbufMu.Lock()
	bs := mbufs[id]
	mbufMu.Unlock()
	if bs == nil {
		return vm.Null(), errf("bin.mbytes: bad mbuf")
	}
	out := make([]byte, len(bs))
	copy(out, bs)
	return vm.Bytes(out), nil
}

func binMFill(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 4 {
		return vm.Null(), errf("bin.mfill(mbuf, off, size, byte)")
	}
	id, err := mbufID(args[0])
	if err != nil {
		return vm.Null(), err
	}
	off, err := args[1].AsInt()
	if err != nil {
		return vm.Null(), err
	}
	size, err := args[2].AsInt()
	if err != nil {
		return vm.Null(), err
	}
	val, err := args[3].AsInt()
	if err != nil {
		return vm.Null(), err
	}
	mbufMu.Lock()
	defer mbufMu.Unlock()
	bs := mbufs[id]
	if bs == nil {
		return vm.Null(), errf("bin.mfill: bad mbuf")
	}
	if off < 0 || size < 0 || int(off+size) > len(bs) {
		return vm.Null(), errf("bin.mfill: OOB")
	}
	b := byte(val & 0xff)
	for i := int(off); i < int(off+size); i++ {
		bs[i] = b
	}
	return vm.Bool(true), nil
}

func binZeros(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	n := int64(0)
	if len(args) >= 1 {
		var err error
		n, err = args[0].AsInt()
		if err != nil {
			return vm.Null(), err
		}
	}
	if n < 0 {
		return vm.Null(), errf("bin.zeros: negative")
	}
	if n > 256<<20 {
		return vm.Null(), errf("bin.zeros: cap 256MiB")
	}
	return vm.Bytes(make([]byte, int(n))), nil
}

func binFromU8(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("bin.from_u8(v)")
	}
	v, err := args[0].AsInt()
	if err != nil {
		return vm.Null(), err
	}
	return vm.Bytes([]byte{byte(v & 0xff)}), nil
}

func needBytes(args []vm.Value, i int) ([]byte, error) {
	if i >= len(args) {
		return nil, errf("missing bytes arg")
	}
	return args[i].AsBytes()
}

func binU8(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	bs, err := needBytes(args, 0)
	if err != nil {
		return vm.Null(), err
	}
	off := int64(0)
	if len(args) > 1 {
		off, err = args[1].AsInt()
		if err != nil {
			return vm.Null(), err
		}
	}
	if off < 0 || int(off) >= len(bs) {
		return vm.Null(), errf("u8 OOB")
	}
	return vm.Int(int64(bs[off])), nil
}

func binU16le(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	bs, err := needBytes(args, 0)
	if err != nil {
		return vm.Null(), err
	}
	off := int64(0)
	if len(args) > 1 {
		off, err = args[1].AsInt()
		if err != nil {
			return vm.Null(), err
		}
	}
	if off < 0 || int(off)+2 > len(bs) {
		return vm.Null(), errf("u16le OOB")
	}
	return vm.Int(int64(binary.LittleEndian.Uint16(bs[off:]))), nil
}

func binU32le(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	bs, err := needBytes(args, 0)
	if err != nil {
		return vm.Null(), err
	}
	off := int64(0)
	if len(args) > 1 {
		off, err = args[1].AsInt()
		if err != nil {
			return vm.Null(), err
		}
	}
	if off < 0 || int(off)+4 > len(bs) {
		return vm.Null(), errf("u32le OOB")
	}
	return vm.Int(int64(binary.LittleEndian.Uint32(bs[off:]))), nil
}

func binU64le(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	bs, err := needBytes(args, 0)
	if err != nil {
		return vm.Null(), err
	}
	off := int64(0)
	if len(args) > 1 {
		off, err = args[1].AsInt()
		if err != nil {
			return vm.Null(), err
		}
	}
	if off < 0 || int(off)+8 > len(bs) {
		return vm.Null(), errf("u64le OOB")
	}
	return vm.Int(int64(binary.LittleEndian.Uint64(bs[off:]))), nil
}

func binSlice(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	bs, err := needBytes(args, 0)
	if err != nil {
		return vm.Null(), err
	}
	if len(args) < 3 {
		return vm.Null(), errf("bin.slice(bytes, start, end)")
	}
	start, err := args[1].AsInt()
	if err != nil {
		return vm.Null(), err
	}
	end, err := args[2].AsInt()
	if err != nil {
		return vm.Null(), err
	}
	if start < 0 {
		start = 0
	}
	if end > int64(len(bs)) {
		end = int64(len(bs))
	}
	if start > end {
		start = end
	}
	return vm.Bytes(bs[start:end]), nil
}

func binRol32(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("bin.rol32(x, n)")
	}
	x, err := args[0].AsInt()
	if err != nil {
		return vm.Null(), err
	}
	n, err := args[1].AsInt()
	if err != nil {
		return vm.Null(), err
	}
	u := uint32(x)
	s := uint(n) & 31
	return vm.Int(int64((u << s) | (u >> (32 - s)))), nil
}

func binRor32(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("bin.ror32(x, n)")
	}
	x, err := args[0].AsInt()
	if err != nil {
		return vm.Null(), err
	}
	n, err := args[1].AsInt()
	if err != nil {
		return vm.Null(), err
	}
	u := uint32(x)
	s := uint(n) & 31
	return vm.Int(int64((u >> s) | (u << (32 - s)))), nil
}

func binFindBytes(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("bin.find_bytes(hay, needle)")
	}
	hay, err := args[0].AsBytes()
	if err != nil {
		return vm.Null(), err
	}
	needle, err := args[1].AsBytes()
	if err != nil {
		// string needle
		needle = []byte(args[1].AsStr())
	}
	i := bytes.Index(hay, needle)
	return vm.Int(int64(i)), nil
}

func binConcat(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	var out []byte
	for _, a := range args {
		bs, err := a.AsBytes()
		if err != nil {
			bs = []byte(a.AsStr())
		}
		out = append(out, bs...)
	}
	return vm.Bytes(out), nil
}

func binLen(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Int(0), nil
	}
	bs, err := args[0].AsBytes()
	if err != nil {
		return vm.Null(), err
	}
	return vm.Int(int64(len(bs))), nil
}

func binFromU32le(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	// pack array of ints as LE dwords
	if len(args) < 1 || args[0].Typ != vm.TypeArray || args[0].Arr == nil {
		return vm.Null(), errf("bin.from_u32le(array)")
	}
	arr := *args[0].Arr
	out := make([]byte, len(arr)*4)
	for i, v := range arr {
		x, err := v.AsInt()
		if err != nil {
			return vm.Null(), err
		}
		binary.LittleEndian.PutUint32(out[i*4:], uint32(x))
	}
	return vm.Bytes(out), nil
}

// binWriteAt returns a new buffer with patch written at offset (copy-on-write; grows if needed).
// bin.write_at(bytes, offset, patch_bytes) -> bytes
func binWriteAt(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 3 {
		return vm.Null(), errf("bin.write_at(bytes, offset, patch_bytes)")
	}
	bs, err := args[0].AsBytes()
	if err != nil {
		return vm.Null(), err
	}
	off, err := args[1].AsInt()
	if err != nil {
		return vm.Null(), err
	}
	if off < 0 {
		return vm.Null(), errf("bin.write_at: negative offset")
	}
	patch, err := args[2].AsBytes()
	if err != nil {
		patch = []byte(args[2].AsStr())
	}
	need := int(off) + len(patch)
	outLen := len(bs)
	if need > outLen {
		outLen = need
	}
	out := make([]byte, outLen)
	copy(out, bs)
	copy(out[int(off):], patch)
	return vm.Bytes(out), nil
}

// binFill returns a new buffer with [offset, offset+size) set to byte_val.
// bin.fill(bytes, offset, size, byte_val) -> bytes
func binFill(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 4 {
		return vm.Null(), errf("bin.fill(bytes, offset, size, byte_val)")
	}
	bs, err := args[0].AsBytes()
	if err != nil {
		return vm.Null(), err
	}
	off, err := args[1].AsInt()
	if err != nil {
		return vm.Null(), err
	}
	size, err := args[2].AsInt()
	if err != nil {
		return vm.Null(), err
	}
	val, err := args[3].AsInt()
	if err != nil {
		return vm.Null(), err
	}
	if off < 0 || size < 0 {
		return vm.Null(), errf("bin.fill: negative offset/size")
	}
	need := int(off + size)
	outLen := len(bs)
	if need > outLen {
		outLen = need
	}
	out := make([]byte, outLen)
	copy(out, bs)
	fill := byte(val & 0xff)
	for i := int(off); i < int(off+size); i++ {
		out[i] = fill
	}
	return vm.Bytes(out), nil
}

// binNopFill fills count bytes with x86 NOP (0x90).
// bin.nop_fill(bytes, offset, count) -> bytes
func binNopFill(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 3 {
		return vm.Null(), errf("bin.nop_fill(bytes, offset, count)")
	}
	return binFill(nil, []vm.Value{args[0], args[1], args[2], vm.Int(0x90)})
}

// binSwap16 swaps endianness of every 2-byte group; leftover trailing byte kept as-is.
func binSwap16(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("bin.swap16(bytes)")
	}
	bs, err := args[0].AsBytes()
	if err != nil {
		return vm.Null(), err
	}
	out := make([]byte, len(bs))
	copy(out, bs)
	for i := 0; i+1 < len(out); i += 2 {
		out[i], out[i+1] = out[i+1], out[i]
	}
	return vm.Bytes(out), nil
}

// binSwap32 swaps endianness of every 4-byte group; leftover trailing bytes kept as-is.
func binSwap32(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("bin.swap32(bytes)")
	}
	bs, err := args[0].AsBytes()
	if err != nil {
		return vm.Null(), err
	}
	out := make([]byte, len(bs))
	copy(out, bs)
	for i := 0; i+3 < len(out); i += 4 {
		out[i], out[i+1], out[i+2], out[i+3] = out[i+3], out[i+2], out[i+1], out[i]
	}
	return vm.Bytes(out), nil
}

// binChecksumSum8 returns sum of all bytes mod 256.
func binChecksumSum8(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Int(0), nil
	}
	bs, err := args[0].AsBytes()
	if err != nil {
		return vm.Null(), err
	}
	var sum uint32
	for _, b := range bs {
		sum += uint32(b)
	}
	return vm.Int(int64(sum & 0xff)), nil
}

// binCRC32 returns IEEE CRC-32 as int.
func binCRC32(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Int(0), nil
	}
	bs, err := args[0].AsBytes()
	if err != nil {
		return vm.Null(), err
	}
	return vm.Int(int64(crc32.ChecksumIEEE(bs))), nil
}
