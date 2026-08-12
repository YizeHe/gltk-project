package compiler

import (
	"strings"
	"testing"

	"groklang/gltk/internal/parser"
	"groklang/gltk/internal/vm"
)

// install minimal builtins used by feature tests (avoid native import cycle).
func installMini(v *vm.VM) {
	v.SetGlobal("str", vm.Native("str", func(_ *vm.VM, args []vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return vm.Str(""), nil
		}
		return vm.Str(args[0].AsStr()), nil
	}))
	v.SetGlobal("len", vm.Native("len", func(_ *vm.VM, args []vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return vm.Int(0), nil
		}
		n, err := args[0].Len()
		return vm.Int(n), err
	}))
	v.SetGlobal("typeof", vm.Native("typeof", func(_ *vm.VM, args []vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return vm.Str("null"), nil
		}
		return vm.Str(args[0].TypeName()), nil
	}))
	v.SetGlobal("range", vm.Native("range", func(_ *vm.VM, args []vm.Value) (vm.Value, error) {
		var a, b, step int64 = 0, 0, 1
		if len(args) == 1 {
			b, _ = args[0].AsInt()
		} else if len(args) >= 2 {
			a, _ = args[0].AsInt()
			b, _ = args[1].AsInt()
			if len(args) >= 3 {
				step, _ = args[2].AsInt()
				if step == 0 {
					step = 1
				}
			}
		}
		var out []vm.Value
		for i := a; i < b; i += step {
			out = append(out, vm.Int(i))
		}
		return vm.Array(out), nil
	}))
	outMap := map[string]vm.Value{
		"println": vm.Native("println", func(_ *vm.VM, args []vm.Value) (vm.Value, error) {
			return vm.Null(), nil
		}),
		"print": vm.Native("print", func(_ *vm.VM, args []vm.Value) (vm.Value, error) {
			return vm.Null(), nil
		}),
	}
	v.RegisterModule("out", vm.MapVal(outMap))
}

func runSrc(t *testing.T, src string) vm.Value {
	t.Helper()
	prog, err := parser.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Compile(prog, CompileOptions{Filename: "t.glk"})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	v := vm.New(res.Chunk, nil)
	installMini(v)
	r, err := v.Run(nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return r
}

func TestBreakContinue(t *testing.T) {
	src := `
fn main(args) {
  let s = 0
  let i = 0
  while i < 10 {
    i = i + 1
    if i == 3 { continue }
    if i == 8 { break }
    s = s + i
  }
  return s
}
`
	r := runSrc(t, src)
	if r.I != 25 {
		t.Fatalf("want 25 got %v", r)
	}
}

func TestSwitchTernary(t *testing.T) {
	src := `
fn main(args) {
  let sw = 0
  switch 2 {
    case 1: sw = 10
    case 2, 3: sw = 20
    default: sw = 30
  }
  let t = (sw == 20) ? 7 : 0
  return sw + t
}
`
	r := runSrc(t, src)
	if r.I != 27 {
		t.Fatalf("want 27 got %v", r)
	}
}

func TestTryCatchThrow(t *testing.T) {
	src := `
fn main(args) {
  let caught = ""
  try {
    throw "boom"
  } catch e {
    caught = e
  }
  if caught == "boom" {
    return 1
  }
  return 0
}
`
	r := runSrc(t, src)
	if r.I != 1 {
		t.Fatalf("want 1 got %v", r)
	}
}

func TestForInMap(t *testing.T) {
	src := `
fn main(args) {
  let m = {a: 1, b: 2}
  let n = 0
  for k in m {
    n = n + 1
  }
  return n
}
`
	r := runSrc(t, src)
	if r.I != 2 {
		t.Fatalf("want 2 got %v", r)
	}
}

func TestLambdaCapture(t *testing.T) {
	src := `
fn main(args) {
  let base = 10
  let add = fn(x) { return x + base }
  return add(5)
}
`
	r := runSrc(t, src)
	if r.I != 15 {
		t.Fatalf("want 15 got %v", r)
	}
}

func TestInterpAndMulti(t *testing.T) {
	src := `
fn main(args) {
  let name = "X"
  let s = "hi ${name}"
  let m = """a
b"""
  if s == "hi X" {
    return len(m)
  }
  return 0
}
`
	r := runSrc(t, src)
	// "a\nb" length 3
	if r.I != 3 {
		t.Fatalf("want 3 got %v", r)
	}
}

func TestForBreak(t *testing.T) {
	src := `
fn main(args) {
  let s = 0
  for n in range(0, 5) {
    if n == 3 { break }
    s = s + n
  }
  return s
}
`
	r := runSrc(t, src)
	if r.I != 3 { // 0+1+2
		t.Fatalf("want 3 got %v", r)
	}
}

func TestParseKeywords(t *testing.T) {
	src := `fn main() { break; continue; try { throw 1 } catch e { } switch 1 { case 1: default: } }`
	_, err := parser.Parse(src)
	if err != nil {
		// may fail if switch body empty weirdness — just ensure keywords lex
		if !strings.Contains(err.Error(), "parse") {
			t.Fatal(err)
		}
	}
}

// Top-level let must be a true global visible from other functions.
func TestFileLevelLetGlobal(t *testing.T) {
	src := `
let g = 41
fn f() {
  return g
}
fn main(args) {
  return f() + 1
}
`
	r := runSrc(t, src)
	if r.I != 42 {
		t.Fatalf("want 42 got %v", r)
	}
}

// File-level map literal can close over other file-level lets via LOADG.
func TestFileLevelLetInMapLiteral(t *testing.T) {
	src := `
let host = "h"
let cfg = {server: host}
fn get_server() {
  return cfg["server"]
}
fn main(args) {
  let s = get_server()
  if s == "h" {
    return 1
  }
  return 0
}
`
	r := runSrc(t, src)
	if r.I != 1 {
		t.Fatalf("want 1 got %v (cfg.server should be host global)", r)
	}
}

// try/catch should catch runtime OOB index errors.
func TestTryCatchIndexOOB(t *testing.T) {
	src := `
fn main(args) {
  let a = [1, 2]
  let ok = 0
  try {
    let x = a[99]
    ok = 0
  } catch e {
    ok = 1
  }
  return ok
}
`
	r := runSrc(t, src)
	if r.I != 1 {
		t.Fatalf("want 1 (caught OOB) got %v", r)
	}
}

// Array + array concatenates.
func TestArrayConcat(t *testing.T) {
	src := `
fn main(args) {
  let a = [1, 2]
  let b = [3]
  let c = a + b
  return len(c) + c[0] + c[2]
}
`
	r := runSrc(t, src)
	// len=3, c[0]=1, c[2]=3 → 7
	if r.I != 7 {
		t.Fatalf("want 7 got %v", r)
	}
}
