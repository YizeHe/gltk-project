package native

import (
	"bytes"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"groklang/gltk/internal/vm"
)

func moduleHTTP() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"get":      httpGet,
		"post":     httpPost,
		"put":      httpPut,
		"delete":   httpDelete,
		"head":     httpHead,
		"patch":    httpPatch,
		"request":  httpRequest,
		"download": httpDownload,
		// form body helper (application/x-www-form-urlencoded)
		"encode_form": httpEncodeForm,
	})
}

// Shared transport for connection reuse (performance).
var (
	httpTrOnce sync.Once
	httpTr     *http.Transport
)

func sharedTransport() *http.Transport {
	httpTrOnce.Do(func() {
		httpTr = &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          64,
			MaxIdleConnsPerHost:   16,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   15 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			// Default RE-friendly; overridable per-request via insecure:false → new client.
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		}
	})
	return httpTr
}

type httpOpts struct {
	timeoutSec     int64
	headers        map[string]string
	userAgent      string
	insecureTLS    bool // default true
	followRedirect bool // default true
	proxyURL       string
	maxBody        int64 // default 32MB
}

func defaultHTTPOpts() httpOpts {
	return httpOpts{
		timeoutSec:     30,
		headers:        map[string]string{},
		userAgent:      "GLTK-HTTP/0.5",
		insecureTLS:    true,
		followRedirect: true,
		maxBody:        32 << 20,
	}
}

// parseHTTPOpts from trailing map/int args. Used by get/post/request.
// Accepts: ... headers_map?, timeout_int?, opts_map?
// opts_map keys: timeout, headers, user_agent, ua, insecure, follow_redirects, proxy, max_body
func parseHTTPOpts(args []vm.Value, start int) httpOpts {
	o := defaultHTTPOpts()
	for i := start; i < len(args); i++ {
		a := args[i]
		if a.Typ == vm.TypeInt {
			if t, err := a.AsInt(); err == nil && t > 0 {
				o.timeoutSec = t
			}
			continue
		}
		if a.Typ == vm.TypeMap && a.Map != nil {
			m := *a.Map
			// bare headers map if no known option keys
			if isOptsMap(m) {
				applyOptsMap(&o, m)
			} else {
				for k, v := range m {
					o.headers[k] = v.AsStr()
				}
			}
		}
	}
	return o
}

func isOptsMap(m map[string]vm.Value) bool {
	keys := []string{"timeout", "headers", "user_agent", "ua", "insecure", "follow_redirects", "proxy", "max_body", "header"}
	for _, k := range keys {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

func applyOptsMap(o *httpOpts, m map[string]vm.Value) {
	if v, ok := m["timeout"]; ok {
		if t, err := v.AsInt(); err == nil && t > 0 {
			o.timeoutSec = t
		}
	}
	if v, ok := m["user_agent"]; ok {
		o.userAgent = v.AsStr()
	}
	if v, ok := m["ua"]; ok {
		o.userAgent = v.AsStr()
	}
	if v, ok := m["insecure"]; ok {
		o.insecureTLS = v.Truthy()
	}
	if v, ok := m["follow_redirects"]; ok {
		o.followRedirect = v.Truthy()
	}
	if v, ok := m["proxy"]; ok {
		o.proxyURL = v.AsStr()
	}
	if v, ok := m["max_body"]; ok {
		if n, err := v.AsInt(); err == nil && n > 0 {
			o.maxBody = n
		}
	}
	if v, ok := m["headers"]; ok && v.Typ == vm.TypeMap && v.Map != nil {
		for k, hv := range *v.Map {
			o.headers[k] = hv.AsStr()
		}
	}
	if v, ok := m["header"]; ok && v.Typ == vm.TypeMap && v.Map != nil {
		for k, hv := range *v.Map {
			o.headers[k] = hv.AsStr()
		}
	}
}

func httpClientFrom(o httpOpts) *http.Client {
	// Fast path: default RE settings → shared transport
	useShared := o.insecureTLS && o.proxyURL == "" && o.followRedirect
	var tr *http.Transport
	if useShared {
		tr = sharedTransport()
	} else {
		tr = sharedTransport().Clone()
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: o.insecureTLS} //nolint:gosec
		if o.proxyURL != "" {
			u, err := url.Parse(o.proxyURL)
			if err == nil {
				tr.Proxy = http.ProxyURL(u)
			}
		}
	}
	c := &http.Client{
		Timeout:   time.Duration(o.timeoutSec) * time.Second,
		Transport: tr,
	}
	if !o.followRedirect {
		c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return c
}

func httpGet(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("http.get(url, headers_or_opts?, timeout?)")
	}
	o := parseHTTPOpts(args, 1)
	return httpDo("GET", args[0].AsStr(), nil, o)
}

func httpPost(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("http.post(url, body?, headers_or_opts?, timeout?)")
	}
	var body []byte
	start := 1
	if len(args) >= 2 && args[1].Typ != vm.TypeMap && args[1].Typ != vm.TypeInt {
		body = valueToBody(args[1])
		start = 2
	}
	o := parseHTTPOpts(args, start)
	if _, ok := o.headers["Content-Type"]; !ok {
		if _, ok := o.headers["content-type"]; !ok && body != nil {
			o.headers["Content-Type"] = "application/json"
		}
	}
	return httpDo("POST", args[0].AsStr(), body, o)
}

func httpPut(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	return httpMethodBody("PUT", args)
}
func httpPatch(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	return httpMethodBody("PATCH", args)
}
func httpDelete(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("http.delete(url, opts?)")
	}
	return httpDo("DELETE", args[0].AsStr(), nil, parseHTTPOpts(args, 1))
}
func httpHead(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("http.head(url, opts?)")
	}
	return httpDo("HEAD", args[0].AsStr(), nil, parseHTTPOpts(args, 1))
}

func httpMethodBody(method string, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("http.%s(url, body?, opts?)", strings.ToLower(method))
	}
	var body []byte
	start := 1
	if len(args) >= 2 && args[1].Typ != vm.TypeMap && args[1].Typ != vm.TypeInt {
		body = valueToBody(args[1])
		start = 2
	}
	return httpDo(method, args[0].AsStr(), body, parseHTTPOpts(args, start))
}

func httpRequest(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("http.request(method, url, body?, opts?)")
	}
	method := strings.ToUpper(args[0].AsStr())
	urlStr := args[1].AsStr()
	var body []byte
	start := 2
	if len(args) >= 3 && args[2].Typ != vm.TypeMap && args[2].Typ != vm.TypeInt {
		body = valueToBody(args[2])
		start = 3
	}
	return httpDo(method, urlStr, body, parseHTTPOpts(args, start))
}

// http.download(url, path, opts?) -> map
func httpDownload(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("http.download(url, path, opts?)")
	}
	urlStr := args[0].AsStr()
	path := args[1].AsStr()
	o := parseHTTPOpts(args, 2)
	if o.maxBody < 1<<30 {
		o.maxBody = 1 << 30 // 1GB for downloads
	}
	o.timeoutSec = max64(o.timeoutSec, 120)

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return httpFail(urlStr, 0, err)
	}
	applyHeaders(req, o)
	resp, err := httpClientFrom(o).Do(req)
	if err != nil {
		return httpFail(urlStr, 0, err)
	}
	defer resp.Body.Close()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		// ignore if dir is .
	}
	f, err := os.Create(path)
	if err != nil {
		return httpFail(urlStr, resp.StatusCode, err)
	}
	defer f.Close()
	n, err := io.Copy(f, io.LimitReader(resp.Body, o.maxBody))
	if err != nil {
		return httpFail(urlStr, resp.StatusCode, err)
	}
	return vm.MapVal(map[string]vm.Value{
		"ok":     vm.Bool(resp.StatusCode >= 200 && resp.StatusCode < 300),
		"status": vm.Int(int64(resp.StatusCode)),
		"path":   vm.Str(path),
		"bytes":  vm.Int(n),
		"url":    vm.Str(urlStr),
		"error":  vm.Str(""),
		"body":   vm.Str(""),
	}), nil
}

func httpEncodeForm(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 || args[0].Typ != vm.TypeMap || args[0].Map == nil {
		return vm.Null(), errf("http.encode_form(map)")
	}
	v := url.Values{}
	for k, val := range *args[0].Map {
		v.Set(k, val.AsStr())
	}
	return vm.Str(v.Encode()), nil
}

func valueToBody(v vm.Value) []byte {
	if v.Typ == vm.TypeNull {
		return nil
	}
	if b, err := v.AsBytes(); err == nil {
		return b
	}
	return []byte(v.AsStr())
}

func applyHeaders(req *http.Request, o httpOpts) {
	if o.userAgent != "" {
		req.Header.Set("User-Agent", o.userAgent)
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "*/*")
	}
	for k, v := range o.headers {
		req.Header.Set(k, v)
	}
}

func httpDo(method, urlStr string, body []byte, o httpOpts) (vm.Value, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, urlStr, rdr)
	if err != nil {
		return httpFail(urlStr, 0, err)
	}
	applyHeaders(req, o)
	resp, err := httpClientFrom(o).Do(req)
	if err != nil {
		return httpFail(urlStr, 0, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, o.maxBody))
	if err != nil {
		return httpFail(urlStr, resp.StatusCode, err)
	}
	hdrs := map[string]vm.Value{}
	for k, vv := range resp.Header {
		if len(vv) == 1 {
			hdrs[k] = vm.Str(vv[0])
		} else if len(vv) > 1 {
			arr := make([]vm.Value, len(vv))
			for i, s := range vv {
				arr[i] = vm.Str(s)
			}
			hdrs[k] = vm.Array(arr)
		}
	}
	return vm.MapVal(map[string]vm.Value{
		"ok":      vm.Bool(resp.StatusCode >= 200 && resp.StatusCode < 300),
		"status":  vm.Int(int64(resp.StatusCode)),
		"body":    vm.Str(string(data)),
		"bytes":   vm.Bytes(data),
		"headers": vm.MapVal(hdrs),
		"url":     vm.Str(resp.Request.URL.String()),
		"error":   vm.Str(""),
	}), nil
}

func httpFail(urlStr string, status int, err error) (vm.Value, error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return vm.MapVal(map[string]vm.Value{
		"ok":      vm.Bool(false),
		"error":   vm.Str(msg),
		"status":  vm.Int(int64(status)),
		"body":    vm.Str(""),
		"bytes":   vm.Bytes(nil),
		"headers": vm.MapVal(map[string]vm.Value{}),
		"url":     vm.Str(urlStr),
	}), nil
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
