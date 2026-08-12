package native

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"groklang/gltk/internal/toolkit"
	"groklang/gltk/internal/vm"
)

func moduleArchive() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"extract":    archiveExtract,
		"list":       archiveList,
		"find_7za":   archiveFind7za,
		"nsis_extract": archiveExtract, // alias — 7za opens NSIS
	})
}

func archiveFind7za(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
	p := find7za()
	if p == "" {
		return vm.Str(""), nil
	}
	return vm.Str(p), nil
}

func find7za() string {
	// Prefer full 7z.exe (has NSIS/RAR/etc). Standalone 7za lacks NSIS.
	// Order matters: never return 7za if any full 7z.exe exists.
	fullFirst := []string{
		`D:\grokbuild\tools\7zip\7z.exe`,
		`D:\grokbuild\groklang\tools\7zip\7z.exe`,
	}
	root := toolkit.Root()
	if root != "" {
		fullFirst = append(fullFirst,
			filepath.Join(root, "tools", "7zip", "7z.exe"),
			filepath.Join(root, "tools", "7z", "7z.exe"),
			filepath.Join(root, "..", "tools", "7zip", "7z.exe"),
			filepath.Join(root, "..", "tools", "7z", "7z.exe"),
		)
	}
	cwd, _ := os.Getwd()
	fullFirst = append(fullFirst,
		filepath.Join(cwd, "tools", "7zip", "7z.exe"),
		filepath.Join(cwd, "..", "tools", "7zip", "7z.exe"),
		filepath.Join(cwd, "..", "..", "tools", "7zip", "7z.exe"),
	)
	if runtime.GOOS == "windows" {
		if p, err := exec.LookPath("7z.exe"); err == nil {
			fullFirst = append(fullFirst, p)
		}
	} else if p, err := exec.LookPath("7z"); err == nil {
		fullFirst = append(fullFirst, p)
	}
	for _, c := range fullFirst {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}

	// Fallback: 7za (no NSIS)
	fallback := []string{}
	if root != "" {
		fallback = append(fallback,
			filepath.Join(root, "tools", "7z", "7za.exe"),
			filepath.Join(root, "tools", "7z", "x64", "7za.exe"),
			filepath.Join(root, "..", "tools", "7z", "7za.exe"),
			filepath.Join(root, "..", "tools", "7z", "x64", "7za.exe"),
		)
	}
	fallback = append(fallback,
		filepath.Join(cwd, "tools", "7z", "7za.exe"),
		filepath.Join(cwd, "..", "tools", "7z", "7za.exe"),
		filepath.Join(cwd, "..", "tools", "7z", "x64", "7za.exe"),
		`D:\grokbuild\groklang\tools\7z\7za.exe`,
		`D:\grokbuild\groklang\tools\7z\x64\7za.exe`,
	)
	if runtime.GOOS == "windows" {
		if p, err := exec.LookPath("7za.exe"); err == nil {
			fallback = append(fallback, p)
		}
	} else if p, err := exec.LookPath("7za"); err == nil {
		fallback = append(fallback, p)
	}
	for _, c := range fallback {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return ""
}

// archiveExtract(archive, out_dir, extra_args_array?)
func archiveExtract(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("archive.extract(archive, out_dir)")
	}
	arc := args[0].AsStr()
	out := args[1].AsStr()
	seven := find7za()
	if seven == "" {
		return vm.MapVal(map[string]vm.Value{
			"ok":    vm.Bool(false),
			"error": vm.Str("7za not found — place tools/7z/7za.exe under gltk or groklang"),
		}), nil
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return vm.Null(), err
	}
	// 7za x -y -oOUT archive
	cmdArgs := []string{"x", "-y", "-o" + out, arc}
	if len(args) >= 3 && args[2].Typ == vm.TypeArray && args[2].Arr != nil {
		for _, v := range *args[2].Arr {
			cmdArgs = append(cmdArgs, v.AsStr())
		}
	}
	cmd := exec.Command(seven, cmdArgs...)
	outb, err := cmd.CombinedOutput()
	res := map[string]vm.Value{
		"ok":      vm.Bool(err == nil),
		"cmdline": vm.Str(seven + " " + strings.Join(cmdArgs, " ")),
		"output":  vm.Str(truncate(string(outb), 8000)),
		"out":     vm.Str(out),
		"seven":   vm.Str(seven),
	}
	if err != nil {
		res["error"] = vm.Str(err.Error())
		if ee, ok := err.(*exec.ExitError); ok {
			res["exit"] = vm.Int(int64(ee.ExitCode()))
		}
	} else {
		res["exit"] = vm.Int(0)
	}
	return vm.MapVal(res), nil
}

func archiveList(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("archive.list(archive)")
	}
	seven := find7za()
	if seven == "" {
		return vm.Null(), errf("7za not found")
	}
	cmd := exec.Command(seven, "l", "-ba", args[0].AsStr())
	outb, err := cmd.CombinedOutput()
	if err != nil {
		return vm.MapVal(map[string]vm.Value{
			"ok":     vm.Bool(false),
			"error":  vm.Str(err.Error()),
			"output": vm.Str(truncate(string(outb), 8000)),
		}), nil
	}
	lines := strings.Split(string(outb), "\n")
	arr := make([]vm.Value, 0, len(lines))
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln != "" {
			arr = append(arr, vm.Str(ln))
		}
	}
	return vm.MapVal(map[string]vm.Value{
		"ok":    vm.Bool(true),
		"lines": vm.Array(arr),
		"count": vm.Int(int64(len(arr))),
	}), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + fmt.Sprintf("\n...(%d more bytes)", len(s)-n)
}
