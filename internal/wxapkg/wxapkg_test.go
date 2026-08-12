package wxapkg

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// build a minimal decrypted wxapkg with one file "hi.txt" = "hello"
func miniPkg(t *testing.T) []byte {
	t.Helper()
	name := "hi.txt"
	body := []byte("hello")
	// Header: firstMark(1) info1(4) indexInfoLength(4) bodyInfoLength(4) lastMark(1) = 14
	// Then fileCount(4); each file: nameLen(4) name offset(4) size(4)
	nameBytes := []byte(name)

	var index []byte
	var nb [4]byte
	binary.BigEndian.PutUint32(nb[:], uint32(len(nameBytes)))
	index = append(index, nb[:]...)
	index = append(index, nameBytes...)
	offPos := len(index)
	index = append(index, 0, 0, 0, 0) // offset placeholder
	binary.BigEndian.PutUint32(nb[:], uint32(len(body)))
	index = append(index, nb[:]...)

	var fc [4]byte
	binary.BigEndian.PutUint32(fc[:], 1)

	pre := 14 + 4 + len(index)
	binary.BigEndian.PutUint32(index[offPos:offPos+4], uint32(pre))

	hdr := make([]byte, 14)
	hdr[0] = 0xBE
	binary.BigEndian.PutUint32(hdr[1:5], 0)
	binary.BigEndian.PutUint32(hdr[5:9], uint32(len(index)+4))
	binary.BigEndian.PutUint32(hdr[9:13], uint32(len(body)))
	hdr[13] = 0xED

	out := append([]byte{}, hdr...)
	out = append(out, fc[:]...)
	out = append(out, index...)
	out = append(out, body...)
	return out
}

func TestListFilesMini(t *testing.T) {
	data := miniPkg(t)
	if !IsDecrypted(data) {
		t.Fatal("expected decrypted marks")
	}
	files, err := ListFiles(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name != "hi.txt" {
		t.Fatalf("%+v", files)
	}
	start := int(files[0].Offset)
	end := start + int(files[0].Size)
	if string(data[start:end]) != "hello" {
		t.Fatalf("body %q", data[start:end])
	}
}

func TestUnpackMini(t *testing.T) {
	data := miniPkg(t)
	dir := t.TempDir()
	pkg := filepath.Join(dir, "t.wxapkg")
	if err := os.WriteFile(pkg, data, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	r := Unpack(pkg, out, UnpackOptions{})
	if !r.OK {
		t.Fatal(r.Error)
	}
	if r.Count != 1 {
		t.Fatalf("count=%d", r.Count)
	}
	b, err := os.ReadFile(filepath.Join(out, "hi.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hello" {
		t.Fatalf("content %q", b)
	}
}

func TestPrettyJSON(t *testing.T) {
	in := []byte(`{"a":1}`)
	out := PrettyJSON(in)
	if len(out) < len(in) {
		t.Fatalf("%s", out)
	}
}
