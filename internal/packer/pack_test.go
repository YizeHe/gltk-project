package packer

import (
	"os"
	"path/filepath"
	"testing"

	"groklang/gltk/internal/bytecode"
)

func TestSealOpen(t *testing.T) {
	plain := []byte("GLKB-test-payload-0123456789")
	ct, key, nonce, err := Seal(plain)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Open(ct, key, nonce)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(plain) {
		t.Fatalf("mismatch")
	}
}

func TestBuildExtractRoundtrip(t *testing.T) {
	// minimal fake stub
	stub := make([]byte, 4096)
	for i := range stub {
		stub[i] = byte(i)
	}
	// minimal valid-ish glkb empty-ish won't decode — just seal random
	plain := append([]byte(bytecode.Magic), make([]byte, 64)...)
	ct, key, nonce, err := Seal(plain)
	if err != nil {
		t.Fatal(err)
	}
	exe, err := BuildExe(stub, ct, key, nonce)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ExtractPayload(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(plain) {
		t.Fatal("payload mismatch")
	}
	// write temp
	dir := t.TempDir()
	p := filepath.Join(dir, "t.exe")
	if err := os.WriteFile(p, exe, 0o755); err != nil {
		t.Fatal(err)
	}
}
