package native

import (
	"os"
	"path/filepath"
	"testing"

	"groklang/gltk/internal/compiler"
	"groklang/gltk/internal/parser"
	"groklang/gltk/internal/vm"
)

func TestOpenReadWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.txt")
	// write via open
	src := `
fn main(args) {
  let p = args[0]
  let f = open(p, "w")
  f.write("hello")
  f.write(" world")
  f.close()
  let g = open(p, "r")
  let s = g.read()
  g.close()
  if s != "hello world" {
    return 1
  }
  if !exists(p) {
    return 2
  }
  return 0
}
`
	prog, err := parser.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	res, err := compiler.Compile(prog, compiler.CompileOptions{Filename: "io.glk"})
	if err != nil {
		t.Fatal(err)
	}
	v := vm.New(res.Chunk, nil)
	InstallGlobals(v)
	r, err := v.Run([]vm.Value{vm.Str(path)})
	if err != nil {
		t.Fatal(err)
	}
	if r.Typ != vm.TypeInt || r.I != 0 {
		t.Fatalf("got %v", r)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hello world" {
		t.Fatalf("file content %q", b)
	}
}

func TestWriteReadConvenience(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.txt")
	src := `
fn main(args) {
  let p = args[0]
  let f = open(p, "w")
  write(f, "abc")
  close(f)
  let g = open(p, "r")
  let s = read(g)
  close(g)
  if s != "abc" { return 1 }
  return 0
}
`
	prog, err := parser.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	res, err := compiler.Compile(prog, compiler.CompileOptions{Filename: "io2.glk"})
	if err != nil {
		t.Fatal(err)
	}
	v := vm.New(res.Chunk, nil)
	InstallGlobals(v)
	r, err := v.Run([]vm.Value{vm.Str(path)})
	if err != nil {
		t.Fatal(err)
	}
	if r.I != 0 {
		t.Fatalf("got %v", r)
	}
}
