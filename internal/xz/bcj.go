// Branch/Call/Jump (BCJ) filters: x86, PowerPC, IA-64, ARM, ARM-Thumb,
// SPARC and ARM64.
//
// Ported from the public-domain XZ Embedded decoder by Lasse Collin as
// vendored by toybox in toys/pending/xzcat.c (0BSD), commit
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb. The filter set is exactly the
// one that file supports; any other filter ID is rejected as unsupported.

package xz

// BCJ filter IDs (the .xz Filter IDs).
const (
	filterX86      = 4
	filterPowerPC  = 5
	filterIA64     = 6
	filterARM      = 7
	filterARMThumb = 8
	filterSPARC    = 9
	filterARM64    = 10
)

// bcjTempSize is big enough for Alignment + 2 * Look-ahead of every filter.
const bcjTempSize = 16

type bcjDec struct {
	typ         byte
	ret         xzRet
	pos         uint32
	x86PrevMask uint32

	out     []byte
	outPos  int
	outSize int

	temp struct {
		filtered int
		size     int
		buf      [bcjTempSize]byte
	}
}

func (s *bcjDec) reset(id byte) xzRet {
	if id < filterX86 || id > filterARM64 {
		return xzOptionsError
	}
	s.typ = id
	s.ret = xzOK
	s.pos = 0
	s.x86PrevMask = 0
	s.temp.filtered = 0
	s.temp.size = 0
	return xzOK
}

func bcjTestMSByte(b byte) bool { return b == 0 || b == 0xFF }

func getLE32(buf []byte) uint32 {
	return uint32(buf[0]) | uint32(buf[1])<<8 | uint32(buf[2])<<16 | uint32(buf[3])<<24
}

func getBE32(buf []byte) uint32 {
	return uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
}

func putLE32(v uint32, buf []byte) {
	buf[0] = byte(v)
	buf[1] = byte(v >> 8)
	buf[2] = byte(v >> 16)
	buf[3] = byte(v >> 24)
}

func putBE32(v uint32, buf []byte) {
	buf[0] = byte(v >> 24)
	buf[1] = byte(v >> 16)
	buf[2] = byte(v >> 8)
	buf[3] = byte(v)
}

func (s *bcjDec) x86(buf []byte) int {
	maskToAllowedStatus := [8]bool{true, true, true, false, true, false, false, false}
	maskToBitNum := [8]int{0, 1, 2, 2, 3, 3, 3, 3}

	prevPos := -1
	prevMask := s.x86PrevMask
	var i int

	if len(buf) <= 4 {
		return 0
	}

	size := len(buf) - 4
	for i = 0; i < size; i++ {
		if buf[i]&0xFE != 0xE8 {
			continue
		}

		prevPos = i - prevPos
		if prevPos > 3 {
			prevMask = 0
		} else {
			shift := uint(prevPos - 1)
			if shift > 31 {
				shift = 31
			}
			prevMask = (prevMask << shift) & 7
			if prevMask != 0 {
				b := buf[i+4-maskToBitNum[prevMask]]
				if !maskToAllowedStatus[prevMask] || bcjTestMSByte(b) {
					prevPos = i
					prevMask = prevMask<<1 | 1
					continue
				}
			}
		}

		prevPos = i

		if bcjTestMSByte(buf[i+4]) {
			src := getLE32(buf[i+1:])
			var dest uint32
			for {
				dest = src - (s.pos + uint32(i) + 5)
				if prevMask == 0 {
					break
				}
				j := uint(maskToBitNum[prevMask] * 8)
				b := byte(dest >> (24 - j))
				if !bcjTestMSByte(b) {
					break
				}
				src = dest ^ (uint32(1)<<(32-j) - 1)
			}

			dest &= 0x01FFFFFF
			dest |= uint32(0) - (dest & 0x01000000)
			putLE32(dest, buf[i+1:])
			i += 4
		} else {
			prevMask = prevMask<<1 | 1
		}
	}

	prevPos = i - prevPos
	if prevPos > 3 {
		s.x86PrevMask = 0
	} else {
		s.x86PrevMask = prevMask << uint(prevPos-1)
	}
	return i
}

func (s *bcjDec) powerPC(buf []byte) int {
	var i int
	for ; i+4 <= len(buf); i += 4 {
		instr := getBE32(buf[i:])
		if instr&0xFC000003 == 0x48000001 {
			instr &= 0x03FFFFFC
			instr -= s.pos + uint32(i)
			instr &= 0x03FFFFFC
			instr |= 0x48000001
			putBE32(instr, buf[i:])
		}
	}
	return i
}

func (s *bcjDec) ia64(buf []byte) int {
	branchTable := [32]byte{
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		4, 4, 6, 6, 0, 0, 7, 7,
		4, 4, 0, 0, 4, 4, 0, 0,
	}

	var i int
	for ; i+16 <= len(buf); i += 16 {
		mask := branchTable[buf[i]&0x1F]
		slot := 0
		bitPos := uint(5)
		for ; slot < 3; slot, bitPos = slot+1, bitPos+41 {
			if (mask>>uint(slot))&1 == 0 {
				continue
			}

			bytePos := bitPos >> 3
			bitRes := bitPos & 7

			var instr uint64
			for j := 0; j < 6; j++ {
				instr |= uint64(buf[i+j+int(bytePos)]) << (8 * uint(j))
			}

			norm := instr >> bitRes

			if (norm>>37)&0x0F == 5 && (norm>>9)&7 == 0 {
				addr := uint32((norm >> 13) & 0x0FFFFF)
				addr |= uint32((norm>>36)&1) << 20
				addr <<= 4
				addr -= s.pos + uint32(i)
				addr >>= 4

				norm &^= uint64(0x8FFFFF) << 13
				norm |= uint64(addr&0x0FFFFF) << 13
				norm |= uint64(addr&0x100000) << (36 - 20)

				instr &= uint64(1)<<bitRes - 1
				instr |= norm << bitRes

				for j := 0; j < 6; j++ {
					buf[i+j+int(bytePos)] = byte(instr >> (8 * uint(j)))
				}
			}
		}
	}
	return i
}

func (s *bcjDec) arm(buf []byte) int {
	var i int
	for ; i+4 <= len(buf); i += 4 {
		if buf[i+3] == 0xEB {
			addr := uint32(buf[i]) | uint32(buf[i+1])<<8 | uint32(buf[i+2])<<16
			addr <<= 2
			addr -= s.pos + uint32(i) + 8
			addr >>= 2
			buf[i] = byte(addr)
			buf[i+1] = byte(addr >> 8)
			buf[i+2] = byte(addr >> 16)
		}
	}
	return i
}

func (s *bcjDec) armThumb(buf []byte) int {
	var i int
	for ; i+4 <= len(buf); i += 2 {
		if buf[i+1]&0xF8 == 0xF0 && buf[i+3]&0xF8 == 0xF8 {
			addr := (uint32(buf[i+1])&7)<<19 | uint32(buf[i])<<11 |
				(uint32(buf[i+3])&7)<<8 | uint32(buf[i+2])
			addr <<= 1
			addr -= s.pos + uint32(i) + 4
			addr >>= 1
			buf[i+1] = byte(0xF0 | ((addr >> 19) & 7))
			buf[i] = byte(addr >> 11)
			buf[i+3] = byte(0xF8 | ((addr >> 8) & 7))
			buf[i+2] = byte(addr)
			i += 2
		}
	}
	return i
}

func (s *bcjDec) sparc(buf []byte) int {
	var i int
	for ; i+4 <= len(buf); i += 4 {
		instr := getBE32(buf[i:])
		if instr>>22 == 0x100 || instr>>22 == 0x1FF {
			instr <<= 2
			instr -= s.pos + uint32(i)
			instr >>= 2
			instr = (0x40000000 - (instr & 0x400000)) | 0x40000000 | (instr & 0x3FFFFF)
			putBE32(instr, buf[i:])
		}
	}
	return i
}

func (s *bcjDec) arm64Filter(buf []byte) int {
	var i int
	for ; i+4 <= len(buf); i += 4 {
		instr := getLE32(buf[i:])

		if instr>>26 == 0x25 {
			// BL instruction
			addr := instr - ((s.pos + uint32(i)) >> 2)
			instr = 0x94000000 | (addr & 0x03FFFFFF)
			putLE32(instr, buf[i:])
		} else if instr&0x9F000000 == 0x90000000 {
			// ADRP instruction
			addr := ((instr >> 29) & 3) | ((instr >> 3) & 0x1FFFFC)

			// Only convert values in the range +/-512 MiB.
			if (addr+0x020000)&0x1C0000 != 0 {
				continue
			}

			addr -= (s.pos + uint32(i)) >> 12

			instr &= 0x9000001F
			instr |= (addr & 3) << 29
			instr |= (addr & 0x03FFFC) << 3
			instr |= (0 - (addr & 0x020000)) & 0xE00000

			putLE32(instr, buf[i:])
		}
	}
	return i
}

// apply runs the selected filter over buf[pos:end] and updates pos and s.pos
// to match the amount of data that got filtered.
func (s *bcjDec) apply(buf []byte, pos *int, end int) {
	b := buf[*pos:end]
	var filtered int
	switch s.typ {
	case filterX86:
		filtered = s.x86(b)
	case filterPowerPC:
		filtered = s.powerPC(b)
	case filterIA64:
		filtered = s.ia64(b)
	case filterARM:
		filtered = s.arm(b)
	case filterARMThumb:
		filtered = s.armThumb(b)
	case filterSPARC:
		filtered = s.sparc(b)
	case filterARM64:
		filtered = s.arm64Filter(b)
	}
	*pos += filtered
	s.pos += uint32(filtered)
}

// flush moves pending filtered data from temp to the output buffer and keeps
// the remaining mixture at the front of temp.
func (s *bcjDec) flush(b *buf) {
	copySize := s.temp.filtered
	if n := b.outSize - b.outPos; n < copySize {
		copySize = n
	}
	copy(b.out[b.outPos:], s.temp.buf[:copySize])
	b.outPos += copySize

	s.temp.filtered -= copySize
	s.temp.size -= copySize
	copy(s.temp.buf[:], s.temp.buf[copySize:copySize+s.temp.size])
}

// run decodes a BCJ + LZMA2 chain.
func (s *bcjDec) run(lzma2 *lzma2Dec, b *buf) xzRet {
	// Flush pending already filtered data first.
	if s.temp.filtered > 0 {
		s.flush(b)
		if s.temp.filtered > 0 {
			return xzOK
		}
		if s.ret == xzStreamEnd {
			return xzStreamEnd
		}
	}

	if s.temp.size < b.outSize-b.outPos || s.temp.size == 0 {
		outStart := b.outPos
		copy(b.out[b.outPos:], s.temp.buf[:s.temp.size])
		b.outPos += s.temp.size

		s.ret = lzma2.run(b)
		if s.ret != xzStreamEnd && s.ret != xzOK {
			return s.ret
		}

		s.apply(b.out, &outStart, b.outPos)

		// The last few bytes are meant to stay unfiltered.
		if s.ret == xzStreamEnd {
			return xzStreamEnd
		}

		s.temp.size = b.outPos - outStart
		b.outPos -= s.temp.size
		copy(s.temp.buf[:], b.out[b.outPos:b.outPos+s.temp.size])

		if b.outPos+s.temp.size < b.outSize {
			return xzOK
		}
	}

	if b.outPos < b.outSize {
		// Point b at temp for the next filter call.
		s.out = b.out
		s.outPos = b.outPos
		s.outSize = b.outSize
		b.out = s.temp.buf[:]
		b.outPos = s.temp.size
		b.outSize = len(s.temp.buf)

		s.ret = lzma2.run(b)

		s.temp.size = b.outPos
		b.out = s.out
		b.outPos = s.outPos
		b.outSize = s.outSize

		if s.ret != xzOK && s.ret != xzStreamEnd {
			return s.ret
		}

		s.apply(s.temp.buf[:], &s.temp.filtered, s.temp.size)

		if s.ret == xzStreamEnd {
			s.temp.filtered = s.temp.size
		}

		s.flush(b)
		if s.temp.filtered > 0 {
			return xzOK
		}
	}

	return s.ret
}
