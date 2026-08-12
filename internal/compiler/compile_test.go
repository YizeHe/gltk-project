package compiler

import (
	"testing"

	"groklang/gltk/internal/parser"
	"groklang/gltk/internal/vm"
)

func TestCompileHello(t *testing.T) {
	src := `
import out
fn main(args) {
  out.println("hello from glvm")
  return 0
}
`
	prog, err := parser.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compile(prog, CompileOptions{Filename: "hello.glk"})
	if err != nil {
		t.Fatal(err)
	}
	chunk := res.Chunk
	if len(chunk.Protos) == 0 {
		t.Fatal("no protos")
	}
	v := vm.New(chunk, nil)
	installMini(v)
	r, err := v.Run(nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Typ != vm.TypeInt || r.I != 0 {
		t.Fatalf("got %v", r)
	}
	if v.Ops == 0 {
		t.Fatal("expected ops > 0 (VM path)")
	}
}

func TestCompileArith(t *testing.T) {
	src := `
fn main(args) {
  let x = 10
  let y = 32
  return x + y
}
`
	prog, err := parser.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compile(prog, CompileOptions{Filename: "a.glk"})
	if err != nil {
		t.Fatal(err)
	}
	v := vm.New(res.Chunk, nil)
	r, err := v.Run(nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.I != 42 {
		t.Fatalf("got %v", r)
	}
}

func TestCompileWhileFor(t *testing.T) {
	src := `
fn main(args) {
  let x = 0
  while x < 3 {
    x = x + 1
  }
  let s = 0
  for i in range(0, 4) {
    s = s + i
  }
  return x + s
}
`
	prog, err := parser.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compile(prog, CompileOptions{Filename: "w.glk"})
	if err != nil {
		t.Fatal(err)
	}
	v := vm.New(res.Chunk, nil)
	installMini(v)
	r, err := v.Run(nil)
	if err != nil {
		t.Fatal(err)
	}
	// x=3, s=0+1+2+3=6, total 9
	if r.I != 9 {
		t.Fatalf("got %v want 9", r)
	}
}

func TestParsePathImport(t *testing.T) {
	src := `
import pe, fs
import "libs/helpers.glk" as helpers
import "./util.glk"
fn main(args) { return 0 }
`
	prog, err := parser.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(prog.Imports) != 3 {
		t.Fatalf("imports=%d", len(prog.Imports))
	}
	if len(prog.Imports[0].Names) != 2 || prog.Imports[0].Path != "" {
		t.Fatalf("bare import: %+v", prog.Imports[0])
	}
	if prog.Imports[1].Path != "libs/helpers.glk" || prog.Imports[1].Alias != "helpers" {
		t.Fatalf("path as: %+v", prog.Imports[1])
	}
	if prog.Imports[2].Path != "./util.glk" {
		t.Fatalf("path: %+v", prog.Imports[2])
	}
}
