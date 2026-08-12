// Package vmp provides VMProtect detection and unpack-assist helpers.
//
// Full VMP devirtualization is out of scope. This package implements the
// practical pipeline used in prior RE work:
//   1) detect VMP 3.x signatures / section layout
//   2) extract on-disk sections + write a triage report
//   3) fix PE headers of a runtime memory dump (raw=VA layout)
// so that dumped images become usable for static analysis tools.
package vmp

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Section describes one PE section.
type Section struct {
	Name            string `json:"name"`
	VirtualAddress  uint32 `json:"virtual_address"`
	VirtualSize     uint32 `json:"virtual_size"`
	PointerToRawData uint32 `json:"pointer_to_raw"`
	SizeOfRawData   uint32 `json:"size_of_raw"`
	Characteristics uint32 `json:"characteristics"`
	DiskEmpty       bool   `json:"disk_empty"` // VirtualSize>0 but SizeOfRawData==0
	VMPLike         bool   `json:"vmp_like"`
}

// Info is a structured VMP triage report.
type Info struct {
	OK              bool       `json:"ok"`
	IsVMP           bool       `json:"is_vmp"`
	Confidence      int        `json:"confidence"` // 0-100
	Reasons         []string   `json:"reasons"`
	Machine         uint16     `json:"machine"`
	Is64            bool       `json:"is64"`
	EntryRVA        uint32     `json:"entry_rva"`
	EntrySection    string     `json:"entry_section"`
	ImageBase       uint64     `json:"image_base"`
	SectionAlign    uint32     `json:"section_align"`
	FileAlign       uint32     `json:"file_align"`
	SizeOfImage     uint32     `json:"size_of_image"`
	NumberOfSections int       `json:"number_of_sections"`
	EmptyDiskSecs   int        `json:"empty_disk_sections"`
	Sections        []Section  `json:"sections"`
	Markers         []string   `json:"markers"`
	Note            string     `json:"note"`
	Error           string     `json:"error,omitempty"`
}

// Detect reports whether data looks VMProtect-protected.
func Detect(data []byte) bool {
	info, err := Analyze(data)
	if err != nil {
		return false
	}
	return info.IsVMP
}

// Analyze returns a full triage report.
func Analyze(data []byte) (*Info, error) {
	info := &Info{OK: true, Reasons: []string{}, Markers: []string{}}
	if len(data) < 0x200 {
		info.OK = false
		info.Error = "file too small"
		return info, fmt.Errorf("file too small")
	}
	if data[0] != 'M' || data[1] != 'Z' {
		info.OK = false
		info.Error = "not MZ"
		return info, fmt.Errorf("not MZ")
	}
	peOff := int(binary.LittleEndian.Uint32(data[0x3C:]))
	if peOff <= 0 || peOff+0x18+0x60 > len(data) {
		info.OK = false
		info.Error = "bad e_lfanew"
		return info, fmt.Errorf("bad e_lfanew")
	}
	if data[peOff] != 'P' || data[peOff+1] != 'E' {
		info.OK = false
		info.Error = "not PE"
		return info, fmt.Errorf("not PE")
	}
	coff := peOff + 4
	info.Machine = binary.LittleEndian.Uint16(data[coff:])
	numSec := int(binary.LittleEndian.Uint16(data[coff+2:]))
	optSize := int(binary.LittleEndian.Uint16(data[coff+16:]))
	optOff := coff + 20
	magic := binary.LittleEndian.Uint16(data[optOff:])
	info.Is64 = magic == 0x20b
	if magic == 0x10b {
		info.EntryRVA = binary.LittleEndian.Uint32(data[optOff+16:])
		info.ImageBase = uint64(binary.LittleEndian.Uint32(data[optOff+28:]))
		info.SectionAlign = binary.LittleEndian.Uint32(data[optOff+32:])
		info.FileAlign = binary.LittleEndian.Uint32(data[optOff+36:])
		info.SizeOfImage = binary.LittleEndian.Uint32(data[optOff+56:])
	} else if magic == 0x20b {
		info.EntryRVA = binary.LittleEndian.Uint32(data[optOff+16:])
		info.ImageBase = binary.LittleEndian.Uint64(data[optOff+24:])
		info.SectionAlign = binary.LittleEndian.Uint32(data[optOff+32:])
		info.FileAlign = binary.LittleEndian.Uint32(data[optOff+36:])
		info.SizeOfImage = binary.LittleEndian.Uint32(data[optOff+56:])
	} else {
		info.OK = false
		info.Error = "unknown optional header magic"
		return info, fmt.Errorf("unknown optional header magic %#x", magic)
	}
	secOff := optOff + optSize
	info.NumberOfSections = numSec
	info.Sections = make([]Section, 0, numSec)

	score := 0
	for i := 0; i < numSec; i++ {
		off := secOff + i*40
		if off+40 > len(data) {
			break
		}
		nameBytes := data[off : off+8]
		name := sectionName(nameBytes)
		vs := binary.LittleEndian.Uint32(data[off+8:])
		va := binary.LittleEndian.Uint32(data[off+12:])
		rs := binary.LittleEndian.Uint32(data[off+16:])
		rp := binary.LittleEndian.Uint32(data[off+20:])
		ch := binary.LittleEndian.Uint32(data[off+36:])
		diskEmpty := vs > 0 && rs == 0
		vmpLike := isVMPSectionName(name)
		sec := Section{
			Name:             name,
			VirtualAddress:   va,
			VirtualSize:      vs,
			PointerToRawData: rp,
			SizeOfRawData:    rs,
			Characteristics:  ch,
			DiskEmpty:        diskEmpty,
			VMPLike:          vmpLike,
		}
		info.Sections = append(info.Sections, sec)
		if diskEmpty {
			info.EmptyDiskSecs++
		}
		if va <= info.EntryRVA && info.EntryRVA < va+maxU32(vs, rs) {
			info.EntrySection = name
		}
	}

	// scoring
	if info.EmptyDiskSecs >= 3 {
		score += 35
		info.Reasons = append(info.Reasons, fmt.Sprintf("%d sections empty on disk (runtime decrypt typical of VMP)", info.EmptyDiskSecs))
	} else if info.EmptyDiskSecs >= 1 {
		score += 15
		info.Reasons = append(info.Reasons, fmt.Sprintf("%d section(s) empty on disk", info.EmptyDiskSecs))
	}
	vmpSecs := 0
	for _, s := range info.Sections {
		if s.VMPLike {
			vmpSecs++
		}
	}
	if vmpSecs > 0 {
		score += 40
		info.Reasons = append(info.Reasons, fmt.Sprintf("%d VMP-like section name(s)", vmpSecs))
	}

	// string markers in first 4MB + last 256KB
	markers := scanMarkers(data)
	info.Markers = markers
	if len(markers) > 0 {
		score += 25
		info.Reasons = append(info.Reasons, "marker strings: "+strings.Join(markers, ", "))
	}

	// entry in high / non-.text section with vmp-like name
	if info.EntrySection != "" {
		if isVMPSectionName(info.EntrySection) || strings.Contains(strings.ToLower(info.EntrySection), "vmp") {
			score += 15
			info.Reasons = append(info.Reasons, "entry point in VMP-like section: "+info.EntrySection)
		} else if info.EntrySection != ".text" && info.EntrySection != "CODE" {
			// mildly suspicious
			score += 5
			info.Reasons = append(info.Reasons, "entry section is "+info.EntrySection)
		}
	}

	// entry prologue heuristic (PUSH R13 / MOVABS R13 common on x64 VMP stubs)
	if ep := rvaToOff(data, secOff, numSec, info.EntryRVA); ep >= 0 && ep+16 <= len(data) {
		// 49 55          push r13
		// 9C             pushfq
		// 49 BD xx..     movabs r13, imm64
		if data[ep] == 0x49 && data[ep+1] == 0x55 {
			score += 10
			info.Reasons = append(info.Reasons, "entry prologue looks like x64 VMP stub (push r13)")
		}
	}

	if score > 100 {
		score = 100
	}
	info.Confidence = score
	info.IsVMP = score >= 40
	if info.IsVMP {
		info.Note = "VMProtect-like packing detected. Full devirtualization is not performed; use extract/fixdump assist."
	} else {
		info.Note = "No strong VMP signature (confidence below threshold)."
	}
	return info, nil
}

// Extract writes raw section blobs and report.json under outDir.
func Extract(data []byte, outDir string) (map[string]interface{}, error) {
	info, err := Analyze(data)
	if err != nil && info == nil {
		return nil, err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	peOff := int(binary.LittleEndian.Uint32(data[0x3C:]))
	coff := peOff + 4
	numSec := int(binary.LittleEndian.Uint16(data[coff+2:]))
	optSize := int(binary.LittleEndian.Uint16(data[coff+16:]))
	secOff := coff + 20 + optSize

	written := []map[string]interface{}{}
	for i, s := range info.Sections {
		item := map[string]interface{}{
			"name":       s.Name,
			"disk_empty": s.DiskEmpty,
			"va":         s.VirtualAddress,
			"vsize":      s.VirtualSize,
			"raw_ptr":    s.PointerToRawData,
			"raw_size":   s.SizeOfRawData,
		}
		if s.SizeOfRawData > 0 && int(s.PointerToRawData+s.SizeOfRawData) <= len(data) {
			safe := sanitizeName(s.Name, i)
			path := filepath.Join(outDir, "section_"+safe+".bin")
			blob := data[s.PointerToRawData : s.PointerToRawData+s.SizeOfRawData]
			if err := os.WriteFile(path, blob, 0o644); err != nil {
				return nil, err
			}
			item["path"] = path
			item["bytes"] = len(blob)
		}
		written = append(written, item)
		_ = secOff
		_ = numSec
	}

	// full file copy for convenience
	fullPath := filepath.Join(outDir, "raw_full.bin")
	_ = os.WriteFile(fullPath, data, 0o644)

	repPath := filepath.Join(outDir, "vmp_report.json")
	jb, _ := json.MarshalIndent(info, "", "  ")
	_ = os.WriteFile(repPath, jb, 0o644)

	// human text report
	txt := filepath.Join(outDir, "vmp_report.txt")
	_ = os.WriteFile(txt, []byte(FormatReport(info)), 0o644)

	return map[string]interface{}{
		"ok":         true,
		"is_vmp":     info.IsVMP,
		"confidence": info.Confidence,
		"out_dir":    outDir,
		"report":     repPath,
		"sections":   written,
		"full":       fullPath,
		"note":       info.Note,
	}, nil
}

// FixDump converts a process memory dump (sections laid out at VA) into a
// file-aligned PE that most static tools can open.
//
// Heuristic: if most sections have PointerToRawData == 0 and VirtualSize > 0,
// or PointerToRawData == VirtualAddress, treat as dump and rebuild headers so
// PointerToRawData = VirtualAddress and FileAlignment = SectionAlignment.
func FixDump(data []byte) ([]byte, *Info, error) {
	info, err := Analyze(data)
	if err != nil && (info == nil || !info.OK) {
		return nil, info, err
	}
	out := make([]byte, len(data))
	copy(out, data)

	peOff := int(binary.LittleEndian.Uint32(out[0x3C:]))
	coff := peOff + 4
	numSec := int(binary.LittleEndian.Uint16(out[coff+2:]))
	optSize := int(binary.LittleEndian.Uint16(out[coff+16:]))
	optOff := coff + 20
	magic := binary.LittleEndian.Uint16(out[optOff:])
	secOff := optOff + optSize

	// Force FileAlignment = SectionAlignment for dump usability
	var secAlign, fileAlign uint32
	if magic == 0x10b || magic == 0x20b {
		secAlign = binary.LittleEndian.Uint32(out[optOff+32:])
		fileAlign = binary.LittleEndian.Uint32(out[optOff+36:])
		if secAlign == 0 {
			secAlign = 0x1000
		}
		// write FileAlignment = SectionAlignment
		binary.LittleEndian.PutUint32(out[optOff+36:], secAlign)
		fileAlign = secAlign
	}

	// Rebuild section file pointers to VA layout
	for i := 0; i < numSec; i++ {
		off := secOff + i*40
		if off+40 > len(out) {
			break
		}
		vs := binary.LittleEndian.Uint32(out[off+8:])
		va := binary.LittleEndian.Uint32(out[off+12:])
		rs := binary.LittleEndian.Uint32(out[off+16:])
		// SizeOfRawData = align(max(vs, rs), fileAlign) but clamp to image
		rawSize := vs
		if rs > rawSize {
			rawSize = rs
		}
		if fileAlign > 0 {
			rawSize = alignUp(rawSize, fileAlign)
		}
		// keep within buffer
		if int(va+rawSize) > len(out) {
			if int(va) >= len(out) {
				rawSize = 0
			} else {
				rawSize = uint32(len(out) - int(va))
			}
		}
		binary.LittleEndian.PutUint32(out[off+16:], rawSize) // SizeOfRawData
		binary.LittleEndian.PutUint32(out[off+20:], va)      // PointerToRawData = VA
	}

	// SizeOfHeaders: keep or set to first section VA
	if magic == 0x10b || magic == 0x20b {
		// OptionalHeader.SizeOfHeaders at +60
		if numSec > 0 && secOff+12+4 <= len(out) {
			firstVA := binary.LittleEndian.Uint32(out[secOff+12:])
			if firstVA > 0 && firstVA < uint32(len(out)) {
				binary.LittleEndian.PutUint32(out[optOff+60:], firstVA)
			}
		}
	}

	info2, _ := Analyze(out)
	if info2 != nil {
		info2.Note = "Headers rewritten for memory-dump layout (PointerToRawData=VA, FileAlignment=SectionAlignment). This is NOT VMP devirtualization."
	}
	return out, info2, nil
}

// FixDumpFile reads path, fixes, writes outPath.
func FixDumpFile(inPath, outPath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(inPath)
	if err != nil {
		return nil, err
	}
	fixed, info, err := FixDump(data)
	if err != nil && fixed == nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}, err
	}
	if err := os.WriteFile(outPath, fixed, 0o644); err != nil {
		return nil, err
	}
	res := map[string]interface{}{
		"ok":       true,
		"in":       inPath,
		"out":      outPath,
		"size":     len(fixed),
		"is_vmp":   info != nil && info.IsVMP,
		"confidence": 0,
		"note":     "dump PE headers fixed",
	}
	if info != nil {
		res["confidence"] = info.Confidence
		res["note"] = info.Note
		res["empty_disk_sections"] = info.EmptyDiskSecs
	}
	return res, nil
}

// Assist runs detect+extract [+ optional fixdump if looks like dump].
func Assist(inPath, outDir string) (map[string]interface{}, error) {
	data, err := os.ReadFile(inPath)
	if err != nil {
		return nil, err
	}
	info, err := Analyze(data)
	if err != nil && info == nil {
		return nil, err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	ext, err := Extract(data, outDir)
	if err != nil {
		return nil, err
	}

	// If file already has VA-style layout (PointerToRawData == VirtualAddress for many secs), emit fixed dump
	dumpLike := 0
	for _, s := range info.Sections {
		if s.VirtualSize > 0 && (s.PointerToRawData == s.VirtualAddress || (s.PointerToRawData == 0 && s.SizeOfRawData == 0)) {
			dumpLike++
		}
	}
	result := map[string]interface{}{
		"ok":         true,
		"path":       inPath,
		"out_dir":    outDir,
		"is_vmp":     info.IsVMP,
		"confidence": info.Confidence,
		"reasons":    info.Reasons,
		"extract":    ext,
		"note":       info.Note,
	}
	if dumpLike >= 2 || (info.EmptyDiskSecs == 0 && dumpLike >= 1) {
		fixedPath := filepath.Join(outDir, "fixed_dump.exe")
		// Prefer FixDump on full buffer always for assist output
		fixed, info2, ferr := FixDump(data)
		if ferr == nil && fixed != nil {
			_ = os.WriteFile(fixedPath, fixed, 0o644)
			result["fixed_dump"] = fixedPath
			if info2 != nil {
				result["fixed_note"] = info2.Note
			}
		}
	} else if info.EmptyDiskSecs > 0 {
		result["hint"] = "Disk image has empty runtime sections. Load the module in a debugger/sandbox, dump the full image from memory after DllMain, then: gltk vmp fixdump <dump.exe> out.exe"
	}
	return result, nil
}

// FormatReport returns a human-readable report.
func FormatReport(info *Info) string {
	if info == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "VMP Assist Report\n")
	fmt.Fprintf(&b, "=================\n")
	fmt.Fprintf(&b, "is_vmp=%v  confidence=%d\n", info.IsVMP, info.Confidence)
	fmt.Fprintf(&b, "machine=0x%X is64=%v entry_rva=0x%X entry_sec=%s\n", info.Machine, info.Is64, info.EntryRVA, info.EntrySection)
	fmt.Fprintf(&b, "image_base=0x%X size_of_image=0x%X sections=%d empty_disk=%d\n", info.ImageBase, info.SizeOfImage, info.NumberOfSections, info.EmptyDiskSecs)
	if len(info.Reasons) > 0 {
		fmt.Fprintf(&b, "\nReasons:\n")
		for _, r := range info.Reasons {
			fmt.Fprintf(&b, "  - %s\n", r)
		}
	}
	if len(info.Markers) > 0 {
		fmt.Fprintf(&b, "\nMarkers: %s\n", strings.Join(info.Markers, ", "))
	}
	fmt.Fprintf(&b, "\nSections:\n")
	for _, s := range info.Sections {
		flag := ""
		if s.DiskEmpty {
			flag += " [DISK_EMPTY]"
		}
		if s.VMPLike {
			flag += " [VMP]"
		}
		fmt.Fprintf(&b, "  %-8s VA=0x%08X VSize=0x%X RawPtr=0x%X RawSize=0x%X%s\n",
			s.Name, s.VirtualAddress, s.VirtualSize, s.PointerToRawData, s.SizeOfRawData, flag)
	}
	fmt.Fprintf(&b, "\nNote: %s\n", info.Note)
	fmt.Fprintf(&b, "\nWorkflow:\n")
	fmt.Fprintf(&b, "  1) gltk vmp assist <file> <outdir>\n")
	fmt.Fprintf(&b, "  2) If sections are DISK_EMPTY: run sample, memory-dump module, then\n")
	fmt.Fprintf(&b, "     gltk vmp fixdump <dump.exe> <fixed.exe>\n")
	fmt.Fprintf(&b, "  3) Full VMP handler de-virtualization is NOT performed.\n")
	return b.String()
}

func sectionName(b []byte) string {
	n := bytes.IndexByte(b, 0)
	if n < 0 {
		n = len(b)
	}
	return string(b[:n])
}

func isVMPSectionName(name string) bool {
	l := strings.ToLower(name)
	if strings.Contains(l, "vmp") {
		return true
	}
	// observed custom VMP section names
	switch name {
	case ".EY,", ".;lt", ".vmp0", ".vmp1", ".vmp2", ".VMP0", ".VMP1":
		return true
	}
	// short weird names often used by VMP mutations
	if strings.HasPrefix(name, ".") && len(name) <= 5 {
		// not all short names are VMP; require non-alphanumeric beyond first
		weird := 0
		for _, c := range name[1:] {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
				weird++
			}
		}
		if weird >= 1 && (strings.Contains(l, "ey") || strings.Contains(name, ";") || strings.Contains(name, ",")) {
			return true
		}
	}
	return false
}

func scanMarkers(data []byte) []string {
	// scan head + tail to avoid huge cost
	chunks := [][]byte{data}
	if len(data) > 5<<20 {
		head := data[:4<<20]
		tail := data[len(data)-256<<10:]
		chunks = [][]byte{head, tail}
	}
	needles := []string{
		"VMProtect",
		"VMProtect begin",
		".vmp0",
		".vmp1",
		".vmp2",
		"VirtualProtect",
	}
	found := []string{}
	seen := map[string]bool{}
	for _, ch := range chunks {
		for _, n := range needles {
			if bytes.Contains(ch, []byte(n)) && !seen[n] {
				seen[n] = true
				found = append(found, n)
			}
		}
	}
	return found
}

func rvaToOff(data []byte, secOff, numSec int, rva uint32) int {
	for i := 0; i < numSec; i++ {
		off := secOff + i*40
		if off+40 > len(data) {
			break
		}
		vs := binary.LittleEndian.Uint32(data[off+8:])
		va := binary.LittleEndian.Uint32(data[off+12:])
		rs := binary.LittleEndian.Uint32(data[off+16:])
		rp := binary.LittleEndian.Uint32(data[off+20:])
		size := vs
		if rs > size {
			size = rs
		}
		if rva >= va && rva < va+size {
			if rs == 0 {
				return -1 // not on disk
			}
			return int(rp + (rva - va))
		}
	}
	return -1
}

func maxU32(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}

func alignUp(v, a uint32) uint32 {
	if a == 0 {
		return v
	}
	return (v + a - 1) &^ (a - 1)
}

func sanitizeName(name string, i int) string {
	var b strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			b.WriteRune(c)
		} else if c == '.' {
			b.WriteByte('_')
		} else {
			b.WriteByte('x')
		}
	}
	s := b.String()
	if s == "" || s == "_" {
		s = fmt.Sprintf("sec%d", i)
	}
	return s
}
