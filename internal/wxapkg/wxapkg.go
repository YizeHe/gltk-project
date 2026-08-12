// Package wxapkg implements WeChat mini-program package decrypt/parse/unpack.
// Ported from gltk/wxapkg/wechat core (no Wails dependency).
package wxapkg

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

// FileInfo is one entry in a wxapkg index.
type FileInfo struct {
	Name   string
	Offset uint32
	Size   uint32
}

// UnpackOptions controls unpack behavior.
type UnpackOptions struct {
	Wxid         string
	Decrypt      bool
	BeautifyJSON bool
	OutDir       string
}

// UnpackResult is returned by Unpack.
type UnpackResult struct {
	OK       bool
	Count    int
	Error    string
	SavePath string
}

// IsEncrypted reports whether data looks like an encrypted wxapkg (not 0xBE...0xED).
func IsEncrypted(data []byte) bool {
	return !IsDecrypted(data)
}

// IsDecrypted reports whether data has a valid decrypted wxapkg header.
func IsDecrypted(data []byte) bool {
	if len(data) < 14 {
		return false
	}
	return data[0] == 0xBE && data[13] == 0xED
}

// Decrypt decrypts an encrypted wxapkg blob using wxid (pbkdf2+AES-CBC + xor tail).
func Decrypt(data []byte, wxid string) ([]byte, error) {
	if len(data) < 6+1024 {
		return nil, fmt.Errorf("wxapkg: data too short to decrypt")
	}
	const (
		salt = "saltiest"
		iv   = "the iv: 16 bytes"
	)
	dk := pbkdf2.Key([]byte(wxid), []byte(salt), 1000, 32, sha1.New)
	block, err := aes.NewCipher(dk)
	if err != nil {
		return nil, err
	}
	blockMode := cipher.NewCBCDecrypter(block, []byte(iv))
	originData := make([]byte, 1024)
	blockMode.CryptBlocks(originData, data[6:1024+6])

	afData := make([]byte, len(data)-1024-6)
	xorKey := byte(0x66)
	if len(wxid) >= 2 {
		xorKey = wxid[len(wxid)-2]
	}
	for i, b := range data[1024+6:] {
		afData[i] = b ^ xorKey
	}
	// original algorithm keeps 1023 bytes of AES block + xor tail
	out := append(originData[:1023], afData...)
	return out, nil
}

// ListFiles parses a decrypted wxapkg and returns the file index (no extract).
func ListFiles(data []byte) ([]FileInfo, error) {
	files, _, err := parseIndex(data)
	return files, err
}

func parseIndex(data []byte) ([]FileInfo, *bytes.Reader, error) {
	if len(data) < 18 {
		return nil, nil, fmt.Errorf("wxapkg: truncated header")
	}
	f := bytes.NewReader(data)
	var (
		firstMark       uint8
		info1           uint32
		indexInfoLength uint32
		bodyInfoLength  uint32
		lastMark        uint8
	)
	_ = binary.Read(f, binary.BigEndian, &firstMark)
	_ = binary.Read(f, binary.BigEndian, &info1)
	_ = binary.Read(f, binary.BigEndian, &indexInfoLength)
	_ = binary.Read(f, binary.BigEndian, &bodyInfoLength)
	_ = binary.Read(f, binary.BigEndian, &lastMark)
	_ = info1
	_ = indexInfoLength
	_ = bodyInfoLength

	if firstMark != 0xBE || lastMark != 0xED {
		return nil, nil, fmt.Errorf("wxapkg: invalid marks (encrypted or corrupt?)")
	}

	var fileCount uint32
	if err := binary.Read(f, binary.BigEndian, &fileCount); err != nil {
		return nil, nil, err
	}
	if fileCount > 102400 {
		return nil, nil, fmt.Errorf("wxapkg: file count %d exceeds limit", fileCount)
	}

	result := make([]FileInfo, 0, fileCount)
	nameSeen := map[string]bool{}
	for i := uint32(0); i < fileCount; i++ {
		var nameLen uint32
		if err := binary.Read(f, binary.BigEndian, &nameLen); err != nil {
			return nil, nil, err
		}
		if nameLen > 1024 {
			return nil, nil, fmt.Errorf("wxapkg: name length %d too large", nameLen)
		}
		nameBytes := make([]byte, nameLen)
		if _, err := io.ReadAtLeast(f, nameBytes, int(nameLen)); err != nil {
			return nil, nil, err
		}
		var offset, size uint32
		_ = binary.Read(f, binary.BigEndian, &offset)
		_ = binary.Read(f, binary.BigEndian, &size)

		name := string(nameBytes)
		// de-dupe like original: a.txt -> a-1.txt
		base := name
		j := 1
		for nameSeen[name] {
			dot := strings.LastIndex(base, ".")
			if dot == -1 {
				name = fmt.Sprintf("%s-%d", base, j)
			} else {
				name = fmt.Sprintf("%s-%d%s", base[:dot], j, base[dot:])
			}
			j++
		}
		nameSeen[name] = true
		if size > 10*1024*1024 {
			return nil, nil, fmt.Errorf("wxapkg: file %s size %d exceeds 10MB limit", name, size)
		}
		result = append(result, FileInfo{Name: name, Offset: offset, Size: size})
	}
	return result, f, nil
}

// Unpack reads a .wxapkg file, optionally decrypts, and extracts into outDir.
func Unpack(path, outDir string, opts UnpackOptions) UnpackResult {
	res := UnpackResult{SavePath: outDir}
	data, err := os.ReadFile(path)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	if !IsDecrypted(data) {
		if !opts.Decrypt {
			res.Error = "file is encrypted; set decrypt=true and provide wxid"
			return res
		}
		wxid := opts.Wxid
		if wxid == "" {
			res.Error = "encrypted file requires wxid/key"
			return res
		}
		data, err = Decrypt(data, wxid)
		if err != nil {
			res.Error = err.Error()
			return res
		}
	}
	files, err := ListFiles(data)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	outAbs, err := filepath.Abs(outDir)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	if err := os.MkdirAll(outAbs, 0o755); err != nil {
		res.Error = err.Error()
		return res
	}
	count := 0
	for _, fi := range files {
		end := int(fi.Offset) + int(fi.Size)
		if int(fi.Offset) < 0 || end > len(data) {
			res.Error = fmt.Sprintf("file %s OOB offset/size", fi.Name)
			return res
		}
		raw := data[fi.Offset:end]
		relName := strings.TrimPrefix(fi.Name, "/")
		relName = strings.TrimPrefix(relName, "\\")
		if relName == "" || strings.Contains(relName, "..") {
			res.Error = fmt.Sprintf("unsafe name: %s", fi.Name)
			return res
		}
		if opts.BeautifyJSON && strings.EqualFold(filepath.Ext(relName), ".json") {
			raw = PrettyJSON(raw)
		}
		savePath, err := filepath.Abs(filepath.Join(outAbs, filepath.FromSlash(relName)))
		if err != nil {
			res.Error = err.Error()
			return res
		}
		// path traversal check
		if !strings.HasPrefix(savePath, outAbs) {
			res.Error = fmt.Sprintf("path traversal: %s", fi.Name)
			return res
		}
		if err := os.MkdirAll(filepath.Dir(savePath), 0o755); err != nil {
			res.Error = err.Error()
			return res
		}
		if err := os.WriteFile(savePath, raw, 0o600); err != nil {
			res.Error = err.Error()
			return res
		}
		count++
	}
	res.OK = true
	res.Count = count
	res.SavePath = outAbs
	return res
}

// PrettyJSON indents JSON when possible; returns original bytes on failure.
func PrettyJSON(data []byte) []byte {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return data
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return data
	}
	return out
}

// Scan lists .wxapkg files under dir. If recursive is false, only the top level.
func Scan(dir string, recursive bool) ([]string, error) {
	var out []string
	if !recursive {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".wxapkg") {
				out = append(out, filepath.Join(dir, e.Name()))
			}
		}
		return out, nil
	}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".wxapkg") {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

// LoadAndMaybeDecrypt reads path and decrypts if needed.
func LoadAndMaybeDecrypt(path, wxid string, doDecrypt bool) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if IsDecrypted(data) {
		return data, nil
	}
	if !doDecrypt {
		return nil, fmt.Errorf("wxapkg encrypted and decrypt not requested")
	}
	if wxid == "" {
		return nil, fmt.Errorf("wxid required to decrypt")
	}
	return Decrypt(data, wxid)
}
