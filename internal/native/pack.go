package native

import (
	"groklang/gltk/internal/obfus"
	"groklang/gltk/internal/packer"
	"groklang/gltk/internal/vm"
)

func modulePack() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"pack":     packPack,
		"stub_ok":  packStubOK,
	})
}

// pack.pack(input, output, opts_map?)
// opts: no_obfus bool, stub str, keep_glkb str
func packPack(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("pack.pack(input, output, opts?)")
	}
	opt := packer.Options{
		Input:  args[0].AsStr(),
		Output: args[1].AsStr(),
		Obfus:  obfus.Default(),
	}
	if len(args) >= 3 && args[2].Typ == vm.TypeMap && args[2].Map != nil {
		m := *args[2].Map
		if v, ok := m["no_obfus"]; ok {
			opt.NoObfus = v.Truthy()
		}
		if v, ok := m["stub"]; ok {
			opt.StubPath = v.AsStr()
		}
		if v, ok := m["keep_glkb"]; ok {
			opt.KeepGLKB = v.AsStr()
		}
	}
	res, err := packer.Pack(opt)
	if err != nil {
		return vm.MapVal(map[string]vm.Value{
			"ok":    vm.Bool(false),
			"error": vm.Str(err.Error()),
		}), nil
	}
	return vm.MapVal(map[string]vm.Value{
		"ok":          vm.Bool(true),
		"output":      vm.Str(res.Output),
		"input_kind":  vm.Str(res.InputKind),
		"obfuscated":  vm.Bool(res.Obfuscated),
		"glkb_size":   vm.Int(int64(res.PlainGLKB)),
		"cipher_size": vm.Int(int64(res.CipherBytes)),
		"exe_size":    vm.Int(int64(res.ExeBytes)),
		"protos":      vm.Int(int64(res.Protos)),
		"consts":      vm.Int(int64(res.Consts)),
		"ms":          vm.Int(res.Elapsed.Milliseconds()),
		"error":       vm.Str(""),
	}), nil
}

func packStubOK(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	stub := ""
	if len(args) >= 1 {
		stub = args[0].AsStr()
	}
	_, err := packer.ResolveStub(stub)
	return vm.Bool(err == nil), nil
}
