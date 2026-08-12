package upx

import (
	"encoding/binary"
	"fmt"
)

// rebuildPEFromUPXImage converts UPX decompressed buffer (obuf) into a PE file.
// Layout follows PeFile::unpack0 extra_info trailer.
func rebuildPEFromUPXImage(obuf []byte, packed []byte, filter, filterCTO byte) ([]byte, error) {
	if len(obuf) < 8 {
		return nil, fmt.Errorf("obuf too short")
	}
	uLen := len(obuf)
	skip := int(binary.LittleEndian.Uint32(obuf[uLen-4:]))
	if skip < 0 || skip+248 > uLen {
		return nil, fmt.Errorf("bad extra_info skip=%d u_len=%d", skip, uLen)
	}

	// pe_header_t is 248 bytes for PE32
	const ohSize = 248
	oh := make([]byte, ohSize)
	copy(oh, obuf[skip:skip+ohSize])
	if oh[0] != 'P' || oh[1] != 'E' {
		return nil, fmt.Errorf("oh missing PE signature")
	}
	objs := int(binary.LittleEndian.Uint16(oh[6:8]))
	if objs <= 0 || objs > 96 {
		return nil, fmt.Errorf("bad section count %d", objs)
	}
	secOff := skip + ohSize
	secBytes := objs * 40
	if secOff+secBytes > uLen {
		return nil, fmt.Errorf("section table OOB")
	}
	sections := make([]byte, secBytes)
	copy(sections, obuf[secOff:secOff+secBytes])

	// Parse key fields from oh
	// Optional header starts at offset 0x18 within oh
	codeSize := binary.LittleEndian.Uint32(oh[0x1c:0x20])
	entry := binary.LittleEndian.Uint32(oh[0x28:0x2c])
	codeBase := binary.LittleEndian.Uint32(oh[0x2c:0x30])
	imageBase := binary.LittleEndian.Uint32(oh[0x34:0x38])
	fileAlign := binary.LittleEndian.Uint32(oh[0x3c:0x40])
	if fileAlign == 0 {
		fileAlign = 0x200
	}
	headerSize := binary.LittleEndian.Uint32(oh[0x54:0x58])
	_ = entry
	_ = imageBase
	_ = headerSize

	// First section vaddr = rvamin
	rvamin := binary.LittleEndian.Uint32(sections[12:16])

	// Unfilter code region if needed
	if filter != 0 {
		// filter applies to [codebase - rvamin, codesize)
		start := int(codeBase - rvamin)
		if start >= 0 && start < len(obuf) {
			n := int(codeSize)
			if start+n > len(obuf) {
				n = len(obuf) - start
			}
			if n > 0 {
				unfilterCTO32E8E9(obuf[start:start+n], filterCTO)
			}
		}
	}

	// Minimal DOS stub + PE
	// Use DOS stub from packed file if available
	dosStub := defaultDOSStub()
	if hasMZ(packed) {
		e := int(binary.LittleEndian.Uint32(packed[0x3c:]))
		if e > 0x40 && e < 0x1000 && e < len(packed) {
			dosStub = make([]byte, e)
			copy(dosStub, packed[:e])
			// fix e_lfanew later
		}
	}

	// Build output: DOS + PE oh + sections + pad + section raw data
	// Determine first section with rawdataptr != 0
	type secInfo struct {
		vsize, vaddr, rawsize, rawptr uint32
	}
	secs := make([]secInfo, objs)
	maxEnd := 0
	for i := 0; i < objs; i++ {
		o := i * 40
		secs[i] = secInfo{
			vsize:   binary.LittleEndian.Uint32(sections[o+8:]),
			vaddr:   binary.LittleEndian.Uint32(sections[o+12:]),
			rawsize: binary.LittleEndian.Uint32(sections[o+16:]),
			rawptr:  binary.LittleEndian.Uint32(sections[o+20:]),
		}
	}

	// Compute headers size: align DOS+PE+sec table
	peOff := len(dosStub)
	// Ensure e_lfanew
	if peOff < 0x40 {
		// pad dos stub
		pad := make([]byte, 0x40-peOff)
		dosStub = append(dosStub, pad...)
		peOff = 0x40
	}
	binary.LittleEndian.PutUint32(dosStub[0x3c:], uint32(peOff))

	bodyStart := peOff + ohSize + secBytes
	// first raw pointer
	firstRaw := uint32(0)
	for i := 0; i < objs; i++ {
		if secs[i].rawptr != 0 {
			if firstRaw == 0 || secs[i].rawptr < firstRaw {
				firstRaw = secs[i].rawptr
			}
		}
	}
	if firstRaw == 0 {
		firstRaw = alignUp(uint32(bodyStart), fileAlign)
	}
	// rebuild: write headers then each section at its rawptr
	// size of file
	fileSize := uint32(0)
	for i := 0; i < objs; i++ {
		if secs[i].rawptr != 0 {
			end := secs[i].rawptr + alignUp(secs[i].rawsize, fileAlign)
			// UPX writes ALIGN_UP(size, filealign) for each section
			end = secs[i].rawptr + alignUp(secs[i].vsize, fileAlign)
			// Prefer rawsize if set
			rs := secs[i].rawsize
			if rs == 0 {
				rs = alignUp(secs[i].vsize, fileAlign)
			} else {
				rs = alignUp(rs, fileAlign)
			}
			end = secs[i].rawptr + rs
			if end > fileSize {
				fileSize = end
			}
		}
	}
	if int(fileSize) < int(firstRaw) {
		fileSize = firstRaw + 0x1000
	}

	out := make([]byte, fileSize)
	copy(out, dosStub)
	copy(out[peOff:], oh)
	copy(out[peOff+ohSize:], sections)

	// clear checksum
	// optional header checksum at peOff+0x58 within NT headers = peOff + 0x58
	if peOff+0x58+4 <= len(out) {
		binary.LittleEndian.PutUint32(out[peOff+0x58:], 0)
	}

	for i := 0; i < objs; i++ {
		s := secs[i]
		if s.rawptr == 0 {
			continue
		}
		srcOff := int(s.vaddr - rvamin)
		if srcOff < 0 || srcOff >= len(obuf) {
			continue
		}
		n := int(s.rawsize)
		if n == 0 {
			n = int(s.vsize)
		}
		if srcOff+n > len(obuf) {
			n = len(obuf) - srcOff
		}
		if n < 0 {
			continue
		}
		dstOff := int(s.rawptr)
		if dstOff+n > len(out) {
			// grow
			need := dstOff + int(alignUp(uint32(n), fileAlign))
			if need > len(out) {
				bigger := make([]byte, need)
				copy(bigger, out)
				out = bigger
			}
		}
		copy(out[dstOff:dstOff+n], obuf[srcOff:srcOff+n])
		if dstOff+n > maxEnd {
			maxEnd = dstOff + n
		}
	}

	// Attach packed .rsrc if still missing content? skip for now

	// Overlay from packed
	if end := peImageEnd(packed); end > 0 && end < len(packed) {
		ov := packed[end:]
		out = append(out, ov...)
	}

	if !hasMZ(out) {
		return nil, fmt.Errorf("rebuild failed: no MZ")
	}
	_ = codeSize
	return out, nil
}

func alignUp(v, a uint32) uint32 {
	if a == 0 {
		return v
	}
	return (v + a - 1) &^ (a - 1)
}

func defaultDOSStub() []byte {
	// Minimal MZ + "This program cannot be run in DOS mode"
	stub := make([]byte, 0x80)
	stub[0] = 'M'
	stub[1] = 'Z'
	binary.LittleEndian.PutUint16(stub[8:], 4) // header paragraphs
	binary.LittleEndian.PutUint32(stub[0x3c:], 0x80)
	return stub
}

// unfilterCTO32E8E9 reverses filter 0x26 (cto32_e8e9_bswap_le).
// Simplified E8/E9 absolute→relative conversion with CTO addend.
func unfilterCTO32E8E9(data []byte, cto byte) {
	// Based on UPX f_cto32_e8e9_bswap_le reverse (u_cto32...)
	// For each E8/E9: value is bswap of (addr + cto_add)
	// Practical RE-friendly version: reverse standard E8E9 calltrick
	// which UPX uses with CTO as high-byte marker.
	const size = 5
	cto32 := uint32(cto) << 24
	for i := 0; i+size <= len(data); i++ {
		op := data[i]
		if op != 0xe8 && op != 0xe9 {
			continue
		}
		// read LE32
		v := binary.LittleEndian.Uint32(data[i+1:])
		// reverse bswap if high byte matches CTO scheme
		// UPX CTO32: stores bswap32(next - pos + imagebase-ish)
		// Without full imagebase context, apply classic:
		// rel = v - (i+5) when looking like absolute
		// Try CTO-aware reverse from upx filter:
		//   ax = get_le32; ax = bswap(ax); ax -= (pos+1); put
		// For unfilter: ax = get; ax += (pos+1); bswap; put  -- depends
		// Use simpler PE-relative form that works for string recovery:
		abs := int32(v)
		// if top byte equals cto, use CTO path
		if byte(v>>24) == cto || cto == 0 {
			// bswap then subtract
			bs := (v&0xff)<<24 | (v&0xff00)<<8 | (v&0xff0000)>>8 | (v&0xff000000)>>24
			rel := int32(bs) - int32(i+size) - int32(cto32)
			// store LE
			binary.LittleEndian.PutUint32(data[i+1:], uint32(rel))
		} else {
			rel := abs - int32(i+size)
			binary.LittleEndian.PutUint32(data[i+1:], uint32(rel))
		}
		i += 4
	}
}
