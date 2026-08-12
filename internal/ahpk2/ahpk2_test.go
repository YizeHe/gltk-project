package ahpk2

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func pkcs7Pad(data []byte, blockSize int) []byte {
	n := blockSize - (len(data) % blockSize)
	if n == 0 {
		n = blockSize
	}
	out := make([]byte, len(data)+n)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(n)
	}
	return out
}

func aesCBCEncrypt(plain, key, iv []byte) []byte {
	padded := pkcs7Pad(plain, aes.BlockSize)
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, padded)
	return out
}

func makeMiniZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("hello ahpk2")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDetectAndDecryptSynthetic(t *testing.T) {
	key, _ := base64.StdEncoding.DecodeString(proKeyB64)
	iv, _ := base64.StdEncoding.DecodeString(proIVB64)

	zipPlain := makeMiniZip(t)
	payload := aesCBCEncrypt(zipPlain, key, iv)
	mac := hmac.New(sha256.New, key)
	mac.Write(payload)
	expMAC := mac.Sum(nil)

	// stub host prefix + payload + trailer
	host := []byte("FAKE-PE-STUB")
	file := append(append(host, payload...), BuildTrailer(int64(len(payload)))...)

	info := DetectBytes(file)
	if !info.OK {
		t.Fatalf("detect: %s", info.Error)
	}
	if info.PayloadSize != int64(len(payload)) {
		t.Fatalf("payload size got %d want %d", info.PayloadSize, len(payload))
	}
	if info.PayloadOffset != int64(len(host)) {
		t.Fatalf("offset got %d want %d", info.PayloadOffset, len(host))
	}

	pl, info2, err := ReadPayloadBytes(file)
	if err != nil {
		t.Fatal(err)
	}
	if !info2.OK || len(pl) != len(payload) {
		t.Fatalf("read payload: %+v len=%d", info2, len(pl))
	}

	prof := Profile{
		Name:        "pro",
		Key:         key,
		IV:          iv,
		ExpectedMAC: expMAC,
	}
	plain, macOK, gotMAC, err := DecryptPayload(pl, prof)
	if err != nil {
		t.Fatal(err)
	}
	if !macOK {
		t.Fatal("mac not ok")
	}
	if !hmac.Equal(gotMAC, expMAC) {
		t.Fatal("mac bytes differ")
	}
	if !IsZip(plain) {
		t.Fatalf("not zip: %q", plain[:min(4, len(plain))])
	}
	if !bytes.Equal(plain, zipPlain) {
		t.Fatalf("plain mismatch len %d vs %d", len(plain), len(zipPlain))
	}
}

func TestUnpackFileSynthetic(t *testing.T) {
	key, _ := base64.StdEncoding.DecodeString(proKeyB64)
	iv, _ := base64.StdEncoding.DecodeString(proIVB64)

	zipPlain := makeMiniZip(t)
	payload := aesCBCEncrypt(zipPlain, key, iv)
	mac := hmac.New(sha256.New, key)
	mac.Write(payload)
	expMAC := mac.Sum(nil)

	dir := t.TempDir()
	sample := filepath.Join(dir, "sample.bin")
	body := append(append([]byte("HOST"), payload...), BuildTrailer(int64(len(payload)))...)
	if err := os.WriteFile(sample, body, 0o644); err != nil {
		t.Fatal(err)
	}

	// Detect without reading whole file path via DetectFile
	info := DetectFile(sample)
	if !info.OK {
		t.Fatal(info.Error)
	}

	out := filepath.Join(dir, "out")
	r := UnpackFile(sample, out, Profile{Name: "pro", Key: key, IV: iv, ExpectedMAC: expMAC})
	if !r.OK {
		t.Fatal(r.Error)
	}
	if !r.MACOK || !r.IsZip {
		t.Fatalf("%+v", r)
	}
	if r.Entries < 1 {
		t.Fatalf("entries %d", r.Entries)
	}
	got, err := os.ReadFile(filepath.Join(r.OutDir, "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello ahpk2" {
		t.Fatalf("got %q", got)
	}
}

func TestDetectBadMagic(t *testing.T) {
	info := DetectBytes([]byte("not-a-trailer!!"))
	if info.OK {
		t.Fatal("expected fail")
	}
}

func TestProfilePro(t *testing.T) {
	p := ProfilePro()
	if p.Name != "pro" || len(p.Key) != 32 || len(p.IV) != 16 || len(p.ExpectedMAC) != 32 {
		t.Fatalf("%+v lens %d %d %d", p.Name, len(p.Key), len(p.IV), len(p.ExpectedMAC))
	}
}

func TestExtractZipTraversal(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	// archive/zip Create will clean names; craft with ../ via Header
	h := &zip.FileHeader{Name: "../evil.txt", Method: zip.Deflate}
	w, err := zw.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("x"))
	_ = zw.Close()

	dir := t.TempDir()
	_, err = ExtractZip(buf.Bytes(), dir)
	if err == nil {
		t.Fatal("expected path traversal reject")
	}
}
