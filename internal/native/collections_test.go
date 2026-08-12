package native

import (
	"testing"

	"groklang/gltk/internal/compiler"
	"groklang/gltk/internal/parser"
	"groklang/gltk/internal/vm"
)

func runWithNatives(t *testing.T, src string) vm.Value {
	t.Helper()
	prog, err := parser.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := compiler.Compile(prog, compiler.CompileOptions{Filename: "c.glk"})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	v := vm.New(res.Chunk, nil)
	InstallGlobals(v)
	r, err := v.Run(nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return r
}

func TestPushPop(t *testing.T) {
	src := `
fn main(args) {
  let a = [1]
  push(a, 2)
  push(a, 3)
  let x = pop(a)
  // a == [1,2], x == 3
  return len(a) * 10 + x + a[1]
}
`
	r := runWithNatives(t, src)
	// 2*10 + 3 + 2 = 25
	if r.I != 25 {
		t.Fatalf("want 25 got %v", r)
	}
}

func TestKeysHasDelete(t *testing.T) {
	src := `
fn main(args) {
  let m = {a: 1, b: 2}
  let ks = keys(m)
  if len(ks) != 2 { return 1 }
  if !has(m, "a") { return 2 }
  if has(m, "z") { return 3 }
  if !has([1, 2, 3], 2) { return 4 }
  if !delete(m, "b") { return 5 }
  if has(m, "b") { return 6 }
  if delete(m, "b") { return 7 }
  return 0
}
`
	r := runWithNatives(t, src)
	if r.I != 0 {
		t.Fatalf("want 0 got %v", r)
	}
}

func TestCloneShallow(t *testing.T) {
	src := `
fn main(args) {
  let a = [1, 2]
  let b = clone(a)
  push(b, 3)
  if len(a) != 2 { return 1 }
  if len(b) != 3 { return 2 }
  let m = {x: 1}
  let n = clone(m)
  n["y"] = 2
  if has(m, "y") { return 3 }
  if !has(n, "y") { return 4 }
  return 0
}
`
	r := runWithNatives(t, src)
	if r.I != 0 {
		t.Fatalf("want 0 got %v", r)
	}
}
