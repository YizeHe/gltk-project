package native

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"groklang/gltk/internal/vm"
)

func moduleAsar() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"list":    asarList,
		"extract": asarExtract,
		"read":    asarRead,
		"info":    asarInfo,
	})
}

// Electron asar:
//   [0:4]  u32 size_of_size_field_pickle (usually 4)
//   [4:8]  u32 headerSize
//   [8:8+headerSize] header pickle containing JSON
//   files base = 8 + headerSize
// offset fields in JSON are often strings.

type asarArchive struct {
	path       string
	headerSize uint32
	baseOffset int64
	root       map[string]interface{}
}

func openAsar(path string) (*asarArchive, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var b [16]byte
	if _, err := io.ReadFull(f, b[:8]); err != nil {
		return nil, err
	}
	// Electron asar pickle layout (common):
	//   u32 size_of_header_size_field (usually 4)
	//   u32 header_size  (bytes that follow until payload)
	//   then a pickle string: u32 json_len, json bytes...
	// Important: do NOT take the first '{' — pickle u32 may start with 0x7B.
	headerSize := binary.LittleEndian.Uint32(b[4:8])
	if headerSize < 16 || headerSize > 64<<20 {
		return nil, fmt.Errorf("asar: unreasonable headerSize %d", headerSize)
	}
	header := make([]byte, headerSize)
	if _, err := io.ReadFull(f, header); err != nil {
		return nil, fmt.Errorf("asar: read header: %w", err)
	}
	jsonBytes, err := asarLocateJSON(header)
	if err != nil {
		return nil, err
	}
	var root map[string]interface{}
	dec := json.NewDecoder(bytes.NewReader(jsonBytes))
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("asar: json: %w", err)
	}
	return &asarArchive{
		path:       path,
		headerSize: headerSize,
		baseOffset: 8 + int64(headerSize),
		root:       root,
	}, nil
}

// asarLocateJSON finds the JSON object inside an asar header pickle blob.
func asarLocateJSON(header []byte) ([]byte, error) {
	// Prefer pickle-string layout: optional u32, then u32 jsonLen, then JSON starting with `{"`.
	// Critical: a pickle length word may itself begin with 0x7B ('{') — never treat bare '{' as JSON.
	if len(header) >= 8 {
		for _, off := range []int{4, 0, 8} {
			if off+4 > len(header) {
				continue
			}
			n := int(binary.LittleEndian.Uint32(header[off:]))
			start := off + 4
			if n < 8 || start+n > len(header) {
				continue
			}
			if header[start] == '{' && header[start+1] == '"' {
				return header[start : start+n], nil
			}
		}
	}
	// Fallback: first real JSON object key start
	idx := bytes.Index(header, []byte(`{"`))
	if idx < 0 {
		return nil, fmt.Errorf("asar: no JSON object in header")
	}
	return header[idx:], nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (a *asarArchive) filesNode() map[string]interface{} {
	if a.root == nil {
		return nil
	}
	if f, ok := a.root["files"].(map[string]interface{}); ok {
		return f
	}
	return nil
}

func walkAsarFiles(node map[string]interface{}, prefix string, out *[]string) {
	if node == nil {
		return
	}
	for name, raw := range node {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		p := name
		if prefix != "" {
			p = prefix + "/" + name
		}
		if sub, ok := m["files"].(map[string]interface{}); ok {
			walkAsarFiles(sub, p, out)
			continue
		}
		*out = append(*out, p)
	}
}

func findAsarEntry(node map[string]interface{}, parts []string) map[string]interface{} {
	if node == nil || len(parts) == 0 {
		return nil
	}
	cur := node
	for i, part := range parts {
		raw, ok := cur[part]
		if !ok {
			return nil
		}
		m, ok := raw.(map[string]interface{})
		if !ok {
			return nil
		}
		if i == len(parts)-1 {
			return m
		}
		sub, ok := m["files"].(map[string]interface{})
		if !ok {
			return nil
		}
		cur = sub
	}
	return nil
}

func asarInfo(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("asar.info(path)")
	}
	a, err := openAsar(args[0].AsStr())
	if err != nil {
		return vm.MapVal(map[string]vm.Value{
			"ok":    vm.Bool(false),
			"error": vm.Str(err.Error()),
		}), nil
	}
	var files []string
	walkAsarFiles(a.filesNode(), "", &files)
	return vm.MapVal(map[string]vm.Value{
		"ok":          vm.Bool(true),
		"path":        vm.Str(a.path),
		"header_size": vm.Int(int64(a.headerSize)),
		"base_offset": vm.Int(a.baseOffset),
		"file_count":  vm.Int(int64(len(files))),
		"error":       vm.Str(""),
	}), nil
}

func asarList(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("asar.list(path)")
	}
	a, err := openAsar(args[0].AsStr())
	if err != nil {
		return vm.Array(nil), nil
	}
	var files []string
	walkAsarFiles(a.filesNode(), "", &files)
	arr := make([]vm.Value, len(files))
	for i, f := range files {
		arr[i] = vm.Str(f)
	}
	return vm.Array(arr), nil
}

func asarRead(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("asar.read(asar_path, inner_path)")
	}
	a, err := openAsar(args[0].AsStr())
	if err != nil {
		return vm.Null(), nil // soft: null when unreadable
	}
	inner := strings.Trim(strings.ReplaceAll(args[1].AsStr(), "\\", "/"), "/")
	parts := strings.Split(inner, "/")
	ent := findAsarEntry(a.filesNode(), parts)
	if ent == nil {
		return vm.Null(), nil // soft: null when missing (RE scripts probe many paths)
	}
	size := jsonNum(ent["size"])
	off := jsonNum(ent["offset"])
	if size < 0 || off < 0 {
		return vm.Null(), errf("asar: not a file entry")
	}
	f, err := os.Open(a.path)
	if err != nil {
		return vm.Null(), err
	}
	defer f.Close()
	buf := make([]byte, size)
	if _, err := f.ReadAt(buf, a.baseOffset+off); err != nil && err != io.EOF {
		return vm.Null(), err
	}
	return vm.Bytes(buf), nil
}

func asarExtract(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("asar.extract(asar_path, out_dir)")
	}
	asarPath := args[0].AsStr()
	outDir := args[1].AsStr()
	a, err := openAsar(asarPath)
	if err != nil {
		// Never hard-fail the VM — RE scripts need soft errors.
		return vm.MapVal(map[string]vm.Value{
			"ok":    vm.Bool(false),
			"count": vm.Int(0),
			"error": vm.Str(err.Error()),
		}), nil
	}
	var files []string
	walkAsarFiles(a.filesNode(), "", &files)
	src, err := os.Open(asarPath)
	if err != nil {
		return vm.Null(), err
	}
	defer src.Close()
	n := 0
	for _, rel := range files {
		parts := strings.Split(rel, "/")
		ent := findAsarEntry(a.filesNode(), parts)
		if ent == nil {
			continue
		}
		size := jsonNum(ent["size"])
		off := jsonNum(ent["offset"])
		if size < 0 || off < 0 {
			continue
		}
		dest := filepath.Join(outDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return vm.Null(), err
		}
		buf := make([]byte, size)
		if _, err := src.ReadAt(buf, a.baseOffset+off); err != nil && err != io.EOF {
			return vm.Null(), fmt.Errorf("asar extract %s: %w", rel, err)
		}
		if err := os.WriteFile(dest, buf, 0o644); err != nil {
			return vm.Null(), err
		}
		n++
	}
	return vm.MapVal(map[string]vm.Value{
		"ok":    vm.Bool(true),
		"count": vm.Int(int64(n)),
		"out":   vm.Str(outDir),
	}), nil
}

func jsonNum(v interface{}) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case json.Number:
		i, _ := t.Int64()
		return i
	case string:
		n, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return -1
		}
		return n
	case int:
		return int64(t)
	case int64:
		return t
	default:
		return -1
	}
}
