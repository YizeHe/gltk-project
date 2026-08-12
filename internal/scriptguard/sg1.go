package scriptguard

import "encoding/binary"

// DecryptSG1 is the classic ScriptGuard (GTAOL Ultra Macro style):
// key = AutoHotkey.exe (or host material), schedule over floor(len/10000)*10000 bytes.
func DecryptSG1(key []byte, dwords []uint32) ([]byte, error) {
	nBytes := (len(key) / 10000) * 10000
	if nBytes < 4 {
		return nil, sgError("scriptguard: key material too small")
	}
	nDwords := nBytes / 4
	state := [4]uint32{0x6f, 0x71, 0x75, 0x77}
	for i := 0; i < nDwords; i++ {
		fd := binary.LittleEndian.Uint32(key[i*4:])
		idx := i & 3
		state[idx] = state[idx]*0x83 + fd
	}
	out := make([]byte, len(dwords)*4)
	st := state
	for i, enc := range dwords {
		idx := i & 3
		k := st[idx]*0x83 + uint32(i)
		st[idx] = k
		p := rol1(enc)
		p = (p - k) & mask32
		p = rol1(p)
		p = (p - k) & mask32
		binary.LittleEndian.PutUint32(out[i*4:], p)
	}
	return out, nil
}

func rol1(x uint32) uint32 {
	return (x << 1) | (x >> 31)
}
