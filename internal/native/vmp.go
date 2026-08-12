package native

import (
	"fmt"
	"os"

	"groklang/gltk/internal/vmp"
	"groklang/gltk/internal/vm"
)

func moduleVMP() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"detect":   vmpDetect,
		"is_vmp":   vmpDetect,
		"info":     vmpInfo,
		"analyze":  vmpInfo,
		"extract":  vmpExtract,
		"fixdump":  vmpFixDump,
		"assist":   vmpAssist,
		"report":   vmpReport,
	})
}

func vmpBytesArg(args []vm.Value) ([]byte, error) {
	if len(args) < 1 {
		return nil, errf("vmp: need path or bytes")
	}
	if args[0].Typ == vm.TypeStr {
		return os.ReadFile(args[0].AsStr())
	}
	return args[0].AsBytes()
}

func vmpDetect(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	data, err := vmpBytesArg(args)
	if err != nil {
		return vm.Null(), err
	}
	return vm.Bool(vmp.Detect(data)), nil
}

func vmpInfoToMap(info *vmp.Info) vm.Value {
	if info == nil {
		return vm.MapVal(map[string]vm.Value{"ok": vm.Bool(false)})
	}
	secs := make([]vm.Value, 0, len(info.Sections))
	for _, s := range info.Sections {
		secs = append(secs, vm.MapVal(map[string]vm.Value{
			"name":        vm.Str(s.Name),
			"va":          vm.Int(int64(s.VirtualAddress)),
			"vsize":       vm.Int(int64(s.VirtualSize)),
			"raw_ptr":     vm.Int(int64(s.PointerToRawData)),
			"raw_size":    vm.Int(int64(s.SizeOfRawData)),
			"disk_empty":  vm.Bool(s.DiskEmpty),
			"vmp_like":    vm.Bool(s.VMPLike),
			"characteristics": vm.Int(int64(s.Characteristics)),
		}))
	}
	reasons := make([]vm.Value, 0, len(info.Reasons))
	for _, r := range info.Reasons {
		reasons = append(reasons, vm.Str(r))
	}
	markers := make([]vm.Value, 0, len(info.Markers))
	for _, m := range info.Markers {
		markers = append(markers, vm.Str(m))
	}
	m := map[string]vm.Value{
		"ok":                 vm.Bool(info.OK),
		"is_vmp":             vm.Bool(info.IsVMP),
		"confidence":         vm.Int(int64(info.Confidence)),
		"reasons":            vm.Array(reasons),
		"machine":            vm.Int(int64(info.Machine)),
		"is64":               vm.Bool(info.Is64),
		"entry_rva":          vm.Int(int64(info.EntryRVA)),
		"entry_section":      vm.Str(info.EntrySection),
		"image_base":         vm.Int(int64(info.ImageBase)),
		"section_align":      vm.Int(int64(info.SectionAlign)),
		"file_align":         vm.Int(int64(info.FileAlign)),
		"size_of_image":      vm.Int(int64(info.SizeOfImage)),
		"number_of_sections": vm.Int(int64(info.NumberOfSections)),
		"empty_disk_sections": vm.Int(int64(info.EmptyDiskSecs)),
		"sections":           vm.Array(secs),
		"markers":            vm.Array(markers),
		"note":               vm.Str(info.Note),
	}
	if info.Error != "" {
		m["error"] = vm.Str(info.Error)
	}
	return vm.MapVal(m)
}

func vmpInfo(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	data, err := vmpBytesArg(args)
	if err != nil {
		return vm.Null(), err
	}
	info, err := vmp.Analyze(data)
	if info == nil {
		return vm.MapVal(map[string]vm.Value{
			"ok":    vm.Bool(false),
			"error": vm.Str(err.Error()),
		}), nil
	}
	return vmpInfoToMap(info), nil
}

func vmpExtract(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("vmp.extract(path|bytes, out_dir)")
	}
	data, err := vmpBytesArg(args)
	if err != nil {
		return vm.Null(), err
	}
	outDir := args[1].AsStr()
	res, err := vmp.Extract(data, outDir)
	if err != nil {
		return vm.MapVal(map[string]vm.Value{"ok": vm.Bool(false), "error": vm.Str(err.Error())}), nil
	}
	return goMapToValue(res), nil
}

func vmpFixDump(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("vmp.fixdump(in_path, out_path?)")
	}
	inPath := args[0].AsStr()
	outPath := inPath + ".fixed.exe"
	if len(args) >= 2 && args[1].AsStr() != "" {
		outPath = args[1].AsStr()
	}
	res, err := vmp.FixDumpFile(inPath, outPath)
	if err != nil {
		return vm.MapVal(map[string]vm.Value{"ok": vm.Bool(false), "error": vm.Str(err.Error())}), nil
	}
	return goMapToValue(res), nil
}

func vmpAssist(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("vmp.assist(in_path, out_dir)")
	}
	inPath := args[0].AsStr()
	outDir := args[1].AsStr()
	res, err := vmp.Assist(inPath, outDir)
	if err != nil {
		return vm.MapVal(map[string]vm.Value{"ok": vm.Bool(false), "error": vm.Str(err.Error())}), nil
	}
	return goMapToValue(res), nil
}

func vmpReport(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	data, err := vmpBytesArg(args)
	if err != nil {
		return vm.Null(), err
	}
	info, err := vmp.Analyze(data)
	if info == nil {
		return vm.Str(err.Error()), nil
	}
	return vm.Str(vmp.FormatReport(info)), nil
}

// goMapToValue converts simple map[string]interface{} from vmp package to vm.Value.
func goMapToValue(m map[string]interface{}) vm.Value {
	out := make(map[string]vm.Value, len(m))
	for k, v := range m {
		out[k] = goAnyToValue(v)
	}
	return vm.MapVal(out)
}

func goAnyToValue(v interface{}) vm.Value {
	switch t := v.(type) {
	case nil:
		return vm.Null()
	case bool:
		return vm.Bool(t)
	case int:
		return vm.Int(int64(t))
	case int64:
		return vm.Int(t)
	case string:
		return vm.Str(t)
	case []map[string]interface{}:
		arr := make([]vm.Value, 0, len(t))
		for _, item := range t {
			arr = append(arr, goMapToValue(item))
		}
		return vm.Array(arr)
	case map[string]interface{}:
		return goMapToValue(t)
	case []interface{}:
		arr := make([]vm.Value, 0, len(t))
		for _, item := range t {
			arr = append(arr, goAnyToValue(item))
		}
		return vm.Array(arr)
	case []string:
		arr := make([]vm.Value, 0, len(t))
		for _, item := range t {
			arr = append(arr, vm.Str(item))
		}
		return vm.Array(arr)
	default:
		return vm.Str(fmt.Sprint(t))
	}
}
