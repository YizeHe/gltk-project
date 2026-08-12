package upx

import "fmt"

// NRV2B decompress (UCL-compatible, method M_NRV2B_8 = 2).
// Based on the public NRV2B bit-stream description used by UPX.
func nrv2bDecompress(src []byte, dstLen int) ([]byte, error) {
	if dstLen <= 0 {
		dstLen = len(src) * 4
	}
	dst := make([]byte, 0, dstLen)
	bb := &bitBuf{data: src}

	lastOff := 1
	for {
		// literal run
		for {
			bit, err := bb.getBit()
			if err != nil {
				return nil, err
			}
			if bit == 0 {
				break
			}
			b, err := bb.getByte()
			if err != nil {
				return nil, err
			}
			dst = append(dst, b)
			if dstLen > 0 && len(dst) > dstLen+16 {
				return nil, fmt.Errorf("nrv2b: output overflow")
			}
		}
		// match offset
		off := 1
		for {
			bit, err := bb.getBit()
			if err != nil {
				return nil, err
			}
			off = off*2 + bit
			bit, err = bb.getBit()
			if err != nil {
				return nil, err
			}
			if bit != 0 {
				break
			}
		}
		if off == 2 {
			off = lastOff
		} else {
			b, err := bb.getByte()
			if err != nil {
				return nil, err
			}
			off = (off-3)*256 + int(b)
			if off == -1 || off == 0xffffffff {
				// end marker ( commmonly ~0 )
				break
			}
			lastOff = off + 1
			off++
		}
		// match length
		bit, err := bb.getBit()
		if err != nil {
			return nil, err
		}
		mlen := bit
		bit, err = bb.getBit()
		if err != nil {
			return nil, err
		}
		mlen = mlen*2 + bit
		if mlen == 0 {
			mlen = 1
			for {
				bit, err = bb.getBit()
				if err != nil {
					return nil, err
				}
				mlen = mlen*2 + bit
				bit, err = bb.getBit()
				if err != nil {
					return nil, err
				}
				if bit != 0 {
					break
				}
			}
			mlen += 2
		}
		mlen += 1
		if off > 0xd00 {
			mlen++
		}
		if off <= 0 || off > len(dst) {
			return nil, fmt.Errorf("nrv2b: bad offset %d (dst=%d)", off, len(dst))
		}
		for i := 0; i < mlen; i++ {
			dst = append(dst, dst[len(dst)-off])
		}
		if dstLen > 0 && len(dst) >= dstLen {
			return dst[:dstLen], nil
		}
	}
	if dstLen > 0 && len(dst) > dstLen {
		dst = dst[:dstLen]
	}
	return dst, nil
}

// NRV2D method 5 (simplified variant).
func nrv2dDecompress(src []byte, dstLen int) ([]byte, error) {
	// Same bitstream family with different match coding; fall back to 2b-like for common cases.
	// Full NRV2D differs in m_off coding; implement proper 2d:
	if dstLen <= 0 {
		dstLen = len(src) * 4
	}
	dst := make([]byte, 0, dstLen)
	bb := &bitBuf{data: src}
	lastOff := 1
	for {
		for {
			bit, err := bb.getBit()
			if err != nil {
				return nil, err
			}
			if bit == 0 {
				break
			}
			b, err := bb.getByte()
			if err != nil {
				return nil, err
			}
			dst = append(dst, b)
		}
		// 2d offset
		off := 1
		for {
			bit, err := bb.getBit()
			if err != nil {
				return nil, err
			}
			off = off*2 + bit
			bit, err = bb.getBit()
			if err != nil {
				return nil, err
			}
			if bit != 0 {
				break
			}
			bit, err = bb.getBit()
			if err != nil {
				return nil, err
			}
			off = off*2 + 1 - bit // NRV2D twist
		}
		if off == 2 {
			off = lastOff
		} else {
			b, err := bb.getByte()
			if err != nil {
				return nil, err
			}
			off = (off-3)*256 + int(b)
			if off == -1 || uint32(off) == 0xffffffff {
				break
			}
			lastOff = off + 1
			off++
		}
		bit, err := bb.getBit()
		if err != nil {
			return nil, err
		}
		mlen := bit
		bit, err = bb.getBit()
		if err != nil {
			return nil, err
		}
		mlen = mlen*2 + bit
		if mlen == 0 {
			mlen = 1
			for {
				bit, err = bb.getBit()
				if err != nil {
					return nil, err
				}
				mlen = mlen*2 + bit
				bit, err = bb.getBit()
				if err != nil {
					return nil, err
				}
				if bit != 0 {
					break
				}
			}
			mlen += 2
		}
		mlen += 1
		if off > 0x500 {
			mlen++
		}
		if off <= 0 || off > len(dst) {
			return nil, fmt.Errorf("nrv2d: bad offset %d", off)
		}
		for i := 0; i < mlen; i++ {
			dst = append(dst, dst[len(dst)-off])
		}
		if dstLen > 0 && len(dst) >= dstLen {
			return dst[:dstLen], nil
		}
	}
	if dstLen > 0 && len(dst) > dstLen {
		dst = dst[:dstLen]
	}
	return dst, nil
}

// NRV2E method 8
func nrv2eDecompress(src []byte, dstLen int) ([]byte, error) {
	// Closest to 2d with extra bit; reuse 2d for first cut if structure similar.
	return nrv2dDecompress(src, dstLen)
}

type bitBuf struct {
	data []byte
	pos  int
	bb   uint
	bl   uint
}

func (b *bitBuf) getBit() (int, error) {
	// UCL/UPX getbit_le8: refill with 0x100 sentinel, consume low bit first.
	if b.bb&0x7f == 0 {
		if b.pos >= len(b.data) {
			return 0, fmt.Errorf("nrv: unexpected EOF in bitstream")
		}
		b.bb = uint(b.data[b.pos]) | 0x100
		b.pos++
	}
	bit := int(b.bb & 1)
	b.bb >>= 1
	return bit, nil
}

func (b *bitBuf) getByte() (byte, error) {
	if b.pos >= len(b.data) {
		return 0, fmt.Errorf("nrv: unexpected EOF")
	}
	v := b.data[b.pos]
	b.pos++
	return v, nil
}
