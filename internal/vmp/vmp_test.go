package vmp

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// minimal PE32+ with a .vmp0 empty-disk section and a normal section
func fakeVMPPE(t *testing.T) []byte {
	t.Helper()
	// Build a tiny synthetic PE-ish buffer with section table only — enough for Analyze.
	// Simpler: craft MZ + PE + 2 sections.
	buf := make([]byte, 0x800)
	buf[0], buf[1] = 'M', 'Z'
	binary.LittleEndian.PutUint32(buf[0x3C:], 0x80)
	pe := 0x80
	buf[pe], buf[pe+1] = 'P', 'E'
	// COFF
	binary.LittleEndian.PutUint16(buf[pe+4:], 0x8664) // machine
	binary.LittleEndian.PutUint16(buf[pe+6:], 2)      // sections
	binary.LittleEndian.PutUint16(buf[pe+4+16:], 0xF0) // size of optional header
	// Optional PE32+
	opt := pe + 4 + 20
	binary.LittleEndian.PutUint16(buf[opt:], 0x20b)
	binary.LittleEndian.PutUint32(buf[opt+16:], 0x2000) // entry
	binary.LittleEndian.PutUint64(buf[opt+24:], 0x140000000)
	binary.LittleEndian.PutUint32(buf[opt+32:], 0x1000) // sec align
	binary.LittleEndian.PutUint32(buf[opt+36:], 0x200)  // file align
	binary.LittleEndian.PutUint32(buf[opt+56:], 0x4000) // size of image
	// sections at opt+0xF0
	sec := opt + 0xF0
	// .text — has raw
	copy(buf[sec:], []byte(".text\x00\x00\x00"))
	binary.LittleEndian.PutUint32(buf[sec+8:], 0x100)  // vsize
	binary.LittleEndian.PutUint32(buf[sec+12:], 0x1000)
	binary.LittleEndian.PutUint32(buf[sec+16:], 0x200)
	binary.LittleEndian.PutUint32(buf[sec+20:], 0x400)
	// .vmp0 — disk empty
	sec2 := sec + 40
	copy(buf[sec2:], []byte(".vmp0\x00\x00\x00"))
	binary.LittleEndian.PutUint32(buf[sec2+8:], 0x1000)
	binary.LittleEndian.PutUint32(buf[sec2+12:], 0x2000)
	binary.LittleEndian.PutUint32(buf[sec2+16:], 0) // raw size 0
	binary.LittleEndian.PutUint32(buf[sec2+20:], 0)
	// marker string
	copy(buf[0x500:], []byte("VMProtect begin\x00"))
	return buf
}

func TestDetectAndAnalyze(t *testing.T) {
	data := fakeVMPPE(t)
	if !Detect(data) {
		t.Fatal("expected Detect=true")
	}
	info, err := Analyze(data)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsVMP {
		t.Fatalf("expected is_vmp, conf=%d reasons=%v", info.Confidence, info.Reasons)
	}
	if info.EmptyDiskSecs < 1 {
		t.Fatalf("empty_disk=%d", info.EmptyDiskSecs)
	}
}

func TestExtractAndFixDump(t *testing.T) {
	data := fakeVMPPE(t)
	dir := t.TempDir()
	res, err := Extract(data, dir)
	if err != nil {
		t.Fatal(err)
	}
	if res["ok"] != true {
		t.Fatalf("%v", res)
	}
	if _, err := os.Stat(filepath.Join(dir, "vmp_report.json")); err != nil {
		t.Fatal(err)
	}
	// FixDump should not panic
	fixed, info, err := FixDump(data)
	if err != nil && fixed == nil {
		t.Fatal(err)
	}
	if len(fixed) != len(data) {
		t.Fatalf("size %d vs %d", len(fixed), len(data))
	}
	_ = info
}

func TestAssist(t *testing.T) {
	data := fakeVMPPE(t)
	dir := t.TempDir()
	in := filepath.Join(dir, "in.exe")
	if err := os.WriteFile(in, data, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	res, err := Assist(in, out)
	if err != nil {
		t.Fatal(err)
	}
	if res["ok"] != true {
		t.Fatalf("%v", res)
	}
}
