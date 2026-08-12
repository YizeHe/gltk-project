// Package native registers Go builtins as GLVM modules.
package native

import (
	"os"

	"groklang/gltk/internal/vm"
)

// DefaultModules returns the standard native module map for the VM.
func DefaultModules() map[string]vm.Value {
	mods := map[string]vm.Value{
		"out":        moduleOut(),
		"fs":         moduleFS(),
		"bin":        moduleBin(),
		"str":        moduleStr(),
		"crypto":     moduleCrypto(),
		"pe":         modulePE(),
		"disasm":     moduleDisasm(),
		"elf":        moduleELF(),
		"ahk":        moduleAHK(),
		"json":       moduleJSON(),
		"wxapkg":     moduleWxapkg(),
		"ahpk2":      moduleAHPK2(),
		"tools":      moduleTools(),
		"file":       moduleFile(),
		"msi":        moduleMSI(),
		"asar":       moduleAsar(),
		"archive":    moduleArchive(),
		"http":       moduleHTTP(),
		"net":        moduleNet(),
		"ws":         moduleWS(),
		"js":         moduleJS(),
		"async":      moduleAsync(),
		"re":         moduleRe(),
		"strings_re": moduleStringsRe(),
		"dotnet":     moduleDotnet(),
		"pack":       modulePack(),
		"upx":        moduleUPX(),
		"vmp":        moduleVMP(),
		"time":       moduleTime(),
		"rand":       moduleRand(),
		"sort":       moduleSort(),
		"gui":        moduleGUI(),
		"clash":      moduleClash(),
		"tensor":     moduleTensor(),
	}
	return mods
}

// InstallGlobals wires modules and free functions (range, len, typeof, I/O, etc.) onto a VM.
func InstallGlobals(v *vm.VM) {
	mods := DefaultModules()
	for name, m := range mods {
		v.RegisterModule(name, m)
	}
	// free functions
	v.SetGlobal("range", vm.Native("range", natRange))
	v.SetGlobal("len", vm.Native("len", natLen))
	v.SetGlobal("typeof", vm.Native("typeof", natTypeof))
	v.SetGlobal("str", vm.Native("str", natStr))
	v.SetGlobal("int", vm.Native("int", natInt))
	// Aliases that survive `import str` / `import int` module shadowing of free names.
	v.SetGlobal("to_str", vm.Native("to_str", natStr))
	v.SetGlobal("to_int", vm.Native("to_int", natInt))
	// free collection helpers
	v.SetGlobal("push", vm.Native("push", natPush))
	v.SetGlobal("pop", vm.Native("pop", natPop))
	v.SetGlobal("keys", vm.Native("keys", natKeys))
	v.SetGlobal("has", vm.Native("has", natHas))
	v.SetGlobal("delete", vm.Native("delete", natDelete))
	v.SetGlobal("clone", vm.Native("clone", natClone))
	// free I/O builtins (no import required)
	installIOGlobals(v)
	// os helpers
	v.SetGlobal("os_env", vm.Native("os_env", natOsEnv))
}

func moduleMap(fns map[string]vm.NativeFunc) vm.Value {
	m := make(map[string]vm.Value, len(fns))
	for name, f := range fns {
		m[name] = vm.Native(name, f)
	}
	return vm.MapVal(m)
}

func natRange(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	// range(n) or range(start, end) or range(start, end, step)
	start, end, step := int64(0), int64(0), int64(1)
	switch len(args) {
	case 1:
		n, err := args[0].AsInt()
		if err != nil {
			return vm.Null(), err
		}
		end = n
	case 2:
		var err error
		start, err = args[0].AsInt()
		if err != nil {
			return vm.Null(), err
		}
		end, err = args[1].AsInt()
		if err != nil {
			return vm.Null(), err
		}
	case 3:
		var err error
		start, err = args[0].AsInt()
		if err != nil {
			return vm.Null(), err
		}
		end, err = args[1].AsInt()
		if err != nil {
			return vm.Null(), err
		}
		step, err = args[2].AsInt()
		if err != nil {
			return vm.Null(), err
		}
		if step == 0 {
			return vm.Null(), errf("range step 0")
		}
	default:
		return vm.Null(), errf("range expects 1..3 args")
	}
	var arr []vm.Value
	if step > 0 {
		for i := start; i < end; i += step {
			arr = append(arr, vm.Int(i))
		}
	} else {
		for i := start; i > end; i += step {
			arr = append(arr, vm.Int(i))
		}
	}
	return vm.Array(arr), nil
}

func natLen(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("len expects 1 arg")
	}
	n, err := args[0].Len()
	if err != nil {
		return vm.Null(), err
	}
	return vm.Int(n), nil
}

func natTypeof(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Str("null"), nil
	}
	return vm.Str(args[0].TypeName()), nil
}

func natStr(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Str(""), nil
	}
	return vm.Str(args[0].AsStr()), nil
}

func natInt(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Int(0), nil
	}
	i, err := args[0].AsInt()
	if err != nil {
		return vm.Null(), err
	}
	return vm.Int(i), nil
}

// push(arr, x) mutates array and returns arr.
func natPush(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("push expects 2 args")
	}
	arr := args[0]
	if arr.Typ != vm.TypeArray {
		return vm.Null(), errf("push expects array")
	}
	if arr.Arr == nil {
		a := []vm.Value{}
		arr.Arr = &a
	}
	*arr.Arr = append(*arr.Arr, args[1])
	return arr, nil
}

// pop(arr) removes last element and returns it (or null if empty).
func natPop(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("pop expects 1 arg")
	}
	arr := args[0]
	if arr.Typ != vm.TypeArray {
		return vm.Null(), errf("pop expects array")
	}
	if arr.Arr == nil || len(*arr.Arr) == 0 {
		return vm.Null(), nil
	}
	n := len(*arr.Arr)
	v := (*arr.Arr)[n-1]
	*arr.Arr = (*arr.Arr)[:n-1]
	return v, nil
}

// keys(map) returns array of map keys.
func natKeys(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("keys expects 1 arg")
	}
	m := args[0]
	if m.Typ != vm.TypeMap || m.Map == nil {
		return vm.Array(nil), nil
	}
	ks := make([]vm.Value, 0, len(*m.Map))
	for k := range *m.Map {
		ks = append(ks, vm.Str(k))
	}
	return vm.Array(ks), nil
}

// has(map, key) or has(arr, val).
func natHas(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("has expects 2 args")
	}
	cont := args[0]
	key := args[1]
	switch cont.Typ {
	case vm.TypeMap:
		if cont.Map == nil {
			return vm.Bool(false), nil
		}
		_, ok := (*cont.Map)[key.AsStr()]
		return vm.Bool(ok), nil
	case vm.TypeArray:
		if cont.Arr == nil {
			return vm.Bool(false), nil
		}
		for _, e := range *cont.Arr {
			if e.Equal(key) {
				return vm.Bool(true), nil
			}
		}
		return vm.Bool(false), nil
	default:
		return vm.Null(), errf("has expects map or array")
	}
}

// delete(map, key) removes key; returns true if present.
func natDelete(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("delete expects 2 args")
	}
	m := args[0]
	if m.Typ != vm.TypeMap {
		return vm.Null(), errf("delete expects map")
	}
	if m.Map == nil {
		return vm.Bool(false), nil
	}
	k := args[1].AsStr()
	if _, ok := (*m.Map)[k]; !ok {
		return vm.Bool(false), nil
	}
	delete(*m.Map, k)
	return vm.Bool(true), nil
}

// clone(arr|map) shallow copy.
func natClone(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("clone expects 1 arg")
	}
	v := args[0]
	switch v.Typ {
	case vm.TypeArray:
		if v.Arr == nil {
			return vm.Array(nil), nil
		}
		cp := make([]vm.Value, len(*v.Arr))
		copy(cp, *v.Arr)
		return vm.Array(cp), nil
	case vm.TypeMap:
		if v.Map == nil {
			return vm.MapVal(map[string]vm.Value{}), nil
		}
		cp := make(map[string]vm.Value, len(*v.Map))
		for k, e := range *v.Map {
			cp[k] = e
		}
		return vm.MapVal(cp), nil
	default:
		return vm.Null(), errf("clone expects array or map")
	}
}

// os_env(name) returns the value of environment variable name, or "" if not set.
func natOsEnv(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Str(""), nil
	}
	return vm.Str(os.Getenv(args[0].AsStr())), nil
}

func errf(format string, args ...interface{}) error {
	return &nativeError{s: sprintf(format, args...)}
}

type nativeError struct{ s string }

func (e *nativeError) Error() string { return e.s }

func sprintf(format string, args ...interface{}) string {
	return fmtSprintf(format, args...)
}
