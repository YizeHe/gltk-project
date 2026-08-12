// Package cli implements gltk command-line commands.
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"groklang/gltk/internal/bytecode"
	"groklang/gltk/internal/compiler"
	"groklang/gltk/internal/module"
	"groklang/gltk/internal/native"
	"groklang/gltk/internal/obfus"
	"groklang/gltk/internal/packer"
	"groklang/gltk/internal/parser"
	"groklang/gltk/internal/toolkit"
	"groklang/gltk/internal/vm"
	"groklang/gltk/internal/wxapkg"
)

const Version = "1.0.0"

// Run dispatches CLI based on os.Args (args[0] is program name).
func Run(args []string) int {
	if len(args) < 2 {
		printUsage()
		return 1
	}
	cmd := args[1]
	rest := args[2:]
	switch cmd {
	case "version", "-v", "--version":
		fmt.Printf("gltk %s (GrokLangToolKit / GLVM)\n", Version)
		return 0
	case "help", "-h", "--help":
		printUsage()
		return 0
	case "run":
		return cmdRun(rest)
	case "compile":
		return cmdCompile(rest)
	case "disasm":
		return cmdDisasm(rest)
	case "bench":
		return cmdBench(rest)
	case "tool":
		return cmdTool(rest)
	case "tools":
		return cmdTools(rest)
	case "course":
		return cmdCourse(rest)
	case "wxapkg":
		return cmdWxapkg(rest)
	case "repl":
		return cmdRepl(rest)
	case "pack":
		return cmdPack(rest)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", cmd)
		printUsage()
		return 1
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `GrokLangToolKit (gltk) — reverse-engineering language toolkit

Usage:
  gltk run <file.glk|.glkb> [-- args...]
  gltk compile <file.glk> -o out.glkb
  gltk pack <file.glk|.glkb> -o out.exe [--no-obfus] [--stub path] [--keep-glkb path]
  gltk disasm <file.glkb>
  gltk version
  gltk bench <file.glk|.glkb>
  gltk repl

Bundled reverse tools (droidReverse / wxapkg / Win labs / Z0F course / awesome list):
  gltk tools list [--category android|wechat|windows|course|resource|builtin] [-q query]
  gltk tools info <id>
  gltk tools path <id>
  gltk tools run <id> -- [args...]
  gltk course list
  gltk course path <id>

Other:
  gltk tool <name> [args]              # python side plugins under tools/python
  gltk wxapkg unpack <file> <outdir> [wxid]

Execution always goes through GLVM bytecode (register VM), never AST eval.
Libraries: import "path/lib.glk" [as alias]; search via GLTK_LIB and stdlib/libs.
GrokLang: import tools  → tools.list() / tools.run(id, args)
`)
}

func cmdRun(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: gltk run <file.glk|.glkb> [-- args...]")
		return 1
	}
	path := args[0]
	scriptArgs := []string{}
	for i := 1; i < len(args); i++ {
		if args[i] == "--" {
			scriptArgs = args[i+1:]
			break
		}
		scriptArgs = append(scriptArgs, args[i])
	}
	chunk, err := loadChunk(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	v := newVM(chunk)
	vals := make([]vm.Value, len(scriptArgs))
	for i, s := range scriptArgs {
		vals[i] = vm.Str(s)
	}
	res, err := v.Run(vals)
	if err != nil {
		fmt.Fprintln(os.Stderr, "runtime error:", err)
		return 1
	}
	if res.Typ == vm.TypeInt && res.I != 0 {
		return int(res.I)
	}
	return 0
}

func cmdCompile(args []string) int {
	in := ""
	out := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "-o" && i+1 < len(args) {
			out = args[i+1]
			i++
			continue
		}
		if in == "" {
			in = args[i]
		}
	}
	if in == "" {
		fmt.Fprintln(os.Stderr, "usage: gltk compile <file.glk> -o out.glkb")
		return 1
	}
	if out == "" {
		out = strings.TrimSuffix(in, filepath.Ext(in)) + ".glkb"
	}
	chunk, err := loadChunk(in)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	data, err := bytecode.Encode(chunk)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("wrote %s (%d bytes, %d protos, %d consts)\n", out, len(data), len(chunk.Protos), len(chunk.Consts))
	return 0
}

func cmdDisasm(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: gltk disasm <file.glkb|.glk>")
		return 1
	}
	chunk, err := loadChunk(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Print(bytecode.Disassemble(chunk))
	return 0
}

func cmdBench(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: gltk bench <file.glk|.glkb>")
		return 1
	}
	chunk, err := loadChunk(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	v := newVM(chunk)
	start := time.Now()
	_, err = v.Run(nil)
	elapsed := time.Since(start)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("ops=%d time=%s ops/s=%.0f\n", v.Ops, elapsed, float64(v.Ops)/elapsed.Seconds())
	return 0
}

func cmdTools(args []string) int {
	if len(args) < 1 {
		args = []string{"list"}
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "list", "ls":
		cat, q := "", ""
		for i := 0; i < len(rest); i++ {
			switch rest[i] {
			case "--category", "-c":
				if i+1 < len(rest) {
					cat = rest[i+1]
					i++
				}
			case "-q", "--query":
				if i+1 < len(rest) {
					q = rest[i+1]
					i++
				}
			default:
				if cat == "" && !strings.HasPrefix(rest[i], "-") {
					cat = rest[i]
				}
			}
		}
		fmt.Print(toolkit.FormatList(toolkit.Filter(cat, q)))
		if !toolkit.JavaOK() {
			fmt.Println("note: java not on PATH — jar tools (apktool/ClassyShark) need JDK/JRE")
		}
		return 0
	case "info":
		if len(rest) < 1 {
			fmt.Fprintln(os.Stderr, "usage: gltk tools info <id>")
			return 1
		}
		e, ok := toolkit.Get(rest[0])
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown tool %q\n", rest[0])
			return 1
		}
		fmt.Printf("id:          %s\nname:        %s\ncategory:    %s\nkind:        %s\navailable:   %v\ngui:         %v\njava_jar:    %v\npath:        %s\nlaunch:      %s\ndescription: %s\ntags:        %s\n",
			e.ID, e.Name, e.Category, e.Kind, e.Available, e.GUI, e.JavaJar, e.AbsPath, e.Launch, e.Description, strings.Join(e.Tags, ", "))
		return 0
	case "path":
		if len(rest) < 1 {
			fmt.Fprintln(os.Stderr, "usage: gltk tools path <id>")
			return 1
		}
		p, err := toolkit.PathOf(rest[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println(p)
		return 0
	case "run":
		if len(rest) < 1 {
			fmt.Fprintln(os.Stderr, "usage: gltk tools run <id> -- [args...]")
			return 1
		}
		id := rest[0]
		toolArgs := []string{}
		for i := 1; i < len(rest); i++ {
			if rest[i] == "--" {
				toolArgs = rest[i+1:]
				break
			}
			toolArgs = append(toolArgs, rest[i])
		}
		res, err := toolkit.Run(id, toolArgs, toolkit.RunOptions{Capture: false})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if res.CmdLine != "" {
			fmt.Fprintln(os.Stderr, "cmdline:", res.CmdLine)
		}
		return res.ExitCode
	case "root":
		fmt.Println(toolkit.Root())
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown tools subcommand %q\n", sub)
		fmt.Fprintln(os.Stderr, "usage: gltk tools list|info|path|run|root")
		return 1
	}
}

func cmdCourse(args []string) int {
	if len(args) < 1 {
		args = []string{"list"}
	}
	switch args[0] {
	case "list", "ls":
		fmt.Print(toolkit.FormatList(toolkit.Filter("course", "")))
		fmt.Println("\nAlso: gltk tools list --category resource  (awesome-re links)")
		fmt.Println("Docs: Z0FCourse_ReverseEngineering/README.md")
		return 0
	case "path", "info":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: gltk course path <id>")
			return 1
		}
		return cmdTools([]string{args[0], args[1]})
	default:
		// treat as id
		return cmdTools([]string{"info", args[0]})
	}
}

func cmdTool(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: gltk tool <name> [args]  (or: gltk tools list)")
		return 1
	}
	name := args[0]
	// Go-side wxapkg tool
	if name == "wxapkg" {
		return cmdWxapkg(args[1:])
	}
	// Prefer bundled toolkit id if registered
	if e, ok := toolkit.Get(name); ok && e.Available && (e.Kind == "jar" || e.Kind == "bat" || e.Kind == "exe" || e.Kind == "script") {
		return cmdTools(append([]string{"run", name, "--"}, args[1:]...))
	}
	// find tools/python relative to executable or cwd
	candidates := []string{
		filepath.Join("tools", "python", name+".py"),
		filepath.Join("tools", "python", "gltk_tool_server.py"),
	}
	var script string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			script = c
			break
		}
	}
	if script == "" {
		fmt.Fprintf(os.Stderr, "tool not found: %s (looked under tools/python/)\n", name)
		return 1
	}
	cmdArgs := []string{script}
	if name != "gltk_tool_server" && !strings.HasSuffix(script, name+".py") {
		// generic server: pass tool name
		cmdArgs = append(cmdArgs, name)
	}
	cmdArgs = append(cmdArgs, args[1:]...)
	cmd := exec.Command("python", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func cmdWxapkg(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: gltk wxapkg unpack <file> <outdir> [wxid]")
		return 1
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "unpack":
		if len(rest) < 2 {
			fmt.Fprintln(os.Stderr, "usage: gltk wxapkg unpack <file> <outdir> [wxid]")
			return 1
		}
		file, outdir := rest[0], rest[1]
		opts := wxapkg.UnpackOptions{Decrypt: false, BeautifyJSON: true}
		if len(rest) >= 3 {
			opts.Wxid = rest[2]
			opts.Decrypt = true
		}
		r := wxapkg.Unpack(file, outdir, opts)
		if !r.OK {
			fmt.Fprintln(os.Stderr, "unpack failed:", r.Error)
			return 1
		}
		fmt.Printf("unpacked %d files -> %s\n", r.Count, r.SavePath)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown wxapkg subcommand %q\n", sub)
		fmt.Fprintln(os.Stderr, "usage: gltk wxapkg unpack <file> <outdir> [wxid]")
		return 1
	}
}

func cmdRepl(args []string) int {
	fmt.Println("GrokLang REPL (GLVM). Type statements; :quit to exit")
	buf := make([]byte, 8192)
	for {
		fmt.Print("glk> ")
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			break
		}
		line := strings.TrimSpace(string(buf[:n]))
		if line == ":quit" || line == ":q" {
			break
		}
		if line == "" {
			continue
		}
		src := "import out\nfn main(args) {\n" + line + "\nreturn 0\n}\n"
		prog, err := parser.Parse(src)
		if err != nil {
			fmt.Println("parse:", err)
			continue
		}
		res, err := compiler.Compile(prog, compiler.CompileOptions{Filename: "<repl>"})
		if err != nil {
			fmt.Println("compile:", err)
			continue
		}
		v := newVM(res.Chunk)
		if _, err := v.Run(nil); err != nil {
			fmt.Println("runtime:", err)
		}
	}
	return 0
}

func cmdPack(args []string) int {
	in, out, stub, keep := "", "", "", ""
	noObfus := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-o" && i+1 < len(args):
			out = args[i+1]
			i++
		case a == "--stub" && i+1 < len(args):
			stub = args[i+1]
			i++
		case a == "--keep-glkb" && i+1 < len(args):
			keep = args[i+1]
			i++
		case a == "--no-obfus", a == "--no-obfuscate":
			noObfus = true
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "unknown pack flag %q\n", a)
			return 1
		default:
			if in == "" {
				in = a
			}
		}
	}
	if in == "" {
		fmt.Fprintln(os.Stderr, "usage: gltk pack <file.glk|.glkb> -o out.exe [--no-obfus] [--stub glrt.exe]")
		return 1
	}
	res, err := packer.Pack(packer.Options{
		Input:    in,
		Output:   out,
		StubPath: stub,
		NoObfus:  noObfus,
		KeepGLKB: keep,
		Obfus:    obfus.Default(),
		Verbose:  true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("packed %s\n  input=%s obfuscated=%v\n  glkb=%d cipher=%d exe=%d protos=%d consts=%d time=%s\n",
		res.Output, res.InputKind, res.Obfuscated, res.PlainGLKB, res.CipherBytes, res.ExeBytes, res.Protos, res.Consts, res.Elapsed)
	return 0
}

func loadChunk(path string) (*bytecode.Chunk, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) >= 4 && string(data[:4]) == bytecode.Magic {
		return bytecode.Decode(data)
	}
	// multi-file aware compile for .glk source
	return module.CompileFile(path, module.Options{
		Filename:    path,
		SearchPaths: module.DefaultSearchPaths(path),
	})
}

func newVM(chunk *bytecode.Chunk) *vm.VM {
	v := vm.New(chunk, nil)
	native.InstallGlobals(v)
	return v
}
