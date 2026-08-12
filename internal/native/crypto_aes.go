package native

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"

	"golang.org/x/crypto/pbkdf2"

	"groklang/gltk/internal/vm"
)

func cryptoAesFns() map[string]vm.NativeFunc {
	return map[string]vm.NativeFunc{
		"b64_encode":       cryptoB64Encode,
		"b64_decode":       cryptoB64Decode,
		"hex_encode":       cryptoHexEncode,
		"hex_decode":       cryptoHexDecode,
		"hmac_sha256":      cryptoHMACSHA256,
		"aes_cbc_decrypt":  cryptoAESCBCDecrypt,
		"aes_cbc_encrypt":  cryptoAESCBCEncrypt,
		"aes_ecb_decrypt":  cryptoAESECBDecrypt,
		"pbkdf2_sha1":      cryptoPBKDF2SHA1,
		"pbkdf2_sha256":    cryptoPBKDF2SHA256,
	}
}

// cryptoArgBytes accepts bytes or string (prefer AsBytes then AsStr).
func cryptoArgBytes(v vm.Value) []byte {
	if b, err := v.AsBytes(); err == nil {
		return b
	}
	return []byte(v.AsStr())
}

func cryptoB64Encode(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("crypto.b64_encode(data)")
	}
	return vm.Str(base64.StdEncoding.EncodeToString(cryptoArgBytes(args[0]))), nil
}

func cryptoB64Decode(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("crypto.b64_decode(str)")
	}
	s := strings.TrimSpace(args[0].AsStr())
	// Prefer standard encoding; fall back to raw (no padding) for RE dumps.
	out, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		out, err = base64.RawStdEncoding.DecodeString(s)
		if err != nil {
			return vm.Null(), errf("crypto.b64_decode: invalid base64")
		}
	}
	return vm.Bytes(out), nil
}

func cryptoHexEncode(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("crypto.hex_encode(data)")
	}
	return vm.Str(hex.EncodeToString(cryptoArgBytes(args[0]))), nil
}

func cryptoHexDecode(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("crypto.hex_decode(str)")
	}
	s := strings.TrimSpace(args[0].AsStr())
	// Strip optional 0x prefix and whitespace-ish separators common in dumps.
	s = strings.TrimPrefix(strings.ToLower(s), "0x")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\t", "")
	out, err := hex.DecodeString(s)
	if err != nil {
		return vm.Null(), errf("crypto.hex_decode: invalid hex")
	}
	return vm.Bytes(out), nil
}

func cryptoHMACSHA256(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("crypto.hmac_sha256(key, data)")
	}
	key := cryptoArgBytes(args[0])
	data := cryptoArgBytes(args[1])
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return vm.Bytes(mac.Sum(nil)), nil
}

func cryptoPaddingMode(args []vm.Value, idx int) (string, error) {
	pad := "pkcs7"
	if len(args) > idx {
		p := strings.ToLower(strings.TrimSpace(args[idx].AsStr()))
		if p == "" {
			p = "pkcs7"
		}
		switch p {
		case "pkcs7", "pkcs5", "none":
			// PKCS5 is PKCS7 with 8-byte blocks; for AES we treat as PKCS7.
			if p == "pkcs5" {
				p = "pkcs7"
			}
			pad = p
		default:
			return "", errf("crypto: padding must be \"pkcs7\" or \"none\"")
		}
	}
	return pad, nil
}

func cryptoNewAES(key []byte) (cipher.Block, error) {
	switch len(key) {
	case 16, 24, 32:
		// ok AES-128/192/256
	default:
		return nil, errf("crypto: AES key length must be 16, 24, or 32 bytes (got %d)", len(key))
	}
	return aes.NewCipher(key)
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	if blockSize <= 0 || blockSize > 255 {
		panic("pkcs7Pad: invalid block size")
	}
	pad := blockSize - (len(data) % blockSize)
	out := make([]byte, len(data)+pad)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(pad)
	}
	return out
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	n := len(data)
	if n == 0 || n%blockSize != 0 {
		return nil, errf("crypto: PKCS7 unpad: invalid length")
	}
	pad := int(data[n-1])
	if pad == 0 || pad > blockSize || pad > n {
		return nil, errf("crypto: PKCS7 unpad: invalid padding")
	}
	for i := 0; i < pad; i++ {
		if data[n-1-i] != byte(pad) {
			return nil, errf("crypto: PKCS7 unpad: invalid padding")
		}
	}
	return data[:n-pad], nil
}

func cryptoAESCBCDecrypt(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 3 {
		return vm.Null(), errf("crypto.aes_cbc_decrypt(data, key, iv, padding?)")
	}
	data := cryptoArgBytes(args[0])
	key := cryptoArgBytes(args[1])
	iv := cryptoArgBytes(args[2])
	pad, err := cryptoPaddingMode(args, 3)
	if err != nil {
		return vm.Null(), err
	}

	block, err := cryptoNewAES(key)
	if err != nil {
		return vm.Null(), err
	}
	bs := block.BlockSize()
	if len(iv) != bs {
		return vm.Null(), errf("crypto.aes_cbc_decrypt: iv length must be %d (got %d)", bs, len(iv))
	}
	if len(data) == 0 || len(data)%bs != 0 {
		return vm.Null(), errf("crypto.aes_cbc_decrypt: ciphertext length must be multiple of block size")
	}

	out := make([]byte, len(data))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(out, data)
	if pad == "pkcs7" {
		out, err = pkcs7Unpad(out, bs)
		if err != nil {
			return vm.Null(), err
		}
	}
	return vm.Bytes(out), nil
}

func cryptoAESCBCEncrypt(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 3 {
		return vm.Null(), errf("crypto.aes_cbc_encrypt(data, key, iv, padding?)")
	}
	data := cryptoArgBytes(args[0])
	key := cryptoArgBytes(args[1])
	iv := cryptoArgBytes(args[2])
	pad, err := cryptoPaddingMode(args, 3)
	if err != nil {
		return vm.Null(), err
	}

	block, err := cryptoNewAES(key)
	if err != nil {
		return vm.Null(), err
	}
	bs := block.BlockSize()
	if len(iv) != bs {
		return vm.Null(), errf("crypto.aes_cbc_encrypt: iv length must be %d (got %d)", bs, len(iv))
	}

	var plain []byte
	if pad == "pkcs7" {
		plain = pkcs7Pad(data, bs)
	} else {
		if len(data)%bs != 0 {
			return vm.Null(), errf("crypto.aes_cbc_encrypt: with padding \"none\", data length must be multiple of block size")
		}
		plain = data
	}

	out := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, plain)
	return vm.Bytes(out), nil
}

func cryptoAESECBDecrypt(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("crypto.aes_ecb_decrypt(data, key, padding?)")
	}
	data := cryptoArgBytes(args[0])
	key := cryptoArgBytes(args[1])
	pad, err := cryptoPaddingMode(args, 2)
	if err != nil {
		return vm.Null(), err
	}

	block, err := cryptoNewAES(key)
	if err != nil {
		return vm.Null(), err
	}
	bs := block.BlockSize()
	if len(data) == 0 || len(data)%bs != 0 {
		return vm.Null(), errf("crypto.aes_ecb_decrypt: ciphertext length must be multiple of block size")
	}

	out := make([]byte, len(data))
	for i := 0; i < len(data); i += bs {
		block.Decrypt(out[i:i+bs], data[i:i+bs])
	}
	if pad == "pkcs7" {
		out, err = pkcs7Unpad(out, bs)
		if err != nil {
			return vm.Null(), err
		}
	}
	return vm.Bytes(out), nil
}

func cryptoPBKDF2SHA1(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 4 {
		return vm.Null(), errf("crypto.pbkdf2_sha1(password, salt, iterations, key_len)")
	}
	password := cryptoArgBytes(args[0])
	salt := cryptoArgBytes(args[1])
	iters, err := args[2].AsInt()
	if err != nil || iters < 1 {
		return vm.Null(), errf("crypto.pbkdf2_sha1: iterations must be positive int")
	}
	keyLen, err := args[3].AsInt()
	if err != nil || keyLen < 1 {
		return vm.Null(), errf("crypto.pbkdf2_sha1: key_len must be positive int")
	}
	dk := pbkdf2.Key(password, salt, int(iters), int(keyLen), sha1.New)
	return vm.Bytes(dk), nil
}

func cryptoPBKDF2SHA256(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 4 {
		return vm.Null(), errf("crypto.pbkdf2_sha256(password, salt, iterations, key_len)")
	}
	password := cryptoArgBytes(args[0])
	salt := cryptoArgBytes(args[1])
	iters, err := args[2].AsInt()
	if err != nil || iters < 1 {
		return vm.Null(), errf("crypto.pbkdf2_sha256: iterations must be positive int")
	}
	keyLen, err := args[3].AsInt()
	if err != nil || keyLen < 1 {
		return vm.Null(), errf("crypto.pbkdf2_sha256: key_len must be positive int")
	}
	dk := pbkdf2.Key(password, salt, int(iters), int(keyLen), sha256.New)
	return vm.Bytes(dk), nil
}
