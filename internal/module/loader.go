// Package module resolves multi-file .glk imports and compiles them into one Chunk.
package module

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"groklang/gltk/internal/ast"
	"groklang/gltk/internal/bytecode"
	"groklang/gltk/internal/compiler"
	"groklang/gltk/internal/parser"
)

// Options control module search and compilation.
type Options struct {
	Filename    string   // entry .glk path
	SearchPaths []string // extra dirs
}

// DefaultSearchPaths returns the standard library search path list for an entry file.
// Order:
//  1. directory of entry file
//  2. ./lib  ./libs  (relative to cwd)
//  3. <gltk root>/stdlib  <gltk root>/libs
//  4. env GLTK_LIB (OS path-list separator)
//  5. cwd
func DefaultSearchPaths(entryFile string) []string {
	var paths []string
	seen := map[string]bool{}
	add := func(p string) {
		if p == "" {
			return
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		if seen[abs] {
			return
		}
		if st, err := os.Stat(abs); err == nil && st.IsDir() {
			seen[abs] = true
			paths = append(paths, abs)
		} else if err != nil {
			// still record relative intent for resolve fallbacks
			if !seen[p] {
				seen[p] = true
				paths = append(paths, p)
			}
		}
	}

	if entryFile != "" {
		add(filepath.Dir(entryFile))
	}
	cwd, _ := os.Getwd()
	add(filepath.Join(cwd, "lib"))
	add(filepath.Join(cwd, "libs"))

	root := findGLTKRoot()
	if root != "" {
		add(filepath.Join(root, "stdlib"))
		add(filepath.Join(root, "libs"))
	}

	if env := os.Getenv("GLTK_LIB"); env != "" {
		for _, p := range strings.Split(env, string(os.PathListSeparator)) {
			add(strings.TrimSpace(p))
		}
	}
	add(cwd)
	return paths
}

func findGLTKRoot() string {
	// 1) next to executable
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, cand := range []string{dir, filepath.Dir(dir)} {
			if isGLTKRoot(cand) {
				return cand
			}
		}
	}
	// 2) walk up from cwd
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	dir := cwd
	for {
		if isGLTKRoot(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func isGLTKRoot(dir string) bool {
	// prefer go.mod module groklang/gltk
	gm := filepath.Join(dir, "go.mod")
	if b, err := os.ReadFile(gm); err == nil {
		if strings.Contains(string(b), "module groklang/gltk") {
			return true
		}
	}
	// or stdlib/ + samples/ present
	if st, err := os.Stat(filepath.Join(dir, "stdlib")); err == nil && st.IsDir() {
		if st2, err2 := os.Stat(filepath.Join(dir, "samples")); err2 == nil && st2.IsDir() {
			return true
		}
	}
	return false
}

// CompileEntry compiles an entry .glk file with full import resolution into one Chunk.
func CompileEntry(path string) (*bytecode.Chunk, error) {
	opts := Options{
		Filename:    path,
		SearchPaths: DefaultSearchPaths(path),
	}
	return CompileFile(path, opts)
}

// CompileFile compiles path using opts (SearchPaths defaulted if empty).
func CompileFile(path string, opts Options) (*bytecode.Chunk, error) {
	if opts.Filename == "" {
		opts.Filename = path
	}
	if len(opts.SearchPaths) == 0 {
		opts.SearchPaths = DefaultSearchPaths(path)
	}
	ld := &loader{
		opts:     opts,
		chunk:    bytecode.NewChunk(path),
		stack:    map[string]bool{},
		compiled: map[string]map[string]int{},
	}
	if err := ld.compileUnit(path, false); err != nil {
		return nil, err
	}
	return ld.chunk, nil
}

type loader struct {
	opts     Options
	chunk    *bytecode.Chunk
	stack    map[string]bool          // cycle detection (abs paths)
	compiled map[string]map[string]int // abs path -> exports
	// path import setups for entry main, deps-first
	setups []compiler.PathImport
}

func (ld *loader) compileUnit(path string, isLib bool) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	if ld.stack[abs] {
		return fmt.Errorf("module: import cycle involving %s", abs)
	}
	// already compiled as library
	if isLib {
		if _, ok := ld.compiled[abs]; ok {
			return nil
		}
	}
	ld.stack[abs] = true
	defer delete(ld.stack, abs)

	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("module: read %s: %w", path, err)
	}
	prog, err := parser.Parse(string(src))
	if err != nil {
		return fmt.Errorf("module: parse %s: %w", path, err)
	}

	// Resolve path imports first (deps before self)
	var localSetups []compiler.PathImport
	entryDir := filepath.Dir(abs)
	for _, imp := range prog.Imports {
		if imp.Path == "" {
			continue
		}
		resolved, err := ld.resolve(imp.Path, entryDir)
		if err != nil {
			return fmt.Errorf("module: %s: import %q: %w", path, imp.Path, err)
		}
		resAbs, _ := filepath.Abs(resolved)
		if err := ld.compileUnit(resolved, true); err != nil {
			return err
		}
		exports := ld.compiled[resAbs]
		alias := imp.Alias
		if alias == "" {
			alias = stemName(resolved)
		}
		pi := compiler.PathImport{Alias: alias, Exports: exports}
		localSetups = append(localSetups, pi)
		// collect for entry main in dependency order
		ld.setups = append(ld.setups, pi)
	}

	if isLib {
		stem := stemName(abs)
		prefix := fmt.Sprintf("lib#%s#", stem)
		res, err := compiler.Compile(prog, compiler.CompileOptions{
			Filename:   path,
			Chunk:      ld.chunk,
			NamePrefix: prefix,
			IsLibrary:  true,
			// library path imports are globals set by entry main preamble
		})
		if err != nil {
			return fmt.Errorf("module: compile lib %s: %w", path, err)
		}
		ld.compiled[abs] = res.Exports
		return nil
	}

	// Entry: use all collected setups (transitive, deps first).
	// Prefer local aliases when the same lib is imported under different names:
	// setups already includes all; entry's direct imports appear last if also in deps —
	// rebuild unique by alias keeping last (entry wins).
	setups := mergeSetups(ld.setups)

	res, err := compiler.Compile(prog, compiler.CompileOptions{
		Filename:    path,
		Chunk:       ld.chunk,
		IsLibrary:   false,
		PathImports: setups,
	})
	if err != nil {
		return fmt.Errorf("module: compile %s: %w", path, err)
	}
	_ = res
	return nil
}

func mergeSetups(in []compiler.PathImport) []compiler.PathImport {
	// preserve order, last alias wins
	idx := map[string]int{}
	var out []compiler.PathImport
	for _, pi := range in {
		if i, ok := idx[pi.Alias]; ok {
			out[i] = pi
			continue
		}
		idx[pi.Alias] = len(out)
		out = append(out, pi)
	}
	return out
}

func stemName(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// resolve finds a .glk path for a path import.
func (ld *loader) resolve(impPath, entryDir string) (string, error) {
	// absolute
	if filepath.IsAbs(impPath) {
		if _, err := os.Stat(impPath); err == nil {
			return impPath, nil
		}
		return "", fmt.Errorf("not found: %s", impPath)
	}

	// relative to entry dir
	cand := filepath.Join(entryDir, impPath)
	if st, err := os.Stat(cand); err == nil && !st.IsDir() {
		return cand, nil
	}

	// relative to cwd
	if cwd, err := os.Getwd(); err == nil {
		cand = filepath.Join(cwd, impPath)
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand, nil
		}
	}

	// search paths: full relative path, then basename
	base := filepath.Base(impPath)
	for _, sp := range ld.opts.SearchPaths {
		for _, name := range []string{impPath, base} {
			cand = filepath.Join(sp, name)
			if st, err := os.Stat(cand); err == nil && !st.IsDir() {
				return cand, nil
			}
		}
	}

	return "", fmt.Errorf("cannot resolve library %q (searched entry dir, cwd, SearchPaths)", impPath)
}

// ParseOnly is a small helper for tests.
func ParseOnly(src string) (*ast.Program, error) {
	return parser.Parse(src)
}
