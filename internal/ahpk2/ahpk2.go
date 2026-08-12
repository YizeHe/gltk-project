// Package ahpk2 implements AHPK2 trailer detection, AES-256-CBC decrypt, and ZIP unpack.
// Format (13-byte trailer from EOF):
//
//	magic "AHPK2" (5) + int64 LE payload_size (8)
//	payload = file[len-13-size : len-13]
//
// Pro profile uses hardcoded AES key/IV and expected HMAC-SHA256(key, payload).
package ahpk2

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	Magic      = "AHPK2"
	TrailerLen = 13 // 5 magic + 8 size
)

// Hardcoded Pro materials (Main path; password Key() is dead code in host).
const (
	proKeyB64 = "QD3D2gPl8Ywm/SwZDMhV8nPzGwFFmRk4WWQdJjKZd44="
	proIVB64  = "IXAwpGDqS//tpxoMHz+gNw=="
	proMACB64 = "yFsDju+vOJDfLam4sujuAMbGuMbMCs0mizGGzGFVBX4="
)

// Info describes an AHPK2 trailer detection result.
type Info struct {
	Magic         string
	PayloadSize   int64
	PayloadOffset int64 // absolute file offset of payload start
	FileSize      int64
	OK            bool
	Error         string
}

// Profile holds AES/HMAC key material.
type Profile struct {
	Name        string
	Key         []byte
	IV          []byte
	ExpectedMAC []byte // optional; empty = skip equality check (still compute MAC)
}

// UnpackResult is returned by UnpackFile.
type UnpackResult struct {
	OK          bool
	Error       string
	PayloadSize int64
	PlainSize   int64
	IsZip       bool
	MACOK       bool
	MACHex      string
	ZipPath     string
	OutDir      string
	Entries     int
	Profile     string
}

// ProfilePro returns the hardcoded GTAOL Ultra Macro Pro profile.
func ProfilePro() Profile {
	key, _ := base64.StdEncoding.DecodeString(proKeyB64)
	iv, _ := base64.StdEncoding.DecodeString(proIVB64)
	mac, _ := base64.StdEncoding.DecodeString(proMACB64)
	return Profile{
		Name:        "pro",
		Key:         key,
		IV:          iv,
		ExpectedMAC: mac,
	}
}

// ProfileFromOpts builds a profile from raw key/iv/mac bytes.
// Empty ExpectedMAC skips equality check. Name defaults to "custom".
func ProfileFromOpts(key, iv, mac []byte) Profile {
	return Profile{
		Name:        "custom",
		Key:         append([]byte(nil), key...),
		IV:          append([]byte(nil), iv...),
		ExpectedMAC: append([]byte(nil), mac...),
	}
}

// ProfileByName returns a named built-in profile, or error if unknown.
func ProfileByName(name string) (Profile, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "pro":
		return ProfilePro(), nil
	default:
		return Profile{}, fmt.Errorf("ahpk2: unknown profile %q", name)
	}
}

// DetectFile reads only the trailer of path (and validates size) without loading the full file.
func DetectFile(path string) Info {
	f, err := os.Open(path)
	if err != nil {
		return Info{Error: err.Error()}
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return Info{Error: err.Error()}
	}
	size := st.Size()
	if size < TrailerLen {
		return Info{FileSize: size, Error: "file too small for AHPK2 trailer"}
	}
	trail := make([]byte, TrailerLen)
	if _, err := f.ReadAt(trail, size-TrailerLen); err != nil {
		return Info{FileSize: size, Error: err.Error()}
	}
	return detectTrailer(trail, size)
}

// DetectBytes detects AHPK2 trailer from an in-memory blob.
func DetectBytes(data []byte) Info {
	n := int64(len(data))
	if n < TrailerLen {
		return Info{FileSize: n, Error: "data too small for AHPK2 trailer"}
	}
	return detectTrailer(data[n-TrailerLen:], n)
}

func detectTrailer(trail []byte, fileSize int64) Info {
	info := Info{FileSize: fileSize}
	if len(trail) < TrailerLen {
		info.Error = "trailer truncated"
		return info
	}
	magic := string(trail[:5])
	info.Magic = magic
	if magic != Magic {
		info.Error = fmt.Sprintf("bad magic: %q", magic)
		return info
	}
	payloadSize := int64(binary.LittleEndian.Uint64(trail[5:13]))
	info.PayloadSize = payloadSize
	if payloadSize < 0 {
		info.Error = "negative payload size"
		return info
	}
	if fileSize < TrailerLen+payloadSize {
		info.Error = fmt.Sprintf("payload size %d exceeds file", payloadSize)
		return info
	}
	info.PayloadOffset = fileSize - TrailerLen - payloadSize
	info.OK = true
	return info
}

// ReadPayloadFile reads only the AHPK2 ciphertext payload from path (efficient for large files).
func ReadPayloadFile(path string) (payload []byte, info Info, err error) {
	info = DetectFile(path)
	if !info.OK {
		if info.Error != "" {
			return nil, info, fmt.Errorf("ahpk2: %s", info.Error)
		}
		return nil, info, fmt.Errorf("ahpk2: detect failed")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, info, err
	}
	defer f.Close()
	payload = make([]byte, info.PayloadSize)
	if _, err := f.ReadAt(payload, info.PayloadOffset); err != nil {
		return nil, info, err
	}
	return payload, info, nil
}

// ReadPayloadBytes extracts the payload from an in-memory AHPK2 blob.
func ReadPayloadBytes(data []byte) (payload []byte, info Info, err error) {
	info = DetectBytes(data)
	if !info.OK {
		if info.Error != "" {
			return nil, info, fmt.Errorf("ahpk2: %s", info.Error)
		}
		return nil, info, fmt.Errorf("ahpk2: detect failed")
	}
	return data[info.PayloadOffset : info.PayloadOffset+info.PayloadSize], info, nil
}

// ComputeMAC returns HMAC-SHA256(key, payload).
func ComputeMAC(key, payload []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(payload)
	return m.Sum(nil)
}

// pkcs7Unpad strips PKCS#7 padding.
func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, fmt.Errorf("ahpk2: invalid padded length %d", len(data))
	}
	n := int(data[len(data)-1])
	if n < 1 || n > blockSize || n > len(data) {
		return nil, fmt.Errorf("ahpk2: invalid PKCS7 padding byte %d", n)
	}
	for i := 0; i < n; i++ {
		if data[len(data)-1-i] != byte(n) {
			return nil, fmt.Errorf("ahpk2: PKCS7 padding mismatch")
		}
	}
	return data[:len(data)-n], nil
}

// DecryptPayload decrypts AES-256-CBC PKCS7 payload with profile.
// Returns plain, macOK, computed MAC, error.
// macOK is true if ExpectedMAC is empty (skip) or equals computed MAC.
// If ExpectedMAC is set and mismatches, decryption still proceeds; macOK=false.
func DecryptPayload(payload []byte, profile Profile) (plain []byte, macOK bool, mac []byte, err error) {
	if len(profile.Key) != 16 && len(profile.Key) != 24 && len(profile.Key) != 32 {
		return nil, false, nil, fmt.Errorf("ahpk2: key length %d invalid (need 16/24/32)", len(profile.Key))
	}
	if len(profile.IV) != aes.BlockSize {
		return nil, false, nil, fmt.Errorf("ahpk2: iv length %d (need %d)", len(profile.IV), aes.BlockSize)
	}
	if len(payload) == 0 || len(payload)%aes.BlockSize != 0 {
		return nil, false, nil, fmt.Errorf("ahpk2: payload length %d not multiple of block size", len(payload))
	}

	mac = ComputeMAC(profile.Key, payload)
	if len(profile.ExpectedMAC) == 0 {
		macOK = true
	} else {
		macOK = hmac.Equal(mac, profile.ExpectedMAC)
	}

	block, err := aes.NewCipher(profile.Key)
	if err != nil {
		return nil, macOK, mac, err
	}
	dst := make([]byte, len(payload))
	cipher.NewCBCDecrypter(block, profile.IV).CryptBlocks(dst, payload)
	plain, err = pkcs7Unpad(dst, aes.BlockSize)
	if err != nil {
		return nil, macOK, mac, err
	}
	return plain, macOK, mac, nil
}

// IsZip reports whether data starts with PK (ZIP local/header signature).
func IsZip(data []byte) bool {
	return len(data) >= 2 && data[0] == 'P' && data[1] == 'K'
}

// ExtractZip extracts a ZIP blob into outDir with path-traversal safety.
// Returns number of file entries written.
func ExtractZip(zipData []byte, outDir string) (entries int, err error) {
	outAbs, err := filepath.Abs(outDir)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(outAbs, 0o755); err != nil {
		return 0, err
	}
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return 0, fmt.Errorf("ahpk2: zip open: %w", err)
	}
	count := 0
	for _, f := range zr.File {
		rel := filepath.Clean(filepath.FromSlash(f.Name))
		if rel == "." || rel == "" {
			continue
		}
		// Reject absolute paths and .. segments after Clean.
		if filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
			return count, fmt.Errorf("ahpk2: unsafe zip path: %s", f.Name)
		}
		if strings.Contains(rel, "..") {
			return count, fmt.Errorf("ahpk2: unsafe zip path: %s", f.Name)
		}
		target := filepath.Join(outAbs, rel)
		// Ensure target stays under outAbs (Windows-safe: add separator).
		if !strings.HasPrefix(target, outAbs+string(filepath.Separator)) && target != outAbs {
			return count, fmt.Errorf("ahpk2: path traversal: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return count, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return count, err
		}
		rc, err := f.Open()
		if err != nil {
			return count, err
		}
		// #nosec G304 — path checked above
		w, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			rc.Close()
			return count, err
		}
		_, copyErr := io.Copy(w, rc)
		closeErr := w.Close()
		rc.Close()
		if copyErr != nil {
			return count, copyErr
		}
		if closeErr != nil {
			return count, closeErr
		}
		count++
	}
	return count, nil
}

// UnpackFile detects, reads payload, decrypts with profile, writes payload.zip, extracts ZIP.
func UnpackFile(path, outDir string, profile Profile) UnpackResult {
	res := UnpackResult{Profile: profile.Name, OutDir: outDir}
	if res.Profile == "" {
		res.Profile = "custom"
	}
	payload, info, err := ReadPayloadFile(path)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.PayloadSize = info.PayloadSize

	plain, macOK, mac, err := DecryptPayload(payload, profile)
	res.MACOK = macOK
	res.MACHex = hex.EncodeToString(mac)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	// If expected MAC was set and failed, still allow unpack but report (caller may care).
	// Match Python: hard fail on MAC mismatch for Pro.
	if len(profile.ExpectedMAC) > 0 && !macOK {
		res.Error = "HMAC mismatch"
		return res
	}

	res.PlainSize = int64(len(plain))
	res.IsZip = IsZip(plain)

	outAbs, err := filepath.Abs(outDir)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	if err := os.MkdirAll(outAbs, 0o755); err != nil {
		res.Error = err.Error()
		return res
	}
	res.OutDir = outAbs

	zipPath := filepath.Join(outAbs, "payload.zip")
	if err := os.WriteFile(zipPath, plain, 0o644); err != nil {
		res.Error = err.Error()
		return res
	}
	res.ZipPath = zipPath

	unzipDir := filepath.Join(outAbs, "unzipped")
	n, err := ExtractZip(plain, unzipDir)
	if err != nil {
		res.Error = err.Error()
		res.Entries = n
		return res
	}
	res.Entries = n
	res.OutDir = unzipDir
	res.OK = true
	return res
}

// BuildTrailer builds a 13-byte AHPK2 trailer for payload size (for tests / packing).
func BuildTrailer(payloadSize int64) []byte {
	t := make([]byte, TrailerLen)
	copy(t[:5], Magic)
	binary.LittleEndian.PutUint64(t[5:13], uint64(payloadSize))
	return t
}
