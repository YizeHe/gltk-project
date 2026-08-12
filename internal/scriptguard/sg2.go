// Package scriptguard implements ScriptGuard decryptors for AutoHotkey compilations.
package scriptguard

import (
	"encoding/binary"
	"regexp"
	"strconv"
	"strings"
)

const mask32 = uint32(0xFFFFFFFF)

// ExtractUDwords parses ScriptGuard s.="u123u456..." fragments into uint32 slice.
func ExtractUDwords(text string) []uint32 {
	re := regexp.MustCompile(`s\.\s*=\s*"([^"]*)"`)
	parts := re.FindAllStringSubmatch(text, -1)
	var blob strings.Builder
	if len(parts) > 0 {
		for _, p := range parts {
			blob.WriteString(p[1])
		}
	} else {
		blob.WriteString(text)
	}
	s := strings.ReplaceAll(blob.String(), " ", "")
	s = strings.ReplaceAll(s, "\t", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	if strings.HasPrefix(s, "u") || strings.HasPrefix(s, "U") {
		s = s[1:]
	}
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == 'u' || r == 'U'
	})
	out := make([]uint32, 0, len(fields))
	for _, f := range fields {
		if f == "" {
			continue
		}
		n, err := strconv.ParseUint(f, 10, 32)
		if err != nil {
			continue
		}
		out = append(out, uint32(n))
	}
	return out
}

// LooksLikeSG2 returns true if text contains the LCG prelayer / Exec shellcode markers
// used by the newer ScriptGuard (YourCrushLY / GTAMacro family).
func LooksLikeSG2(text string) bool {
	if strings.Contains(text, "0x5A3B521D") || strings.Contains(text, "0x41C64E6D") {
		return true
	}
	if strings.Contains(text, "_fileIV") && strings.Contains(text, "_lcg") {
		return true
	}
	// x64 shellcode hex blob + s.= payload
	if strings.Contains(text, `x64:="`) && strings.Contains(text, `s.="u`) {
		return true
	}
	return false
}

// LCGUnmask applies the AHK-side LCG XOR prelayer.
// First dword is fileIV; remaining dwords are unmasked in place-style new slice.
func LCGUnmask(dwords []uint32) (fileIV uint32, payload []uint32) {
	if len(dwords) == 0 {
		return 0, nil
	}
	fileIV = dwords[0]
	payload = make([]uint32, len(dwords)-1)
	lcg := (0x5A3B521D ^ fileIV) & mask32
	for i, val := range dwords[1:] {
		lcg = (lcg*0x41C64E6D + 0x3039) & mask32
		payload[i] = (val ^ lcg) & mask32
	}
	return fileIV, payload
}

func ror32(x, n uint32) uint32 {
	n &= 31
	return ((x >> n) | (x << (32 - n))) & mask32
}

func rol32(x, n uint32) uint32 {
	n &= 31
	return ((x << n) | (x >> (32 - n))) & mask32
}

// BuildXorshiftTable returns the 256-dword PRNG table used by SG2 shellcode.
func BuildXorshiftTable() [256]uint32 {
	var table [256]uint32
	edx := uint32(0x3F8A2C19)
	for i := 0; i < 256; i++ {
		eax := edx
		eax = ((eax << 13) ^ edx) & mask32
		t := (eax >> 17) & mask32
		eax = (eax ^ t) & mask32
		edx = ((eax << 5) ^ eax) & mask32
		table[i] = edx
	}
	return table
}

func decryptRound(buf []uint32, table *[256]uint32, r15, p58, p60, p68, p70 uint32) {
	n := len(buf)
	if n == 0 {
		return
	}

	ecx := (table[r15&0xFF] ^ 0xA57C8E31) & mask32
	r10 := table[p58&0xFF]

	if n <= 1 {
		if n == 1 {
			buf[0] = (buf[0] - ror32(ecx, 15)) & mask32
			buf[0] = (buf[0] - rol32(r10^0x9D2E4B0F, 5)) & mask32
		}
		return
	}

	// Pass A: buf[i] -= ror(buf[i+1], 15)
	for i := 0; i < n-1; i++ {
		buf[i] = (buf[i] - ror32(buf[i+1], 15)) & mask32
	}

	ecx = ror32(ecx, 15)
	r10 = (r10 ^ 0x9D2E4B0F) & mask32

	// Pass B: chain from end
	eax := (buf[n-1] - ecx) & mask32
	buf[n-1] = eax
	ecxI := n - 2
	for {
		rolv := rol32(buf[ecxI], 5)
		eax = (eax - rolv) & mask32
		buf[ecxI+1] = eax
		if ecxI == 0 {
			break
		}
		eax = buf[ecxI]
		ecxI--
	}

	// Pass C
	eax = buf[0]
	r10 = rol32(r10, 5)
	buf[0] = (eax - r10) & mask32

	// Pass D: table-driven mix
	ebx := 0
	r11 := p68 & mask32
	ebp := p70 & mask32
	edi := uint32(0xDAA66D2B)
	r13 := (p60 + 0x6F) & mask32

	for ebx < n {
		eax = buf[ebx]
		edx := (r11 + 0xAC) & mask32
		r10i := (r13 + uint32(ebx)) & mask32
		r9 := ebp
		r8 := edi
		for {
			cl := edx & 0xFF
			eax = (eax - r8) & mask32
			edx = (edx - 0x2B) & mask32
			r8 = (r8 + 0x61C88647) & mask32
			eax = (eax ^ table[cl]) & mask32
			cl = r9 & 0xFF
			r9 = (r9 - 0x3B) & mask32
			eax = ror32(eax, table[cl]&0xFF)
			cl = r10i & 0xFF
			r10i = (r10i - 0x25) & mask32
			eax = (eax - table[cl]) & mask32
			if edx == r11 {
				break
			}
		}
		buf[ebx] = eax
		ebx++
		ebp = (ebp + 3) & mask32
		r11 = (r11 + 7) & mask32
		edi = (edi - 0x61C88647) & mask32
	}
}

// ShellcodeDecrypt applies the two-round SG2 shellcode transform (post-LCG).
func ShellcodeDecrypt(dwords []uint32) []uint32 {
	buf := make([]uint32, len(dwords))
	copy(buf, dwords)
	table := BuildXorshiftTable()
	// Round 0
	decryptRound(buf, &table, 0x64, 0x3D, 0x7F, 0xBB, 0x145)
	// Round 1
	decryptRound(buf, &table, 0x21, 0x0, 0x0, 0x30, 0xC2)
	return buf
}

// DwordsToBytes packs LE uint32s.
func DwordsToBytes(dwords []uint32) []byte {
	out := make([]byte, len(dwords)*4)
	for i, d := range dwords {
		binary.LittleEndian.PutUint32(out[i*4:], d)
	}
	return out
}

// DecryptSG2Text is the full pipeline: parse u-dwords → LCG → shellcode → bytes.
// Returns plaintext bytes (typically UTF-16-LE with BOM).
func DecryptSG2Text(text string) (plain []byte, fileIV uint32, nDwords int, err error) {
	raw := ExtractUDwords(text)
	if len(raw) < 2 {
		return nil, 0, 0, errTooShort
	}
	iv, mid := LCGUnmask(raw)
	dec := ShellcodeDecrypt(mid)
	return DwordsToBytes(dec), iv, len(raw), nil
}

// DecryptSG2Dwords runs LCG+shellcode on already-parsed dwords (including fileIV as first).
func DecryptSG2Dwords(raw []uint32) (plain []byte, fileIV uint32) {
	if len(raw) < 2 {
		return nil, 0
	}
	iv, mid := LCGUnmask(raw)
	return DwordsToBytes(ShellcodeDecrypt(mid)), iv
}

type sgError string

func (e sgError) Error() string { return string(e) }

const errTooShort = sgError("scriptguard: not enough dwords")
