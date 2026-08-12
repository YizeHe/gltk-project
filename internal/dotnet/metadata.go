package dotnet

import (
	"encoding/binary"
	"fmt"
)

// stream holds one metadata stream (#~, #Strings, #US, #Blob, #GUID, #-).
type stream struct {
	Name   string
	Offset uint32 // relative to metadata root
	Size   uint32
	Data   []byte
}

// metadataRoot is the BSJB metadata root + stream map.
type metadataRoot struct {
	Version  string
	Streams  map[string]*stream
	Tables   *tablesHeap
	StringsHeap []byte
	USHeap      []byte
	BlobHeap    []byte
	GUIDHeap    []byte
}

func parseMetadata(pe *peInfo) (*metadataRoot, error) {
	h := pe.CLI
	raw, err := pe.sliceRVA(h.MetaDataRVA, h.MetaDataSize)
	if err != nil {
		return nil, err
	}
	// Need at least signature + version header
	if len(raw) < 20 {
		return nil, fmt.Errorf("dotnet: metadata too small")
	}
	if string(raw[0:4]) != "BSJB" {
		return nil, fmt.Errorf("dotnet: bad metadata signature")
	}
	// major, minor at 4,6; reserved at 8; length at 12
	verLen := int(binary.LittleEndian.Uint32(raw[12:]))
	if 16+verLen+4 > len(raw) {
		return nil, fmt.Errorf("dotnet: truncated version string")
	}
	verBytes := raw[16 : 16+verLen]
	// trim NULs
	verEnd := len(verBytes)
	for verEnd > 0 && verBytes[verEnd-1] == 0 {
		verEnd--
	}
	version := string(verBytes[:verEnd])

	// version is padded to 4-byte boundary (already included in Length field padding in file;
	// ECMA: Length is actual string length including null; next fields after padded version)
	// Stream headers start at 16 + aligned(verLen, 4)
	verPad := (verLen + 3) &^ 3
	hdrOff := 16 + verPad
	if hdrOff+4 > len(raw) {
		return nil, fmt.Errorf("dotnet: truncated stream header count")
	}
	// Flags u16, Streams u16
	nStreams := int(binary.LittleEndian.Uint16(raw[hdrOff+2:]))
	pos := hdrOff + 4

	root := &metadataRoot{
		Version: version,
		Streams: make(map[string]*stream, nStreams),
	}

	for i := 0; i < nStreams; i++ {
		if pos+8 > len(raw) {
			return nil, fmt.Errorf("dotnet: truncated stream header %d", i)
		}
		off := binary.LittleEndian.Uint32(raw[pos:])
		size := binary.LittleEndian.Uint32(raw[pos+4:])
		// name null-terminated, padded to 4
		nameStart := pos + 8
		nameEnd := nameStart
		for nameEnd < len(raw) && raw[nameEnd] != 0 {
			nameEnd++
		}
		if nameEnd >= len(raw) {
			return nil, fmt.Errorf("dotnet: bad stream name")
		}
		name := string(raw[nameStart:nameEnd])
		// advance past name + NUL, align to 4 relative to pos
		nameLenWithNul := (nameEnd - nameStart) + 1
		padded := (nameLenWithNul + 3) &^ 3
		pos = nameStart + padded

		var data []byte
		if int(off)+int(size) <= len(raw) {
			data = raw[off : int(off)+int(size)]
		} else if int(off) < len(raw) {
			data = raw[off:]
		}
		s := &stream{Name: name, Offset: off, Size: size, Data: data}
		root.Streams[name] = s
	}

	// bind heaps (support both #~ and #- compressed)
	if s := root.Streams["#Strings"]; s != nil {
		root.StringsHeap = s.Data
	}
	if s := root.Streams["#US"]; s != nil {
		root.USHeap = s.Data
	}
	if s := root.Streams["#Blob"]; s != nil {
		root.BlobHeap = s.Data
	}
	if s := root.Streams["#GUID"]; s != nil {
		root.GUIDHeap = s.Data
	}

	tblStream := root.Streams["#~"]
	if tblStream == nil {
		tblStream = root.Streams["#-"]
	}
	if tblStream == nil {
		return nil, fmt.Errorf("dotnet: missing #~ / #- tables stream")
	}
	th, err := parseTablesHeap(tblStream.Data, root)
	if err != nil {
		return nil, err
	}
	root.Tables = th
	return root, nil
}

// String returns #Strings heap entry at 1-based index (0 = empty).
func (m *metadataRoot) String(idx uint32) string {
	if idx == 0 || m.StringsHeap == nil {
		return ""
	}
	i := int(idx)
	if i >= len(m.StringsHeap) {
		return ""
	}
	end := i
	for end < len(m.StringsHeap) && m.StringsHeap[end] != 0 {
		end++
	}
	return string(m.StringsHeap[i:end])
}

// Blob returns #Blob heap entry at index (0 = empty/null blob).
func (m *metadataRoot) Blob(idx uint32) []byte {
	if idx == 0 || m.BlobHeap == nil {
		return nil
	}
	i := int(idx)
	if i >= len(m.BlobHeap) {
		return nil
	}
	size, n := readCompressedUint(m.BlobHeap[i:])
	i += n
	if i+int(size) > len(m.BlobHeap) {
		return m.BlobHeap[i:]
	}
	return m.BlobHeap[i : i+int(size)]
}

// UserString returns a single #US entry (token index, not US heap offset with type).
// usIndex is the offset into #US (low 24 bits of user string token 0x70xxxxxx).
func (m *metadataRoot) UserString(usIndex uint32) string {
	if usIndex == 0 || m.USHeap == nil {
		return ""
	}
	i := int(usIndex)
	if i >= len(m.USHeap) {
		return ""
	}
	size, n := readCompressedUint(m.USHeap[i:])
	i += n
	if size == 0 {
		return ""
	}
	// final byte is terminal flag; payload is UTF-16 LE of size-1 bytes
	byteLen := int(size) - 1
	if byteLen <= 0 {
		return ""
	}
	if i+byteLen > len(m.USHeap) {
		byteLen = len(m.USHeap) - i
	}
	if byteLen < 0 {
		return ""
	}
	// even length expected
	nChars := byteLen / 2
	runes := make([]rune, nChars)
	for c := 0; c < nChars; c++ {
		runes[c] = rune(binary.LittleEndian.Uint16(m.USHeap[i+c*2:]))
	}
	return string(runes)
}

// AllUserStrings enumerates every non-empty #US string.
func (m *metadataRoot) AllUserStrings() []string {
	if len(m.USHeap) < 1 {
		return nil
	}
	var out []string
	// first byte is always 0 (empty)
	i := 1
	for i < len(m.USHeap) {
		if m.USHeap[i] == 0 {
			i++
			continue
		}
		size, n := readCompressedUint(m.USHeap[i:])
		if n == 0 {
			break
		}
		start := i
		i += n
		if size == 0 {
			continue
		}
		if i+int(size) > len(m.USHeap) {
			break
		}
		s := m.UserString(uint32(start))
		if s != "" {
			out = append(out, s)
		}
		i += int(size)
	}
	return out
}

// readCompressedUint reads ECMA-335 §II.24.2.4 compressed unsigned integer.
func readCompressedUint(b []byte) (value uint32, nbytes int) {
	if len(b) == 0 {
		return 0, 0
	}
	if b[0]&0x80 == 0 {
		return uint32(b[0]), 1
	}
	if b[0]&0xC0 == 0x80 {
		if len(b) < 2 {
			return 0, 0
		}
		return uint32(b[0]&0x3F)<<8 | uint32(b[1]), 2
	}
	if len(b) < 4 {
		return 0, 0
	}
	return uint32(b[0]&0x1F)<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), 4
}
