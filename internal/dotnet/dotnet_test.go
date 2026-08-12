package dotnet_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"groklang/gltk/internal/dotnet"
)

func proPath() string {
	candidates := []string{
		`D:\grokbuild\groklang\test\GTAOL Ultra Macro Pro.exe`,
		filepath.Join("..", "..", "..", "test", "GTAOL Ultra Macro Pro.exe"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && st.Size() > 0 {
			return p
		}
	}
	return ""
}

func TestProSample(t *testing.T) {
	path := proPath()
	if path == "" {
		t.Skip("Pro sample not found")
	}
	data, err := dotnet.ReadPEImage(path)
	if err != nil {
		t.Fatal(err)
	}
	if !dotnet.IsCLR(data) {
		t.Fatal("expected CLR")
	}
	// large file should not load full 66MB
	if len(data) > 2<<20 {
		t.Fatalf("ReadPEImage too large: %d", len(data))
	}
	asm, err := dotnet.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if !asm.IsILOnly {
		t.Error("expected ILONLY")
	}
	if !strings.Contains(asm.RuntimeVersion, "v4.") {
		t.Errorf("runtime version: %q", asm.RuntimeVersion)
	}
	us := asm.UserStrings()
	foundAHPK := false
	foundB64 := false
	for _, s := range us {
		if s == "AHPK2" {
			foundAHPK = true
		}
		if strings.HasSuffix(s, "=") && len(s) >= 16 {
			foundB64 = true
		}
	}
	if !foundAHPK {
		t.Errorf("AHPK2 not in #US; strings=%v", us)
	}
	if !foundB64 {
		t.Errorf("expected base64-ish user string; got %v", us)
	}
	types := asm.Types()
	if len(types) < 2 {
		t.Fatalf("expected types, got %d", len(types))
	}
	methods := asm.Methods("")
	if len(methods) == 0 {
		t.Fatal("no methods")
	}
	var anyIL string
	for _, m := range methods {
		if m.RVA == 0 {
			continue
		}
		il, err := asm.DumpIL(m)
		if err != nil {
			t.Errorf("DumpIL %s: %v", m.Name, err)
			continue
		}
		if strings.Contains(il, "ldstr") {
			anyIL = il
		}
		t.Logf("--- %s::%s ---\n%s", m.TypeName, m.Name, il)
	}
	if anyIL == "" {
		t.Error("expected some ldstr in IL")
	} else if !strings.Contains(anyIL, "AHPK2") && !strings.Contains(anyIL, "ldstr") {
		t.Error("IL missing expected content")
	}
	// interesting should include AHPK2
	hit := false
	for _, s := range asm.InterestingStrings() {
		if s == "AHPK2" || strings.Contains(strings.ToLower(s), "password") {
			hit = true
		}
	}
	if !hit {
		t.Errorf("interesting_strings missed keys: %v", asm.InterestingStrings())
	}
}
