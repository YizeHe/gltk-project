//go:build !windows

package native

import "groklang/gltk/internal/vm"

// moduleGUI provides stubs so non-Windows builds can import gui without crash.
// available() is false; other calls return null/false/empty or a soft error.
func moduleGUI() vm.Value {
	stubFalse := func(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
		return vm.Bool(false), nil
	}
	stubNull := func(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
		return vm.Null(), errf("gui: not available on this platform")
	}
	stubEmpty := func(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
		return vm.Str(""), nil
	}
	return moduleMap(map[string]vm.NativeFunc{
		"available":   stubFalse,
		"init":        stubFalse,
		"window":      stubNull,
		"show":        stubFalse,
		"hide":        stubFalse,
		"run":         stubFalse,
		"quit":        stubFalse,
		"set_title":   stubFalse,
		"label":       stubNull,
		"button":      stubNull,
		"lineedit":    stubNull,
		"textedit":    stubNull,
		"checkbox":    stubNull,
		"set_text":    stubFalse,
		"get_text":    stubEmpty,
		"append_text": stubFalse,
		"set_bounds":  stubFalse,
		"on_click":    stubFalse,
		"on_close":    stubFalse,
		"msgbox":      stubFalse,
		"vbox":        stubNull,
		"hbox":        stubNull,
		"add":         stubFalse,
		"open_file":   stubEmpty,
		"is_checked":  stubFalse,
		"set_checked": stubFalse,
		"version": func(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
			return vm.Str("cui+gltk-1.0.0"), nil
		},
	})
}
