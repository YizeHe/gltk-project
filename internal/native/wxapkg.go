package native

import (
	"os"

	"groklang/gltk/internal/vm"
	"groklang/gltk/internal/wxapkg"
)

func moduleWxapkg() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"is_encrypted": wxIsEncrypted,
		"decrypt":      wxDecrypt,
		"list_files":   wxListFiles,
		"unpack":       wxUnpack,
		"scan":         wxScan,
	})
}

func wxIsEncrypted(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Bool(false), errf("wxapkg.is_encrypted(bytes|path)")
	}
	data, err := wxDataArg(args[0])
	if err != nil {
		return vm.Null(), err
	}
	return vm.Bool(wxapkg.IsEncrypted(data)), nil
}

func wxDecrypt(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("wxapkg.decrypt(data_bytes, wxid)")
	}
	data, err := wxDataArg(args[0])
	if err != nil {
		return vm.Null(), err
	}
	wxid := args[1].AsStr()
	out, err := wxapkg.Decrypt(data, wxid)
	if err != nil {
		return vm.Null(), err
	}
	return vm.Bytes(out), nil
}

func wxListFiles(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("wxapkg.list_files(data_bytes)")
	}
	data, err := wxDataArg(args[0])
	if err != nil {
		return vm.Null(), err
	}
	// if looks encrypted and path-like was given as bytes fail; allow decrypted only
	if wxapkg.IsEncrypted(data) {
		return vm.Null(), errf("wxapkg.list_files: data is encrypted; decrypt first")
	}
	files, err := wxapkg.ListFiles(data)
	if err != nil {
		return vm.Null(), err
	}
	arr := make([]vm.Value, 0, len(files))
	for _, f := range files {
		arr = append(arr, vm.MapVal(map[string]vm.Value{
			"name":   vm.Str(f.Name),
			"offset": vm.Int(int64(f.Offset)),
			"size":   vm.Int(int64(f.Size)),
		}))
	}
	return vm.Array(arr), nil
}

func wxUnpack(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("wxapkg.unpack(path, out_dir, opts_map?)")
	}
	path := args[0].AsStr()
	outDir := args[1].AsStr()
	opts := wxapkg.UnpackOptions{OutDir: outDir}
	if len(args) >= 3 && args[2].Typ == vm.TypeMap && args[2].Map != nil {
		m := *args[2].Map
		if v, ok := m["wxid"]; ok {
			opts.Wxid = v.AsStr()
		}
		if v, ok := m["key"]; ok && opts.Wxid == "" {
			opts.Wxid = v.AsStr()
		}
		if v, ok := m["decrypt"]; ok {
			opts.Decrypt = v.Truthy()
		}
		if v, ok := m["beautify_json"]; ok {
			opts.BeautifyJSON = v.Truthy()
		}
	}
	r := wxapkg.Unpack(path, outDir, opts)
	return vm.MapVal(map[string]vm.Value{
		"ok":        vm.Bool(r.OK),
		"count":     vm.Int(int64(r.Count)),
		"error":     vm.Str(r.Error),
		"save_path": vm.Str(r.SavePath),
	}), nil
}

func wxScan(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("wxapkg.scan(dir, recursive?)")
	}
	dir := args[0].AsStr()
	recursive := false
	if len(args) >= 2 {
		recursive = args[1].Truthy()
	}
	paths, err := wxapkg.Scan(dir, recursive)
	if err != nil {
		return vm.Null(), err
	}
	arr := make([]vm.Value, len(paths))
	for i, p := range paths {
		arr[i] = vm.Str(p)
	}
	return vm.Array(arr), nil
}

// wxDataArg accepts bytes or a filesystem path string.
func wxDataArg(v vm.Value) ([]byte, error) {
	if v.Typ == vm.TypeBytes {
		return v.Bytes, nil
	}
	if v.Typ == vm.TypeStr {
		// prefer file if path exists, else treat as raw (unlikely)
		p := v.S
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return os.ReadFile(p)
		}
		return []byte(p), nil
	}
	bs, err := v.AsBytes()
	if err != nil {
		return nil, errf("wxapkg: expected bytes or path")
	}
	return bs, nil
}
