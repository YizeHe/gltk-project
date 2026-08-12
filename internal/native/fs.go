package native

import (
	"os"
	"path/filepath"

	"groklang/gltk/internal/vm"
)

func moduleFS() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"read_bytes":  fsReadBytes,
		"write_bytes": fsWriteBytes,
		"mkdir_all":   fsMkdirAll,
		"exists":      fsExists,
		"read_text":   fsReadText,
		"write_text":  fsWriteText,
		"open":        natOpen, // alias of global open
		"remove":      fsRemove,
		"list_dir":    fsListDir,
		"walk":        fsWalk,
		"file_size":   fsFileSize,
		"read_range":  fsReadRange,
		"head":        fsHead,
		"tail":        fsTail,
	})
}

func fsFileSize(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Int(-1), nil
	}
	st, err := os.Stat(args[0].AsStr())
	if err != nil {
		return vm.Int(-1), nil
	}
	return vm.Int(st.Size()), nil
}

func fsReadRange(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 3 {
		return vm.Null(), errf("fs.read_range(path, offset, size)")
	}
	path := args[0].AsStr()
	off, err := args[1].AsInt()
	if err != nil {
		return vm.Null(), err
	}
	sz, err := args[2].AsInt()
	if err != nil {
		return vm.Null(), err
	}
	if sz < 0 {
		return vm.Null(), errf("negative size")
	}
	if sz > 64<<20 {
		sz = 64 << 20 // hard cap 64MB per call
	}
	f, err := os.Open(path)
	if err != nil {
		return vm.Null(), err
	}
	defer f.Close()
	buf := make([]byte, int(sz))
	n, err := f.ReadAt(buf, off)
	if n > 0 {
		return vm.Bytes(buf[:n]), nil
	}
	if err != nil {
		return vm.Null(), err
	}
	return vm.Bytes(nil), nil
}

func fsHead(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	n := int64(4096)
	if len(args) < 1 {
		return vm.Null(), errf("fs.head(path, n?)")
	}
	if len(args) >= 2 {
		if v, err := args[1].AsInt(); err == nil && v > 0 {
			n = v
		}
	}
	return fsReadRange(nil, []vm.Value{args[0], vm.Int(0), vm.Int(n)})
}

func fsTail(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("fs.tail(path, n?)")
	}
	n := int64(4096)
	if len(args) >= 2 {
		if v, err := args[1].AsInt(); err == nil && v > 0 {
			n = v
		}
	}
	st, err := os.Stat(args[0].AsStr())
	if err != nil {
		return vm.Null(), err
	}
	off := st.Size() - n
	if off < 0 {
		off = 0
		n = st.Size()
	}
	return fsReadRange(nil, []vm.Value{args[0], vm.Int(off), vm.Int(n)})
}

func fsRemove(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("fs.remove(path)")
	}
	if err := os.Remove(args[0].AsStr()); err != nil {
		return vm.Null(), err
	}
	return vm.Bool(true), nil
}

func fsListDir(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("fs.list_dir(path)")
	}
	entries, err := os.ReadDir(args[0].AsStr())
	if err != nil {
		return vm.Null(), err
	}
	arr := make([]vm.Value, 0, len(entries))
	for _, e := range entries {
		arr = append(arr, vm.Str(e.Name()))
	}
	return vm.Array(arr), nil
}

func fsWalk(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("fs.walk(path)")
	}
	root := args[0].AsStr()
	var out []vm.Value
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			out = append(out, vm.Str(path))
		}
		return nil
	})
	if err != nil {
		return vm.Null(), err
	}
	return vm.Array(out), nil
}

func fsReadBytes(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("fs.read_bytes(path)")
	}
	path := args[0].AsStr()
	b, err := os.ReadFile(path)
	if err != nil {
		return vm.Null(), err
	}
	return vm.Bytes(b), nil
}

func fsWriteBytes(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("fs.write_bytes(path, bytes)")
	}
	path := args[0].AsStr()
	bs, err := args[1].AsBytes()
	if err != nil {
		// also accept string
		bs = []byte(args[1].AsStr())
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		// ignore if dir is .
	}
	if err := os.WriteFile(path, bs, 0o644); err != nil {
		return vm.Null(), err
	}
	return vm.Int(int64(len(bs))), nil
}

func fsMkdirAll(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("fs.mkdir_all(path)")
	}
	if err := os.MkdirAll(args[0].AsStr(), 0o755); err != nil {
		return vm.Null(), err
	}
	return vm.Bool(true), nil
}

func fsExists(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Bool(false), nil
	}
	_, err := os.Stat(args[0].AsStr())
	return vm.Bool(err == nil), nil
}

func fsReadText(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("fs.read_text(path)")
	}
	b, err := os.ReadFile(args[0].AsStr())
	if err != nil {
		return vm.Null(), err
	}
	return vm.Str(string(b)), nil
}

func fsWriteText(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("fs.write_text(path, text)")
	}
	path := args[0].AsStr()
	text := args[1].AsStr()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return vm.Null(), err
	}
	return vm.Int(int64(len(text))), nil
}
