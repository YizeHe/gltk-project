package upx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectGSP5(t *testing.T) {
	p := filepath.Join("..", "..", "testfile", "2", "gsp", "Control", "GSP5.exe")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Skip("sample not present:", err)
	}
	if !Detect(b) {
		t.Fatal("expected UPX detect")
	}
	info, err := Analyze(b)
	if err != nil {
		t.Fatal(err)
	}
	if info.PackHeader == nil || info.PackHeader.Method != M_LZMA {
		t.Fatalf("expected LZMA packheader, got %+v", info.PackHeader)
	}
}

func TestUnpackGSP5Size(t *testing.T) {
	p := filepath.Join("..", "..", "testfile", "2", "gsp", "Control", "GSP5.exe")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Skip(err)
	}
	out, info, err := UnpackPE(b)
	if err != nil {
		t.Fatal(err)
	}
	// Official upx -d yields 3837656 for this sample
	if len(out) != 3837656 {
		t.Fatalf("size %d want 3837656 note=%s", len(out), info.Note)
	}
	if !hasMZ(out) {
		t.Fatal("no MZ")
	}
	if findSection(out, "UPX0") != nil {
		t.Fatal("still has UPX0")
	}
	if findSection(out, ".text") == nil {
		t.Fatal("missing .text")
	}
}
