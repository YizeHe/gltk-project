package native

import (
	"bytes"
	"encoding/binary"
	"os"
	"strings"

	"groklang/gltk/internal/vm"
)

func moduleFile() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"detect":      fileDetect,
		"detect_path": fileDetectPath,
		"size":        fileSize,
		"markers":     fileMarkers,
	})
}

func fileSize(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Int(0), nil
	}
	st, err := os.Stat(args[0].AsStr())
	if err != nil {
		return vm.Int(-1), nil
	}
	return vm.Int(st.Size()), nil
}

func fileDetectPath(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("file.detect_path(path)")
	}
	path := args[0].AsStr()
	f, err := os.Open(path)
	if err != nil {
		return vm.Null(), err
	}
	defer f.Close()
	st, _ := f.Stat()
	// Read 256KiB head for installer markers (NSIS/Inno often past first 4K)
	headN := 256 * 1024
	if st != nil && st.Size() < int64(headN) {
		headN = int(st.Size())
	}
	head := make([]byte, headN)
	n, _ := f.Read(head)
	head = head[:n]
	// also sample near end + start of potential PE overlay (~100KB)
	var tail []byte
	if st != nil && st.Size() > 8192 {
		tail = make([]byte, 64*1024)
		off := st.Size() - int64(len(tail))
		if off < 0 {
			off = 0
		}
		if _, err := f.ReadAt(tail, off); err != nil {
			tail = nil
		}
		// mid sample for large SFX (around 1MB offset often has NSIS/7z body)
		if st.Size() > 2<<20 {
			mid := make([]byte, 32*1024)
			if _, err := f.ReadAt(mid, 1<<20); err == nil {
				tail = append(tail, mid...)
			}
		}
	}
	info := detectBytes(head, tail, st.Size())
	info["path"] = vm.Str(path)
	if st != nil {
		info["size"] = vm.Int(st.Size())
	}
	return vm.MapVal(info), nil
}

func fileDetect(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("file.detect(bytes)")
	}
	bs, err := args[0].AsBytes()
	if err != nil {
		return vm.Null(), err
	}
	var tail []byte
	if len(bs) > 8192 {
		tail = bs[len(bs)-4096:]
	}
	return vm.MapVal(detectBytes(bs, tail, int64(len(bs)))), nil
}

func detectBytes(head, tail []byte, size int64) map[string]vm.Value {
	fmtName := "unknown"
	sub := ""
	notes := []string{}

	if len(head) >= 2 && head[0] == 'M' && head[1] == 'Z' {
		fmtName = "pe"
		// try PE signature
		if len(head) >= 0x40 {
			e := binary.LittleEndian.Uint32(head[0x3C:])
			if int(e)+4 <= len(head) && string(head[e:e+4]) == "PE\x00\x00" {
				fmtName = "pe"
			} else {
				fmtName = "mz" // DOS MZ, PE header not in head window
			}
		}
	} else if len(head) >= 8 && head[0] == 0xD0 && head[1] == 0xCF && head[2] == 0x11 && head[3] == 0xE0 {
		fmtName = "cfb" // OLE Compound File (MSI/DOC/…)
		sub = "ole"
		if hasASCII(head, "MSI") || hasASCII(head, "Installation Database") {
			sub = "msi"
		}
	} else if len(head) >= 4 && head[0] == 0x50 && head[1] == 0x4B && (head[2] == 0x03 || head[2] == 0x05 || head[2] == 0x07) {
		fmtName = "zip"
	} else if len(head) >= 6 && string(head[:6]) == "7z\xbc\xaf\x27\x1c" {
		fmtName = "7z"
	} else if len(head) >= 4 && string(head[:4]) == "Rar!" {
		fmtName = "rar"
	} else if len(head) >= 4 && head[0] == 0x7F && string(head[1:4]) == "ELF" {
		fmtName = "elf"
	} else if len(head) >= 4 && (string(head[:4]) == "\x00asm" || string(head[:4]) == "%PDF") {
		if string(head[:4]) == "%PDF" {
			fmtName = "pdf"
		}
	}

	// installer / framework markers in head+tail
	blob := append([]byte{}, head...)
	blob = append(blob, tail...)
	markers := collectMarkers(blob)
	if fmtName == "pe" || fmtName == "mz" {
		for _, m := range markers {
			switch m {
			case "NullsoftInst", "NSIS", "Nullsoft Install System", "Nullsoft.NSIS.exehead":
				sub = "nsis"
			case "Inno Setup", "InnoSetupLdr":
				if sub == "" {
					sub = "inno"
				}
			case ";!@Install@!UTF-8!", "7z Setup SFX", "7zS2.sfx":
				if sub == "" {
					sub = "7z-sfx"
				}
			case "Electron", "chrome_elf", "app.asar":
				if sub == "" || sub == "nsis" {
					// electron often NSIS outer
					if sub == "nsis" {
						notes = append(notes, "possibly Electron app installer")
					} else {
						sub = "electron"
					}
				}
			case "InstallShield":
				if sub == "" {
					sub = "installshield"
				}
			case "WinRAR SFX", "Rar!":
				if sub == "" {
					sub = "rar-sfx"
				}
			case "bat2exe", "Bat To Exe", "Advanced BAT to EXE":
				sub = "bat2exe"
			case "AutoHotkey", "AHK":
				if sub == "" {
					sub = "autohotkey"
				}
			case "AutoIt":
				if sub == "" {
					sub = "autoit"
				}
			case "!This program cannot be run in DOS mode":
				// common
			}
		}
	}

	arr := make([]vm.Value, len(markers))
	for i, m := range markers {
		arr[i] = vm.Str(m)
	}
	na := make([]vm.Value, len(notes))
	for i, n := range notes {
		na[i] = vm.Str(n)
	}
	return map[string]vm.Value{
		"format":  vm.Str(fmtName),
		"subtype": vm.Str(sub),
		"size":    vm.Int(size),
		"markers": vm.Array(arr),
		"notes":   vm.Array(na),
		"magic":   vm.Str(hexPreview(head, 16)),
	}
}

func fileMarkers(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Array(nil), nil
	}
	bs, err := args[0].AsBytes()
	if err != nil {
		return vm.Null(), err
	}
	ms := collectMarkers(bs)
	arr := make([]vm.Value, len(ms))
	for i, m := range ms {
		arr[i] = vm.Str(m)
	}
	return vm.Array(arr), nil
}

func collectMarkers(b []byte) []string {
	cands := []string{
		"NullsoftInst", "Nullsoft Install System", "NSIS Error",
		"Inno Setup Setup Data", "Inno Setup", "InnoSetupLdr",
		"InstallShield", "Wise Installation",
		"Electron", "chrome_elf.dll", "app.asar", "ELECTRON_RUN_AS_NODE",
		"WinRAR", "Rar!", "SFX", "WinRAR SFX",
		"AutoHotkey", "Ahk2Exe", ">AUTOHOTKEY SCRIPT<", "ScriptGuard",
		"AutoIt", "AU3!",
		"bat2exe", "Bat To Exe Converter", "Advanced BAT to EXE Converter",
		"UPX0", "UPX1", "UPX!",
		"This program cannot be run in DOS mode",
		"mscoree.dll", ".NET", "clr.dll",
		"python", "PyInstaller", "MEI\x0c\x0b",
		"node.dll", "nw.pak", "v8::",
		"KeePass", "KeePassXC",
		"Microsoft Cabinet", "MSC!",
		"#!", "PK\x03\x04",
		"!@Install@", "7z\xbc\xaf\x27\x1c",
		"WiX", "Windows Installer", "Installation Database",
	}
	var found []string
	seen := map[string]bool{}
	for _, c := range cands {
		if len(c) == 0 {
			continue
		}
		if bytes.Contains(b, []byte(c)) {
			if !seen[c] {
				seen[c] = true
				found = append(found, c)
			}
		}
		// case-insensitive for pure ascii letters
		if isAlphaName(c) && bytes.Contains(bytes.ToLower(b), bytes.ToLower([]byte(c))) {
			if !seen[c] {
				seen[c] = true
				found = append(found, c)
			}
		}
	}
	return found
}

func isAlphaName(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 128 {
			return false
		}
	}
	return true
}

func hasASCII(b []byte, s string) bool {
	return bytes.Contains(b, []byte(s)) || bytes.Contains(bytes.ToLower(b), []byte(strings.ToLower(s)))
}

func hexPreview(b []byte, n int) string {
	if len(b) < n {
		n = len(b)
	}
	const hexdigits = "0123456789ABCDEF"
	out := make([]byte, 0, n*3)
	for i := 0; i < n; i++ {
		if i > 0 {
			out = append(out, ' ')
		}
		out = append(out, hexdigits[b[i]>>4], hexdigits[b[i]&0xf])
	}
	return string(out)
}
