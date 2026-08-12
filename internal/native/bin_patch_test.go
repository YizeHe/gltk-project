package native

import (
	"hash/crc32"
	"testing"

	"groklang/gltk/internal/vm"
)

func TestBinWriteAtFillNop(t *testing.T) {
	base := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	v, err := binWriteAt(nil, []vm.Value{vm.Bytes(base), vm.Int(2), vm.Bytes([]byte{0xaa, 0xbb})})
	if err != nil {
		t.Fatal(err)
	}
	got := v.Bytes
	if len(got) != 8 || got[2] != 0xaa || got[3] != 0xbb || got[0] != 0 || got[7] != 7 {
		t.Fatalf("write_at got %v", got)
	}
	// grow
	v, err = binWriteAt(nil, []vm.Value{vm.Bytes(base), vm.Int(7), vm.Bytes([]byte{9, 10, 11})})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Bytes) != 10 || v.Bytes[7] != 9 || v.Bytes[9] != 11 {
		t.Fatalf("write_at grow got %v", v.Bytes)
	}

	v, err = binFill(nil, []vm.Value{vm.Bytes(base), vm.Int(1), vm.Int(3), vm.Int(0xff)})
	if err != nil {
		t.Fatal(err)
	}
	if v.Bytes[1] != 0xff || v.Bytes[2] != 0xff || v.Bytes[3] != 0xff || v.Bytes[4] != 4 {
		t.Fatalf("fill got %v", v.Bytes)
	}

	v, err = binNopFill(nil, []vm.Value{vm.Bytes(base), vm.Int(0), vm.Int(3)})
	if err != nil {
		t.Fatal(err)
	}
	if v.Bytes[0] != 0x90 || v.Bytes[1] != 0x90 || v.Bytes[2] != 0x90 || v.Bytes[3] != 3 {
		t.Fatalf("nop_fill got %v", v.Bytes)
	}
}

func TestBinSwapChecksumCRC(t *testing.T) {
	in := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	v, err := binSwap16(nil, []vm.Value{vm.Bytes(in)})
	if err != nil {
		t.Fatal(err)
	}
	want16 := []byte{0x02, 0x01, 0x04, 0x03, 0x05}
	for i := range want16 {
		if v.Bytes[i] != want16[i] {
			t.Fatalf("swap16 got %v want %v", v.Bytes, want16)
		}
	}
	v, err = binSwap32(nil, []vm.Value{vm.Bytes(in)})
	if err != nil {
		t.Fatal(err)
	}
	want32 := []byte{0x04, 0x03, 0x02, 0x01, 0x05}
	for i := range want32 {
		if v.Bytes[i] != want32[i] {
			t.Fatalf("swap32 got %v want %v", v.Bytes, want32)
		}
	}

	data := []byte{1, 2, 3, 250}
	v, err = binChecksumSum8(nil, []vm.Value{vm.Bytes(data)})
	if err != nil {
		t.Fatal(err)
	}
	if v.I != int64((1+2+3+250)&0xff) {
		t.Fatalf("sum8=%d", v.I)
	}
	v, err = binCRC32(nil, []vm.Value{vm.Bytes(data)})
	if err != nil {
		t.Fatal(err)
	}
	if uint32(v.I) != crc32.ChecksumIEEE(data) {
		t.Fatalf("crc32 mismatch %d", v.I)
	}
}
