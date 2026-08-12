package native

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"groklang/gltk/internal/vm"
)

// Install free I/O builtins on the VM (no import required).
func installIOGlobals(v *vm.VM) {
	v.SetGlobal("input", vm.Native("input", natInput))
	v.SetGlobal("output", vm.Native("output", natOutput))
	v.SetGlobal("print", vm.Native("print", natPrint))
	v.SetGlobal("println", vm.Native("println", natPrintln))
	v.SetGlobal("open", vm.Native("open", natOpen))
	v.SetGlobal("close", vm.Native("close", natClose))
	v.SetGlobal("read", vm.Native("read", natRead))
	v.SetGlobal("write", vm.Native("write", natWrite))
	v.SetGlobal("exists", vm.Native("exists", natExists))
	v.SetGlobal("exit", vm.Native("exit", natExit))
}

func natInput(v *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) >= 1 {
		v.Out(args[0].AsStr())
	}
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return vm.Null(), err
	}
	line = strings.TrimRight(line, "\r\n")
	return vm.Str(line), nil
}

func natOutput(v *vm.VM, args []vm.Value) (vm.Value, error) {
	return outPrintln(v, args)
}

func natPrint(v *vm.VM, args []vm.Value) (vm.Value, error) {
	return outPrint(v, args)
}

func natPrintln(v *vm.VM, args []vm.Value) (vm.Value, error) {
	return outPrintln(v, args)
}

func natExists(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Bool(false), nil
	}
	_, err := os.Stat(args[0].AsStr())
	return vm.Bool(err == nil), nil
}

func natExit(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	code := 0
	if len(args) >= 1 {
		i, err := args[0].AsInt()
		if err == nil {
			code = int(i)
		}
	}
	os.Exit(code)
	return vm.Null(), nil
}

func natOpen(v *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("open(path, mode?)")
	}
	path := args[0].AsStr()
	mode := "r"
	if len(args) >= 2 {
		mode = args[1].AsStr()
		if mode == "" {
			mode = "r"
		}
	}
	f, err := openByMode(path, mode)
	if err != nil {
		return vm.Null(), err
	}
	fid := v.AllocFile(f)
	return makeFileHandle(v, path, mode, fid), nil
}

func openByMode(path, mode string) (*os.File, error) {
	switch mode {
	case "r", "rb":
		return os.Open(path)
	case "w", "wb":
		return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	case "a", "ab":
		return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	default:
		return nil, fmt.Errorf("open: unknown mode %q (use r,w,a,rb,wb,ab)", mode)
	}
}

func makeFileHandle(v *vm.VM, path, mode string, fid int64) vm.Value {
	m := map[string]vm.Value{
		"path":   vm.Str(path),
		"mode":   vm.Str(mode),
		"closed": vm.Bool(false),
		"_fid":   vm.Int(fid),
	}
	// methods close over path/mode via map fields + _fid
	m["read"] = vm.Native("handle.read", func(vmv *vm.VM, args []vm.Value) (vm.Value, error) {
		return handleReadText(vmv, m)
	})
	m["read_bytes"] = vm.Native("handle.read_bytes", func(vmv *vm.VM, args []vm.Value) (vm.Value, error) {
		return handleReadBytes(vmv, m)
	})
	m["write"] = vm.Native("handle.write", func(vmv *vm.VM, args []vm.Value) (vm.Value, error) {
		if len(args) < 1 {
			return vm.Null(), errf("handle.write(data)")
		}
		return handleWrite(vmv, m, []byte(args[0].AsStr()))
	})
	m["write_bytes"] = vm.Native("handle.write_bytes", func(vmv *vm.VM, args []vm.Value) (vm.Value, error) {
		if len(args) < 1 {
			return vm.Null(), errf("handle.write_bytes(bytes)")
		}
		bs, err := args[0].AsBytes()
		if err != nil {
			bs = []byte(args[0].AsStr())
		}
		return handleWrite(vmv, m, bs)
	})
	m["readline"] = vm.Native("handle.readline", func(vmv *vm.VM, args []vm.Value) (vm.Value, error) {
		return handleReadLine(vmv, m)
	})
	m["close"] = vm.Native("handle.close", func(vmv *vm.VM, args []vm.Value) (vm.Value, error) {
		return handleClose(vmv, m)
	})
	return vm.MapVal(m)
}

func handleFID(m map[string]vm.Value) (int64, error) {
	fv, ok := m["_fid"]
	if !ok {
		return 0, errf("invalid file handle")
	}
	return fv.AsInt()
}

func handleClosed(m map[string]vm.Value) bool {
	if c, ok := m["closed"]; ok && c.Typ == vm.TypeBool {
		return c.B
	}
	return false
}

func handleReadText(v *vm.VM, m map[string]vm.Value) (vm.Value, error) {
	if handleClosed(m) {
		return vm.Null(), errf("read on closed handle")
	}
	fid, err := handleFID(m)
	if err != nil {
		return vm.Null(), err
	}
	f := v.GetFile(fid)
	if f == nil {
		return vm.Null(), errf("invalid file id")
	}
	// read remaining from current offset
	b, err := io.ReadAll(f)
	if err != nil {
		return vm.Null(), err
	}
	return vm.Str(string(b)), nil
}

func handleReadBytes(v *vm.VM, m map[string]vm.Value) (vm.Value, error) {
	if handleClosed(m) {
		return vm.Null(), errf("read_bytes on closed handle")
	}
	fid, err := handleFID(m)
	if err != nil {
		return vm.Null(), err
	}
	f := v.GetFile(fid)
	if f == nil {
		return vm.Null(), errf("invalid file id")
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return vm.Null(), err
	}
	return vm.Bytes(b), nil
}

func handleWrite(v *vm.VM, m map[string]vm.Value, data []byte) (vm.Value, error) {
	if handleClosed(m) {
		return vm.Null(), errf("write on closed handle")
	}
	fid, err := handleFID(m)
	if err != nil {
		return vm.Null(), err
	}
	f := v.GetFile(fid)
	if f == nil {
		return vm.Null(), errf("invalid file id")
	}
	n, err := f.Write(data)
	if err != nil {
		return vm.Null(), err
	}
	return vm.Int(int64(n)), nil
}

func handleReadLine(v *vm.VM, m map[string]vm.Value) (vm.Value, error) {
	if handleClosed(m) {
		return vm.Null(), errf("readline on closed handle")
	}
	fid, err := handleFID(m)
	if err != nil {
		return vm.Null(), err
	}
	f := v.GetFile(fid)
	if f == nil {
		return vm.Null(), errf("invalid file id")
	}
	r := bufio.NewReader(f)
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return vm.Null(), err
	}
	if err == io.EOF && line == "" {
		return vm.Null(), nil
	}
	return vm.Str(strings.TrimRight(line, "\r\n")), nil
}

func handleClose(v *vm.VM, m map[string]vm.Value) (vm.Value, error) {
	if handleClosed(m) {
		return vm.Bool(true), nil
	}
	fid, err := handleFID(m)
	if err != nil {
		return vm.Null(), err
	}
	if err := v.CloseFile(fid); err != nil {
		return vm.Null(), err
	}
	m["closed"] = vm.Bool(true)
	return vm.Bool(true), nil
}

func natClose(v *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("close(handle)")
	}
	h := args[0]
	if h.Typ != vm.TypeMap || h.Map == nil {
		return vm.Null(), errf("close expects file handle map")
	}
	return handleClose(v, *h.Map)
}

func natRead(v *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("read(handle)")
	}
	h := args[0]
	if h.Typ != vm.TypeMap || h.Map == nil {
		return vm.Null(), errf("read expects file handle map")
	}
	return handleReadText(v, *h.Map)
}

func natWrite(v *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("write(handle, data)")
	}
	h := args[0]
	if h.Typ != vm.TypeMap || h.Map == nil {
		return vm.Null(), errf("write expects file handle map")
	}
	bs, err := args[1].AsBytes()
	if err != nil {
		bs = []byte(args[1].AsStr())
	}
	return handleWrite(v, *h.Map, bs)
}
