package upx

// applyFilter undoes UPX calltrick / e8e9 filters on decompressed image.
// filter id is the PackHeader.filter byte; cto is filter_cto.
func applyFilter(data []byte, filter, cto byte) {
	switch filter {
	case 0x00, 0x01:
		// none / trivial
		return
	case 0x0e, 0x0f, 0x46, 0x49: // E8 / E8E9 family (common x86)
		filterE8E9(data, filter == 0x0f || filter == 0x49)
	case 0x26, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x24, 0x25, 0x36:
		// CT16/CT32 and variants — treat as E8E9 for PE i386
		filterE8E9(data, true)
	default:
		// try E8E9 if looks PE
		if len(data) > 0x40 && data[0] == 'M' && data[1] == 'Z' {
			filterE8E9(data, true)
		}
	}
	_ = cto
}

func filterE8E9(data []byte, e9 bool) {
	// Reverse of UPX calltrick: for each E8/E9, convert absolute back relative.
	// UPX during pack converts relative CALL/JMP displacement to absolute;
	// unpack must convert absolute → relative.
	// abs = (rel + (current_offset+5)) ; reverse: rel = abs - (pos+5)
	const size = 5
	for i := 0; i+size <= len(data); i++ {
		op := data[i]
		if op != 0xe8 && !(e9 && op == 0xe9) {
			continue
		}
		// read 32-bit LE as "absolute" as stored by filter
		abs := int(uint32(data[i+1]) | uint32(data[i+2])<<8 | uint32(data[i+3])<<16 | uint32(data[i+4])<<24)
		// In UPX filter implementation, the stored value is typically:
		//   add eax, position  (absolute address)
		// reverse for file image (position = i):
		rel := abs - (i + size)
		data[i+1] = byte(rel)
		data[i+2] = byte(rel >> 8)
		data[i+3] = byte(rel >> 16)
		data[i+4] = byte(rel >> 24)
		i += 4
	}
}
