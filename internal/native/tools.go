package native

import (
	"groklang/gltk/internal/toolkit"
	"groklang/gltk/internal/vm"
)

func moduleTools() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"root":   toolsRoot,
		"list":   toolsList,
		"info":   toolsInfo,
		"path":   toolsPath,
		"run":    toolsRun,
		"java_ok": toolsJavaOK,
	})
}

func toolsRoot(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
	return vm.Str(toolkit.Root()), nil
}

func toolsList(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	cat, q := "", ""
	if len(args) >= 1 {
		cat = args[0].AsStr()
	}
	if len(args) >= 2 {
		q = args[1].AsStr()
	}
	entries := toolkit.Filter(cat, q)
	arr := make([]vm.Value, 0, len(entries))
	for _, e := range entries {
		arr = append(arr, entryToMap(e))
	}
	return vm.Array(arr), nil
}

func toolsInfo(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("tools.info(id)")
	}
	e, ok := toolkit.Get(args[0].AsStr())
	if !ok {
		return vm.Null(), errf("unknown tool %q", args[0].AsStr())
	}
	return entryToMap(e), nil
}

func toolsPath(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("tools.path(id)")
	}
	p, err := toolkit.PathOf(args[0].AsStr())
	if err != nil {
		return vm.Null(), err
	}
	return vm.Str(p), nil
}

func toolsRun(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("tools.run(id, args_array?)")
	}
	id := args[0].AsStr()
	var targs []string
	if len(args) >= 2 && args[1].Typ == vm.TypeArray && args[1].Arr != nil {
		for _, v := range *args[1].Arr {
			targs = append(targs, v.AsStr())
		}
	}
	res, err := toolkit.Run(id, targs, toolkit.RunOptions{Capture: true})
	if err != nil {
		return vm.MapVal(map[string]vm.Value{
			"ok":       vm.Bool(false),
			"error":    vm.Str(err.Error()),
			"exit":     vm.Int(1),
			"cmdline":  vm.Str(""),
			"output":   vm.Str(""),
		}), nil
	}
	return vm.MapVal(map[string]vm.Value{
		"ok":      vm.Bool(res.ExitCode == 0 && res.Error == ""),
		"error":   vm.Str(res.Error),
		"exit":    vm.Int(int64(res.ExitCode)),
		"cmdline": vm.Str(res.CmdLine),
		"output":  vm.Str(res.Stdout),
	}), nil
}

func toolsJavaOK(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
	return vm.Bool(toolkit.JavaOK()), nil
}

func entryToMap(e toolkit.Entry) vm.Value {
	tags := make([]vm.Value, len(e.Tags))
	for i, t := range e.Tags {
		tags[i] = vm.Str(t)
	}
	return vm.MapVal(map[string]vm.Value{
		"id":          vm.Str(e.ID),
		"name":        vm.Str(e.Name),
		"category":    vm.Str(string(e.Category)),
		"description": vm.Str(e.Description),
		"path":        vm.Str(e.AbsPath),
		"rel":         vm.Str(e.RelPath),
		"kind":        vm.Str(e.Kind),
		"launch":      vm.Str(e.Launch),
		"available":   vm.Bool(e.Available),
		"gui":         vm.Bool(e.GUI),
		"java_jar":    vm.Bool(e.JavaJar),
		"tags":        vm.Array(tags),
	})
}
