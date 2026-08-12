package native

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"groklang/gltk/internal/vm"
)

// buildMinimalELF64 creates a tiny little-endian ELF64 with:
//   - one PT_LOAD segment
//   - sections: NULL, .text, .shstrtab, .strtab, .symtab
//   - one GLOBAL FUNC symbol "main"
func buildMinimalELF64() []byte {
	// Layout plan (file offsets):
	//  0x00: ELF header (64)
	//  0x40: program header (56)  -> phoff=0x40, phnum=1
	//  0x78: .text (4 bytes: 90 c3 90 c3)
	//  0x7c: .shstrtab
	//  ... : .strtab
	//  ... : .symtab (2 entries: null + main)
	//  ... : section headers (5 * 64)

	text := []byte{0x90, 0xc3, 0x90, 0xc3}
	// shstrtab: \0 .text \0 .shstrtab \0 .strtab \0 .symtab \0
	shstr := []byte{
		0,
		'.', 't', 'e', 'x', 't', 0,
		'.', 's', 'h', 's', 't', 'r', 't', 'a', 'b', 0,
		'.', 's', 't', 'r', 't', 'a', 'b', 0,
		'.', 's', 'y', 'm', 't', 'a', 'b', 0,
	}
	// name offsets in shstrtab
	const (
		shNameText     = 1
		shNameShstrtab = 7
		shNameStrtab   = 17
		shNameSymtab   = 25
	)
	// strtab for symbols: \0 main \0
	strtab := []byte{0, 'm', 'a', 'i', 'n', 0}

	const (
		ehSize  = 64
		phSize  = 56
		shSize  = 64
		symSize = 24
		phnum   = 1
		shnum   = 5
	)

	textOff := ehSize + phSize // 0x78
	shstrOff := textOff + len(text)
	strtabOff := shstrOff + len(shstr)
	symtabOff := strtabOff + len(strtab)
	// pad to 8-byte for cleanliness
	for (symtabOff % 8) != 0 {
		// insert pad into file by adjusting offsets after strtab — easier: pad strtab
		strtab = append(strtab, 0)
		symtabOff = strtabOff + len(strtab)
	}
	shoff := symtabOff + symSize*2
	fileSize := shoff + shSize*shnum

	buf := make([]byte, fileSize)
	// e_ident
	copy(buf[0:], []byte{0x7f, 'E', 'L', 'F', 2, 1, 1, 0})
	// rest of header
	bo := binary.LittleEndian
	bo.PutUint16(buf[16:], 2)  // ET_EXEC
	bo.PutUint16(buf[18:], 62) // EM_X86_64
	bo.PutUint32(buf[20:], 1)  // EV_CURRENT
	entry := uint64(0x401000)
	bo.PutUint64(buf[24:], entry)
	bo.PutUint64(buf[32:], ehSize) // e_phoff
	bo.PutUint64(buf[40:], uint64(shoff))
	bo.PutUint32(buf[48:], 0)
	bo.PutUint16(buf[52:], ehSize)
	bo.PutUint16(buf[54:], phSize)
	bo.PutUint16(buf[56:], phnum)
	bo.PutUint16(buf[58:], shSize)
	bo.PutUint16(buf[60:], shnum)
	bo.PutUint16(buf[62:], 2) // e_shstrndx = .shstrtab index

	// program header PT_LOAD
	ph := ehSize
	bo.PutUint32(buf[ph:], 1)   // PT_LOAD
	bo.PutUint32(buf[ph+4:], 5) // PF_R|PF_X
	bo.PutUint64(buf[ph+8:], uint64(textOff))
	bo.PutUint64(buf[ph+16:], entry)
	bo.PutUint64(buf[ph+24:], entry)
	bo.PutUint64(buf[ph+32:], uint64(len(text)))
	bo.PutUint64(buf[ph+40:], uint64(len(text)))
	bo.PutUint64(buf[ph+48:], 0x1000)

	copy(buf[textOff:], text)
	copy(buf[shstrOff:], shstr)
	copy(buf[strtabOff:], strtab)

	// symbols: [0]=null, [1]=main FUNC GLOBAL
	// Elf64_Sym
	// main at index 1
	sym1 := symtabOff + symSize
	bo.PutUint32(buf[sym1:], 1) // name -> "main"
	buf[sym1+4] = (1 << 4) | 2  // GLOBAL|FUNC
	buf[sym1+5] = 0
	bo.PutUint16(buf[sym1+6:], 1) // shndx = .text
	bo.PutUint64(buf[sym1+8:], entry)
	bo.PutUint64(buf[sym1+16:], 4)

	writeShdr := func(idx int, nameOff, typ uint32, flags, addr, off, size, link, info, align, entsz uint64) {
		o := shoff + idx*shSize
		bo.PutUint32(buf[o:], nameOff)
		bo.PutUint32(buf[o+4:], typ)
		bo.PutUint64(buf[o+8:], flags)
		bo.PutUint64(buf[o+16:], addr)
		bo.PutUint64(buf[o+24:], off)
		bo.PutUint64(buf[o+32:], size)
		bo.PutUint32(buf[o+40:], uint32(link))
		bo.PutUint32(buf[o+44:], uint32(info))
		bo.PutUint64(buf[o+48:], align)
		bo.PutUint64(buf[o+56:], entsz)
	}
	// 0 NULL
	writeShdr(0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
	// 1 .text PROGBITS
	writeShdr(1, shNameText, 1, 6 /*ALLOC|EXEC*/, entry, uint64(textOff), uint64(len(text)), 0, 0, 16, 0)
	// 2 .shstrtab STRTAB
	writeShdr(2, shNameShstrtab, 3, 0, 0, uint64(shstrOff), uint64(len(shstr)), 0, 0, 1, 0)
	// 3 .strtab STRTAB
	writeShdr(3, shNameStrtab, 3, 0, 0, uint64(strtabOff), uint64(len(strtab)), 0, 0, 1, 0)
	// 4 .symtab SYMTAB link=.strtab
	writeShdr(4, shNameSymtab, 2, 0, 0, uint64(symtabOff), uint64(symSize*2), 3, 1, 8, symSize)

	return buf
}

func TestELFIsELF(t *testing.T) {
	raw := buildMinimalELF64()
	v, err := elfIsELF(nil, []vm.Value{vm.Bytes(raw)})
	if err != nil {
		t.Fatal(err)
	}
	if !v.B {
		t.Fatal("expected true for valid ELF")
	}
	v, _ = elfIsELF(nil, []vm.Value{vm.Bytes([]byte("MZ..."))})
	if v.B {
		t.Fatal("expected false for non-ELF")
	}
}

func TestELFParseMinimal(t *testing.T) {
	raw := buildMinimalELF64()
	v, err := elfParse(nil, []vm.Value{vm.Bytes(raw)})
	if err != nil {
		t.Fatal(err)
	}
	if v.Typ != vm.TypeMap || v.Map == nil {
		t.Fatalf("want map, got %v", v)
	}
	m := *v.Map
	if m["class"].S != "ELF64" {
		t.Errorf("class = %q", m["class"].S)
	}
	if m["data"].S != "LE" {
		t.Errorf("data = %q", m["data"].S)
	}
	if m["type"].S != "EXEC" {
		t.Errorf("type = %q", m["type"].S)
	}
	if m["machine"].S != "x86_64" {
		t.Errorf("machine = %q", m["machine"].S)
	}
	if m["entry"].I != 0x401000 {
		t.Errorf("entry = %#x", m["entry"].I)
	}
	secs := *m["sections"].Arr
	if len(secs) != 5 {
		t.Fatalf("sections = %d", len(secs))
	}
	// find .text
	foundText := false
	for _, s := range secs {
		sm := *s.Map
		if sm["name"].S == ".text" {
			foundText = true
			if sm["size"].I != 4 {
				t.Errorf(".text size = %d", sm["size"].I)
			}
		}
	}
	if !foundText {
		t.Error("missing .text section")
	}
	segs := *m["segments"].Arr
	if len(segs) != 1 {
		t.Fatalf("segments = %d", len(segs))
	}
	if (*segs[0].Map)["type"].S != "LOAD" {
		t.Errorf("segment type = %q", (*segs[0].Map)["type"].S)
	}
	syms := *m["symbols"].Arr
	foundMain := false
	for _, s := range syms {
		sm := *s.Map
		if sm["name"].S == "main" {
			foundMain = true
			if sm["bind"].S != "GLOBAL" || sm["type"].S != "FUNC" {
				t.Errorf("main bind/type = %s/%s", sm["bind"].S, sm["type"].S)
			}
			if sm["value"].I != 0x401000 {
				t.Errorf("main value = %#x", sm["value"].I)
			}
		}
	}
	if !foundMain {
		t.Error("missing symbol main")
	}
}

func TestELFParsePath(t *testing.T) {
	raw := buildMinimalELF64()
	dir := t.TempDir()
	path := filepath.Join(dir, "tiny.elf")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	v, err := elfParse(nil, []vm.Value{vm.Str(path)})
	if err != nil {
		t.Fatal(err)
	}
	m := *v.Map
	if m["machine"].S != "x86_64" {
		t.Errorf("machine = %q", m["machine"].S)
	}
	if m["path"].S != path {
		t.Errorf("path = %q", m["path"].S)
	}
}

func TestELFSummary(t *testing.T) {
	raw := buildMinimalELF64()
	v, err := elfSummary(nil, []vm.Value{vm.Bytes(raw)})
	if err != nil {
		t.Fatal(err)
	}
	m := *v.Map
	if !m["light"].B {
		t.Error("expected light=true")
	}
	if _, ok := m["symbols"]; ok {
		t.Error("summary should not include symbols list")
	}
	if m["section_count"].I != 5 {
		t.Errorf("section_count = %d", m["section_count"].I)
	}
	if m["symbol_count"].I < 2 {
		t.Errorf("symbol_count = %d", m["symbol_count"].I)
	}
}

func TestELF32Header(t *testing.T) {
	// Minimal ELF32 LE header only (no sections) — should still parse class/machine.
	buf := make([]byte, 52)
	copy(buf, []byte{0x7f, 'E', 'L', 'F', 1, 1, 1, 0})
	bo := binary.LittleEndian
	bo.PutUint16(buf[16:], 3) // ET_DYN
	bo.PutUint16(buf[18:], 3) // EM_386
	bo.PutUint32(buf[20:], 1)
	bo.PutUint32(buf[24:], 0x8048000) // entry
	bo.PutUint32(buf[28:], 0)         // phoff
	bo.PutUint32(buf[32:], 0)         // shoff
	bo.PutUint32(buf[36:], 0)
	bo.PutUint16(buf[40:], 52)
	bo.PutUint16(buf[42:], 32)
	bo.PutUint16(buf[44:], 0)
	bo.PutUint16(buf[46:], 40)
	bo.PutUint16(buf[48:], 0)
	bo.PutUint16(buf[50:], 0)

	v, err := elfParseBytes(buf, "", false)
	if err != nil {
		t.Fatal(err)
	}
	m := *v.Map
	if m["class"].S != "ELF32" {
		t.Errorf("class = %q", m["class"].S)
	}
	if m["machine"].S != "i386" {
		t.Errorf("machine = %q", m["machine"].S)
	}
	if m["type"].S != "DYN" {
		t.Errorf("type = %q", m["type"].S)
	}
	if m["entry"].I != 0x8048000 {
		t.Errorf("entry = %#x", m["entry"].I)
	}
}
