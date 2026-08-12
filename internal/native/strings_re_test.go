package native

import (
	"os"
	"path/filepath"
	"testing"

	"groklang/gltk/internal/vm"
)

func TestStringsExtractASCII(t *testing.T) {
	// "ab" too short; "Hello" ok; "World!" ok
	data := []byte{0, 0, 'a', 'b', 0, 'H', 'e', 'l', 'l', 'o', 0xff, 'W', 'o', 'r', 'l', 'd', '!'}
	v, err := stringsExtract(nil, []vm.Value{vm.Bytes(data), vm.Int(4)})
	if err != nil {
		t.Fatal(err)
	}
	if v.Arr == nil || len(*v.Arr) != 2 {
		t.Fatalf("want 2 strings, got %v", v)
	}
	m0 := *(*v.Arr)[0].Map
	if m0["value"].AsStr() != "Hello" || m0["offset"].I != 5 {
		t.Fatalf("first=%v", m0)
	}
	m1 := *(*v.Arr)[1].Map
	if m1["value"].AsStr() != "World!" {
		t.Fatalf("second=%v", m1)
	}
}

func TestStringsExtractUTF16(t *testing.T) {
	// "Hi!!" as UTF-16LE: H\0 i\0 !\0 !\0
	data := []byte{'H', 0, 'i', 0, '!', 0, '!', 0, 0, 1}
	v, err := stringsExtractUTF16(nil, []vm.Value{vm.Bytes(data), vm.Int(4)})
	if err != nil {
		t.Fatal(err)
	}
	if v.Arr == nil || len(*v.Arr) != 1 {
		t.Fatalf("want 1 string, got %v", v)
	}
	m := *(*v.Arr)[0].Map
	if m["value"].AsStr() != "Hi!!" || m["offset"].I != 0 {
		t.Fatalf("got %v", m)
	}
}

func TestStringsEntropy(t *testing.T) {
	zeros := make([]byte, 256)
	v, err := stringsEntropyAll(nil, []vm.Value{vm.Bytes(zeros)})
	if err != nil {
		t.Fatal(err)
	}
	if v.F != 0 {
		t.Fatalf("zeros entropy=%v want 0", v.F)
	}
	// all distinct bytes -> entropy ~8
	all := make([]byte, 256)
	for i := 0; i < 256; i++ {
		all[i] = byte(i)
	}
	v, err = stringsEntropyAll(nil, []vm.Value{vm.Bytes(all)})
	if err != nil {
		t.Fatal(err)
	}
	if v.F < 7.9 || v.F > 8.0 {
		t.Fatalf("full spectrum entropy=%v want ~8", v.F)
	}

	// region
	mixed := append(append([]byte{}, zeros...), all...)
	v, err = stringsEntropy(nil, []vm.Value{vm.Bytes(mixed), vm.Int(256), vm.Int(256)})
	if err != nil {
		t.Fatal(err)
	}
	if v.F < 7.9 {
		t.Fatalf("region entropy=%v", v.F)
	}
}

func TestStringsEntropyMapAndHigh(t *testing.T) {
	// low then high entropy
	low := make([]byte, 32)
	for i := range low {
		low[i] = 'A'
	}
	high := make([]byte, 32)
	for i := range high {
		high[i] = byte(i * 7)
	}
	data := append(low, high...)
	v, err := stringsEntropyMap(nil, []vm.Value{vm.Bytes(data), vm.Int(32)})
	if err != nil {
		t.Fatal(err)
	}
	if v.Arr == nil || len(*v.Arr) != 2 {
		t.Fatalf("map blocks=%v", v)
	}
	h0 := (*(*v.Arr)[0].Map)["entropy"].F
	h1 := (*(*v.Arr)[1].Map)["entropy"].F
	if h0 >= h1 {
		t.Fatalf("expected low < high: %v %v", h0, h1)
	}

	v, err = stringsFindHighEntropy(nil, []vm.Value{vm.Bytes(data), vm.Float(4.0), vm.Int(32)})
	if err != nil {
		t.Fatal(err)
	}
	if v.Arr == nil || len(*v.Arr) < 1 {
		t.Fatalf("no high regions: %v", v)
	}
	reg := *(*v.Arr)[0].Map
	if reg["offset"].I != 32 {
		t.Fatalf("region offset=%d", reg["offset"].I)
	}
}

func TestStringsExtractPath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "t.bin")
	if err := os.WriteFile(p, []byte("xxxxHELLOyyyy"), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err := stringsExtract(nil, []vm.Value{vm.Str(p), vm.Int(5)})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range *v.Arr {
		if (*e.Map)["value"].AsStr() == "xxxxHELLOyyyy" || (*e.Map)["value"].AsStr() == "HELLO" {
			found = true
		}
	}
	// whole run is printable so one string
	if !found && (v.Arr == nil || len(*v.Arr) == 0) {
		t.Fatalf("path extract empty")
	}
	if v.Arr == nil || len(*v.Arr) != 1 || (*(*v.Arr)[0].Map)["value"].AsStr() != "xxxxHELLOyyyy" {
		t.Fatalf("path extract got %v", v)
	}
}
