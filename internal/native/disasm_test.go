package native

import (
	"encoding/hex"
	"testing"

	"groklang/gltk/internal/vm"
)

func TestDisasmX64KnownBytes(t *testing.T) {
	// 90 nop; c3 ret; 48 89 c0 mov rax, rax
	raw, err := hex.DecodeString("90c34889c0")
	if err != nil {
		t.Fatal(err)
	}
	insns, err := disasmDecode(raw, "x86", 64, 0x1000, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(insns) != 3 {
		t.Fatalf("want 3 insns, got %d: %v", len(insns), insns)
	}

	check := func(i int, wantAddr int64, wantBytes, wantMn string) {
		m := *insns[i].Map
		if m["addr"].I != wantAddr {
			t.Errorf("insn[%d].addr = %#x, want %#x", i, m["addr"].I, wantAddr)
		}
		if m["bytes"].S != wantBytes {
			t.Errorf("insn[%d].bytes = %q, want %q", i, m["bytes"].S, wantBytes)
		}
		if m["mnemonic"].S != wantMn {
			t.Errorf("insn[%d].mnemonic = %q, want %q", i, m["mnemonic"].S, wantMn)
		}
	}
	check(0, 0x1000, "90", "nop")
	check(1, 0x1001, "c3", "ret")
	check(2, 0x1002, "4889c0", "mov")
	// op_str for mov should mention rax
	op := (*insns[2].Map)["op_str"].S
	if op == "" {
		t.Errorf("mov op_str empty")
	}
	if !containsFold(op, "rax") {
		t.Errorf("mov op_str = %q, want to contain rax", op)
	}
}

func TestDisasmOne(t *testing.T) {
	raw := []byte{0x90, 0xc3}
	v, err := disasmOne(nil, []vm.Value{vm.Bytes(raw), vm.Str("x64")})
	if err != nil {
		t.Fatal(err)
	}
	if v.Typ != vm.TypeMap || v.Map == nil {
		t.Fatalf("want map, got %v", v)
	}
	m := *v.Map
	if m["mnemonic"].S != "nop" {
		t.Errorf("mnemonic = %q", m["mnemonic"].S)
	}
	if m["bytes"].S != "90" {
		t.Errorf("bytes = %q", m["bytes"].S)
	}
}

func TestDisasmArchAliases(t *testing.T) {
	for _, arch := range []string{"x64", "amd64", "x86_64", "X86_64"} {
		a, mode, err := disasmParseArch(arch)
		if err != nil {
			t.Fatalf("%s: %v", arch, err)
		}
		if a != "x86" || mode != 64 {
			t.Fatalf("%s -> %s mode %d", arch, a, mode)
		}
	}
	for _, arch := range []string{"x86", "i386"} {
		a, mode, err := disasmParseArch(arch)
		if err != nil {
			t.Fatalf("%s: %v", arch, err)
		}
		if a != "x86" || mode != 32 {
			t.Fatalf("%s -> %s mode %d", arch, a, mode)
		}
	}
	a, _, err := disasmParseArch("arm64")
	if err != nil || a != "arm64" {
		t.Fatalf("arm64 parse: %v %s", err, a)
	}
}

func TestDisasmX86Nop(t *testing.T) {
	// Mode32 decode of 90 nop
	insns, err := disasmDecode([]byte{0x90}, "x86", 32, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(insns) != 1 || (*insns[0].Map)["mnemonic"].S != "nop" {
		t.Fatalf("got %v", insns)
	}
}

func TestDisasmArm64Nop(t *testing.T) {
	// little-endian encoding of NOP: d503201f
	raw := []byte{0x1f, 0x20, 0x03, 0xd5}
	insns, err := disasmDecode(raw, "arm64", 64, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(insns) != 1 {
		t.Fatalf("want 1, got %d", len(insns))
	}
	m := *insns[0].Map
	if m["mnemonic"].S != "nop" {
		t.Errorf("mnemonic = %q", m["mnemonic"].S)
	}
	if m["bytes"].S != "1f2003d5" {
		t.Errorf("bytes = %q", m["bytes"].S)
	}
}

func TestDisasmManyAPI(t *testing.T) {
	raw := []byte{0x90, 0xc3}
	v, err := disasmMany(nil, []vm.Value{
		vm.Bytes(raw),
		vm.Str("x64"),
		vm.Int(0),
		vm.Int(2),
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.Typ != vm.TypeArray || v.Arr == nil || len(*v.Arr) != 2 {
		t.Fatalf("got %v", v)
	}
}

func containsFold(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub ||
		len(sub) == 0 ||
		indexFold(s, sub) >= 0)
}

func indexFold(s, sub string) int {
	// simple ASCII lower compare
	ls := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		ls[i] = c
	}
	lb := make([]byte, len(sub))
	for i := 0; i < len(sub); i++ {
		c := sub[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		lb[i] = c
	}
	return indexBytes(ls, lb)
}

func indexBytes(h, n []byte) int {
	if len(n) == 0 {
		return 0
	}
	for i := 0; i+len(n) <= len(h); i++ {
		ok := true
		for j := 0; j < len(n); j++ {
			if h[i+j] != n[j] {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}
