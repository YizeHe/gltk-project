package packer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"groklang/gltk/internal/bytecode"
	"groklang/gltk/internal/module"
	"groklang/gltk/internal/obfus"
)

// Options for packing.
type Options struct {
	Input      string
	Output     string
	StubPath   string // prebuilt glrt.exe; empty = auto-discover
	NoObfus    bool
	KeepGLKB   string // optional path to write intermediate .glkb
	Obfus      obfus.Options
	Verbose    bool
}

// Result of a pack operation.
type Result struct {
	Output       string
	InputKind    string // glk|glkb
	Obfuscated   bool
	PlainGLKB    int
	CipherBytes  int
	ExeBytes     int
	Protos       int
	Consts       int
	Elapsed      time.Duration
}

// Pack loads/compiles input, optionally obfuscates, encrypts, and writes a standalone EXE.
func Pack(opt Options) (*Result, error) {
	start := time.Now()
	if opt.Input == "" {
		return nil, fmt.Errorf("packer: empty input")
	}
	if opt.Output == "" {
		base := strings.TrimSuffix(filepath.Base(opt.Input), filepath.Ext(opt.Input))
		opt.Output = base + ".exe"
	}
	stubPath, err := resolveStub(opt.StubPath)
	if err != nil {
		return nil, err
	}
	stub, err := os.ReadFile(stubPath)
	if err != nil {
		return nil, fmt.Errorf("packer: read stub: %w", err)
	}

	chunk, kind, err := loadInput(opt.Input)
	if err != nil {
		return nil, err
	}

	obfuscated := !opt.NoObfus
	if obfuscated {
		ob := opt.Obfus
		if ob.NOPDensity == 0 && !ob.ShuffleConsts && !ob.StripMeta {
			ob = obfus.Default()
		}
		chunk, err = obfus.Apply(chunk, ob)
		if err != nil {
			return nil, fmt.Errorf("packer: obfuscate: %w", err)
		}
	}

	plain, err := bytecode.Encode(chunk)
	if err != nil {
		return nil, err
	}
	if opt.KeepGLKB != "" {
		_ = os.WriteFile(opt.KeepGLKB, plain, 0o644)
	}

	ct, key, nonce, err := Seal(plain)
	if err != nil {
		return nil, err
	}
	exe, err := BuildExe(stub, ct, key, nonce)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(absOrDot(opt.Output)), 0o755); err != nil {
		// Dir may be "."
	}
	if err := os.WriteFile(opt.Output, exe, 0o755); err != nil {
		return nil, err
	}

	return &Result{
		Output:      opt.Output,
		InputKind:   kind,
		Obfuscated:  obfuscated,
		PlainGLKB:   len(plain),
		CipherBytes: len(ct),
		ExeBytes:    len(exe),
		Protos:      len(chunk.Protos),
		Consts:      len(chunk.Consts),
		Elapsed:     time.Since(start),
	}, nil
}

func absOrDot(p string) string {
	if filepath.Dir(p) == "." {
		return p
	}
	return p
}

func loadInput(path string) (*bytecode.Chunk, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	if len(data) >= 4 && string(data[:4]) == bytecode.Magic {
		c, err := bytecode.Decode(data)
		return c, "glkb", err
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".glk" && ext != "" {
		// try decode anyway, else compile
		if c, err := bytecode.Decode(data); err == nil {
			return c, "glkb", nil
		}
	}
	c, err := module.CompileFile(path, module.Options{
		Filename:    path,
		SearchPaths: module.DefaultSearchPaths(path),
	})
	return c, "glk", err
}

// ResolveStub returns path to glrt runtime stub.
func ResolveStub(explicit string) (string, error) {
	return resolveStub(explicit)
}

func resolveStub(explicit string) (string, error) {
	if explicit != "" {
		if st, err := os.Stat(explicit); err == nil && !st.IsDir() {
			return explicit, nil
		}
		return "", fmt.Errorf("packer: stub not found: %s", explicit)
	}
	// candidates relative to exe and cwd
	self, _ := os.Executable()
	selfDir := filepath.Dir(self)
	cands := []string{
		filepath.Join(selfDir, "glrt.exe"),
		filepath.Join(selfDir, "glrt"),
		filepath.Join(".", "glrt.exe"),
		filepath.Join(".", "glrt"),
		filepath.Join("cmd", "glrt", "glrt.exe"),
		`D:\grokbuild\groklang\gltk\glrt.exe`,
	}
	// walk up from cwd looking for gltk tree
	cwd, _ := os.Getwd()
	cands = append(cands,
		filepath.Join(cwd, "glrt.exe"),
		filepath.Join(cwd, "gltk", "glrt.exe"),
	)
	for _, c := range cands {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("packer: glrt stub not found — build with: go build -o glrt.exe ./cmd/glrt")
}
