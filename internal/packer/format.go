// Package packer encrypts GLKB payloads and attaches them to a runtime stub EXE.
package packer

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"

	"golang.org/x/crypto/chacha20poly1305"
)

const (
	// Magic marks the trailer at EOF.
	Magic = "GPK1"
	// TrailerSize fixed footer (little-endian).
	// magic[4] ver u16 flags u16 payload_off u64 payload_len u32 nonce[12] tag[16] key_blob[32] reserved[8]
	TrailerSize = 4 + 2 + 2 + 8 + 4 + 12 + 16 + 32 + 8
	Version     = uint16(1)
	FlagNone    = uint16(0)
)

// Trailer is the on-disk package footer.
type Trailer struct {
	Version     uint16
	Flags       uint16
	PayloadOff  uint64
	PayloadLen  uint32
	Nonce       [12]byte
	Tag         [16]byte // unused when using AEAD seal (tag embedded in ciphertext); keep for layout
	KeyBlob     [32]byte // key XOR mask(stubHash)
	Reserved    [8]byte
}

// Seal encrypts plain with a random key; returns ciphertext (nonce is separate in trailer).
func Seal(plain []byte) (ciphertext, key, nonce []byte, err error) {
	key = make([]byte, chacha20poly1305.KeySize)
	if _, err = rand.Read(key); err != nil {
		return nil, nil, nil, err
	}
	nonce = make([]byte, chacha20poly1305.NonceSize)
	if _, err = rand.Read(nonce); err != nil {
		return nil, nil, nil, err
	}
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, nil, nil, err
	}
	// ciphertext = seal(plain) including poly1305 tag at end
	ciphertext = aead.Seal(nil, nonce, plain, []byte(Magic))
	return ciphertext, key, nonce, nil
}

// Open decrypts ciphertext with key/nonce.
func Open(ciphertext, key, nonce []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, ciphertext, []byte(Magic))
}

// StubHash is SHA-256 of the first min(len, 1<<20) bytes of stub (PE image binding).
func StubHash(stub []byte) [32]byte {
	n := len(stub)
	if n > 1<<20 {
		n = 1 << 20
	}
	return sha256.Sum256(stub[:n])
}

// WrapKey XOR-encodes key with stub hash (not secrecy alone — anti-copy of payload body).
func WrapKey(key []byte, stubHash [32]byte) (blob [32]byte) {
	for i := 0; i < 32; i++ {
		blob[i] = key[i] ^ stubHash[i]
	}
	return blob
}

// UnwrapKey reverses WrapKey.
func UnwrapKey(blob [32]byte, stubHash [32]byte) []byte {
	key := make([]byte, 32)
	for i := 0; i < 32; i++ {
		key[i] = blob[i] ^ stubHash[i]
	}
	return key
}

// BuildExe concatenates stub + ciphertext + trailer.
func BuildExe(stub, ciphertext, key, nonce []byte) ([]byte, error) {
	if len(key) != 32 || len(nonce) != 12 {
		return nil, fmt.Errorf("packer: bad key/nonce size")
	}
	h := StubHash(stub)
	var tr Trailer
	tr.Version = Version
	tr.Flags = FlagNone
	tr.PayloadOff = uint64(len(stub))
	tr.PayloadLen = uint32(len(ciphertext))
	copy(tr.Nonce[:], nonce)
	tr.KeyBlob = WrapKey(key, h)
	// fill reserved with random
	_, _ = rand.Read(tr.Reserved[:])

	out := make([]byte, 0, len(stub)+len(ciphertext)+TrailerSize)
	out = append(out, stub...)
	out = append(out, ciphertext...)
	out = append(out, encodeTrailer(tr)...)
	return out, nil
}

func encodeTrailer(t Trailer) []byte {
	b := make([]byte, TrailerSize)
	copy(b[0:4], Magic)
	binary.LittleEndian.PutUint16(b[4:6], t.Version)
	binary.LittleEndian.PutUint16(b[6:8], t.Flags)
	binary.LittleEndian.PutUint64(b[8:16], t.PayloadOff)
	binary.LittleEndian.PutUint32(b[16:20], t.PayloadLen)
	copy(b[20:32], t.Nonce[:])
	copy(b[32:48], t.Tag[:])
	copy(b[48:80], t.KeyBlob[:])
	copy(b[80:88], t.Reserved[:])
	return b
}

// ParseTrailer reads footer from packed exe bytes.
func ParseTrailer(data []byte) (Trailer, error) {
	var t Trailer
	if len(data) < TrailerSize {
		return t, fmt.Errorf("packer: file too small")
	}
	off := len(data) - TrailerSize
	b := data[off:]
	if string(b[0:4]) != Magic {
		return t, fmt.Errorf("packer: bad magic %q", b[0:4])
	}
	t.Version = binary.LittleEndian.Uint16(b[4:6])
	t.Flags = binary.LittleEndian.Uint16(b[6:8])
	t.PayloadOff = binary.LittleEndian.Uint64(b[8:16])
	t.PayloadLen = binary.LittleEndian.Uint32(b[16:20])
	copy(t.Nonce[:], b[20:32])
	copy(t.Tag[:], b[32:48])
	copy(t.KeyBlob[:], b[48:80])
	copy(t.Reserved[:], b[80:88])
	return t, nil
}

// ExtractPayload decrypts glkb from a packed executable image.
func ExtractPayload(exe []byte) ([]byte, error) {
	tr, err := ParseTrailer(exe)
	if err != nil {
		return nil, err
	}
	if tr.PayloadOff+uint64(tr.PayloadLen) > uint64(len(exe))-uint64(TrailerSize) {
		return nil, fmt.Errorf("packer: payload bounds")
	}
	ct := exe[tr.PayloadOff : tr.PayloadOff+uint64(tr.PayloadLen)]
	stub := exe[:tr.PayloadOff]
	key := UnwrapKey(tr.KeyBlob, StubHash(stub))
	return Open(ct, key, tr.Nonce[:])
}

// ReadFile is a tiny helper.
func ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
