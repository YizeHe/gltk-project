package native

import (
	"fmt"
	"os"

	"groklang/gltk/internal/dotnet"
	"groklang/gltk/internal/vm"
)

func moduleDotnet() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"is_clr":  dnIsCLR,
		"info":    dnInfo,
		"strings": dnStrings,
		"types":   dnTypes,
		"methods": dnMethods,
		"il":      dnIL,
		"dump":    dnDump,
		"summary": dnSummary,
	})
}

// dnDataArg accepts path string or bytes; for large files uses PE image working set.
func dnDataArg(v vm.Value) ([]byte, error) {
	if v.Typ == vm.TypeStr {
		return dotnet.ReadPEImage(v.AsStr())
	}
	return v.AsBytes()
}

func dnLoad(v vm.Value) (*dotnet.Assembly, error) {
	if v.Typ == vm.TypeStr {
		return dotnet.ParseFile(v.AsStr())
	}
	data, err := v.AsBytes()
	if err != nil {
		return nil, err
	}
	return dotnet.Parse(data)
}

func dnIsCLR(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Bool(false), errf("dotnet.is_clr(path|bytes)")
	}
	data, err := dnDataArg(args[0])
	if err != nil {
		return vm.Bool(false), nil
	}
	return vm.Bool(dotnet.IsCLR(data)), nil
}

func dnInfo(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("dotnet.info(path|bytes)")
	}
	asm, err := dnLoad(args[0])
	if err != nil {
		return vm.MapVal(map[string]vm.Value{
			"ok":    vm.Bool(false),
			"error": vm.Str(err.Error()),
		}), nil
	}
	info := asm.Info()
	m := map[string]vm.Value{
		"ok":               vm.Bool(true),
		"version":          vm.Str(fmt.Sprint(info["version"])),
		"flags":            vm.Int(int64(info["flags"].(uint32))),
		"entry_point":      vm.Str(fmt.Sprint(info["entry_point"])),
		"entry_point_tok":  vm.Int(int64(info["entry_point_tok"].(uint32))),
		"is_ilonly":        vm.Bool(info["is_ilonly"].(bool)),
		"is_32bit":         vm.Bool(info["is_32bit"].(bool)),
		"module":           vm.Str(fmt.Sprint(info["module"])),
		"modules":          vm.Array([]vm.Value{vm.Str(fmt.Sprint(info["module"]))}),
		"assembly":         vm.Str(fmt.Sprint(info["assembly"])),
		"assembly_version": vm.Str(fmt.Sprint(info["assembly_version"])),
		"type_count":       vm.Int(int64(info["type_count"].(int))),
		"method_count":     vm.Int(int64(info["method_count"].(int))),
		"typeref_count":    vm.Int(int64(info["typeref_count"].(int))),
		"memberref_count":  vm.Int(int64(info["memberref_count"].(int))),
		"field_count":      vm.Int(int64(info["field_count"].(int))),
		"error":            vm.Str(""),
	}
	return vm.MapVal(m), nil
}

func dnStrings(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("dotnet.strings(path|bytes)")
	}
	asm, err := dnLoad(args[0])
	if err != nil {
		return vm.Null(), err
	}
	ss := asm.UserStrings()
	arr := make([]vm.Value, len(ss))
	for i, s := range ss {
		arr[i] = vm.Str(s)
	}
	return vm.Array(arr), nil
}

func dnTypes(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("dotnet.types(path|bytes)")
	}
	asm, err := dnLoad(args[0])
	if err != nil {
		return vm.Null(), err
	}
	types := asm.Types()
	arr := make([]vm.Value, 0, len(types))
	for _, t := range types {
		mc := 0
		if t.MethodStart != 0 && t.MethodEnd >= t.MethodStart {
			mc = int(t.MethodEnd - t.MethodStart + 1)
		}
		arr = append(arr, vm.MapVal(map[string]vm.Value{
			"namespace":    vm.Str(t.Namespace),
			"name":         vm.Str(t.Name),
			"full_name":    vm.Str(t.FullName),
			"flags":        vm.Int(int64(t.Flags)),
			"token":        vm.Int(int64(t.Token)),
			"method_count": vm.Int(int64(mc)),
		}))
	}
	return vm.Array(arr), nil
}

func dnMethods(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("dotnet.methods(path|bytes, type_filter?)")
	}
	asm, err := dnLoad(args[0])
	if err != nil {
		return vm.Null(), err
	}
	filter := ""
	if len(args) >= 2 {
		filter = args[1].AsStr()
	}
	methods := asm.Methods(filter)
	arr := make([]vm.Value, 0, len(methods))
	for _, m := range methods {
		arr = append(arr, vm.MapVal(map[string]vm.Value{
			"type":   vm.Str(m.TypeName),
			"name":   vm.Str(m.Name),
			"rva":    vm.Int(int64(m.RVA)),
			"token":  vm.Int(int64(m.Token)),
			"flags":  vm.Int(int64(m.Flags)),
			"impl":   vm.Int(int64(m.ImplFlags)),
		}))
	}
	return vm.Array(arr), nil
}

func dnIL(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 3 {
		return vm.MapVal(map[string]vm.Value{
			"ok":    vm.Bool(false),
			"error": vm.Str("dotnet.il(path|bytes, type_name, method_name)"),
		}), nil
	}
	asm, err := dnLoad(args[0])
	if err != nil {
		return vm.MapVal(map[string]vm.Value{
			"ok":    vm.Bool(false),
			"error": vm.Str(err.Error()),
		}), nil
	}
	typeName := args[1].AsStr()
	methodName := args[2].AsStr()
	m, ok := asm.FindMethod(typeName, methodName)
	if !ok {
		return vm.MapVal(map[string]vm.Value{
			"ok":    vm.Bool(false),
			"error": vm.Str(fmt.Sprintf("method not found: %s::%s", typeName, methodName)),
		}), nil
	}
	il, err := asm.DumpIL(m)
	if err != nil {
		return vm.MapVal(map[string]vm.Value{
			"ok":    vm.Bool(false),
			"rva":   vm.Int(int64(m.RVA)),
			"error": vm.Str(err.Error()),
		}), nil
	}
	return vm.MapVal(map[string]vm.Value{
		"ok":     vm.Bool(true),
		"il":     vm.Str(il),
		"rva":    vm.Int(int64(m.RVA)),
		"token":  vm.Int(int64(m.Token)),
		"type":   vm.Str(m.TypeName),
		"name":   vm.Str(m.Name),
		"error":  vm.Str(""),
	}), nil
}

func dnDump(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("dotnet.dump(path|bytes, opts?)")
	}
	asm, err := dnLoad(args[0])
	if err != nil {
		return vm.Str(""), err
	}
	opts := dotnet.DumpOptions{IL: false, Strings: true}
	if len(args) >= 2 && args[1].Typ == vm.TypeMap && args[1].Map != nil {
		m := *args[1].Map
		if v, ok := m["il"]; ok {
			opts.IL = v.Truthy()
		}
		if v, ok := m["strings"]; ok {
			opts.Strings = v.Truthy()
		}
		if v, ok := m["all"]; ok {
			opts.All = v.Truthy()
		}
		if v, ok := m["type"]; ok {
			opts.TypeFilter = v.AsStr()
		}
		if v, ok := m["include_module"]; ok {
			opts.IncludeModule = v.Truthy()
		}
	}
	return vm.Str(asm.Dump(opts)), nil
}

func dnSummary(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("dotnet.summary(path)")
	}
	path := args[0].AsStr()
	// quick is_clr without full parse failure
	data, err := dotnet.ReadPEImage(path)
	if err != nil {
		return vm.MapVal(map[string]vm.Value{
			"ok":     vm.Bool(false),
			"is_clr": vm.Bool(false),
			"error":  vm.Str(err.Error()),
			"path":   vm.Str(path),
		}), nil
	}
	if !dotnet.IsCLR(data) {
		st, _ := os.Stat(path)
		sz := int64(0)
		if st != nil {
			sz = st.Size()
		}
		return vm.MapVal(map[string]vm.Value{
			"ok":     vm.Bool(true),
			"is_clr": vm.Bool(false),
			"path":   vm.Str(path),
			"size":   vm.Int(sz),
		}), nil
	}
	asm, err := dotnet.Parse(data)
	if err != nil {
		return vm.MapVal(map[string]vm.Value{
			"ok":     vm.Bool(false),
			"is_clr": vm.Bool(true),
			"error":  vm.Str(err.Error()),
			"path":   vm.Str(path),
		}), nil
	}
	info := asm.Info()
	interesting := asm.InterestingStrings()
	iarr := make([]vm.Value, len(interesting))
	for i, s := range interesting {
		iarr[i] = vm.Str(s)
	}
	// also surface a few types for triage
	types := asm.Types()
	tnames := make([]vm.Value, 0, len(types))
	for _, t := range types {
		if t.Name == "<Module>" {
			continue
		}
		tnames = append(tnames, vm.Str(t.FullName))
	}
	st, _ := os.Stat(path)
	sz := int64(len(data))
	if st != nil {
		sz = st.Size()
	}
	return vm.MapVal(map[string]vm.Value{
		"ok":                   vm.Bool(true),
		"is_clr":               vm.Bool(true),
		"ilonly":               vm.Bool(asm.IsILOnly),
		"is_ilonly":            vm.Bool(asm.IsILOnly),
		"version":              vm.Str(asm.RuntimeVersion),
		"module":               vm.Str(asm.ModuleName),
		"assembly":             vm.Str(asm.AssemblyName),
		"assembly_version":     vm.Str(asm.AssemblyVersion),
		"flags":                vm.Int(int64(asm.Flags)),
		"entry_point":          vm.Str(fmt.Sprintf("0x%08X", asm.EntryPointToken)),
		"type_count":           vm.Int(int64(info["type_count"].(int))),
		"method_count":         vm.Int(int64(info["method_count"].(int))),
		"interesting_strings":  vm.Array(iarr),
		"types":                vm.Array(tnames),
		"path":                 vm.Str(path),
		"size":                 vm.Int(sz),
		"image_read":           vm.Int(int64(len(data))),
		"error":                vm.Str(""),
	}), nil
}

