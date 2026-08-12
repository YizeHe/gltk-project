package module

import (
	"os"
	"path/filepath"
	"testing"

	"groklang/gltk/internal/native"
	"groklang/gltk/internal/vm"
)

func TestCompileEntryWithLib(t *testing.T) {
	dir := t.TempDir()
	libDir := filepath.Join(dir, "libs")
	_ = os.MkdirAll(libDir, 0o755)
	_ = os.WriteFile(filepath.Join(libDir, "math.glk"), []byte(`
fn triple(x) { return x * 3 }
`), 0o644)
	entry := filepath.Join(dir, "main.glk")
	_ = os.WriteFile(entry, []byte(`
import "libs/math.glk" as math
fn main(args) {
  return math.triple(14)
}
`), 0o644)

	chunk, err := CompileEntry(entry)
	if err != nil {
		t.Fatal(err)
	}
	v := vm.New(chunk, nil)
	native.InstallGlobals(v)
	r, err := v.Run(nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.I != 42 {
		t.Fatalf("got %v want 42", r)
	}
}

func TestImportCycle(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.glk")
	b := filepath.Join(dir, "b.glk")
	_ = os.WriteFile(a, []byte(`import "b.glk" as b
fn fa() { return 1 }
`), 0o644)
	_ = os.WriteFile(b, []byte(`import "a.glk" as a
fn fb() { return 2 }
`), 0o644)
	entry := filepath.Join(dir, "main.glk")
	_ = os.WriteFile(entry, []byte(`import "a.glk" as a
fn main(args) { return 0 }
`), 0o644)
	_, err := CompileEntry(entry)
	if err == nil {
		t.Fatal("expected cycle error")
	}
}
