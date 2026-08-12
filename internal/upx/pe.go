package upx

import (
	"encoding/binary"
	"fmt"
	"os"
)

// Method IDs (from UPX conf.h)
const (
	M_NRV2B_8 = 2
	M_NRV2D_8 = 5
	M_NRV2E_8 = 8
	M_LZMA    = 14
)

// PackHeader is the UPX pack header found after "UPX!" magic.
type PackHeader struct {
	Offset     int
	Version    byte
	Format     byte
	Method     byte
	Level      byte
	UAdler     uint32
	CAdler     uint32
	ULen       uint32
	CLen       uint32
	UFileSize  uint32
	Filter     byte
	FilterCTO  byte
	NMRU       byte
	HasChecksum byte
}

// Info summarizes UPX packing on a PE file.
type Info struct {
	IsUPX      bool
	VersionStr string
	PackHeader *PackHeader
	UPX0Raw    uint32
	UPX0Virt   uint32
	UPX1Raw    uint32
	UPX1Virt   uint32
	UPX1Ptr    uint32
	EntryRVA   uint32
	OverlayOff int
	OverlaySz  int
	MethodName string
	Note       string
}

// Detect reports whether data looks UPX-packed PE.
func Detect(data []byte) bool {
	if len(data) < 0x200 {
		return false
	}
	if !hasMZ(data) {
		return false
	}
	if findSection(data, "UPX0") != nil || findSection(data, "UPX1") != nil {
		return true
	}
	return findPackHeader(data) != nil
}

// Analyze returns detailed UPX info.
func Analyze(data []byte) (*Info, error) {
	info := &Info{}
	if !hasMZ(data) {
		return info, fmt.Errorf("not MZ")
	}
	info.IsUPX = Detect(data)
	// version string "3.08"
	if i := findBytes(data, []byte("UPX!")); i >= 5 {
		// look back for "x.yy"
		for j := i - 1; j >= i-8 && j >= 0; j-- {
			if data[j] >= '0' && data[j] <= '9' {
				// try parse
			}
		}
	}
	if i := findBytes(data, []byte{0x33, 0x2e, 0x30}); i >= 0 && i+4 < len(data) { // 3.0
		// "3.0x\0"
		if data[i+3] >= '0' && data[i+3] <= '9' {
			info.VersionStr = string(data[i : i+4])
		}
	}
	if ph := findPackHeader(data); ph != nil {
		info.PackHeader = ph
		info.MethodName = methodName(ph.Method)
	}
	if s := findSection(data, "UPX0"); s != nil {
		info.UPX0Raw = s.SizeOfRawData
		info.UPX0Virt = s.VirtualSize
	}
	if s := findSection(data, "UPX1"); s != nil {
		info.UPX1Raw = s.SizeOfRawData
		info.UPX1Virt = s.VirtualSize
		info.UPX1Ptr = s.PointerToRawData
	}
	if ep, err := peEntryRVA(data); err == nil {
		info.EntryRVA = ep
	}
	if end := peImageEnd(data); end > 0 && end < len(data) {
		info.OverlayOff = end
		info.OverlaySz = len(data) - end
	}
	if !info.IsUPX {
		info.Note = "no UPX sections/header"
	}
	return info, nil
}

// UnpackPE decompresses a standard UPX-packed PE into an (approximately) original PE image.
func UnpackPE(data []byte) ([]byte, *Info, error) {
	info, err := Analyze(data)
	if err != nil {
		return nil, info, err
	}
	if !info.IsUPX {
		return nil, info, fmt.Errorf("not UPX packed")
	}
	ph := info.PackHeader
	if ph == nil {
		return nil, info, fmt.Errorf("UPX pack header not found")
	}
	upx1 := findSection(data, "UPX1")
	if upx1 == nil {
		return nil, info, fmt.Errorf("missing UPX1 section")
	}
	if int(upx1.PointerToRawData)+int(upx1.SizeOfRawData) > len(data) {
		return nil, info, fmt.Errorf("UPX1 out of range")
	}
	secData := data[upx1.PointerToRawData : upx1.PointerToRawData+upx1.SizeOfRawData]

	// Compressed blob is typically the first CLen bytes of UPX1 (loader at the end).
	cLen := int(ph.CLen)
	uLen := int(ph.ULen)
	if cLen <= 0 || cLen > len(secData) {
		// fallback: entire section minus small stub tail
		cLen = len(secData)
		if cLen > 0x1000 {
			cLen -= 0x800
		}
	}
	if uLen <= 0 {
		if info.UPX0Virt > 0 {
			uLen = int(info.UPX0Virt + info.UPX1Virt)
		} else {
			uLen = cLen * 4
		}
	}
	comp := secData[:cLen]

	var raw []byte
	switch ph.Method {
	case M_NRV2B_8:
		raw, err = nrv2bDecompress(comp, uLen)
	case M_NRV2D_8:
		raw, err = nrv2dDecompress(comp, uLen)
	case M_NRV2E_8:
		raw, err = nrv2eDecompress(comp, uLen)
	case M_LZMA:
		raw, err = upxLZMADecompress(comp, uLen)
	default:
		// try LZMA then NRV2B
		raw, err = upxLZMADecompress(comp, uLen)
		if err != nil {
			raw, err = nrv2bDecompress(comp, uLen)
		}
	}
	if err != nil {
		return nil, info, fmt.Errorf("decompress method=%d: %w", ph.Method, err)
	}

	// Prefer if looks like PE already
	if hasMZ(raw) {
		raw = mergeResources(data, raw)
		info.Note = fmt.Sprintf("unpacked method=%s u_len=%d out=%d", methodName(ph.Method), uLen, len(raw))
		return raw, info, nil
	}

	// PE32 UPX stores original NT headers at extra_info trailer — rebuild
	rebuilt, rerr := rebuildPEFromUPXImage(raw, data, ph.Filter, ph.FilterCTO)
	if rerr == nil && hasMZ(rebuilt) {
		info.Note = fmt.Sprintf("unpacked+rebuild method=%s u_len=%d out=%d", methodName(ph.Method), uLen, len(rebuilt))
		return rebuilt, info, nil
	}

	// Sometimes decompressed body is section dump without DOS stub — try prepend / search MZ
	if i := findBytes(raw, []byte{'M', 'Z'}); i > 0 && i < 0x1000 {
		raw = raw[i:]
		if hasMZ(raw) {
			info.Note = "unpacked (shifted to MZ)"
			return raw, info, nil
		}
	}

	// Return raw image for further analysis even if rebuild failed
	note := fmt.Sprintf("decompressed but rebuild failed: %v; returning raw image", rerr)
	if rerr == nil {
		note = "decompressed but no MZ header; returning raw image"
	}
	info.Note = note
	return raw, info, nil
}

// UnpackFile reads path, unpacks, writes outPath.
func UnpackFile(inPath, outPath string) (*Info, error) {
	data, err := os.ReadFile(inPath)
	if err != nil {
		return nil, err
	}
	out, info, err := UnpackPE(data)
	if err != nil {
		return info, err
	}
	if err := os.WriteFile(outPath, out, 0o644); err != nil {
		return info, err
	}
	return info, nil
}

func methodName(m byte) string {
	switch m {
	case M_NRV2B_8:
		return "NRV2B"
	case M_NRV2D_8:
		return "NRV2D"
	case M_NRV2E_8:
		return "NRV2E"
	case M_LZMA:
		return "LZMA"
	default:
		return fmt.Sprintf("method_%d", m)
	}
}

func findPackHeader(data []byte) *PackHeader {
	// Scan for UPX! + plausible version/format/method
	for i := 0; i+24 < len(data); i++ {
		if data[i] != 'U' || data[i+1] != 'P' || data[i+2] != 'X' || data[i+3] != '!' {
			continue
		}
		ver := data[i+4]
		format := data[i+5]
		method := data[i+6]
		level := data[i+7]
		// version typically 10-15 for UPX 3.x; format 1-15 PE variants
		if ver < 8 || ver > 16 {
			continue
		}
		if method != M_NRV2B_8 && method != M_NRV2D_8 && method != M_NRV2E_8 && method != M_LZMA {
			continue
		}
		uLen := binary.LittleEndian.Uint32(data[i+16 : i+20])
		cLen := binary.LittleEndian.Uint32(data[i+20 : i+24])
		ufs := binary.LittleEndian.Uint32(data[i+24 : i+28])
		// sanity
		if cLen == 0 || cLen > uint32(len(data)) {
			continue
		}
		if uLen == 0 || uLen > 256<<20 {
			continue
		}
		if ufs > 256<<20 {
			continue
		}
		// format 9 = W32_PE common; also allow others
		_ = format
		_ = level
		ph := &PackHeader{
			Offset:      i,
			Version:     ver,
			Format:      format,
			Method:      method,
			Level:       level,
			UAdler:      binary.LittleEndian.Uint32(data[i+8 : i+12]),
			CAdler:      binary.LittleEndian.Uint32(data[i+12 : i+16]),
			ULen:        uLen,
			CLen:        cLen,
			UFileSize:   ufs,
		}
		if i+32 <= len(data) {
			ph.Filter = data[i+28]
			ph.FilterCTO = data[i+29]
			ph.NMRU = data[i+30]
			ph.HasChecksum = data[i+31]
		}
		return ph
	}
	return nil
}

func mergeResources(packed, unpacked []byte) []byte {
	// If unpacked PE already has resources, keep as-is.
	if findSection(unpacked, ".rsrc") != nil {
		return unpacked
	}
	rsrc := findSection(packed, ".rsrc")
	if rsrc == nil || rsrc.SizeOfRawData == 0 {
		return unpacked
	}
	// For many UPX packs, resources stay as third section in packed file and
	// are also reconstructed inside uncompressed stream — if missing, append note only.
	return unpacked
}

// --- minimal PE helpers ---

type peSection struct {
	Name             string
	VirtualSize      uint32
	VirtualAddress   uint32
	SizeOfRawData    uint32
	PointerToRawData uint32
}

func hasMZ(data []byte) bool {
	return len(data) >= 0x40 && data[0] == 'M' && data[1] == 'Z'
}

func peEntryRVA(data []byte) (uint32, error) {
	if len(data) < 0x40 {
		return 0, fmt.Errorf("short")
	}
	e := int(binary.LittleEndian.Uint32(data[0x3c:]))
	if e+0x28 > len(data) {
		return 0, fmt.Errorf("bad pe")
	}
	// OptionalHeader.AddressOfEntryPoint at +16 from optional header start
	// PE sig 4 + COFF 20 = 24, then +16
	return binary.LittleEndian.Uint32(data[e+24+16:]), nil
}

func peImageEnd(data []byte) int {
	secs := peSections(data)
	end := 0
	for _, s := range secs {
		e := int(s.PointerToRawData + s.SizeOfRawData)
		if e > end {
			end = e
		}
	}
	return end
}

func peSections(data []byte) []peSection {
	if len(data) < 0x40 {
		return nil
	}
	e := int(binary.LittleEndian.Uint32(data[0x3c:]))
	if e+24 > len(data) || data[e] != 'P' || data[e+1] != 'E' {
		return nil
	}
	num := int(binary.LittleEndian.Uint16(data[e+6:]))
	opt := int(binary.LittleEndian.Uint16(data[e+20:]))
	off := e + 24 + opt
	var out []peSection
	for i := 0; i < num; i++ {
		so := off + i*40
		if so+40 > len(data) {
			break
		}
		name := string(data[so : so+8])
		for j := 0; j < len(name); j++ {
			if name[j] == 0 {
				name = name[:j]
				break
			}
		}
		out = append(out, peSection{
			Name:             name,
			VirtualSize:      binary.LittleEndian.Uint32(data[so+8:]),
			VirtualAddress:   binary.LittleEndian.Uint32(data[so+12:]),
			SizeOfRawData:    binary.LittleEndian.Uint32(data[so+16:]),
			PointerToRawData: binary.LittleEndian.Uint32(data[so+20:]),
		})
	}
	return out
}

func findSection(data []byte, name string) *peSection {
	secs := peSections(data)
	for i := range secs {
		if secs[i].Name == name {
			return &secs[i]
		}
	}
	return nil
}

func findBytes(data, pat []byte) int {
	if len(pat) == 0 || len(data) < len(pat) {
		return -1
	}
	for i := 0; i+len(pat) <= len(data); i++ {
		ok := true
		for j := 0; j < len(pat); j++ {
			if data[i+j] != pat[j] {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}
