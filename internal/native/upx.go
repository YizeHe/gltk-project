package native

import (
	"os"

	"groklang/gltk/internal/upx"
	"groklang/gltk/internal/vm"
)

func moduleUPX() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"detect":  upxDetect,
		"info":    upxInfo,
		"unpack":  upxUnpack,
		"is_upx":  upxDetect, // alias
	})
}

func upxBytesArg(args []vm.Value) ([]byte, error) {
	if len(args) < 1 {
		return nil, errf("upx: need path or bytes")
	}
	if args[0].Typ == vm.TypeStr {
		return os.ReadFile(args[0].AsStr())
	}
	return args[0].AsBytes()
}

func upxDetect(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	data, err := upxBytesArg(args)
	if err != nil {
		return vm.Null(), err
	}
	return vm.Bool(upx.Detect(data)), nil
}

func upxInfoToMap(info *upx.Info) vm.Value {
	m := map[string]vm.Value{
		"is_upx":      vm.Bool(info.IsUPX),
		"version_str": vm.Str(info.VersionStr),
		"method":      vm.Str(info.MethodName),
		"entry_rva":   vm.Int(int64(info.EntryRVA)),
		"upx0_raw":    vm.Int(int64(info.UPX0Raw)),
		"upx0_virt":   vm.Int(int64(info.UPX0Virt)),
		"upx1_raw":    vm.Int(int64(info.UPX1Raw)),
		"upx1_virt":   vm.Int(int64(info.UPX1Virt)),
		"upx1_ptr":    vm.Int(int64(info.UPX1Ptr)),
		"overlay_off": vm.Int(int64(info.OverlayOff)),
		"overlay_size": vm.Int(int64(info.OverlaySz)),
		"note":        vm.Str(info.Note),
	}
	if info.PackHeader != nil {
		ph := info.PackHeader
		m["ph_offset"] = vm.Int(int64(ph.Offset))
		m["ph_version"] = vm.Int(int64(ph.Version))
		m["ph_format"] = vm.Int(int64(ph.Format))
		m["ph_method"] = vm.Int(int64(ph.Method))
		m["ph_level"] = vm.Int(int64(ph.Level))
		m["u_len"] = vm.Int(int64(ph.ULen))
		m["c_len"] = vm.Int(int64(ph.CLen))
		m["u_file_size"] = vm.Int(int64(ph.UFileSize))
		m["filter"] = vm.Int(int64(ph.Filter))
		m["filter_cto"] = vm.Int(int64(ph.FilterCTO))
	}
	return vm.MapVal(m)
}

func upxInfo(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	data, err := upxBytesArg(args)
	if err != nil {
		return vm.Null(), err
	}
	info, err := upx.Analyze(data)
	if err != nil {
		return vm.MapVal(map[string]vm.Value{
			"is_upx": vm.Bool(false),
			"error":  vm.Str(err.Error()),
		}), nil
	}
	return upxInfoToMap(info), nil
}

// upx.unpack(in_path_or_bytes, out_path?) -> map
// If out_path given, write file; always return {ok, size, note, ...info, data?}
func upxUnpack(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("upx.unpack(path|bytes, out_path?)")
	}
	var data []byte
	var err error
	inPath := ""
	if args[0].Typ == vm.TypeStr {
		inPath = args[0].AsStr()
		data, err = os.ReadFile(inPath)
	} else {
		data, err = args[0].AsBytes()
	}
	if err != nil {
		return vm.MapVal(map[string]vm.Value{"ok": vm.Bool(false), "error": vm.Str(err.Error())}), nil
	}
	out, info, err := upx.UnpackPE(data)
	res := map[string]vm.Value{"ok": vm.Bool(err == nil)}
	if info != nil {
		im := upxInfoToMap(info)
		// merge
		if im.Map != nil {
			for k, v := range *im.Map {
				res[k] = v
			}
		}
	}
	if err != nil {
		res["error"] = vm.Str(err.Error())
		return vm.MapVal(res), nil
	}
	res["size"] = vm.Int(int64(len(out)))
	if len(args) >= 2 && args[1].Typ == vm.TypeStr {
		outPath := args[1].AsStr()
		if werr := os.WriteFile(outPath, out, 0o644); werr != nil {
			res["ok"] = vm.Bool(false)
			res["error"] = vm.Str(werr.Error())
			return vm.MapVal(res), nil
		}
		res["out"] = vm.Str(outPath)
	} else {
		// return bytes when no out path (may be large)
		res["data"] = vm.Bytes(out)
	}
	return vm.MapVal(res), nil
}
