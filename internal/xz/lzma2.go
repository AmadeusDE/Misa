// LZMA2 decoder: dictionary, range decoder, LZMA symbol decoder and the
// LZMA2 control layer.
//
// Ported from the public-domain XZ Embedded decoder by Lasse Collin as
// vendored by toybox in toys/pending/xzcat.c (0BSD), commit
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb. See xz.go for the package
// documentation and the list of deliberate differences.

package xz

// xzRet mirrors the xz_ret enum of the reference decoder.
type xzRet int

const (
	xzOK xzRet = iota
	xzStreamEnd
	xzNotXZ
	xzOptionsError
	xzMemoryError
	xzError
)

// buf is a window on the caller's input and output buffers, mirroring
// struct xz_buf. The decoder advances inPos and outPos.
type buf struct {
	in      []byte
	inPos   int
	inSize  int
	out     []byte
	outPos  int
	outSize int
}

// LZMA constants (xz_lzma2.h).
const (
	states      = 12
	litStates   = 7
	matchLenMin = 2

	posStatesMax = 1 << 4

	distModelStart = 4
	distModelEnd   = 14
	distSlots      = 1 << 6
	fullDistances  = 1 << (distModelEnd / 2)

	alignBits = 4
	alignSize = 1 << alignBits

	reps = 4

	lenLowSymbols  = 1 << 3
	lenMidSymbols  = 1 << 3
	lenHighSymbols = 1 << 8

	literalCoderSize = 0x300
	literalCodersMax = 1 << 4

	lzmaInRequired = 21
	rcInitBytes    = 5

	rcBitModelTotalBits = 11
	rcBitModelTotal     = 1 << rcBitModelTotalBits
	rcMoveBits          = 5
)

// lzmaState tracks which LZMA symbols have occurred most recently, which
// predicts the next symbol (enum lzma_state).
type lzmaState int

const (
	stateLitLit lzmaState = iota
	stateMatchLitLit
	stateRepLitLit
	stateShortrepLitLit
	stateMatchLit
	stateRepLit
	stateShortrepLit
	stateLitMatch
	stateLitLongrep
	stateLitShortrep
	stateNonlitMatch
	stateNonlitRep
)

func (st *lzmaState) literal() {
	switch {
	case *st <= stateShortrepLitLit:
		*st = stateLitLit
	case *st <= stateLitShortrep:
		*st -= 3
	default:
		*st -= 6
	}
}

func (st *lzmaState) match() {
	if *st < litStates {
		*st = stateLitMatch
	} else {
		*st = stateNonlitMatch
	}
}

func (st *lzmaState) longRep() {
	if *st < litStates {
		*st = stateLitLongrep
	} else {
		*st = stateNonlitRep
	}
}

func (st *lzmaState) shortRep() {
	if *st < litStates {
		*st = stateLitShortrep
	} else {
		*st = stateNonlitRep
	}
}

/**************
 * Dictionary *
 **************/

// dictionary is the history buffer. Every access is bounds checked: a
// malformed stream must never index outside the buffer. A distance beyond
// the data seen so far yields zero, which only affects the decoded bytes and
// is caught by the integrity check at the end of the block.
type dictionary struct {
	buf              []byte
	start, pos       int
	full, limit, end int
	size             int
}

func (d *dictionary) reset() {
	d.start = 0
	d.pos = 0
	d.limit = 0
	d.full = 0
}

// setLimit caps this call's output at outMax bytes.
func (d *dictionary) setLimit(outMax int) {
	if d.end-d.pos <= outMax {
		d.limit = d.end
	} else {
		d.limit = d.pos + outMax
	}
}

func (d *dictionary) hasSpace() bool { return d.pos < d.limit }

func (d *dictionary) get(dist uint32) byte {
	offset := d.pos - int(dist) - 1
	if int(dist) >= d.pos {
		offset += d.end
	}
	if d.full <= 0 || offset < 0 || offset >= len(d.buf) {
		return 0
	}
	return d.buf[offset]
}

func (d *dictionary) put(b byte) {
	d.buf[d.pos] = b
	d.pos++
	if d.full < d.pos {
		d.full = d.pos
	}
}

// repeat copies n bytes from the given distance and updates n to the number
// of bytes still pending. It returns false if the distance is invalid.
func (d *dictionary) repeat(n *int, dist uint32) bool {
	if int(dist) >= d.full || int(dist) >= d.size {
		return false
	}

	left := d.limit - d.pos
	if *n < left {
		left = *n
	}
	*n -= left

	back := d.pos - int(dist) - 1
	if int(dist) >= d.pos {
		back += d.end
	}

	for left > 0 {
		d.buf[d.pos] = d.buf[back]
		d.pos++
		back++
		if back == d.end {
			back = 0
		}
		left--
	}

	if d.full < d.pos {
		d.full = d.pos
	}
	return true
}

// uncompressed copies stored (uncompressed) bytes to the dictionary and the
// output buffer at once.
func (d *dictionary) uncompressed(b *buf, left *int) {
	for *left > 0 && b.inPos < b.inSize && b.outPos < b.outSize {
		copySize := b.inSize - b.inPos
		if n := b.outSize - b.outPos; n < copySize {
			copySize = n
		}
		if n := d.end - d.pos; n < copySize {
			copySize = n
		}
		if copySize > *left {
			copySize = *left
		}
		*left -= copySize

		copy(d.buf[d.pos:], b.in[b.inPos:b.inPos+copySize])
		d.pos += copySize
		if d.full < d.pos {
			d.full = d.pos
		}
		if d.pos == d.end {
			d.pos = 0
		}

		copy(b.out[b.outPos:], b.in[b.inPos:b.inPos+copySize])
		d.start = d.pos

		b.outPos += copySize
		b.inPos += copySize
	}
}

// flush moves the decoded bytes from the dictionary to b.out.
func (d *dictionary) flush(b *buf) int {
	copySize := d.pos - d.start
	if d.pos == d.end {
		d.pos = 0
	}
	copy(b.out[b.outPos:], d.buf[d.start:d.start+copySize])
	d.start = d.pos
	b.outPos += copySize
	return copySize
}

/*****************
 * Range decoder *
 *****************/

type rcDec struct {
	rng, code     uint32
	initBytesLeft int

	in             []byte
	inPos, inLimit int
}

func (rc *rcDec) reset() {
	rc.rng = 0xFFFFFFFF
	rc.code = 0
	rc.initBytesLeft = rcInitBytes
}

// readInit consumes the five range coder initialisation bytes.
func (rc *rcDec) readInit(b *buf) bool {
	for rc.initBytesLeft > 0 {
		if b.inPos == b.inSize {
			return false
		}
		rc.code = rc.code<<8 + uint32(b.in[b.inPos])
		b.inPos++
		rc.initBytesLeft--
	}
	return true
}

func (rc *rcDec) limitExceeded() bool { return rc.inPos > rc.inLimit }

// inByte reads one byte of range coder input. The reference decoder relies
// on the caller's padding; a malformed stream is padded with zero here so a
// corrupt file can never read out of bounds.
func (rc *rcDec) inByte() byte {
	if rc.inPos < len(rc.in) {
		b := rc.in[rc.inPos]
		rc.inPos++
		return b
	}
	rc.inPos++
	return 0
}

func (rc *rcDec) normalize() {
	if rc.rng < 1<<24 {
		rc.rng <<= 8
		rc.code = rc.code<<8 + uint32(rc.inByte())
	}
}

func (rc *rcDec) bit(prob *uint16) int {
	rc.normalize()
	bound := (rc.rng >> rcBitModelTotalBits) * uint32(*prob)
	if rc.code < bound {
		rc.rng = bound
		*prob += uint16((rcBitModelTotal - uint32(*prob)) >> rcMoveBits)
		return 0
	}
	rc.rng -= bound
	rc.code -= bound
	*prob -= *prob >> rcMoveBits
	return 1
}

// bittree decodes a bit tree starting from the most significant bit.
func (rc *rcDec) bittree(probs []uint16, limit uint32) uint32 {
	symbol := uint32(1)
	for {
		if rc.bit(&probs[symbol]) != 0 {
			symbol = symbol<<1 + 1
		} else {
			symbol <<= 1
		}
		if symbol >= limit {
			return symbol
		}
	}
}

// bittreeReverse decodes a bit tree starting from the least significant bit.
func (rc *rcDec) bittreeReverse(probs []uint16, dest *uint32, limit uint32) {
	symbol := uint32(1)
	var i uint32
	for {
		if rc.bit(&probs[symbol]) != 0 {
			symbol = symbol<<1 + 1
			*dest += 1 << i
		} else {
			symbol <<= 1
		}
		i++
		if i >= limit {
			return
		}
	}
}

// direct decodes bits with fixed fifty-fifty probability.
func (rc *rcDec) direct(dest *uint32, limit uint32) {
	for {
		rc.normalize()
		rc.rng >>= 1
		rc.code -= rc.rng
		mask := uint32(0) - (rc.code >> 31)
		rc.code += rc.rng & mask
		*dest = *dest<<1 + (mask + 1)
		limit--
		if limit == 0 {
			return
		}
	}
}

/********
 * LZMA *
 ********/

type lzmaLenDec struct {
	choice, choice2 uint16
	low             [posStatesMax][lenLowSymbols]uint16
	mid             [posStatesMax][lenMidSymbols]uint16
	high            [lenHighSymbols]uint16
}

type lzmaDec struct {
	rep0, rep1, rep2, rep3 uint32
	state                  lzmaState
	len                    int

	lc, literalPosMask, posMask uint32

	isMatch    [states][posStatesMax]uint16
	isRep      [states]uint16
	isRep0     [states]uint16
	isRep1     [states]uint16
	isRep2     [states]uint16
	isRep0Long [states][posStatesMax]uint16

	distSlot [4][distSlots]uint16

	// distSpecial holds the probability trees for the additional bits of
	// the distances 4-127. The reference code points one entry before the
	// array for the shortest tree, so the table carries a leading entry
	// and is indexed by the accumulated distance (not by symbol 1).
	distSpecial [fullDistances - distModelEnd + 1]uint16
	distAlign   [alignSize]uint16

	matchLenDec lzmaLenDec
	repLenDec   lzmaLenDec

	literal [literalCodersMax][literalCoderSize]uint16
}

// resetProbs sets every probability to one half. The reference decoder does
// this with a single loop over the whole probability block; the arrays here
// are separate fields, so each is cleared in turn.
func (l *lzmaDec) resetProbs() {
	const p = rcBitModelTotal / 2
	zero := func(s []uint16) {
		for i := range s {
			s[i] = p
		}
	}
	for i := range l.isMatch {
		zero(l.isMatch[i][:])
	}
	zero(l.isRep[:])
	zero(l.isRep0[:])
	zero(l.isRep1[:])
	zero(l.isRep2[:])
	for i := range l.isRep0Long {
		zero(l.isRep0Long[i][:])
	}
	for i := range l.distSlot {
		zero(l.distSlot[i][:])
	}
	zero(l.distSpecial[:])
	zero(l.distAlign[:])
	for _, d := range []*lzmaLenDec{&l.matchLenDec, &l.repLenDec} {
		d.choice = p
		d.choice2 = p
		for i := range d.low {
			zero(d.low[i][:])
		}
		for i := range d.mid {
			zero(d.mid[i][:])
		}
		zero(d.high[:])
	}
	for i := range l.literal {
		zero(l.literal[i][:])
	}
}

// literalProbs returns the probability array for the next literal.
func (s *lzma2Dec) literalProbs() []uint16 {
	prevByte := s.dict.get(0)
	low := uint32(prevByte) >> (8 - s.lzma.lc)
	high := (uint32(s.dict.pos) & s.lzma.literalPosMask) << s.lzma.lc
	return s.lzma.literal[low+high][:]
}

func (s *lzma2Dec) literal() {
	probs := s.literalProbs()
	l := &s.lzma

	var symbol uint32
	if l.state < litStates {
		symbol = s.rc.bittree(probs, 0x100)
	} else {
		symbol = 1
		matchByte := uint32(s.dict.get(l.rep0)) << 1
		offset := uint32(0x100)

		for {
			matchBit := matchByte & offset
			matchByte <<= 1
			i := offset + matchBit + symbol

			if s.rc.bit(&probs[i]) != 0 {
				symbol = symbol<<1 + 1
				offset &= matchBit
			} else {
				symbol <<= 1
				offset &^= matchBit
			}
			if symbol >= 0x100 {
				break
			}
		}
	}

	s.dict.put(byte(symbol))
	l.state.literal()
}

func (s *lzma2Dec) length(d *lzmaLenDec, posState uint32) {
	var probs []uint16
	var limit uint32

	if s.rc.bit(&d.choice) == 0 {
		probs = d.low[posState][:]
		limit = lenLowSymbols
		s.lzma.len = matchLenMin
	} else if s.rc.bit(&d.choice2) == 0 {
		probs = d.mid[posState][:]
		limit = lenMidSymbols
		s.lzma.len = matchLenMin + lenLowSymbols
	} else {
		probs = d.high[:]
		limit = lenHighSymbols
		s.lzma.len = matchLenMin + lenLowSymbols + lenMidSymbols
	}

	s.lzma.len += int(s.rc.bittree(probs, limit) - limit)
}

func distState(length int) uint32 {
	if length < distModelStart+matchLenMin {
		return uint32(length - matchLenMin)
	}
	return distModelStart - 1
}

func (s *lzma2Dec) match(posState uint32) {
	l := &s.lzma
	l.state.match()

	l.rep3 = l.rep2
	l.rep2 = l.rep1
	l.rep1 = l.rep0

	s.length(&l.matchLenDec, posState)

	distSlot := s.rc.bittree(l.distSlot[distState(l.len)][:], distSlots) - distSlots

	if distSlot < distModelStart {
		l.rep0 = distSlot
		return
	}

	limit := (distSlot >> 1) - 1
	l.rep0 = 2 + (distSlot & 1)

	if distSlot < distModelEnd {
		l.rep0 <<= limit
		probs := l.distSpecial[int(l.rep0)-int(distSlot):]
		s.rc.bittreeReverse(probs, &l.rep0, limit)
	} else {
		s.rc.direct(&l.rep0, limit-alignBits)
		l.rep0 <<= alignBits
		s.rc.bittreeReverse(l.distAlign[:], &l.rep0, alignBits)
	}
}

func (s *lzma2Dec) repMatch(posState uint32) {
	l := &s.lzma

	if s.rc.bit(&l.isRep0[l.state]) == 0 {
		if s.rc.bit(&l.isRep0Long[l.state][posState]) == 0 {
			l.state.shortRep()
			l.len = 1
			return
		}
	} else {
		var tmp uint32
		if s.rc.bit(&l.isRep1[l.state]) == 0 {
			tmp = l.rep1
		} else {
			if s.rc.bit(&l.isRep2[l.state]) == 0 {
				tmp = l.rep2
			} else {
				tmp = l.rep3
				l.rep3 = l.rep2
			}
			l.rep2 = l.rep1
		}
		l.rep1 = l.rep0
		l.rep0 = tmp
	}

	l.state.longRep()
	s.length(&l.repLenDec, posState)
}

// main decodes LZMA symbols into the dictionary until it is full or the
// input limit is reached.
func (s *lzma2Dec) main() bool {
	l := &s.lzma

	// Finish a repeat left pending by the previous call.
	if s.dict.hasSpace() && l.len > 0 {
		s.dict.repeat(&l.len, l.rep0)
	}

	for s.dict.hasSpace() && !s.rc.limitExceeded() {
		posState := uint32(s.dict.pos) & l.posMask

		if s.rc.bit(&l.isMatch[l.state][posState]) == 0 {
			s.literal()
			continue
		}

		if s.rc.bit(&l.isRep[l.state]) != 0 {
			s.repMatch(posState)
		} else {
			s.match(posState)
		}

		if !s.dict.repeat(&l.len, l.rep0) {
			return false
		}
	}

	// Leaving the range decoder normalized keeps chunk ends simple.
	s.rc.normalize()

	return true
}

// resetLZMA resets the LZMA decoder and range decoder state. The dictionary
// is not touched: LZMA state may be reset without resetting the dictionary.
func (s *lzma2Dec) resetLZMA() {
	s.lzma.state = stateLitLit
	s.lzma.rep0 = 0
	s.lzma.rep1 = 0
	s.lzma.rep2 = 0
	s.lzma.rep3 = 0
	s.lzma.resetProbs()
	s.rc.reset()
}

// props decodes the one-byte LZMA properties (lc/lp/pb).
func (s *lzma2Dec) props(p byte) bool {
	if p > (4*5+4)*9+8 {
		return false
	}

	s.lzma.posMask = 0
	for p >= 9*5 {
		p -= 9 * 5
		s.lzma.posMask++
	}
	s.lzma.posMask = 1<<s.lzma.posMask - 1

	s.lzma.literalPosMask = 0
	for p >= 9 {
		p -= 9
		s.lzma.literalPosMask++
	}
	s.lzma.lc = uint32(p)

	if s.lzma.lc+s.lzma.literalPosMask > 4 {
		return false
	}
	s.lzma.literalPosMask = 1<<s.lzma.literalPosMask - 1

	s.resetLZMA()

	return true
}

/*********
 * LZMA2 *
 *********/

// lzma2Dec is the LZMA2 chunk layer plus the LZMA decoder state.
type lzma2Dec struct {
	rc   rcDec
	dict dictionary
	lzma lzmaDec

	seq           lzma2Seq
	nextSeq       lzma2Seq
	uncompressed  int
	compressed    int
	needDictReset bool
	needProps     bool

	temp struct {
		size int
		buf  [3 * lzmaInRequired]byte
	}
}

type lzma2Seq int

const (
	seqControl lzma2Seq = iota
	seqUncompressed1
	seqUncompressed2
	seqCompressed0
	seqCompressed1
	seqProperties
	seqLzmaPrepare
	seqLzmaRun
	seqCopy
)

// lzma2Lzma feeds the LZMA decoder, keeping LZMA_IN_REQUIRED bytes of input
// available across buffer boundaries.
func (s *lzma2Dec) lzma2Lzma(b *buf) bool {
	inAvail := b.inSize - b.inPos

	if s.temp.size > 0 || s.compressed == 0 {
		tmp := 2*lzmaInRequired - s.temp.size
		if tmp > s.compressed-s.temp.size {
			tmp = s.compressed - s.temp.size
		}
		if tmp > inAvail {
			tmp = inAvail
		}

		copy(s.temp.buf[s.temp.size:], b.in[b.inPos:b.inPos+tmp])

		if s.temp.size+tmp == s.compressed {
			for i := s.temp.size + tmp; i < len(s.temp.buf); i++ {
				s.temp.buf[i] = 0
			}
			s.rc.inLimit = s.temp.size + tmp
		} else if s.temp.size+tmp < lzmaInRequired {
			s.temp.size += tmp
			b.inPos += tmp
			return true
		} else {
			s.rc.inLimit = s.temp.size + tmp - lzmaInRequired
		}

		s.rc.in = s.temp.buf[:]
		s.rc.inPos = 0

		if !s.main() || s.rc.inPos > s.temp.size+tmp {
			return false
		}

		s.compressed -= s.rc.inPos

		if s.rc.inPos < s.temp.size {
			s.temp.size -= s.rc.inPos
			copy(s.temp.buf[:], s.temp.buf[s.rc.inPos:s.rc.inPos+s.temp.size])
			return true
		}

		b.inPos += s.rc.inPos - s.temp.size
		s.temp.size = 0
	}

	inAvail = b.inSize - b.inPos
	if inAvail >= lzmaInRequired {
		s.rc.in = b.in
		s.rc.inPos = b.inPos

		if inAvail >= s.compressed+lzmaInRequired {
			s.rc.inLimit = b.inPos + s.compressed
		} else {
			s.rc.inLimit = b.inSize - lzmaInRequired
		}

		if !s.main() {
			return false
		}

		inAvail = s.rc.inPos - b.inPos
		if inAvail > s.compressed {
			return false
		}
		s.compressed -= inAvail
		b.inPos = s.rc.inPos
	}

	inAvail = b.inSize - b.inPos
	if inAvail < lzmaInRequired {
		if inAvail > s.compressed {
			inAvail = s.compressed
		}
		copy(s.temp.buf[:], b.in[b.inPos:b.inPos+inAvail])
		s.temp.size = inAvail
		b.inPos += inAvail
	}

	return true
}

// run is the LZMA2 control layer.
func (s *lzma2Dec) run(b *buf) xzRet {
	for b.inPos < b.inSize || s.seq == seqLzmaRun {
		switch s.seq {
		case seqControl:
			control := b.in[b.inPos]
			b.inPos++

			if control == 0 {
				return xzStreamEnd
			}

			if control >= 0xE0 || control == 1 {
				s.needProps = true
				s.needDictReset = false
				s.dict.reset()
			} else if s.needDictReset {
				return xzError
			}

			if control >= 0x80 {
				s.uncompressed = int(control&0x1F) << 16
				s.seq = seqUncompressed1

				if control >= 0xC0 {
					s.needProps = false
					s.nextSeq = seqProperties
				} else if s.needProps {
					return xzError
				} else {
					s.nextSeq = seqLzmaPrepare
					if control >= 0xA0 {
						s.resetLZMA()
					}
				}
			} else {
				if control > 2 {
					return xzError
				}
				s.seq = seqCompressed0
				s.nextSeq = seqCopy
			}

		case seqUncompressed1:
			s.uncompressed += int(b.in[b.inPos]) << 8
			b.inPos++
			s.seq = seqUncompressed2

		case seqUncompressed2:
			s.uncompressed += int(b.in[b.inPos]) + 1
			b.inPos++
			s.seq = seqCompressed0

		case seqCompressed0:
			s.compressed = int(b.in[b.inPos]) << 8
			b.inPos++
			s.seq = seqCompressed1

		case seqCompressed1:
			s.compressed += int(b.in[b.inPos]) + 1
			b.inPos++
			s.seq = s.nextSeq

		case seqProperties:
			if !s.props(b.in[b.inPos]) {
				return xzError
			}
			b.inPos++
			s.seq = seqLzmaPrepare

		case seqLzmaPrepare:
			if s.compressed < rcInitBytes {
				return xzError
			}
			if !s.rc.readInit(b) {
				return xzOK
			}
			s.compressed -= rcInitBytes
			s.seq = seqLzmaRun

		case seqLzmaRun:
			outMax := b.outSize - b.outPos
			if s.uncompressed < outMax {
				outMax = s.uncompressed
			}
			s.dict.setLimit(outMax)

			if !s.lzma2Lzma(b) {
				return xzError
			}

			s.uncompressed -= s.dict.flush(b)

			if s.uncompressed == 0 {
				if s.compressed > 0 || s.lzma.len > 0 || s.rc.code != 0 {
					return xzError
				}
				s.rc.reset()
				s.seq = seqControl
			} else if b.outPos == b.outSize ||
				(b.inPos == b.inSize && s.temp.size < s.compressed) {
				return xzOK
			}

		case seqCopy:
			s.dict.uncompressed(b, &s.compressed)
			if s.compressed > 0 {
				return xzOK
			}
			s.seq = seqControl
		}
	}

	return xzOK
}

// reset decodes the LZMA2 properties (one byte) and resets the decoder.
func (s *lzma2Dec) reset(props byte, dictLimit int) xzRet {
	// The reference decoder rejects property bytes above 39 here; the
	// largest dictionary it describes is 3 GiB.
	if props > 39 {
		return xzOptionsError
	}

	size := int(2 + (props & 1))
	size <<= uint(props>>1) + 11

	// Never allocate from an untrusted dictionary size: streams that ask
	// for more than the limit are refused.
	if size > dictLimit {
		return xzMemoryError
	}

	s.dict.size = size
	s.dict.end = size
	if len(s.dict.buf) < size {
		s.dict.buf = make([]byte, size)
	}

	s.lzma.len = 0

	s.seq = seqControl
	s.needDictReset = true
	s.temp.size = 0

	return xzOK
}
