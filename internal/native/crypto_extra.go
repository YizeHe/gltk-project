package native

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"groklang/gltk/internal/vm"
)

func cryptoExtraFns() map[string]vm.NativeFunc {
	return map[string]vm.NativeFunc{
		"hmac_sha256_hex": cryptoHMACSHA256Hex,
		"sign_query":      cryptoSignQuery,
		"uuid4":           cryptoUUID4,
		"now_ms":          cryptoNowMS,
	}
}

func cryptoHMACSHA256Hex(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("crypto.hmac_sha256_hex(key, data)")
	}
	key := []byte(args[0].AsStr())
	if b, err := args[0].AsBytes(); err == nil {
		key = b
	}
	data := []byte(args[1].AsStr())
	if b, err := args[1].AsBytes(); err == nil {
		data = b
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return vm.Str(hex.EncodeToString(mac.Sum(nil))), nil
}

// crypto.sign_query(key, map) -> {msg, sign, query}
// GreenHub style: sort keys, join k=v with &, HMAC-SHA256 base64.
func cryptoSignQuery(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 || args[1].Typ != vm.TypeMap || args[1].Map == nil {
		return vm.Null(), errf("crypto.sign_query(key, params_map)")
	}
	key := []byte(args[0].AsStr())
	m := *args[1].Map
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+m[k].AsStr())
	}
	msg := strings.Join(parts, "&")
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msg))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return vm.MapVal(map[string]vm.Value{
		"msg":  vm.Str(msg),
		"sign": vm.Str(sign),
	}), nil
}

func cryptoUUID4(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return vm.Null(), err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	s := fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
	return vm.Str(s), nil
}

func cryptoNowMS(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
	return vm.Int(time.Now().UnixMilli()), nil
}
