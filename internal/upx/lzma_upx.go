package upx

import (
	"bytes"
	"fmt"
	"io"

	"github.com/ulikunitz/xz/lzma"
)

// upxLZMADecompress implements upx_lzma_decompress (UPX-style 2-byte props + raw LZMA).
// See upx/src/compress/compress_lzma.cpp.
func upxLZMADecompress(src []byte, dstLen int) ([]byte, error) {
	if len(src) < 3 {
		return nil, fmt.Errorf("upx lzma: short input")
	}
	pb := int(src[0] & 7)
	lp := int(src[1] >> 4)
	lc := int(src[1] & 15)
	if pb >= 5 || lp >= 5 || lc >= 9 {
		return nil, fmt.Errorf("upx lzma: bad props pb=%d lp=%d lc=%d", pb, lp, lc)
	}
	// UPX extra check: high 5 bits of byte0 must equal lc+lp
	if int(src[0]>>3) != lc+lp {
		return nil, fmt.Errorf("upx lzma: props checksum mismatch")
	}
	payload := src[2:]
	// classic props byte for .lzma header
	props := byte((pb*5+lp)*9 + lc)

	dicts := []uint32{8 << 20, 4 << 20, 2 << 20, 1 << 23, 1 << 22, 1 << 21, 16 << 20}
	// dict often equals or related to u_len; try dstLen-ish
	if dstLen > 4096 {
		d := uint32(dstLen)
		// round up to power of two-ish caps
		dicts = append([]uint32{d, nextPow2(d)}, dicts...)
	}

	var lastErr error
	for _, dict := range dicts {
		out, err := decodeClassicLZMA(props, dict, payload, dstLen)
		if err != nil {
			lastErr = err
			continue
		}
		if dstLen > 0 {
			if len(out) >= dstLen {
				return out[:dstLen], nil
			}
			if len(out) >= dstLen*9/10 {
				return out, nil
			}
			lastErr = fmt.Errorf("short out %d want %d", len(out), dstLen)
			continue
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("upx lzma: %w", lastErr)
	}
	return nil, fmt.Errorf("upx lzma: failed")
}

func nextPow2(n uint32) uint32 {
	if n <= 4096 {
		return 4096
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n++
	if n > 1<<26 {
		return 1 << 26
	}
	return n
}

func decodeClassicLZMA(props byte, dict uint32, payload []byte, dstLen int) ([]byte, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("empty payload")
	}
	hdr := make([]byte, 13)
	hdr[0] = props
	hdr[1] = byte(dict)
	hdr[2] = byte(dict >> 8)
	hdr[3] = byte(dict >> 16)
	hdr[4] = byte(dict >> 24)
	var usize uint64 = 1<<64 - 1
	if dstLen > 0 {
		usize = uint64(dstLen)
	}
	for i := 0; i < 8; i++ {
		hdr[5+i] = byte(usize >> (8 * uint(i)))
	}
	stream := append(hdr, payload...)
	r, err := lzma.NewReader(bytes.NewReader(stream))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if dstLen > 0 {
		buf.Grow(dstLen)
	}
	tmp := make([]byte, 64*1024)
	for {
		n, er := r.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
			if dstLen > 0 && buf.Len() >= dstLen {
				return buf.Bytes()[:dstLen], nil
			}
		}
		if er != nil {
			if er == io.EOF || er == io.ErrUnexpectedEOF {
				break
			}
			if buf.Len() > 0 {
				break
			}
			return nil, er
		}
	}
	return buf.Bytes(), nil
}
