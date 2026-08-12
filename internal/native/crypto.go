package native

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"

	"groklang/gltk/internal/vm"
)

func moduleCrypto() vm.Value {
	fns := map[string]vm.NativeFunc{
		"md5_hex":         cryptoMD5Hex,
		"sha256_hex":      cryptoSHA256Hex,
		"hmac_sha256_b64": cryptoHMACSHA256B64,
	}
	for k, v := range cryptoExtraFns() {
		fns[k] = v
	}
	for k, v := range cryptoAesFns() {
		fns[k] = v
	}
	return moduleMap(fns)
}

func cryptoMD5Hex(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("crypto.md5_hex(data)")
	}
	bs, err := args[0].AsBytes()
	if err != nil {
		bs = []byte(args[0].AsStr())
	}
	sum := md5.Sum(bs)
	return vm.Str(hex.EncodeToString(sum[:])), nil
}

func cryptoSHA256Hex(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("crypto.sha256_hex(data)")
	}
	bs, err := args[0].AsBytes()
	if err != nil {
		bs = []byte(args[0].AsStr())
	}
	sum := sha256.Sum256(bs)
	return vm.Str(hex.EncodeToString(sum[:])), nil
}

func cryptoHMACSHA256B64(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("crypto.hmac_sha256_b64(key, data)")
	}
	key, err := args[0].AsBytes()
	if err != nil {
		key = []byte(args[0].AsStr())
	}
	data, err := args[1].AsBytes()
	if err != nil {
		data = []byte(args[1].AsStr())
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return vm.Str(base64.StdEncoding.EncodeToString(mac.Sum(nil))), nil
}
