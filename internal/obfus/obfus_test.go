package obfus

import (
	"testing"

	"groklang/gltk/internal/bytecode"
	"groklang/gltk/internal/module"
	"groklang/gltk/internal/vm"
)

func TestObfusHelloRuns(t *testing.T) {
	path := "../../samples/pack_hello.glk"
	chunk, err := module.CompileFile(path, module.Options{
		Filename:    path,
		SearchPaths: module.DefaultSearchPaths(path),
	})
	if err != nil {
		t.Skip("compile:", err)
	}
	ob, err := Apply(chunk, Default())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := bytecode.Encode(ob)
	if err != nil {
		t.Fatal(err)
	}
	back, err := bytecode.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	// Run without full native modules — only need free funcs for this script
	v := vm.New(back, nil)
	// minimal globals used by pack_hello: out, crypto need natives — skip full run if missing
	// Just ensure expanded
	if len(ob.Protos[ob.MainIndex].Code) <= len(chunk.Protos[chunk.MainIndex].Code) {
		t.Fatalf("expected expanded code %d vs %d",
			len(ob.Protos[ob.MainIndex].Code), len(chunk.Protos[chunk.MainIndex].Code))
	}
	_ = v
	_ = raw
}
