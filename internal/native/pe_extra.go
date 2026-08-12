package native

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"

	"groklang/gltk/internal/vm"
)

// Extra PE APIs registered alongside pe.parse
func peExtraFns() map[string]vm.NativeFunc {
	return map[string]vm.NativeFunc{
		"parse_file": peParseFile,
		"imports":    peImportsOnly,
		"exports":    peExportsOnly,
		"overlay":    peOverlayInfo,
		"summary":    peSummaryFile,
	}
}

// peParseFile(path, light_bool?)
// Reads only what's needed for headers; light=true skips resource data blobs.
func peParseFile(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("pe.parse_file(path, light?)")
	}
	path := args[0].AsStr()
	light := true
	if len(args) >= 2 {
		light = args[1].Truthy()
	}
	data, err := readPEWorkingSet(path)
	if err != nil {
		return vm.Null(), err
	}
	return peParseBytes(data, light, path)
}

// peSummaryFile — optimized for multi-hundred-MB installers:
// headers + imports + overlay + marker scan head/tail only.
func peSummaryFile(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("pe.summary(path)")
	}
	path := args[0].AsStr()
	st, err := os.Stat(path)
	if err != nil {
		return vm.Null(), err
	}
	f, err := os.Open(path)
	if err != nil {
		return vm.Null(), err
	}
	defer f.Close()

	// Read first 4MB + last 1MB
	const headN = 4 << 20
	const tailN = 1 << 20
	head := make([]byte, headN)
	n, _ := f.Read(head)
	head = head[:n]
	var tail []byte
	if st.Size() > headN+tailN {
		tail = make([]byte, tailN)
		_, _ = f.ReadAt(tail, st.Size()-tailN)
	} else if st.Size() > int64(n) {
		// rest of file already small — re-read all if under 32MB
		if st.Size() <= 32<<20 {
			all, err := os.ReadFile(path)
			if err != nil {
				return vm.Null(), err
			}
			return peParseBytes(all, true, path)
		}
	}

	// Ensure we have PE headers — if e_lfanew points beyond head, extend
	if len(head) >= 0x40 && head[0] == 'M' && head[1] == 'Z' {
		e := int(binary.LittleEndian.Uint32(head[0x3C:]))
		need := e + 0x200 + 40*96 // plenty for headers+sections
		if need > len(head) && int64(need) < st.Size() && need < 16<<20 {
			bigger := make([]byte, need)
			_, err := f.ReadAt(bigger, 0)
			if err == nil {
				head = bigger
			}
		}
	}

	info, err := peParseBytes(head, true, path)
	if err != nil {
		// still return detect-style info
		return vm.MapVal(map[string]vm.Value{
			"ok":     vm.Bool(false),
			"error":  vm.Str(err.Error()),
			"size":   vm.Int(st.Size()),
			"path":   vm.Str(path),
			"partial": vm.Bool(true),
		}), nil
	}
	// peParseBytes returns MapVal — extract map
	m := *info.Map
	m["size"] = vm.Int(st.Size())
	m["path"] = vm.Str(path)
	m["partial"] = vm.Bool(true)
	m["read_head"] = vm.Int(int64(len(head)))
	m["read_tail"] = vm.Int(int64(len(tail)))

	// Force overlay vs REAL file size (not the partial buffer length).
	if secEnd := peImageEnd(head); secEnd > 0 {
		m["image_end"] = vm.Int(int64(secEnd))
		m["overlay_off"] = vm.Int(int64(secEnd))
		ov := st.Size() - int64(secEnd)
		if ov < 0 {
			ov = 0
		}
		m["overlay_size"] = vm.Int(ov)
	}

	// markers from head+tail + start of overlay (installer body)
	blob := append([]byte{}, head...)
	blob = append(blob, tail...)
	if secEnd := peImageEnd(head); secEnd > 0 && st.Size() > int64(secEnd)+16 {
		ovn := 256 * 1024
		if st.Size()-int64(secEnd) < int64(ovn) {
			ovn = int(st.Size() - int64(secEnd))
		}
		ovbuf := make([]byte, ovn)
		if _, err := f.ReadAt(ovbuf, int64(secEnd)); err == nil {
			blob = append(blob, ovbuf...)
			m["overlay_magic"] = vm.Str(hexPreview(ovbuf, 16))
		}
	}
	ms := collectMarkers(blob)
	arr := make([]vm.Value, len(ms))
	for i, s := range ms {
		arr[i] = vm.Str(s)
	}
	m["markers"] = vm.Array(arr)

	// interesting strings from head (and tail), capped
	hits := scanInteresting(blob, 6, 80)
	m["interesting"] = vm.Array(hits)

	return vm.MapVal(m), nil
}

func peImportsOnly(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	bs, err := peBytesArg(args)
	if err != nil {
		return vm.Null(), err
	}
	imps := peParseImports(bs)
	return vm.Array(imps), nil
}

func peExportsOnly(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	bs, err := peBytesArg(args)
	if err != nil {
		return vm.Null(), err
	}
	return vm.Array(peParseExports(bs)), nil
}

func peOverlayInfo(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("pe.overlay(path|bytes)")
	}
	var size int64
	var data []byte
	if args[0].Typ == vm.TypeStr {
		st, err := os.Stat(args[0].AsStr())
		if err != nil {
			return vm.Null(), err
		}
		size = st.Size()
		// only need headers
		data, err = readPEWorkingSet(args[0].AsStr())
		if err != nil {
			return vm.Null(), err
		}
	} else {
		var err error
		data, err = args[0].AsBytes()
		if err != nil {
			return vm.Null(), err
		}
		size = int64(len(data))
	}
	end := peImageEnd(data)
	if end <= 0 {
		return vm.MapVal(map[string]vm.Value{
			"image_end": vm.Int(0),
			"overlay_size": vm.Int(0),
		}), nil
	}
	ov := size - int64(end)
	if ov < 0 {
		ov = 0
	}
	return vm.MapVal(map[string]vm.Value{
		"image_end":    vm.Int(int64(end)),
		"overlay_off":  vm.Int(int64(end)),
		"overlay_size": vm.Int(ov),
		"file_size":    vm.Int(size),
	}), nil
}

func peBytesArg(args []vm.Value) ([]byte, error) {
	if len(args) < 1 {
		return nil, errf("missing arg")
	}
	if args[0].Typ == vm.TypeStr {
		return os.ReadFile(args[0].AsStr())
	}
	return args[0].AsBytes()
}

// readPEWorkingSet loads min(file, 32MB) or enough for full PE image end if smaller.
func readPEWorkingSet(path string) ([]byte, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if st.Size() <= 32<<20 {
		return os.ReadFile(path)
	}
	// large: read 8MB head first; if overlay huge still enough for PE structure
	const n = 8 << 20
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, n)
	rn, err := f.Read(buf)
	if err != nil && rn == 0 {
		return nil, err
	}
	return buf[:rn], nil
}

func peParseBytes(data []byte, light bool, path string) (vm.Value, error) {
	if len(data) < 0x40 {
		return vm.Null(), errf("pe: too small")
	}
	if data[0] != 'M' || data[1] != 'Z' {
		return vm.Null(), errf("pe: not MZ")
	}
	e_lfanew := int(binary.LittleEndian.Uint32(data[0x3C:]))
	if e_lfanew <= 0 || e_lfanew+4+20+2 > len(data) {
		return vm.Null(), errf("pe: bad e_lfanew")
	}
	if string(data[e_lfanew:e_lfanew+4]) != "PE\x00\x00" {
		return vm.Null(), errf("pe: bad PE signature")
	}
	coff := e_lfanew + 4
	machine := binary.LittleEndian.Uint16(data[coff:])
	numSec := int(binary.LittleEndian.Uint16(data[coff+2:]))
	timedate := binary.LittleEndian.Uint32(data[coff+4:])
	chars := binary.LittleEndian.Uint16(data[coff+18:])
	optSize := int(binary.LittleEndian.Uint16(data[coff+16:]))
	optOff := coff + 20
	if optOff+optSize > len(data) {
		return vm.Null(), errf("pe: truncated optional header")
	}
	magic := binary.LittleEndian.Uint16(data[optOff:])
	var entryRVA, imageBase uint64
	var subsystem, dllChars uint16
	var sizeOfImage, sizeOfHeaders uint32
	var dataDirsOff int
	var numDataDirs uint32
	if magic == 0x10b {
		entryRVA = uint64(binary.LittleEndian.Uint32(data[optOff+16:]))
		imageBase = uint64(binary.LittleEndian.Uint32(data[optOff+28:]))
		sizeOfImage = binary.LittleEndian.Uint32(data[optOff+56:])
		sizeOfHeaders = binary.LittleEndian.Uint32(data[optOff+60:])
		subsystem = binary.LittleEndian.Uint16(data[optOff+68:])
		dllChars = binary.LittleEndian.Uint16(data[optOff+70:])
		if optOff+96 <= len(data) {
			numDataDirs = binary.LittleEndian.Uint32(data[optOff+92:])
		}
		dataDirsOff = optOff + 96
	} else if magic == 0x20b {
		entryRVA = uint64(binary.LittleEndian.Uint32(data[optOff+16:]))
		imageBase = binary.LittleEndian.Uint64(data[optOff+24:])
		sizeOfImage = binary.LittleEndian.Uint32(data[optOff+56:])
		sizeOfHeaders = binary.LittleEndian.Uint32(data[optOff+60:])
		subsystem = binary.LittleEndian.Uint16(data[optOff+68:])
		dllChars = binary.LittleEndian.Uint16(data[optOff+70:])
		if optOff+112 <= len(data) {
			numDataDirs = binary.LittleEndian.Uint32(data[optOff+108:])
		}
		dataDirsOff = optOff + 112
	} else {
		return vm.Null(), errf("pe: unknown optional magic %#x", magic)
	}

	secOff := optOff + optSize
	var sections []vm.Value
	var maxEnd uint32
	for i := 0; i < numSec; i++ {
		off := secOff + i*40
		if off+40 > len(data) {
			break
		}
		name := string(data[off : off+8])
		if z := indexByte(name, 0); z >= 0 {
			name = name[:z]
		}
		vsize := binary.LittleEndian.Uint32(data[off+8:])
		vaddr := binary.LittleEndian.Uint32(data[off+12:])
		rawsize := binary.LittleEndian.Uint32(data[off+16:])
		rawptr := binary.LittleEndian.Uint32(data[off+20:])
		charsSec := binary.LittleEndian.Uint32(data[off+36:])
		end := rawptr + rawsize
		if end > maxEnd {
			maxEnd = end
		}
		entropy := -1.0
		if !light && int(rawptr+rawsize) <= len(data) && rawsize > 0 && rawsize < 4<<20 {
			entropy = shannon(data[rawptr : rawptr+rawsize])
		}
		m := map[string]vm.Value{
			"name":    vm.Str(name),
			"vsize":   vm.Int(int64(vsize)),
			"vaddr":   vm.Int(int64(vaddr)),
			"rawsize": vm.Int(int64(rawsize)),
			"rawptr":  vm.Int(int64(rawptr)),
			"chars":   vm.Int(int64(charsSec)),
			"entropy": vm.Float(entropy),
		}
		sections = append(sections, vm.MapVal(m))
	}

	var resources []vm.Value
	if numDataDirs > 2 && dataDirsOff+3*8 <= len(data) {
		rva := binary.LittleEndian.Uint32(data[dataDirsOff+2*8:])
		size := binary.LittleEndian.Uint32(data[dataDirsOff+2*8+4:])
		if rva != 0 && size != 0 {
			raw := rvaToFile(data, secOff, numSec, rva)
			if raw >= 0 {
				resources = walkResourcesLight(data, secOff, numSec, uint32(raw), uint32(raw), 0, "", "", light)
			}
		}
	}

	imports := peParseImports(data)
	exports := peParseExports(data)

	overlaySize := int64(0)
	if maxEnd > 0 && int64(maxEnd) < int64(len(data)) {
		overlaySize = int64(len(data)) - int64(maxEnd)
	}

	result := map[string]vm.Value{
		"ok":             vm.Bool(true),
		"machine":        vm.Int(int64(machine)),
		"machine_name":   vm.Str(peMachineName(machine)),
		"entry":          vm.Int(int64(entryRVA)),
		"image_base":     vm.Int(int64(imageBase)),
		"timestamp":      vm.Int(int64(timedate)),
		"characteristics": vm.Int(int64(chars)),
		"subsystem":      vm.Int(int64(subsystem)),
		"subsystem_name": vm.Str(peSubsystemName(subsystem)),
		"dll_chars":      vm.Int(int64(dllChars)),
		"sizeof_image":   vm.Int(int64(sizeOfImage)),
		"sizeof_headers": vm.Int(int64(sizeOfHeaders)),
		"sections":       vm.Array(sections),
		"section_count":  vm.Int(int64(len(sections))),
		"resources":      vm.Array(resources),
		"resource_count": vm.Int(int64(len(resources))),
		"imports":        vm.Array(imports),
		"import_count":   vm.Int(int64(len(imports))),
		"exports":        vm.Array(exports),
		"export_count":   vm.Int(int64(len(exports))),
		"pe32plus":       vm.Bool(magic == 0x20b),
		"image_end":      vm.Int(int64(maxEnd)),
		"overlay_size":   vm.Int(overlaySize),
		"light":          vm.Bool(light),
		"bytes_loaded":   vm.Int(int64(len(data))),
	}
	if path != "" {
		result["path"] = vm.Str(path)
	}
	// markers + interesting strings from loaded bytes (includes overlay if fully loaded)
	ms := collectMarkers(data)
	marr := make([]vm.Value, len(ms))
	for i, s := range ms {
		marr[i] = vm.Str(s)
	}
	result["markers"] = vm.Array(marr)
	result["interesting"] = vm.Array(scanInteresting(data, 6, 60))

	// manifest snippet if present
	for _, r := range resources {
		if r.Map == nil {
			continue
		}
		mm := *r.Map
		if mm["type"].AsStr() == "MANIFEST" {
			if d, err := mm["data"].AsBytes(); err == nil && len(d) > 0 {
				s := string(d)
				if len(s) > 500 {
					s = s[:500]
				}
				result["manifest_head"] = vm.Str(s)
			}
		}
	}
	return vm.MapVal(result), nil
}

func peMachineName(m uint16) string {
	switch m {
	case 0x14c:
		return "I386"
	case 0x8664:
		return "AMD64"
	case 0xAA64:
		return "ARM64"
	case 0x1c0:
		return "ARM"
	default:
		return fmt.Sprintf("0x%X", m)
	}
}

func peSubsystemName(s uint16) string {
	switch s {
	case 1:
		return "NATIVE"
	case 2:
		return "WINDOWS_GUI"
	case 3:
		return "WINDOWS_CUI"
	case 7:
		return "POSIX_CUI"
	case 9:
		return "WINDOWS_CE_GUI"
	case 10:
		return "EFI_APP"
	default:
		return fmt.Sprintf("%d", s)
	}
}

func peImageEnd(data []byte) int {
	if len(data) < 0x40 || data[0] != 'M' || data[1] != 'Z' {
		return 0
	}
	e := int(binary.LittleEndian.Uint32(data[0x3C:]))
	if e+24 > len(data) || string(data[e:e+4]) != "PE\x00\x00" {
		return 0
	}
	coff := e + 4
	numSec := int(binary.LittleEndian.Uint16(data[coff+2:]))
	optSize := int(binary.LittleEndian.Uint16(data[coff+16:]))
	secOff := coff + 20 + optSize
	var maxEnd int
	for i := 0; i < numSec; i++ {
		off := secOff + i*40
		if off+40 > len(data) {
			break
		}
		rawsize := int(binary.LittleEndian.Uint32(data[off+16:]))
		rawptr := int(binary.LittleEndian.Uint32(data[off+20:]))
		if rawptr+rawsize > maxEnd {
			maxEnd = rawptr + rawsize
		}
	}
	return maxEnd
}

func walkResourcesLight(data []byte, secOff, numSec int, resBase, dirOff uint32, level int, typeName, resName string, light bool) []vm.Value {
	var out []vm.Value
	if int(dirOff)+16 > len(data) {
		return out
	}
	numNamed := binary.LittleEndian.Uint16(data[dirOff+12:])
	numId := binary.LittleEndian.Uint16(data[dirOff+14:])
	n := int(numNamed) + int(numId)
	entryStart := dirOff + 16
	for i := 0; i < n; i++ {
		eoff := int(entryStart) + i*8
		if eoff+8 > len(data) {
			break
		}
		nameOrId := binary.LittleEndian.Uint32(data[eoff:])
		offsetToData := binary.LittleEndian.Uint32(data[eoff+4:])
		var name string
		if nameOrId&0x80000000 != 0 {
			soff := int(resBase) + int(nameOrId&0x7fffffff)
			if soff+2 <= len(data) {
				nchars := int(binary.LittleEndian.Uint16(data[soff:]))
				soff += 2
				if soff+nchars*2 <= len(data) {
					u16s := make([]uint16, nchars)
					for j := 0; j < nchars; j++ {
						u16s[j] = binary.LittleEndian.Uint16(data[soff+j*2:])
					}
					name = utf16ToString(u16s)
				}
			}
		} else {
			name = fmt.Sprintf("%d", nameOrId)
			if level == 0 {
				name = peResTypeName(nameOrId)
			}
		}
		if offsetToData&0x80000000 != 0 {
			sub := resBase + (offsetToData & 0x7fffffff)
			tn, rn := typeName, resName
			if level == 0 {
				tn = name
			} else if level == 1 {
				rn = name
			}
			out = append(out, walkResourcesLight(data, secOff, numSec, resBase, sub, level+1, tn, rn, light)...)
		} else {
			de := int(resBase + offsetToData)
			if de+16 > len(data) {
				continue
			}
			dataRVA := binary.LittleEndian.Uint32(data[de:])
			dataSize := binary.LittleEndian.Uint32(data[de+4:])
			fileOff := rvaToFile(data, secOff, numSec, dataRVA)
			var blob []byte
			// always keep small manifests/version; light skips large data
			keep := !light || dataSize <= 64*1024 || typeName == "MANIFEST" || typeName == "VERSION" || typeName == "RCDATA"
			if keep && fileOff >= 0 && fileOff+int(dataSize) <= len(data) {
				if light && dataSize > 256*1024 && typeName != "MANIFEST" {
					// skip huge
				} else {
					blob = data[fileOff : fileOff+int(dataSize)]
				}
			}
			m := map[string]vm.Value{
				"type": vm.Str(typeName),
				"name": vm.Str(resName),
				"lang": vm.Str(name),
				"size": vm.Int(int64(dataSize)),
				"off":  vm.Int(int64(fileOff)),
				"data": vm.Bytes(blob),
			}
			out = append(out, vm.MapVal(m))
		}
	}
	return out
}

func peParseImports(data []byte) []vm.Value {
	hdr, ok := peHeaders(data)
	if !ok || hdr.numDataDirs < 2 {
		return nil
	}
	rva := binary.LittleEndian.Uint32(data[hdr.dataDirsOff+1*8:])
	if rva == 0 {
		return nil
	}
	off := rvaToFile(data, hdr.secOff, hdr.numSec, rva)
	if off < 0 {
		return nil
	}
	var out []vm.Value
	for i := 0; i < 512; i++ {
		e := off + i*20
		if e+20 > len(data) {
			break
		}
		nameRVA := binary.LittleEndian.Uint32(data[e+12:])
		iltRVA := binary.LittleEndian.Uint32(data[e:]) // OriginalFirstThunk
		if nameRVA == 0 {
			break
		}
		noff := rvaToFile(data, hdr.secOff, hdr.numSec, nameRVA)
		if noff < 0 || noff >= len(data) {
			continue
		}
		dll := readCString(data, noff)
		var funcs []vm.Value
		thunk := iltRVA
		if thunk == 0 {
			thunk = binary.LittleEndian.Uint32(data[e+16:]) // FirstThunk
		}
		toff := rvaToFile(data, hdr.secOff, hdr.numSec, thunk)
		if toff >= 0 {
			step := 4
			if hdr.pe32plus {
				step = 8
			}
			for j := 0; j < 256; j++ {
				p := toff + j*step
				if p+step > len(data) {
					break
				}
				var v uint64
				if step == 8 {
					v = binary.LittleEndian.Uint64(data[p:])
				} else {
					v = uint64(binary.LittleEndian.Uint32(data[p:]))
				}
				if v == 0 {
					break
				}
				// ordinal bit
				ordBit := uint64(0x80000000)
				if step == 8 {
					ordBit = 0x8000000000000000
				}
				if v&ordBit != 0 {
					funcs = append(funcs, vm.Str(fmt.Sprintf("ord:%d", v&0xffff)))
				} else {
					hoff := rvaToFile(data, hdr.secOff, hdr.numSec, uint32(v&0x7fffffff))
					if hoff >= 0 && hoff+2 < len(data) {
						// hint 2 bytes + name
						fname := readCString(data, hoff+2)
						funcs = append(funcs, vm.Str(fname))
					}
				}
			}
		}
		out = append(out, vm.MapVal(map[string]vm.Value{
			"dll":   vm.Str(dll),
			"funcs": vm.Array(funcs),
			"count": vm.Int(int64(len(funcs))),
		}))
	}
	return out
}

func peParseExports(data []byte) []vm.Value {
	hdr, ok := peHeaders(data)
	if !ok || hdr.numDataDirs < 1 {
		return nil
	}
	rva := binary.LittleEndian.Uint32(data[hdr.dataDirsOff+0*8:])
	if rva == 0 {
		return nil
	}
	off := rvaToFile(data, hdr.secOff, hdr.numSec, rva)
	if off < 0 || off+40 > len(data) {
		return nil
	}
	// IMAGE_EXPORT_DIRECTORY
	numNames := binary.LittleEndian.Uint32(data[off+24:])
	namesRVA := binary.LittleEndian.Uint32(data[off+32:])
	namesOff := rvaToFile(data, hdr.secOff, hdr.numSec, namesRVA)
	if namesOff < 0 {
		return nil
	}
	var out []vm.Value
	limit := int(numNames)
	if limit > 200 {
		limit = 200
	}
	for i := 0; i < limit; i++ {
		p := namesOff + i*4
		if p+4 > len(data) {
			break
		}
		nrva := binary.LittleEndian.Uint32(data[p:])
		no := rvaToFile(data, hdr.secOff, hdr.numSec, nrva)
		if no < 0 {
			continue
		}
		out = append(out, vm.Str(readCString(data, no)))
	}
	return out
}

type peHdr struct {
	secOff      int
	numSec      int
	dataDirsOff int
	numDataDirs uint32
	pe32plus    bool
}

func peHeaders(data []byte) (peHdr, bool) {
	var h peHdr
	if len(data) < 0x40 || data[0] != 'M' || data[1] != 'Z' {
		return h, false
	}
	e := int(binary.LittleEndian.Uint32(data[0x3C:]))
	if e+24 > len(data) || string(data[e:e+4]) != "PE\x00\x00" {
		return h, false
	}
	coff := e + 4
	h.numSec = int(binary.LittleEndian.Uint16(data[coff+2:]))
	optSize := int(binary.LittleEndian.Uint16(data[coff+16:]))
	optOff := coff + 20
	if optOff+2 > len(data) {
		return h, false
	}
	magic := binary.LittleEndian.Uint16(data[optOff:])
	h.secOff = optOff + optSize
	if magic == 0x10b {
		if optOff+96 > len(data) {
			return h, false
		}
		h.numDataDirs = binary.LittleEndian.Uint32(data[optOff+92:])
		h.dataDirsOff = optOff + 96
	} else if magic == 0x20b {
		if optOff+112 > len(data) {
			return h, false
		}
		h.numDataDirs = binary.LittleEndian.Uint32(data[optOff+108:])
		h.dataDirsOff = optOff + 112
		h.pe32plus = true
	} else {
		return h, false
	}
	return h, true
}

func readCString(data []byte, off int) string {
	if off < 0 || off >= len(data) {
		return ""
	}
	end := off
	for end < len(data) && data[end] != 0 && end-off < 512 {
		end++
	}
	return string(data[off:end])
}

func shannon(b []byte) float64 {
	if len(b) == 0 {
		return 0
	}
	var freq [256]int
	for _, c := range b {
		freq[c]++
	}
	var h float64
	n := float64(len(b))
	for _, c := range freq {
		if c == 0 {
			continue
		}
		p := float64(c) / n
		h -= p * log2(p)
	}
	return h
}

func log2(p float64) float64 {
	return math.Log2(p)
}
