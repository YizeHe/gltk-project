package native

import (
	"encoding/binary"
	"os"
	"strings"

	"groklang/gltk/internal/vm"
)

func moduleMSI() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"info": msiInfo,
	})
}

// msiInfo reads OLE CFB header + directory entry names (best-effort for MSI).
func msiInfo(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("msi.info(path)")
	}
	path := args[0].AsStr()
	f, err := os.Open(path)
	if err != nil {
		return vm.Null(), err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return vm.Null(), err
	}
	head := make([]byte, 512)
	if _, err := f.Read(head); err != nil {
		return vm.Null(), err
	}
	if len(head) < 76 || head[0] != 0xD0 || head[1] != 0xCF {
		return vm.MapVal(map[string]vm.Value{
			"ok":     vm.Bool(false),
			"error":  vm.Str("not OLE CFB / MSI"),
			"size":   vm.Int(st.Size()),
			"path":   vm.Str(path),
		}), nil
	}
	sectorShift := binary.LittleEndian.Uint16(head[30:])
	miniSectorShift := binary.LittleEndian.Uint16(head[32:])
	numFAT := binary.LittleEndian.Uint32(head[44:])
	firstDir := binary.LittleEndian.Uint32(head[48:])
	sectorSize := uint32(1) << sectorShift
	if sectorSize == 0 {
		sectorSize = 512
	}

	// Read first few directory sectors and extract UTF-16LE entry names
	var names []vm.Value
	seen := map[string]bool{}
	// Directory entries are 128 bytes each; stream follows sector chain — simplified: read sequential sectors from firstDir
	maxSectors := 64
	for s := 0; s < maxSectors; s++ {
		secIdx := firstDir + uint32(s)
		// sector n is at offset (n+1)*sectorSize in file for version 3 CFB
		off := int64(secIdx+1) * int64(sectorSize)
		if off+int64(sectorSize) > st.Size() {
			break
		}
		buf := make([]byte, sectorSize)
		if _, err := f.ReadAt(buf, off); err != nil {
			break
		}
		for i := 0; i+128 <= len(buf); i += 128 {
			// name is UTF-16LE, max 32 wchar, length at offset 64 (u16 bytes including NUL)
			nameLen := int(binary.LittleEndian.Uint16(buf[i+64:]))
			if nameLen < 2 || nameLen > 64 {
				continue
			}
			raw := buf[i : i+nameLen-2]
			if len(raw)%2 != 0 {
				continue
			}
			var b strings.Builder
			ok := true
			for j := 0; j+1 < len(raw); j += 2 {
				u := binary.LittleEndian.Uint16(raw[j:])
				if u == 0 {
					break
				}
				if u < 32 || u > 0x7e {
					// allow some unicode
					if u < 0x20 {
						ok = false
						break
					}
				}
				b.WriteRune(rune(u))
			}
			name := b.String()
			if !ok || name == "" || name == "Root Entry" {
				if name == "Root Entry" && !seen[name] {
					seen[name] = true
					names = append(names, vm.Str(name))
				}
				continue
			}
			if !seen[name] {
				seen[name] = true
				names = append(names, vm.Str(name))
			}
		}
		// if we got many names, stop
		if len(names) > 80 {
			break
		}
	}

	// Heuristic MSI streams
	isMSI := false
	interesting := []string{}
	for _, nv := range names {
		n := nv.AsStr()
		ln := strings.ToLower(n)
		if strings.Contains(ln, "tables") || strings.Contains(ln, "summaryinformation") ||
			strings.Contains(ln, "\x05summary") || n == "Tables" || strings.HasPrefix(n, "\x05") {
			isMSI = true
		}
		if len(interesting) < 40 {
			interesting = append(interesting, n)
		}
	}
	// Also scan head for product clues
	headStr := string(head)
	_ = headStr

	arrInt := make([]vm.Value, len(interesting))
	for i, s := range interesting {
		arrInt[i] = vm.Str(s)
	}

	return vm.MapVal(map[string]vm.Value{
		"ok":            vm.Bool(true),
		"size":          vm.Int(st.Size()),
		"path":          vm.Str(path),
		"sector_size":   vm.Int(int64(sectorSize)),
		"mini_sector":   vm.Int(int64(1 << miniSectorShift)),
		"fat_sectors":   vm.Int(int64(numFAT)),
		"likely_msi":    vm.Bool(isMSI || strings.HasSuffix(strings.ToLower(path), ".msi")),
		"stream_names":  vm.Array(names),
		"streams_shown": vm.Array(arrInt),
		"stream_count":  vm.Int(int64(len(names))),
	}), nil
}
