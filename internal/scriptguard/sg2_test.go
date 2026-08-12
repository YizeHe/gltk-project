package scriptguard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSG2GTAMacroRCDATA(t *testing.T) {
	// Prefer extracted RCDATA next to work tree
	cands := []string{
		filepath.Join("..", "..", "work", "gtamacro", "res", "RCDATA_1_2388445.bin"),
		filepath.Join("..", "..", "work", "gtamacro", "res", "RCDATA_1_2388445.bin"),
	}
	// also absolute-ish from module
	wd, _ := os.Getwd()
	cands = append(cands,
		filepath.Join(wd, "..", "..", "work", "gtamacro", "res", "RCDATA_1_2388445.bin"),
		`D:\grokbuild\groklang\gltk\work\gtamacro\res\RCDATA_1_2388445.bin`,
	)
	var text string
	for _, p := range cands {
		b, err := os.ReadFile(p)
		if err == nil && len(b) > 1000 {
			text = string(b)
			t.Log("using", p)
			break
		}
	}
	if text == "" {
		t.Skip("GTAMacro RCDATA not present")
	}
	if !LooksLikeSG2(text) {
		t.Fatal("expected SG2 markers")
	}
	plain, iv, n, err := DecryptSG2Text(text)
	if err != nil {
		t.Fatal(err)
	}
	if n < 1000 {
		t.Fatalf("too few dwords: %d", n)
	}
	if iv != 0x79debacc {
		t.Fatalf("fileIV got %08x want 79debacc", iv)
	}
	// UTF-16-LE BOM
	if len(plain) < 4 || plain[0] != 0xFF || plain[1] != 0xFE {
		t.Fatalf("expected UTF-16 BOM, head=%x", plain[:min(16, len(plain))])
	}
	// decode roughly
	var b strings.Builder
	for i := 2; i+1 < len(plain) && i < 400; i += 2 {
		u := uint16(plain[i]) | uint16(plain[i+1])<<8
		if u == 0 {
			break
		}
		b.WriteRune(rune(u))
	}
	s := b.String()
	if !strings.Contains(s, "ListLines") && !strings.Contains(strings.ToLower(s), "script") {
		// still may have junk comments first — search more
		full := decodeU16(plain)
		if !strings.Contains(full, "ListLines") && !strings.Contains(full, "YourCrushLY") {
			t.Fatalf("plain missing AHK markers, head=%q", full[:min(200, len(full))])
		}
	}
}

func decodeU16(b []byte) string {
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = uint16(b[i*2]) | uint16(b[i*2+1])<<8
	}
	if len(u) > 0 && u[0] == 0xFEFF {
		u = u[1:]
	}
	var r []rune
	for _, x := range u {
		if x == 0 {
			break
		}
		r = append(r, rune(x))
	}
	return string(r)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestXorshiftTableDeterministic(t *testing.T) {
	a := BuildXorshiftTable()
	b := BuildXorshiftTable()
	if a[0] != b[0] || a[255] != b[255] {
		t.Fatal("table not deterministic")
	}
	if a[0] == 0 && a[1] == 0 {
		t.Fatal("table looks empty")
	}
}
