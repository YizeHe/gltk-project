package native

import (
	"encoding/binary"
	"fmt"

	"groklang/gltk/internal/vm"
)

func modulePE() vm.Value {
	fns := map[string]vm.NativeFunc{
		"parse": peParse,
	}
	for k, v := range peExtraFns() {
		fns[k] = v
	}
	return moduleMap(fns)
}

// peParse parses PE bytes into a map (full path still available via pe.parse_file).
// Optional 2nd arg light(bool): skip large resource blobs (default false for compat).
func peParse(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("pe.parse(bytes, light?)")
	}
	data, err := args[0].AsBytes()
	if err != nil {
		return vm.Null(), err
	}
	light := false
	if len(args) >= 2 {
		light = args[1].Truthy()
	}
	return peParseBytes(data, light, "")
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func rvaToFile(data []byte, secOff, numSec int, rva uint32) int {
	for i := 0; i < numSec; i++ {
		off := secOff + i*40
		if off+40 > len(data) {
			break
		}
		vsize := binary.LittleEndian.Uint32(data[off+8:])
		vaddr := binary.LittleEndian.Uint32(data[off+12:])
		rawsize := binary.LittleEndian.Uint32(data[off+16:])
		rawptr := binary.LittleEndian.Uint32(data[off+20:])
		size := vsize
		if rawsize > size {
			size = rawsize
		}
		if rva >= vaddr && rva < vaddr+size {
			return int(rawptr + (rva - vaddr))
		}
	}
	// fallback: identity if within file
	if int(rva) < len(data) {
		return int(rva)
	}
	return -1
}

// walkResources walks IMAGE_RESOURCE_DIRECTORY tree.
// level 0 type, 1 name, 2 lang
func walkResources(data []byte, secOff, numSec int, resBase, dirOff uint32, level int, typeName, resName string) []vm.Value {
	var out []vm.Value
	if int(dirOff)+16 > len(data) {
		return out
	}
	// Characteristics u32, TimeDateStamp u32, Major u16, Minor u16, NumNamed u16, NumId u16
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
			// string at resBase + (nameOrId & 0x7fffffff)
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
			// well-known type ids
			if level == 0 {
				name = peResTypeName(nameOrId)
			}
		}
		if offsetToData&0x80000000 != 0 {
			// subdirectory
			sub := resBase + (offsetToData & 0x7fffffff)
			tn, rn := typeName, resName
			if level == 0 {
				tn = name
			} else if level == 1 {
				rn = name
			}
			out = append(out, walkResources(data, secOff, numSec, resBase, sub, level+1, tn, rn)...)
		} else {
			// data entry
			de := int(resBase + offsetToData)
			if de+16 > len(data) {
				continue
			}
			dataRVA := binary.LittleEndian.Uint32(data[de:])
			dataSize := binary.LittleEndian.Uint32(data[de+4:])
			// codepage := binary.LittleEndian.Uint32(data[de+8:])
			fileOff := rvaToFile(data, secOff, numSec, dataRVA)
			var blob []byte
			if fileOff >= 0 && fileOff+int(dataSize) <= len(data) {
				// zero-copy view
				blob = data[fileOff : fileOff+int(dataSize)]
			}
			lang := name
			m := map[string]vm.Value{
				"type": vm.Str(typeName),
				"name": vm.Str(resName),
				"lang": vm.Str(lang),
				"size": vm.Int(int64(dataSize)),
				"data": vm.Bytes(blob),
			}
			out = append(out, vm.MapVal(m))
		}
	}
	return out
}

func peResTypeName(id uint32) string {
	switch id {
	case 1:
		return "CURSOR"
	case 2:
		return "BITMAP"
	case 3:
		return "ICON"
	case 4:
		return "MENU"
	case 5:
		return "DIALOG"
	case 6:
		return "STRING"
	case 7:
		return "FONTDIR"
	case 8:
		return "FONT"
	case 9:
		return "ACCELERATOR"
	case 10:
		return "RCDATA"
	case 11:
		return "MESSAGETABLE"
	case 12:
		return "GROUP_CURSOR"
	case 14:
		return "GROUP_ICON"
	case 16:
		return "VERSION"
	case 24:
		return "MANIFEST"
	default:
		return fmt.Sprintf("%d", id)
	}
}

func utf16ToString(u []uint16) string {
	// simple BMP
	r := make([]rune, len(u))
	for i, v := range u {
		r[i] = rune(v)
	}
	return string(r)
}
