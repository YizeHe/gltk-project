package native

import (
	"encoding/binary"
	"os"
	"strings"

	"groklang/gltk/internal/vm"
)

const elfMaxSymbols = 5000

func moduleELF() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"parse":   elfParse,
		"is_elf":  elfIsELF,
		"summary": elfSummary,
	})
}

// elf.parse(bytes|path) -> map
func elfParse(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("elf.parse(bytes|path)")
	}
	data, path, err := elfInput(args[0])
	if err != nil {
		return vm.Null(), err
	}
	info, err := elfParseBytes(data, path, false)
	if err != nil {
		return vm.Null(), err
	}
	return info, nil
}

// elf.is_elf(bytes|path) -> bool
func elfIsELF(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Bool(false), nil
	}
	data, _, err := elfInput(args[0])
	if err != nil {
		return vm.Bool(false), nil
	}
	return vm.Bool(isELFMagic(data)), nil
}

// elf.summary(path) -> lighter map (or bytes|path like parse)
func elfSummary(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("elf.summary(path)")
	}
	data, path, err := elfInput(args[0])
	if err != nil {
		return vm.Null(), err
	}
	return elfParseBytes(data, path, true)
}

// elfInput accepts bytes or a filesystem path (string that names an existing file).
// Non-path strings are treated as raw byte content (same as AsBytes).
func elfInput(v vm.Value) (data []byte, path string, err error) {
	if v.Typ == vm.TypeBytes {
		return v.Bytes, "", nil
	}
	if v.Typ == vm.TypeStr {
		p := v.S
		if st, e := os.Stat(p); e == nil && !st.IsDir() {
			b, e := os.ReadFile(p)
			if e != nil {
				return nil, p, e
			}
			return b, p, nil
		}
		return []byte(p), "", nil
	}
	b, e := v.AsBytes()
	return b, "", e
}

func isELFMagic(data []byte) bool {
	return len(data) >= 4 && data[0] == 0x7f && data[1] == 'E' && data[2] == 'L' && data[3] == 'F'
}

type elfReader struct {
	data []byte
	bo   binary.ByteOrder
	cls  int // 1=32, 2=64
}

func (r *elfReader) u16(off int) uint16 {
	if off+2 > len(r.data) {
		return 0
	}
	return r.bo.Uint16(r.data[off:])
}

func (r *elfReader) u32(off int) uint32 {
	if off+4 > len(r.data) {
		return 0
	}
	return r.bo.Uint32(r.data[off:])
}

func (r *elfReader) u64(off int) uint64 {
	if off+8 > len(r.data) {
		return 0
	}
	return r.bo.Uint64(r.data[off:])
}

func (r *elfReader) addr(off int) uint64 {
	if r.cls == 2 {
		return r.u64(off)
	}
	return uint64(r.u32(off))
}

func (r *elfReader) offSize() int {
	if r.cls == 2 {
		return 8
	}
	return 4
}

// elfParseBytes builds the full or light summary map.
// light: skip symbol table body / dynamic (still reports counts when cheap).
func elfParseBytes(data []byte, path string, light bool) (vm.Value, error) {
	if !isELFMagic(data) {
		return vm.Null(), errf("elf: not an ELF file")
	}
	if len(data) < 16 {
		return vm.Null(), errf("elf: too small")
	}
	cls := int(data[4]) // EI_CLASS
	enc := data[5]      // EI_DATA
	if cls != 1 && cls != 2 {
		return vm.Null(), errf("elf: unsupported class %d", cls)
	}
	var bo binary.ByteOrder
	var dataName string
	switch enc {
	case 1:
		bo = binary.LittleEndian
		dataName = "LE"
	case 2:
		bo = binary.BigEndian
		dataName = "BE"
	default:
		return vm.Null(), errf("elf: unsupported endianness %d", enc)
	}
	ehSize := 52
	if cls == 2 {
		ehSize = 64
	}
	if len(data) < ehSize {
		return vm.Null(), errf("elf: truncated header")
	}
	r := &elfReader{data: data, bo: bo, cls: cls}

	eType := r.u16(16)
	eMachine := r.u16(18)
	// e_version at 20
	var eEntry, ePhoff, eShoff uint64
	var eFlags uint32
	var eEhsize, ePhentsize, ePhnum, eShentsize, eShnum, eShstrndx uint16
	if cls == 2 {
		eEntry = r.u64(24)
		ePhoff = r.u64(32)
		eShoff = r.u64(40)
		eFlags = r.u32(48)
		eEhsize = r.u16(52)
		ePhentsize = r.u16(54)
		ePhnum = r.u16(56)
		eShentsize = r.u16(58)
		eShnum = r.u16(60)
		eShstrndx = r.u16(62)
	} else {
		eEntry = uint64(r.u32(24))
		ePhoff = uint64(r.u32(28))
		eShoff = uint64(r.u32(32))
		eFlags = r.u32(36)
		eEhsize = r.u16(40)
		ePhentsize = r.u16(42)
		ePhnum = r.u16(44)
		eShentsize = r.u16(46)
		eShnum = r.u16(48)
		eShstrndx = r.u16(50)
	}
	_ = eEhsize
	_ = eFlags

	className := "ELF32"
	if cls == 2 {
		className = "ELF64"
	}

	// segments
	segments := make([]vm.Value, 0, int(ePhnum))
	if ePhoff > 0 && ePhentsize > 0 && ePhnum > 0 {
		for i := 0; i < int(ePhnum); i++ {
			off := int(ePhoff) + i*int(ePhentsize)
			if off+int(ePhentsize) > len(data) {
				break
			}
			var pType, pFlags uint32
			var pOffset, pVaddr, pPaddr, pFilesz, pMemsz, pAlign uint64
			if cls == 2 {
				pType = r.u32(off)
				pFlags = r.u32(off + 4)
				pOffset = r.u64(off + 8)
				pVaddr = r.u64(off + 16)
				pPaddr = r.u64(off + 24)
				pFilesz = r.u64(off + 32)
				pMemsz = r.u64(off + 40)
				pAlign = r.u64(off + 48)
			} else {
				pType = r.u32(off)
				pOffset = uint64(r.u32(off + 4))
				pVaddr = uint64(r.u32(off + 8))
				pPaddr = uint64(r.u32(off + 12))
				pFilesz = uint64(r.u32(off + 16))
				pMemsz = uint64(r.u32(off + 20))
				pFlags = r.u32(off + 24)
				pAlign = uint64(r.u32(off + 28))
			}
			_ = pPaddr
			_ = pAlign
			segments = append(segments, vm.MapVal(map[string]vm.Value{
				"type":      vm.Str(elfPhTypeName(pType)),
				"type_id":   vm.Int(int64(pType)),
				"offset":    vm.Int(int64(pOffset)),
				"vaddr":     vm.Int(int64(pVaddr)),
				"filesz":    vm.Int(int64(pFilesz)),
				"memsz":     vm.Int(int64(pMemsz)),
				"flags":     vm.Int(int64(pFlags)),
				"flags_str": vm.Str(elfPhFlags(pFlags)),
			}))
		}
	}

	// section headers
	type shdr struct {
		nameOff            uint32
		typ                uint32
		flags, addr, off   uint64
		size, link, info   uint64
		addralign, entsize uint64
	}
	shdrs := make([]shdr, 0, int(eShnum))
	if eShoff > 0 && eShentsize > 0 && eShnum > 0 {
		for i := 0; i < int(eShnum); i++ {
			off := int(eShoff) + i*int(eShentsize)
			if off+int(eShentsize) > len(data) {
				break
			}
			var s shdr
			if cls == 2 {
				s.nameOff = r.u32(off)
				s.typ = r.u32(off + 4)
				s.flags = r.u64(off + 8)
				s.addr = r.u64(off + 16)
				s.off = r.u64(off + 24)
				s.size = r.u64(off + 32)
				s.link = uint64(r.u32(off + 40))
				s.info = uint64(r.u32(off + 44))
				s.addralign = r.u64(off + 48)
				s.entsize = r.u64(off + 56)
			} else {
				s.nameOff = r.u32(off)
				s.typ = r.u32(off + 4)
				s.flags = uint64(r.u32(off + 8))
				s.addr = uint64(r.u32(off + 12))
				s.off = uint64(r.u32(off + 16))
				s.size = uint64(r.u32(off + 20))
				s.link = uint64(r.u32(off + 24))
				s.info = uint64(r.u32(off + 28))
				s.addralign = uint64(r.u32(off + 32))
				s.entsize = uint64(r.u32(off + 36))
			}
			shdrs = append(shdrs, s)
		}
	}

	// section string table
	var shstr []byte
	if int(eShstrndx) < len(shdrs) {
		ss := shdrs[eShstrndx]
		if ss.off+ss.size <= uint64(len(data)) {
			shstr = data[ss.off : ss.off+ss.size]
		}
	}

	sections := make([]vm.Value, 0, len(shdrs))
	for _, s := range shdrs {
		name := elfCString(shstr, int(s.nameOff))
		sections = append(sections, vm.MapVal(map[string]vm.Value{
			"name":    vm.Str(name),
			"type":    vm.Str(elfShTypeName(s.typ)),
			"type_id": vm.Int(int64(s.typ)),
			"addr":    vm.Int(int64(s.addr)),
			"offset":  vm.Int(int64(s.off)),
			"size":    vm.Int(int64(s.size)),
			"flags":   vm.Int(int64(s.flags)),
			"link":    vm.Int(int64(s.link)),
			"info":    vm.Int(int64(s.info)),
			"entsize": vm.Int(int64(s.entsize)),
		}))
	}

	// symbols from SHT_SYMTAB + SHT_DYNSYM
	symbols := make([]vm.Value, 0)
	symTotal := 0
	if !light {
		for _, s := range shdrs {
			if s.typ != 2 /*SYMTAB*/ && s.typ != 11 /*DYNSYM*/ {
				continue
			}
			entsz := s.entsize
			if entsz == 0 {
				if cls == 2 {
					entsz = 24
				} else {
					entsz = 16
				}
			}
			if entsz == 0 || s.size == 0 {
				continue
			}
			// string table linked via sh_link
			var strtab []byte
			if int(s.link) < len(shdrs) {
				st := shdrs[s.link]
				if st.off+st.size <= uint64(len(data)) {
					strtab = data[st.off : st.off+st.size]
				}
			}
			n := int(s.size / entsz)
			symTotal += n
			for i := 0; i < n && len(symbols) < elfMaxSymbols; i++ {
				off := int(s.off) + i*int(entsz)
				if off+int(entsz) > len(data) {
					break
				}
				var nameOff uint32
				var value, size uint64
				var info, other byte
				var shndx uint16
				if cls == 2 {
					// Elf64_Sym: name u32, info u8, other u8, shndx u16, value u64, size u64
					nameOff = r.u32(off)
					info = data[off+4]
					other = data[off+5]
					shndx = r.u16(off + 6)
					value = r.u64(off + 8)
					size = r.u64(off + 16)
				} else {
					// Elf32_Sym: name u32, value u32, size u32, info u8, other u8, shndx u16
					nameOff = r.u32(off)
					value = uint64(r.u32(off + 4))
					size = uint64(r.u32(off + 8))
					info = data[off+12]
					other = data[off+13]
					shndx = r.u16(off + 14)
				}
				_ = other
				_ = shndx
				bind := info >> 4
				typ := info & 0xf
				name := elfCString(strtab, int(nameOff))
				// skip null symbol with empty name at index 0 of each table? keep all for fidelity
				symbols = append(symbols, vm.MapVal(map[string]vm.Value{
					"name":  vm.Str(name),
					"value": vm.Int(int64(value)),
					"size":  vm.Int(int64(size)),
					"bind":  vm.Str(elfSymBind(bind)),
					"type":  vm.Str(elfSymType(typ)),
				}))
			}
		}
	} else {
		// light: only count symbols
		for _, s := range shdrs {
			if s.typ != 2 && s.typ != 11 {
				continue
			}
			entsz := s.entsize
			if entsz == 0 {
				if cls == 2 {
					entsz = 24
				} else {
					entsz = 16
				}
			}
			if entsz > 0 {
				symTotal += int(s.size / entsz)
			}
		}
	}

	// dynamic tags (optional, full parse only)
	dynamic := make([]vm.Value, 0)
	if !light {
		for _, s := range shdrs {
			if s.typ != 6 /*SHT_DYNAMIC*/ {
				continue
			}
			entsz := s.entsize
			if entsz == 0 {
				if cls == 2 {
					entsz = 16
				} else {
					entsz = 8
				}
			}
			n := int(s.size / entsz)
			for i := 0; i < n; i++ {
				off := int(s.off) + i*int(entsz)
				if off+int(entsz) > len(data) {
					break
				}
				var tag int64
				var val uint64
				if cls == 2 {
					tag = int64(r.u64(off))
					val = r.u64(off + 8)
				} else {
					tag = int64(int32(r.u32(off)))
					val = uint64(r.u32(off + 4))
				}
				dynamic = append(dynamic, vm.MapVal(map[string]vm.Value{
					"tag":    vm.Str(elfDynTag(tag)),
					"tag_id": vm.Int(tag),
					"value":  vm.Int(int64(val)),
				}))
				if tag == 0 { // DT_NULL
					break
				}
			}
			break
		}
	}

	result := map[string]vm.Value{
		"ok":            vm.Bool(true),
		"class":         vm.Str(className),
		"data":          vm.Str(dataName),
		"type":          vm.Str(elfTypeName(eType)),
		"type_id":       vm.Int(int64(eType)),
		"machine":       vm.Str(elfMachineName(eMachine)),
		"machine_id":    vm.Int(int64(eMachine)),
		"entry":         vm.Int(int64(eEntry)),
		"size":          vm.Int(int64(len(data))),
		"sections":      vm.Array(sections),
		"segments":      vm.Array(segments),
		"section_count": vm.Int(int64(len(sections))),
		"segment_count": vm.Int(int64(len(segments))),
		"symbol_count":  vm.Int(int64(symTotal)),
	}
	if path != "" {
		result["path"] = vm.Str(path)
	}
	if light {
		// lighter: keep section names/types but drop symbol list & dynamic
		result["light"] = vm.Bool(true)
	} else {
		result["symbols"] = vm.Array(symbols)
		result["dynamic"] = vm.Array(dynamic)
		if len(symbols) >= elfMaxSymbols {
			result["symbols_truncated"] = vm.Bool(true)
		}
	}
	return vm.MapVal(result), nil
}

func elfCString(tab []byte, off int) string {
	if off < 0 || off >= len(tab) {
		return ""
	}
	end := off
	for end < len(tab) && tab[end] != 0 {
		end++
	}
	return string(tab[off:end])
}

func elfTypeName(t uint16) string {
	switch t {
	case 0:
		return "NONE"
	case 1:
		return "REL"
	case 2:
		return "EXEC"
	case 3:
		return "DYN"
	case 4:
		return "CORE"
	default:
		return sprintf("UNKNOWN(%d)", t)
	}
}

func elfMachineName(m uint16) string {
	switch m {
	case 3:
		return "i386"
	case 8:
		return "MIPS"
	case 20:
		return "PPC"
	case 21:
		return "PPC64"
	case 40:
		return "ARM"
	case 50:
		return "IA_64"
	case 62:
		return "x86_64"
	case 183:
		return "ARM64"
	case 243:
		return "RISCV"
	default:
		return sprintf("UNKNOWN(%d)", m)
	}
}

func elfPhTypeName(t uint32) string {
	switch t {
	case 0:
		return "NULL"
	case 1:
		return "LOAD"
	case 2:
		return "DYNAMIC"
	case 3:
		return "INTERP"
	case 4:
		return "NOTE"
	case 5:
		return "SHLIB"
	case 6:
		return "PHDR"
	case 7:
		return "TLS"
	case 0x6474e550:
		return "GNU_EH_FRAME"
	case 0x6474e551:
		return "GNU_STACK"
	case 0x6474e552:
		return "GNU_RELRO"
	default:
		return sprintf("UNKNOWN(0x%x)", t)
	}
}

func elfPhFlags(f uint32) string {
	var b strings.Builder
	if f&4 != 0 {
		b.WriteByte('R')
	} else {
		b.WriteByte('-')
	}
	if f&2 != 0 {
		b.WriteByte('W')
	} else {
		b.WriteByte('-')
	}
	if f&1 != 0 {
		b.WriteByte('X')
	} else {
		b.WriteByte('-')
	}
	return b.String()
}

func elfShTypeName(t uint32) string {
	switch t {
	case 0:
		return "NULL"
	case 1:
		return "PROGBITS"
	case 2:
		return "SYMTAB"
	case 3:
		return "STRTAB"
	case 4:
		return "RELA"
	case 5:
		return "HASH"
	case 6:
		return "DYNAMIC"
	case 7:
		return "NOTE"
	case 8:
		return "NOBITS"
	case 9:
		return "REL"
	case 10:
		return "SHLIB"
	case 11:
		return "DYNSYM"
	case 14:
		return "INIT_ARRAY"
	case 15:
		return "FINI_ARRAY"
	case 0x6ffffff6:
		return "GNU_HASH"
	case 0x6fffffff:
		return "VERSYM"
	case 0x6ffffffd:
		return "VERNEED"
	case 0x6ffffffe:
		return "VERDEF"
	default:
		return sprintf("UNKNOWN(0x%x)", t)
	}
}

func elfSymBind(b byte) string {
	switch b {
	case 0:
		return "LOCAL"
	case 1:
		return "GLOBAL"
	case 2:
		return "WEAK"
	default:
		return sprintf("UNKNOWN(%d)", b)
	}
}

func elfSymType(t byte) string {
	switch t {
	case 0:
		return "NOTYPE"
	case 1:
		return "OBJECT"
	case 2:
		return "FUNC"
	case 3:
		return "SECTION"
	case 4:
		return "FILE"
	case 6:
		return "TLS"
	default:
		return sprintf("UNKNOWN(%d)", t)
	}
}

func elfDynTag(tag int64) string {
	switch tag {
	case 0:
		return "NULL"
	case 1:
		return "NEEDED"
	case 2:
		return "PLTRELSZ"
	case 3:
		return "PLTGOT"
	case 4:
		return "HASH"
	case 5:
		return "STRTAB"
	case 6:
		return "SYMTAB"
	case 7:
		return "RELA"
	case 8:
		return "RELASZ"
	case 9:
		return "RELAENT"
	case 10:
		return "STRSZ"
	case 11:
		return "SYMENT"
	case 12:
		return "INIT"
	case 13:
		return "FINI"
	case 14:
		return "SONAME"
	case 15:
		return "RPATH"
	case 16:
		return "SYMBOLIC"
	case 17:
		return "REL"
	case 18:
		return "RELSZ"
	case 19:
		return "RELENT"
	case 20:
		return "PLTREL"
	case 21:
		return "DEBUG"
	case 23:
		return "JMPREL"
	case 25:
		return "INIT_ARRAY"
	case 26:
		return "FINI_ARRAY"
	case 0x6ffffef5:
		return "GNU_HASH"
	case 0x6ffffff0:
		return "VERSYM"
	case 0x6ffffffe:
		return "VERNEED"
	case 0x6fffffff:
		return "VERNEEDNUM"
	default:
		return sprintf("UNKNOWN(%d)", tag)
	}
}
