package toolkit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// RunResult is the outcome of launching a tool.
type RunResult struct {
	ExitCode int
	CmdLine  string
	Stdout   string
	Stderr   string
	Error    string
}

// RunOptions controls process execution.
type RunOptions struct {
	// Capture if true, collect stdout/stderr instead of inheriting.
	Capture bool
	// Dir working directory (default: tool dir or root).
	Dir string
	// Env extra env KEY=VAL.
	Env []string
}

// Run launches a catalog entry with args. For course/resource/dir kinds, returns error suggesting path.
func Run(id string, args []string, opt RunOptions) (*RunResult, error) {
	e, ok := Get(id)
	if !ok {
		return nil, fmt.Errorf("toolkit: unknown tool id %q (gltk tools list)", id)
	}
	if !e.Available && e.Kind != "builtin" {
		return nil, fmt.Errorf("toolkit: tool %q not found on disk: %s", id, e.AbsPath)
	}
	switch e.Kind {
	case "course", "resource", "dir", "builtin":
		return nil, fmt.Errorf("toolkit: %q is %s — path: %s (not an executable launcher)", id, e.Kind, e.AbsPath)
	}

	cmd, cmdline, err := buildCmd(e, args)
	if err != nil {
		return nil, err
	}
	if opt.Dir != "" {
		cmd.Dir = opt.Dir
	} else if e.Kind == "bat" || e.Kind == "script" {
		cmd.Dir = filepath.Dir(e.Launch)
	} else {
		cmd.Dir = Root()
	}
	if len(opt.Env) > 0 {
		cmd.Env = append(os.Environ(), opt.Env...)
	}

	res := &RunResult{CmdLine: cmdline}
	if opt.Capture {
		out, err := cmd.CombinedOutput()
		res.Stdout = string(out)
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				res.ExitCode = ee.ExitCode()
			} else {
				res.Error = err.Error()
				res.ExitCode = 1
			}
		}
		return res, nil
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			res.ExitCode = ee.ExitCode()
			return res, nil
		}
		res.Error = err.Error()
		res.ExitCode = 1
		return res, err
	}
	return res, nil
}

func buildCmd(e Entry, args []string) (*exec.Cmd, string, error) {
	launch := e.Launch
	if launch == "" {
		launch = e.AbsPath
	}
	if e.JavaJar || strings.HasSuffix(strings.ToLower(launch), ".jar") {
		jar := launch
		if !strings.HasSuffix(strings.ToLower(jar), ".jar") {
			jar = e.AbsPath
		}
		all := append([]string{"-jar", jar}, args...)
		cmd := exec.Command("java", all...)
		return cmd, "java " + strings.Join(all, " "), nil
	}
	switch strings.ToLower(filepath.Ext(launch)) {
	case ".bat", ".cmd":
		if runtime.GOOS != "windows" {
			return nil, "", fmt.Errorf("toolkit: %s requires Windows (or use .sh sibling)", launch)
		}
		// cmd.exe /c call bat args...
		all := append([]string{"/c", "call", launch}, args...)
		cmd := exec.Command("cmd.exe", all...)
		return cmd, "cmd /c call " + launch + " " + strings.Join(args, " "), nil
	case ".sh":
		all := append([]string{launch}, args...)
		cmd := exec.Command("bash", all...)
		return cmd, "bash " + launch + " " + strings.Join(args, " "), nil
	default:
		cmd := exec.Command(launch, args...)
		return cmd, launch + " " + strings.Join(args, " "), nil
	}
}

// PathOf returns absolute path for an id.
func PathOf(id string) (string, error) {
	e, ok := Get(id)
	if !ok {
		return "", fmt.Errorf("unknown tool %q", id)
	}
	if e.AbsPath != "" {
		return e.AbsPath, nil
	}
	return filepath.Join(Root(), filepath.FromSlash(e.RelPath)), nil
}

// JavaOK reports whether java is on PATH (for jar tools).
func JavaOK() bool {
	_, err := exec.LookPath("java")
	return err == nil
}
