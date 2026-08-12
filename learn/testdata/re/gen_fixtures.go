package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"unicode/utf16"
)

func main() {
	dir := "learn/testdata/re"
	_ = os.MkdirAll(dir, 0o755)

	// 01: ASCII + UTF16LE strings blob
	ascii := []byte("Pad\x00AutoHotkey\x00ScriptGuard\x00#NoEnv\x00SendInput\x00FileInstall\x00Ahk2Exe\x00")
	ascii = append(ascii, []byte("https://evil.example/path?x=1\x00")...)
	ascii = append(ascii, []byte("password=Secret123\x00")...)
	u16 := utf16.Encode([]rune("UTF16_MARKER_GrokLang"))
	var u16b []byte
	for _, c := range u16 {
		var tmp [2]byte
		binary.LittleEndian.PutUint16(tmp[:], c)
		u16b = append(u16b, tmp[:]...)
	}
	blob1 := append(ascii, 0, 0)
	blob1 = append(blob1, u16b...)
	write(dir, "01_strings_ahk_markers.bin", blob1)

	// 02: high vs low entropy
	low := []byte("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	high := make([]byte, 64)
	for i := range high {
		high[i] = byte(i*37 + 11)
	}
	write(dir, "02_entropy_mixed.bin", append(low, high...))

	// 03: fake config text
	write(dir, "03_config.txt", []byte("# sample\nhost=127.0.0.1\nport=8080\nkey=demo-key-001\nemail=alice@test.com\n"))

	// 04: minimal MZ-only stub (not full PE)
	mz := make([]byte, 128)
	mz[0], mz[1] = 'M', 'Z'
	binary.LittleEndian.PutUint16(mz[0x3C:], 0x40) // e_lfanew
	// PE signature at 0x40 without full optional header - for manual bin parse demos
	copy(mz[0x40:], []byte("PE\x00\x00"))
	binary.LittleEndian.PutUint16(mz[0x44:], 0x8664) // Machine
	write(dir, "04_mz_pe_sig_stub.bin", mz)

	// 05: repeated patterns for find_bytes
	pat := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	body := make([]byte, 256)
	for i := range body {
		body[i] = byte(i)
	}
	copy(body[16:], pat)
	copy(body[100:], pat)
	copy(body[200:], pat)
	write(dir, "05_pattern_deadbeef.bin", body)

	// 06: scriptguard-like u-dwords line (synthetic text)
	write(dir, "06_scriptguard_u_style.txt", []byte(`s.="u123u456u789u101112"`+"\n#NoEnv\n"))

	// 07: JSON sample
	write(dir, "07_sample_report.json", []byte(`{"ok":true,"file":"demo.exe","hits":[{"off":16,"tag":"DEADBEEF"},{"off":100,"tag":"DEADBEEF"}],"meta":{"tool":"gltk","chapter":7}}`))

	// 08: email/url haystack
	write(dir, "08_iocs.txt", []byte("contact alice@test.com or bob@ssgreen.org\nC2: http://malicious.example/gate\nip=203.0.113.9\nuuid=5f5749e77a9b4c2a\n"))

	// 09: xor-obfuscated ascii (key 0x55)
	plain := []byte("HiddenPayloadXOR")
	xored := make([]byte, len(plain))
	for i, b := range plain {
		xored[i] = b ^ 0x55
	}
	write(dir, "09_xor55_payload.bin", xored)

	// 10: multi-section fake layout description
	write(dir, "10_readme_fixtures.txt", []byte(`GrokLang learn/testdata/re fixtures
01 strings/ahk markers
02 entropy mixed
03 config kv
04 mz+pe signature stub (not valid PE for full pe.parse)
05 DEADBEEF patterns
06 scriptguard u-style text
07 json report
08 iocs
09 xor 0x55
Use with ch07_*.glk examples.
`))

	// Generate a few more small variants
	for i := 0; i < 5; i++ {
		b := make([]byte, 32)
		for j := range b {
			b[j] = byte(i*3 + j*5)
		}
		write(dir, filepath.Join("batch", "sample_"+itoa(i)+".bin"), b)
	}
}

func write(dir, name string, b []byte) {
	p := filepath.Join(dir, name)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		panic(err)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}
