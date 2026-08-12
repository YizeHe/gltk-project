// Command glrt is the packed GLVM runtime stub.
// Packed EXEs are: glrt.exe body + encrypted GLKB + GPK1 trailer.
// When run as a plain stub (no trailer), prints usage.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"groklang/gltk/internal/bytecode"
	"groklang/gltk/internal/native"
	"groklang/gltk/internal/packer"
	"groklang/gltk/internal/vm"
)

func main() {
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "glrt: executable path:", err)
		os.Exit(1)
	}
	self, _ = filepath.EvalSymlinks(self)
	data, err := os.ReadFile(self)
	if err != nil {
		fmt.Fprintln(os.Stderr, "glrt: read self:", err)
		os.Exit(1)
	}
	payload, err := packer.ExtractPayload(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, "glrt: not a packed program (or corrupt):", err)
		fmt.Fprintln(os.Stderr, "usage: produce packages with: gltk pack <in.glk|in.glkb> -o out.exe")
		os.Exit(2)
	}
	chunk, err := bytecode.Decode(payload)
	if err != nil {
		fmt.Fprintln(os.Stderr, "glrt: decode glkb:", err)
		os.Exit(1)
	}
	v := vm.New(chunk, nil)
	native.InstallGlobals(v)
	args := os.Args[1:]
	vals := make([]vm.Value, len(args))
	for i, s := range args {
		vals[i] = vm.Str(s)
	}
	res, err := v.Run(vals)
	if err != nil {
		fmt.Fprintln(os.Stderr, "runtime error:", err)
		os.Exit(1)
	}
	if res.Typ == vm.TypeInt && res.I != 0 {
		os.Exit(int(res.I))
	}
}
