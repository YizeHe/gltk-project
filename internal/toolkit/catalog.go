// Package toolkit discovers and launches bundled reverse-engineering tools.
package toolkit

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Category groups bundled assets.
type Category string

const (
	CatAndroid  Category = "android"
	CatWeChat   Category = "wechat"
	CatWindows  Category = "windows"
	CatCourse   Category = "course"
	CatResource Category = "resource"
	CatBuiltin  Category = "builtin"
)

// Entry describes one tool, lab binary, course chapter, or resource pack.
type Entry struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Category    Category `json:"category"`
	Description string   `json:"description"`
	// RelPath is relative to GLTK root (directory or primary file).
	RelPath string `json:"path"`
	// Kind: jar | bat | exe | script | gui | dir | course | resource
	Kind string `json:"kind"`
	// Launch is the preferred launcher path relative to root (optional).
	Launch string `json:"launch,omitempty"`
	// JavaJar if set means: java -jar <Launch or RelPath>
	JavaJar bool `json:"java_jar,omitempty"`
	// GUI hints interactive tool (may block).
	GUI bool `json:"gui,omitempty"`
	// Tags for filtering.
	Tags []string `json:"tags,omitempty"`
	// Available is filled by Scan when path exists.
	Available bool `json:"available"`
	// AbsPath filled by Scan.
	AbsPath string `json:"abs_path,omitempty"`
}

// Root finds the GrokLangToolKit installation directory.
func Root() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, cand := range []string{dir, filepath.Dir(dir)} {
			if isRoot(cand) {
				return cand
			}
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	dir := cwd
	for {
		if isRoot(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return cwd
}

func isRoot(dir string) bool {
	b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err == nil && strings.Contains(string(b), "module groklang/gltk") {
		return true
	}
	// bundles present even without go.mod check
	if st, err := os.Stat(filepath.Join(dir, "droidReverse")); err == nil && st.IsDir() {
		if st2, err2 := os.Stat(filepath.Join(dir, "samples")); err2 == nil && st2.IsDir() {
			return true
		}
	}
	return false
}

// Definitions is the static registry of known bundled tools (paths may be missing).
func Definitions() []Entry {
	return []Entry{
		// --- Android (droidReverse) ---
		{
			ID: "apktool", Name: "Apktool", Category: CatAndroid,
			Description: "Decode/rebuild Android APK resources and smali",
			RelPath: "droidReverse/apktool.jar", Launch: "droidReverse/apktool.jar",
			Kind: "jar", JavaJar: true, Tags: []string{"apk", "smali", "android"},
		},
		{
			ID: "dex2jar", Name: "dex2jar (d2j-dex2jar)", Category: CatAndroid,
			Description: "Convert DEX to JAR",
			RelPath: "droidReverse/dex2jar-0.0.9.15", Launch: "droidReverse/dex2jar-0.0.9.15/d2j-dex2jar.bat",
			Kind: "bat", Tags: []string{"dex", "jar", "android"},
		},
		{
			ID: "dex2jar-suite", Name: "dex2jar suite", Category: CatAndroid,
			Description: "Full dex2jar tools directory (dex-dump, jar2dex, ...)",
			RelPath: "droidReverse/dex2jar-0.0.9.15", Kind: "dir", Tags: []string{"dex", "android"},
		},
		{
			ID: "jadx", Name: "jadx (CLI)", Category: CatAndroid,
			Description: "Dex/APK decompiler CLI",
			RelPath: "droidReverse/jadx/bin/jadx.bat", Launch: "droidReverse/jadx/bin/jadx.bat",
			Kind: "bat", Tags: []string{"decompile", "android"},
		},
		{
			ID: "jadx-gui", Name: "jadx-gui", Category: CatAndroid,
			Description: "Dex/APK decompiler GUI",
			RelPath: "droidReverse/jadx/bin/jadx-gui.bat", Launch: "droidReverse/jadx/bin/jadx-gui.bat",
			Kind: "bat", GUI: true, Tags: []string{"decompile", "gui", "android"},
		},
		{
			ID: "jadx-0.6.1", Name: "jadx 0.6.1 (legacy)", Category: CatAndroid,
			Description: "Older jadx bundle",
			RelPath: "droidReverse/jadx-0.6.1/bin/jadx.bat", Launch: "droidReverse/jadx-0.6.1/bin/jadx.bat",
			Kind: "bat", Tags: []string{"decompile", "android", "legacy"},
		},
		{
			ID: "classyshark", Name: "ClassyShark", Category: CatAndroid,
			Description: "Google APK/dex browser and dependency inspector",
			RelPath: "droidReverse/ClassyShark.jar", Launch: "droidReverse/ClassyShark.jar",
			Kind: "jar", JavaJar: true, GUI: true, Tags: []string{"apk", "gui", "android"},
		},
		{
			ID: "jd-gui", Name: "JD-GUI", Category: CatAndroid,
			Description: "Java decompiler GUI for .class/.jar",
			RelPath: "droidReverse/jd-gui", Kind: "dir", GUI: true, Tags: []string{"java", "gui"},
		},
		{
			ID: "jeb", Name: "JEB (demo)", Category: CatAndroid,
			Description: "JEB reverse engineering platform (bundled demo)",
			RelPath: "droidReverse/jeb", Launch: "droidReverse/jeb/jeb_wincon.bat",
			Kind: "bat", GUI: true, Tags: []string{"android", "gui", "commercial-demo"},
		},
		{
			ID: "smali2java", Name: "Smali2Java UI", Category: CatAndroid,
			Description: "Smali to Java GUI helper",
			RelPath: "droidReverse/smali2java/Smali2JavaUI.exe", Launch: "droidReverse/smali2java/Smali2JavaUI.exe",
			Kind: "exe", GUI: true, Tags: []string{"smali", "android", "gui"},
		},
		{
			ID: "droid-bundle", Name: "droidReverse bundle", Category: CatAndroid,
			Description: "Full Android reverse toolkit directory",
			RelPath: "droidReverse", Kind: "dir", Tags: []string{"android", "meta"},
		},

		// --- WeChat ---
		{
			ID: "wxapkg-gui", Name: "wxapkg (Wails GUI)", Category: CatWeChat,
			Description: "WeChat mini-program .wxapkg decrypt/unpack desktop app source",
			RelPath: "wxapkg", Kind: "dir", GUI: true, Tags: []string{"wechat", "wxapkg"},
		},
		{
			ID: "wxapkg-core", Name: "wxapkg core (GLVM native)", Category: CatWeChat,
			Description: "Use GrokLang: import wxapkg — decrypt/list/unpack without GUI",
			RelPath: "internal/native/wxapkg.go", Kind: "builtin", Tags: []string{"wechat", "wxapkg", "glk"},
		},

		// --- Windows practice samples ---
		{
			ID: "win-hello-x64", Name: "WinAPI hello_world x64", Category: CatWindows,
			Description: "Practice binary: hello world (x64)",
			RelPath: "Reverse-Engineering/0x0001-hello_world-x64 Debug/0x0001-hello_world-x64.exe",
			Kind: "exe", Tags: []string{"lab", "windows", "x64"},
		},
		{
			ID: "win-hello-x86", Name: "WinAPI hello_world x86", Category: CatWindows,
			Description: "Practice binary: hello world (x86)",
			RelPath: "Reverse-Engineering/0x0001-hello_world-x86 Debug/0x0001-hello_world-x86.exe",
			Kind: "exe", Tags: []string{"lab", "windows", "x86"},
		},
		{
			ID: "win-directories", Name: "WinAPI directories", Category: CatWindows,
			Description: "Practice: directory APIs sample",
			RelPath: "Reverse-Engineering/0x0006-directories-x64 Debug/0x0006-directories.exe",
			Kind: "exe", Tags: []string{"lab", "windows"},
		},
		{
			ID: "win-copyfile", Name: "WinAPI copyfile", Category: CatWindows,
			Description: "Practice: CopyFile sample",
			RelPath: "Reverse-Engineering/0x0007-copyfile-x64 Debug/0x000b-copyfile.exe",
			Kind: "exe", Tags: []string{"lab", "windows"},
		},
		{
			ID: "win-movefile", Name: "WinAPI movefile", Category: CatWindows,
			Description: "Practice: MoveFile sample",
			RelPath: "Reverse-Engineering/0x0010-movefile-x64 Debug/0x0010-movefile.exe",
			Kind: "exe", Tags: []string{"lab", "windows"},
		},
		{
			ID: "win-createfile", Name: "WinAPI createfile", Category: CatWindows,
			Description: "Practice: CreateFile sample",
			RelPath: "Reverse-Engineering/0x0011-createfile-x64 Debug/0x0011-createfile.exe",
			Kind: "exe", Tags: []string{"lab", "windows"},
		},
		{
			ID: "win-writefile", Name: "WinAPI writefile", Category: CatWindows,
			Description: "Practice: WriteFile sample",
			RelPath: "Reverse-Engineering/0x0012-writefile-x64 Debug/0x0012-writefile.exe",
			Kind: "exe", Tags: []string{"lab", "windows"},
		},
		{
			ID: "win-readfile", Name: "WinAPI readfile", Category: CatWindows,
			Description: "Practice: ReadFile sample",
			RelPath: "Reverse-Engineering/0x0013-readfile-x86 Debug/0x0013-readfile.exe",
			Kind: "exe", Tags: []string{"lab", "windows"},
		},
		{
			ID: "win-labs", Name: "Reverse-Engineering labs root", Category: CatWindows,
			Description: "Windows API reverse-engineering sample projects + binaries",
			RelPath: "Reverse-Engineering", Kind: "dir", Tags: []string{"lab", "windows", "meta"},
		},

		// --- Courses / resources ---
		{
			ID: "z0f-course", Name: "Z0F Reverse Engineering Course", Category: CatCourse,
			Description: "Beginner→intermediate x64 Windows RE course (markdown)",
			RelPath: "Z0FCourse_ReverseEngineering", Kind: "course",
			Tags: []string{"course", "windows", "assembly"},
		},
		{
			ID: "z0f-toc", Name: "Z0F Table of Contents", Category: CatCourse,
			Description: "Course table of contents",
			RelPath: "Z0FCourse_ReverseEngineering/TableOfContents.md", Kind: "resource",
			Tags: []string{"course"},
		},
		{
			ID: "z0f-ch1", Name: "Z0F Chapter 1 - Introduction", Category: CatCourse,
			Description: "Course chapter 1",
			RelPath: "Z0FCourse_ReverseEngineering/Chapter 1 - Introduction", Kind: "course",
		},
		{
			ID: "z0f-ch2", Name: "Z0F Chapter 2 - Binary Basics", Category: CatCourse,
			Description: "Course chapter 2",
			RelPath: "Z0FCourse_ReverseEngineering/Chapter 2 - BinaryBasics", Kind: "course",
		},
		{
			ID: "z0f-ch3", Name: "Z0F Chapter 3 - Assembly", Category: CatCourse,
			Description: "Course chapter 3",
			RelPath: "Z0FCourse_ReverseEngineering/Chapter 3 - Assembly", Kind: "course",
		},
		{
			ID: "z0f-ch4", Name: "Z0F Chapter 4 - Tools", Category: CatCourse,
			Description: "Course chapter 4",
			RelPath: "Z0FCourse_ReverseEngineering/Chapter 4 - Tools", Kind: "course",
		},
		{
			ID: "z0f-ch5", Name: "Z0F Chapter 5 - Basic Reversing", Category: CatCourse,
			Description: "Course chapter 5",
			RelPath: "Z0FCourse_ReverseEngineering/Chapter 5 - BasicReversing", Kind: "course",
		},
		{
			ID: "z0f-ch6", Name: "Z0F Chapter 6 - DLL", Category: CatCourse,
			Description: "Course chapter 6 + lab files",
			RelPath: "Z0FCourse_ReverseEngineering/Chapter 6 - DLL", Kind: "course",
		},
		{
			ID: "z0f-ch7", Name: "Z0F Chapter 7 - Windows", Category: CatCourse,
			Description: "Course chapter 7",
			RelPath: "Z0FCourse_ReverseEngineering/Chapter 7 - Windows", Kind: "course",
		},
		{
			ID: "z0f-ch8", Name: "Z0F Chapter 8 - Generic Table", Category: CatCourse,
			Description: "Course chapter 8",
			RelPath: "Z0FCourse_ReverseEngineering/Chapter 8 - Generic Table", Kind: "course",
		},
		{
			ID: "z0f-files", Name: "Z0F FilesNeeded labs", Category: CatCourse,
			Description: "Course binary/lab files (DLL samples etc.)",
			RelPath: "Z0FCourse_ReverseEngineering/FilesNeeded", Kind: "dir",
		},
		{
			ID: "awesome-re", Name: "Awesome Reverse Engineering list", Category: CatResource,
			Description: "Curated reversing books/tools/links (wtsxDev list)",
			RelPath: "resverse-engineering2/reverse-engineering/README.md", Kind: "resource",
			Tags: []string{"links", "books", "awesome"},
		},
		{
			ID: "awesome-re-root", Name: "Awesome RE pack root", Category: CatResource,
			Description: "Full awesome reverse-engineering repository checkout",
			RelPath: "resverse-engineering2/reverse-engineering", Kind: "dir",
		},

		// --- Built-in GLVM modules (reference) ---
		{
			ID: "glvm-pe", Name: "GLVM pe module", Category: CatBuiltin,
			Description: "import pe — PE parse/resources (GrokLang)",
			RelPath: "internal/native/pe.go", Kind: "builtin", Tags: []string{"glk"},
		},
		{
			ID: "glvm-ahk", Name: "GLVM ahk/ScriptGuard", Category: CatBuiltin,
			Description: "import ahk — ScriptGuard decrypt helpers",
			RelPath: "internal/native/ahk.go", Kind: "builtin", Tags: []string{"glk", "ahk"},
		},
	}
}

// Scan resolves paths against root and marks availability.
func Scan() []Entry {
	root := Root()
	defs := Definitions()
	out := make([]Entry, 0, len(defs))
	for _, e := range defs {
		e = resolveEntry(root, e)
		out = append(out, e)
	}
	// Discover extra lab exes under Reverse-Engineering *Debug folders not already listed
	out = append(out, discoverWinLabs(root, out)...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func resolveEntry(root string, e Entry) Entry {
	rel := filepath.FromSlash(e.RelPath)
	abs := filepath.Join(root, rel)
	if _, err := os.Stat(abs); err == nil {
		e.Available = true
		e.AbsPath = abs
	} else {
		e.Available = false
		e.AbsPath = abs
	}
	if e.Launch != "" {
		lp := filepath.Join(root, filepath.FromSlash(e.Launch))
		if _, err := os.Stat(lp); err == nil {
			e.Launch = lp
			if !e.Available {
				// launch exists even if RelPath missing
				e.Available = true
				e.AbsPath = lp
			}
		} else {
			// keep relative launch for display; mark unavailable if primary missing
			e.Launch = lp
		}
	} else if e.Available {
		e.Launch = e.AbsPath
	}
	// Platform tweak: prefer .sh on non-windows for bat tools if sibling exists
	if runtime.GOOS != "windows" && strings.HasSuffix(strings.ToLower(e.Launch), ".bat") {
		sh := strings.TrimSuffix(e.Launch, ".bat") + ".sh"
		if _, err := os.Stat(sh); err == nil {
			e.Launch = sh
			e.Kind = "script"
		}
	}
	return e
}

func discoverWinLabs(root string, existing []Entry) []Entry {
	seenID := map[string]bool{}
	seenBase := map[string]bool{} // basename.exe already covered by static defs
	for _, e := range existing {
		seenID[e.ID] = true
		if e.AbsPath != "" {
			seenBase[strings.ToLower(filepath.Base(e.AbsPath))] = true
		}
		if e.RelPath != "" {
			seenBase[strings.ToLower(filepath.Base(e.RelPath))] = true
		}
	}
	base := filepath.Join(root, "Reverse-Engineering")
	var extra []Entry
	// Only pick prebuilt *Debug folder siblings under Reverse-Engineering root
	// (avoid nested VS project Debug/ trees duplicating the same labs).
	ents, err := os.ReadDir(base)
	if err != nil {
		return extra
	}
	for _, de := range ents {
		if !de.IsDir() {
			continue
		}
		name := de.Name()
		if !strings.Contains(name, "Debug") {
			continue
		}
		dir := filepath.Join(base, name)
		files, _ := os.ReadDir(dir)
		for _, f := range files {
			if f.IsDir() || !strings.EqualFold(filepath.Ext(f.Name()), ".exe") {
				continue
			}
			path := filepath.Join(dir, f.Name())
			baseLower := strings.ToLower(f.Name())
			if seenBase[baseLower] {
				continue
			}
			rel, _ := filepath.Rel(root, path)
			id := "win-lab-" + slug(f.Name())
			if seenID[id] {
				continue
			}
			seenID[id] = true
			seenBase[baseLower] = true
			extra = append(extra, Entry{
				ID: id, Name: f.Name(), Category: CatWindows,
				Description: "Discovered Windows lab binary (" + name + ")",
				RelPath:     filepath.ToSlash(rel),
				Kind:        "exe", Launch: path, AbsPath: path, Available: true,
				Tags: []string{"lab", "windows", "discovered"},
			})
		}
	}
	return extra
}

func slug(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := b.String()
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	return strings.Trim(out, "-")
}

// Get returns an entry by id (scanned).
func Get(id string) (Entry, bool) {
	for _, e := range Scan() {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

// Filter by category and/or substring.
func Filter(cat string, query string) []Entry {
	cat = strings.ToLower(strings.TrimSpace(cat))
	query = strings.ToLower(strings.TrimSpace(query))
	var out []Entry
	for _, e := range Scan() {
		if cat != "" && string(e.Category) != cat {
			continue
		}
		if query != "" {
			blob := strings.ToLower(e.ID + " " + e.Name + " " + e.Description + " " + strings.Join(e.Tags, " "))
			if !strings.Contains(blob, query) {
				continue
			}
		}
		out = append(out, e)
	}
	return out
}

// FormatList returns a human-readable table.
func FormatList(entries []Entry) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%-18s %-10s %-8s %s\n", "ID", "CATEGORY", "READY", "NAME"))
	b.WriteString(strings.Repeat("-", 72) + "\n")
	for _, e := range entries {
		ready := "no"
		if e.Available {
			ready = "yes"
		}
		b.WriteString(fmt.Sprintf("%-18s %-10s %-8s %s\n", e.ID, e.Category, ready, e.Name))
	}
	b.WriteString(fmt.Sprintf("\nroot: %s\ntotal: %d (use: gltk tools info <id> | gltk tools run <id> -- args)\n", Root(), len(entries)))
	return b.String()
}
