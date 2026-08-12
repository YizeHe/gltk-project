package native

import (
	"os"
	"regexp"
	"strings"

	"groklang/gltk/internal/vm"
)

func moduleJS() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"find_urls":     jsFindURLs,
		"find_hex_keys": jsFindHexKeys,
		"find_all":      jsFindAll,
		"scan_file":     jsScanFile,
		"contains":      jsContainsFile,
	})
}

func jsFindURLs(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Array(nil), nil
	}
	text := args[0].AsStr()
	if b, err := args[0].AsBytes(); err == nil {
		text = string(b)
	}
	re := regexp.MustCompile(`https?://[a-zA-Z0-9._~:/?#\[\]@!$&'()*+,;=%\-]+`)
	ms := re.FindAllString(text, -1)
	// dedupe preserve order
	seen := map[string]bool{}
	var out []vm.Value
	for _, m := range ms {
		// trim trailing junk common in minified js
		m = strings.TrimRight(m, `",');}`)
		if seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, vm.Str(m))
	}
	return vm.Array(out), nil
}

func jsFindHexKeys(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Array(nil), nil
	}
	text := argText(args[0])
	minLen := 12
	if len(args) >= 2 {
		if n, err := args[1].AsInt(); err == nil && n > 0 {
			minLen = int(n)
		}
	}
	// hex strings of minLen+ (quoted)
	re := regexp.MustCompile(`["']([0-9a-fA-F]{` + itoa(minLen) + `,64})["']`)
	ms := re.FindAllStringSubmatch(text, -1)
	seen := map[string]bool{}
	var out []vm.Value
	for _, m := range ms {
		if len(m) < 2 {
			continue
		}
		k := m[1]
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, vm.Str(k))
	}
	return vm.Array(out), nil
}

func jsFindAll(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Array(nil), nil
	}
	text := argText(args[0])
	pat := args[1].AsStr()
	re, err := regexp.Compile(pat)
	if err != nil {
		return vm.Null(), err
	}
	ms := re.FindAllString(text, 5000)
	arr := make([]vm.Value, len(ms))
	for i, m := range ms {
		arr[i] = vm.Str(m)
	}
	return vm.Array(arr), nil
}

func jsScanFile(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("js.scan_file(path)")
	}
	b, err := os.ReadFile(args[0].AsStr())
	if err != nil {
		return vm.Null(), err
	}
	text := string(b)
	urls, _ := jsFindURLs(nil, []vm.Value{vm.Str(text)})
	keys, _ := jsFindHexKeys(nil, []vm.Value{vm.Str(text), vm.Int(12)})
	// greenhub-ish patterns
	cf := regexp.MustCompile(`https://[a-z0-9]+\.cloudfront\.net`).FindAllString(text, -1)
	seen := map[string]bool{}
	var cfs []vm.Value
	for _, u := range cf {
		if !seen[u] {
			seen[u] = true
			cfs = append(cfs, vm.Str(u))
		}
	}
	hmacKey := ""
	if strings.Contains(text, "5f5749e77a9b") {
		hmacKey = "5f5749e77a9b"
	}
	// also pick 12-char hex that appears near "hmac" or "sign"
	if hmacKey == "" {
		reNear := regexp.MustCompile(`(?i)(?:hmac|sign|secret|key)[^0-9a-fA-F]{0,40}([0-9a-fA-F]{12,32})`)
		if m := reNear.FindStringSubmatch(text); len(m) > 1 {
			hmacKey = m[1]
		}
	}
	return vm.MapVal(map[string]vm.Value{
		"path":           vm.Str(args[0].AsStr()),
		"size":           vm.Int(int64(len(b))),
		"urls":           urls,
		"hex_keys":       keys,
		"cloudfront":     vm.Array(cfs),
		"hmac_key_hint":  vm.Str(hmacKey),
		"has_wes":        vm.Bool(strings.Contains(text, "/wes")),
		"has_server_list": vm.Bool(strings.Contains(text, "server_list")),
		"has_account":    vm.Bool(strings.Contains(text, "/account") || strings.Contains(text, "getNodeAccount") || strings.Contains(text, "GET_NODE")),
	}), nil
}

func jsContainsFile(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Bool(false), nil
	}
	b, err := os.ReadFile(args[0].AsStr())
	if err != nil {
		return vm.Bool(false), nil
	}
	return vm.Bool(strings.Contains(string(b), args[1].AsStr())), nil
}

func argText(v vm.Value) string {
	if b, err := v.AsBytes(); err == nil {
		return string(b)
	}
	return v.AsStr()
}

func itoa(n int) string {
	if n <= 0 {
		return "1"
	}
	// small int to string without strconv import churn
	const digits = "0123456789"
	if n < 10 {
		return string(digits[n])
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = digits[n%10]
		n /= 10
	}
	return string(b[i:])
}
