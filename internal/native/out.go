package native

import "groklang/gltk/internal/vm"

func moduleOut() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"print":   outPrint,
		"println": outPrintln,
	})
}

func outPrint(v *vm.VM, args []vm.Value) (vm.Value, error) {
	for i, a := range args {
		if i > 0 {
			v.Out(" ")
		}
		v.Out(a.AsStr())
	}
	return vm.Null(), nil
}

func outPrintln(v *vm.VM, args []vm.Value) (vm.Value, error) {
	for i, a := range args {
		if i > 0 {
			v.Out(" ")
		}
		v.Out(a.AsStr())
	}
	v.Out("\n")
	return vm.Null(), nil
}
