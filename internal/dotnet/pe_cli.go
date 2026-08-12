// Package dotnet parses .NET CLI metadata and disassembles IL (ECMA-335).
package dotnet

import (
	"encoding/binary"
	"fmt"
	"os"
)

// section describes one PE section for RVA→file mapping.
type section struct {
	Name            string
	VirtualSize     uint32
	VirtualAddress  uint32
	SizeOfRawData   uint32
	PointerToRawData uint32
}

// cliHeader is IMAGE_COR20_HEADER.
type cliHeader struct {
	Cb                   uint32
	MajorRuntimeVersion  uint16
	MinorRuntimeVersion  uint16
	MetaDataRVA          uint32
	MetaDataSize         uint32
	Flags                uint32
	EntryPointToken      uint32
	ResourcesRVA         uint32
	ResourcesSize        uint32
	StrongNameSigRVA     uint32
	StrongNameSigSize    uint32
}

// peInfo holds enough PE layout to map RVAs and locate CLI.
type peInfo struct {
	Data     []byte
	Sections []section
	IsPE32Plus bool
	CLI      cliHeader
	HasCLI   bool
}

// COMIMAGE flags
const (
	FlagILOnly            = 0x00000001
	Flag32BitRequired     = 0x00000002
	FlagILLibrary         = 0x00000004
	FlagStrongNameSigned  = 0x00000008
	FlagNativeEntryPoint  = 0x00000010
	FlagTrackDebugData    = 0x00010000
	Flag32BitPreferred    = 0x00020000
)

// IsCLR reports whether data is a PE with a CLI (COM) descriptor.
func IsCLR(data []byte) bool {
	pe, err := parsePE(data)
	return err == nil && pe.HasCLI
}

// Parse parses a .NET assembly from PE bytes already in memory.
func Parse(data []byte) (*Assembly, error) {
	pe, err := parsePE(data)
	if err != nil {
		return nil, err
	}
	if !pe.HasCLI {
		return nil, fmt.Errorf("dotnet: no CLI header (not a managed assembly)")
	}
	return parseAssembly(pe)
}

// ParseFile reads a PE working set (image end for large files) and parses CLI.
// Files larger than 8MB are truncated to max(section end, 8MB) so overlays
// (e.g. 66MB host wrapping a ~440KB .NET image) do not force full reads.
func ParseFile(path string) (*Assembly, error) {
	data, err := ReadPEImage(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// ReadPEImage loads PE bytes needed for CLI/metadata.
// Prefer image_end from section table when file > 8MB.
func ReadPEImage(path string) ([]byte, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	const threshold = 8 << 20
	if st.Size() <= threshold {
		return os.ReadFile(path)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read enough for headers + section table
	headN := 1 << 20 // 1MB head
	if int64(headN) > st.Size() {
		headN = int(st.Size())
	}
	head := make([]byte, headN)
	n, err := f.Read(head)
	if err != nil && n == 0 {
		return nil, err
	}
	head = head[:n]

	end := peImageEnd(head)
	if end <= 0 {
		// fall back to 8MB head
		end = threshold
		if int64(end) > st.Size() {
			end = int(st.Size())
		}
	}
	// pad a little for safety
	if end < n {
		return head[:end], nil
	}
	if int64(end) > st.Size() {
		end = int(st.Size())
	}
	// re-read full image working set
	buf := make([]byte, end)
	rn, err := f.ReadAt(buf, 0)
	if err != nil && rn == 0 {
		return nil, err
	}
	return buf[:rn], nil
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

func parsePE(data []byte) (*peInfo, error) {
	if len(data) < 0x40 {
		return nil, fmt.Errorf("dotnet: PE too small")
	}
	if data[0] != 'M' || data[1] != 'Z' {
		return nil, fmt.Errorf("dotnet: not MZ")
	}
	e := int(binary.LittleEndian.Uint32(data[0x3C:]))
	if e <= 0 || e+24 > len(data) {
		return nil, fmt.Errorf("dotnet: bad e_lfanew")
	}
	if string(data[e:e+4]) != "PE\x00\x00" {
		return nil, fmt.Errorf("dotnet: bad PE signature")
	}
	coff := e + 4
	numSec := int(binary.LittleEndian.Uint16(data[coff+2:]))
	optSize := int(binary.LittleEndian.Uint16(data[coff+16:]))
	optOff := coff + 20
	if optOff+2 > len(data) {
		return nil, fmt.Errorf("dotnet: truncated optional header")
	}
	magic := binary.LittleEndian.Uint16(data[optOff:])
	pe32plus := magic == 0x20b
	if magic != 0x10b && magic != 0x20b {
		return nil, fmt.Errorf("dotnet: unknown optional magic 0x%x", magic)
	}

	// DataDirectory[14] = COM descriptor
	var numDD uint32
	var ddOff int
	if pe32plus {
		if optOff+112 > len(data) {
			return nil, fmt.Errorf("dotnet: truncated PE32+ header")
		}
		numDD = binary.LittleEndian.Uint32(data[optOff+108:])
		ddOff = optOff + 112
	} else {
		if optOff+96 > len(data) {
			return nil, fmt.Errorf("dotnet: truncated PE32 header")
		}
		numDD = binary.LittleEndian.Uint32(data[optOff+92:])
		ddOff = optOff + 96
	}

	secOff := optOff + optSize
	secs := make([]section, 0, numSec)
	for i := 0; i < numSec; i++ {
		off := secOff + i*40
		if off+40 > len(data) {
			break
		}
		name := string(data[off : off+8])
		for j := 0; j < len(name); j++ {
			if name[j] == 0 {
				name = name[:j]
				break
			}
		}
		secs = append(secs, section{
			Name:             name,
			VirtualSize:      binary.LittleEndian.Uint32(data[off+8:]),
			VirtualAddress:   binary.LittleEndian.Uint32(data[off+12:]),
			SizeOfRawData:    binary.LittleEndian.Uint32(data[off+16:]),
			PointerToRawData: binary.LittleEndian.Uint32(data[off+20:]),
		})
	}

	info := &peInfo{
		Data:       data,
		Sections:   secs,
		IsPE32Plus: pe32plus,
	}

	if numDD < 15 {
		return info, nil
	}
	cliDD := ddOff + 14*8
	if cliDD+8 > len(data) {
		return info, nil
	}
	cliRVA := binary.LittleEndian.Uint32(data[cliDD:])
	cliSize := binary.LittleEndian.Uint32(data[cliDD+4:])
	if cliRVA == 0 || cliSize < 72 {
		return info, nil
	}
	cliOff := rvaToFile(secs, cliRVA)
	if cliOff < 0 || cliOff+72 > len(data) {
		return info, nil
	}
	h := cliHeader{
		Cb:                  binary.LittleEndian.Uint32(data[cliOff:]),
		MajorRuntimeVersion: binary.LittleEndian.Uint16(data[cliOff+4:]),
		MinorRuntimeVersion: binary.LittleEndian.Uint16(data[cliOff+6:]),
		MetaDataRVA:         binary.LittleEndian.Uint32(data[cliOff+8:]),
		MetaDataSize:        binary.LittleEndian.Uint32(data[cliOff+12:]),
		Flags:               binary.LittleEndian.Uint32(data[cliOff+16:]),
		EntryPointToken:     binary.LittleEndian.Uint32(data[cliOff+20:]),
		ResourcesRVA:        binary.LittleEndian.Uint32(data[cliOff+24:]),
		ResourcesSize:       binary.LittleEndian.Uint32(data[cliOff+28:]),
		StrongNameSigRVA:    binary.LittleEndian.Uint32(data[cliOff+32:]),
		StrongNameSigSize:   binary.LittleEndian.Uint32(data[cliOff+36:]),
	}
	if h.MetaDataRVA == 0 {
		return info, nil
	}
	info.CLI = h
	info.HasCLI = true
	return info, nil
}

func rvaToFile(secs []section, rva uint32) int {
	for _, s := range secs {
		size := s.VirtualSize
		if s.SizeOfRawData > size {
			size = s.SizeOfRawData
		}
		if rva >= s.VirtualAddress && rva < s.VirtualAddress+size {
			return int(s.PointerToRawData + (rva - s.VirtualAddress))
		}
	}
	return -1
}

func (p *peInfo) rva(rva uint32) int {
	return rvaToFile(p.Sections, rva)
}

func (p *peInfo) sliceRVA(rva, size uint32) ([]byte, error) {
	off := p.rva(rva)
	if off < 0 {
		return nil, fmt.Errorf("dotnet: RVA 0x%x not mapped", rva)
	}
	end := off + int(size)
	if end > len(p.Data) {
		// clamp for partial reads
		if off >= len(p.Data) {
			return nil, fmt.Errorf("dotnet: RVA 0x%x beyond image", rva)
		}
		return p.Data[off:], nil
	}
	return p.Data[off:end], nil
}
