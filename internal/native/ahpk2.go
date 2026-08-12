package native

import (
	"encoding/base64"
	"os"
	"strings"

	"groklang/gltk/internal/ahpk2"
	"groklang/gltk/internal/vm"
)

func moduleAHPK2() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"detect":   ahpk2Detect,
		"info":     ahpk2Info,
		"profiles": ahpk2Profiles,
		"decrypt":  ahpk2Decrypt,
		"unpack":   ahpk2Unpack,
	})
}

func ahpk2Profiles(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
	return vm.Array([]vm.Value{vm.Str("pro")}), nil
}

func infoToMap(info ahpk2.Info) vm.Value {
	m := map[string]vm.Value{
		"ok":             vm.Bool(info.OK),
		"magic":          vm.Str(info.Magic),
		"payload_size":   vm.Int(info.PayloadSize),
		"payload_offset": vm.Int(info.PayloadOffset),
		"file_size":      vm.Int(info.FileSize),
		"error":          vm.Str(info.Error),
	}
	return vm.MapVal(m)
}

// ahpk2.detect(path|bytes) -> map
func ahpk2Detect(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("ahpk2.detect(path|bytes)")
	}
	info, err := ahpk2DetectArg(args[0])
	if err != nil {
		return infoToMap(ahpk2.Info{Error: err.Error()}), nil
	}
	return infoToMap(info), nil
}

// ahpk2.info(path) -> same as detect (+ notes for path)
func ahpk2Info(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("ahpk2.info(path)")
	}
	path := args[0].AsStr()
	info := ahpk2.DetectFile(path)
	m := infoToMap(info).Map
	if m == nil {
		return infoToMap(info), nil
	}
	out := make(map[string]vm.Value, len(*m)+2)
	for k, v := range *m {
		out[k] = v
	}
	// Optional note: large high-entropy overlay typical of AHPK2
	if info.OK && info.PayloadSize > 1024*1024 {
		out["note"] = vm.Str("AHPK2 encrypted overlay (large payload; expect high entropy AES ciphertext)")
	} else if info.OK {
		out["note"] = vm.Str("AHPK2 trailer present")
	} else {
		out["note"] = vm.Str("")
	}
	out["path"] = vm.Str(path)
	return vm.MapVal(out), nil
}

// ahpk2.decrypt(path|bytes, opts?) -> map
func ahpk2Decrypt(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("ahpk2.decrypt(path|bytes, opts?)")
	}
	payload, info, err := ahpk2PayloadArg(args[0])
	if err != nil {
		return vm.MapVal(map[string]vm.Value{
			"ok":    vm.Bool(false),
			"error": vm.Str(err.Error()),
		}), nil
	}
	prof, err := ahpk2ProfileFromOpts(args, 1)
	if err != nil {
		return vm.MapVal(map[string]vm.Value{
			"ok":    vm.Bool(false),
			"error": vm.Str(err.Error()),
		}), nil
	}
	plain, macOK, mac, err := ahpk2.DecryptPayload(payload, prof)
	if err != nil {
		return vm.MapVal(map[string]vm.Value{
			"ok":         vm.Bool(false),
			"error":      vm.Str(err.Error()),
			"mac_ok":     vm.Bool(macOK),
			"mac_b64":    vm.Str(base64.StdEncoding.EncodeToString(mac)),
			"size":       vm.Int(info.PayloadSize),
			"profile":    vm.Str(prof.Name),
			"payload_size": vm.Int(info.PayloadSize),
		}), nil
	}
	// Match Pro semantics: fail if expected MAC set and mismatch
	if len(prof.ExpectedMAC) > 0 && !macOK {
		return vm.MapVal(map[string]vm.Value{
			"ok":      vm.Bool(false),
			"error":   vm.Str("HMAC mismatch"),
			"mac_ok":  vm.Bool(false),
			"mac_b64": vm.Str(base64.StdEncoding.EncodeToString(mac)),
			"size":    vm.Int(int64(len(payload))),
			"profile": vm.Str(prof.Name),
		}), nil
	}
	return vm.MapVal(map[string]vm.Value{
		"ok":           vm.Bool(true),
		"error":        vm.Str(""),
		"plain":        vm.Bytes(plain),
		"mac_ok":       vm.Bool(macOK),
		"mac_b64":      vm.Str(base64.StdEncoding.EncodeToString(mac)),
		"is_zip":       vm.Bool(ahpk2.IsZip(plain)),
		"size":         vm.Int(int64(len(plain))),
		"payload_size": vm.Int(info.PayloadSize),
		"profile":      vm.Str(prof.Name),
	}), nil
}

// ahpk2.unpack(path, out_dir, opts?) -> map
func ahpk2Unpack(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("ahpk2.unpack(path, out_dir, opts?)")
	}
	path := args[0].AsStr()
	outDir := args[1].AsStr()
	prof, err := ahpk2ProfileFromOpts(args, 2)
	if err != nil {
		return vm.MapVal(map[string]vm.Value{
			"ok":    vm.Bool(false),
			"error": vm.Str(err.Error()),
		}), nil
	}
	r := ahpk2.UnpackFile(path, outDir, prof)
	return vm.MapVal(map[string]vm.Value{
		"ok":           vm.Bool(r.OK),
		"error":        vm.Str(r.Error),
		"entries":      vm.Int(int64(r.Entries)),
		"zip_path":     vm.Str(r.ZipPath),
		"out_dir":      vm.Str(r.OutDir),
		"mac_ok":       vm.Bool(r.MACOK),
		"plain_size":   vm.Int(r.PlainSize),
		"payload_size": vm.Int(r.PayloadSize),
		"is_zip":       vm.Bool(r.IsZip),
		"profile":      vm.Str(r.Profile),
	}), nil
}

func ahpk2DetectArg(v vm.Value) (ahpk2.Info, error) {
	if v.Typ == vm.TypeBytes {
		return ahpk2.DetectBytes(v.Bytes), nil
	}
	if v.Typ == vm.TypeStr {
		p := v.S
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return ahpk2.DetectFile(p), nil
		}
		// treat as raw bytes of string content
		return ahpk2.DetectBytes([]byte(p)), nil
	}
	bs, err := v.AsBytes()
	if err != nil {
		return ahpk2.Info{}, errf("ahpk2: expected path or bytes")
	}
	return ahpk2.DetectBytes(bs), nil
}

func ahpk2PayloadArg(v vm.Value) ([]byte, ahpk2.Info, error) {
	if v.Typ == vm.TypeBytes {
		return ahpk2.ReadPayloadBytes(v.Bytes)
	}
	if v.Typ == vm.TypeStr {
		p := v.S
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return ahpk2.ReadPayloadFile(p)
		}
		return ahpk2.ReadPayloadBytes([]byte(p))
	}
	bs, err := v.AsBytes()
	if err != nil {
		return nil, ahpk2.Info{}, errf("ahpk2: expected path or bytes")
	}
	return ahpk2.ReadPayloadBytes(bs)
}

// ahpk2ProfileFromOpts reads opts map at args[idx] if present.
// Keys: profile ("pro" default), key/key_b64, iv/iv_b64, mac/mac_b64.
func ahpk2ProfileFromOpts(args []vm.Value, idx int) (ahpk2.Profile, error) {
	name := "pro"
	var key, iv, mac []byte
	var haveKey, haveIV, haveMAC bool

	if len(args) > idx && args[idx].Typ == vm.TypeMap && args[idx].Map != nil {
		m := *args[idx].Map
		if v, ok := m["profile"]; ok {
			s := strings.TrimSpace(v.AsStr())
			if s != "" {
				name = s
			}
		}
		if b, ok := optBytes(m, "key", "key_b64"); ok {
			key, haveKey = b, true
		}
		if b, ok := optBytes(m, "iv", "iv_b64"); ok {
			iv, haveIV = b, true
		}
		if b, ok := optBytes(m, "mac", "mac_b64"); ok {
			mac, haveMAC = b, true
		}
	}

	// Custom material path: if any of key/iv provided, build custom profile.
	if haveKey || haveIV || (haveMAC && (haveKey || haveIV)) {
		base, err := ahpk2.ProfileByName(name)
		if err != nil {
			// allow fully custom without named base
			base = ahpk2.Profile{Name: "custom"}
		}
		if haveKey {
			base.Key = key
		}
		if haveIV {
			base.IV = iv
		}
		if haveMAC {
			base.ExpectedMAC = mac
		}
		if base.Name == "pro" && (haveKey || haveIV || haveMAC) {
			base.Name = "custom"
		}
		if !haveKey || !haveIV {
			// still need both key and iv
			if len(base.Key) == 0 || len(base.IV) == 0 {
				return ahpk2.Profile{}, errf("ahpk2: key and iv required")
			}
		}
		return base, nil
	}

	return ahpk2.ProfileByName(name)
}

// optBytes reads raw bytes or base64 field from opts map.
func optBytes(m map[string]vm.Value, rawKey, b64Key string) ([]byte, bool) {
	if v, ok := m[rawKey]; ok {
		if v.Typ == vm.TypeBytes {
			return v.Bytes, true
		}
		if v.Typ == vm.TypeStr && v.S != "" {
			// raw key as string bytes, or if looks like b64 for convenience try decode?
			return []byte(v.S), true
		}
	}
	if v, ok := m[b64Key]; ok {
		s := strings.TrimSpace(v.AsStr())
		if s == "" {
			return nil, false
		}
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, false
		}
		return b, true
	}
	return nil, false
}
