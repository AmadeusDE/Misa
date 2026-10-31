// Package xz decodes .xz files: the LZMA2 filter and the BCJ branch filters,
// the block, index and stream container, and concatenated streams.
//
// The decoder is a Go port of the public-domain XZ Embedded decoder by Lasse
// Collin as vendored by toybox in toys/pending/xzcat.c (0BSD), commit
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb. The check types, the filter set
// and the error taxonomy are the same as that file's.
//
// Deliberate differences from toys/pending/xzcat.c, all of them safety or
// completeness fixes:
//   - the LZMA2 dictionary is capped (DefaultDictLimit, 64 MiB, the toybox
//     limit); a stream asking for more fails with ErrMemoryLimit instead of
//     being allocated, and no buffer is ever sized from untrusted input
//     without that cap.
//   - concatenated streams and stream padding are decoded (toybox stops at
//     the end of the first stream).
//   - truncation is reported as io.ErrUnexpectedEOF rather than a generic
//     "corrupt" error.
//   - the Index comparison uses the two decoded size fields instead of
//     memcmp()ing the whole struct, so struct padding cannot matter.
//   - all reads are bounds checked; a malformed stream returns an error
//     instead of reading past a buffer.
//
// Integrity checks: NONE, CRC32 and CRC64 are verified; every other check
// type (SHA-256 and the reserved IDs) has its field skipped, exactly like the
// reference decoder.
package xz

import (
	"errors"
	"hash/crc32"
	"hash/crc64"
	"io"
)

// DefaultDictLimit is the largest LZMA2 dictionary the decoder will
// allocate, matching toybox's 64 MiB cap.
const DefaultDictLimit = 64 << 20

var (
	// ErrNotXZ reports input that does not start with the .xz stream
	// header magic.
	ErrNotXZ = errors.New("not in xz format")
	// ErrCorrupt reports malformed or corrupt compressed data.
	ErrCorrupt = errors.New("file is corrupt")
	// ErrOptions reports .xz features and filters the decoder does not
	// support (unknown filter IDs, non-zero reserved flags).
	ErrOptions = errors.New("unsupported options in the .xz headers")
	// ErrMemoryLimit reports an LZMA2 dictionary larger than the decoder's
	// limit.
	ErrMemoryLimit = errors.New("memory limit exceeded: LZMA2 dictionary is too big")
)

const (
	streamHeaderSize = 12
	headerMagicSize  = 6
	footerMagicSize  = 2

	// checkNONE, checkCRC32, checkCRC64 and checkSHA256 are the .xz Check
	// IDs; only the first three are verified.
	checkNone   = 0
	checkCRC32  = 1
	checkCRC64  = 4
	checkSHA256 = 10
	checkMax    = 15

	vliBytesMax = 9
	vliUnknown  = ^uint64(0)
)

var (
	headerMagic = [headerMagicSize]byte{0xFD, '7', 'z', 'X', 'Z', 0}
	// checkSizes maps a Check ID to the size of its Check field.
	checkSizes = [16]int{0, 4, 4, 4, 8, 8, 8, 16, 16, 16, 32, 32, 32, 64, 64, 64}
	crc64Table = crc64.MakeTable(crc64.ECMA)
)

// xzHash is the hash used to validate the Index field: the sums of the
// Unpadded Size and Uncompressed Size values.
type xzHash struct {
	unpadded     uint64
	uncompressed uint64
}

// streamSeq is the position in the stream decoder (dec_main).
type streamSeq int

const (
	seqStreamHeader streamSeq = iota
	seqBlockStart
	seqBlockHeader
	seqBlockUncompress
	seqBlockPadding
	seqBlockCheck
	seqIndex
	seqIndexPadding
	seqIndexCRC32
	seqStreamFooter
)

type indexSeq int

const (
	seqIndexCount indexSeq = iota
	seqIndexUnpadded
	seqIndexUncompressed
)

// stream is one .xz stream decoder.
type stream struct {
	seq streamSeq

	// pos is the position in a variable-length integer or Check field.
	pos int
	vli uint64

	inStart, outStart int

	crc       uint64
	checkType int

	blockHeader struct {
		compressed   uint64
		uncompressed uint64
		size         int
	}

	block struct {
		compressed   uint64
		uncompressed uint64
		count        uint64
		hash         xzHash
	}

	index struct {
		seq   indexSeq
		size  uint64
		count uint64
		hash  xzHash
	}

	temp struct {
		pos  int
		size int
		buf  [1024]byte
	}

	lzma2 *lzma2Dec
	bcj   *bcjDec

	bcjActive bool
	dictLimit int
}

func newStream(dictLimit int) *stream {
	s := &stream{
		seq:       seqStreamHeader,
		lzma2:     &lzma2Dec{},
		bcj:       &bcjDec{},
		dictLimit: dictLimit,
	}
	s.temp.size = streamHeaderSize
	return s
}

// fillTemp copies input into temp.buf until temp.size has been read.
func (s *stream) fillTemp(b *buf) bool {
	copySize := b.inSize - b.inPos
	if n := s.temp.size - s.temp.pos; n < copySize {
		copySize = n
	}
	copy(s.temp.buf[s.temp.pos:], b.in[b.inPos:b.inPos+copySize])
	b.inPos += copySize
	s.temp.pos += copySize

	if s.temp.pos == s.temp.size {
		s.temp.pos = 0
		return true
	}
	return false
}

// decVLI decodes a variable-length integer (little-endian base-128).
func (s *stream) decVLI(in []byte, inPos *int, inSize int) xzRet {
	if s.pos == 0 {
		s.vli = 0
	}

	for *inPos < inSize {
		by := in[*inPos]
		*inPos++

		s.vli |= uint64(by&0x7F) << uint(s.pos)

		if by&0x80 == 0 {
			// Non-minimal encodings are not allowed.
			if by == 0 && s.pos != 0 {
				return xzError
			}
			s.pos = 0
			return xzStreamEnd
		}

		s.pos += 7
		if s.pos == 7*vliBytesMax {
			return xzError
		}
	}

	return xzOK
}

// decBlock decodes the Compressed Data field of a block, updating and
// validating the observed sizes and the integrity check.
func (s *stream) decBlock(b *buf) xzRet {
	s.inStart = b.inPos
	s.outStart = b.outPos

	var ret xzRet
	if s.bcjActive {
		ret = s.bcj.run(s.lzma2, b)
	} else {
		ret = s.lzma2.run(b)
	}

	s.block.compressed += uint64(b.inPos - s.inStart)
	s.block.uncompressed += uint64(b.outPos - s.outStart)

	// The observed sizes are always smaller than vliUnknown.
	if s.block.compressed > s.blockHeader.compressed ||
		s.block.uncompressed > s.blockHeader.uncompressed {
		return xzError
	}

	switch s.checkType {
	case checkCRC32:
		s.crc = uint64(crc32.Update(uint32(s.crc), crc32.IEEETable, b.out[s.outStart:b.outPos]))
	case checkCRC64:
		s.crc = crc64.Update(s.crc, crc64Table, b.out[s.outStart:b.outPos])
	}

	if ret != xzStreamEnd {
		return ret
	}

	if s.blockHeader.compressed != vliUnknown && s.blockHeader.compressed != s.block.compressed {
		return xzError
	}
	if s.blockHeader.uncompressed != vliUnknown && s.blockHeader.uncompressed != s.block.uncompressed {
		return xzError
	}

	s.block.hash.unpadded += uint64(s.blockHeader.size) + s.block.compressed
	s.block.hash.unpadded += uint64(checkSizes[s.checkType])
	s.block.hash.uncompressed += s.block.uncompressed

	s.block.count++

	return ret
}

// indexUpdate adds the bytes consumed by the Index field to its size and CRC.
func (s *stream) indexUpdate(b *buf) {
	inUsed := b.inPos - s.inStart
	s.index.size += uint64(inUsed)
	s.crc = uint64(crc32.Update(uint32(s.crc), crc32.IEEETable, b.in[s.inStart:b.inPos]))
}

// decIndex decodes the Number of Records, Unpadded Size and Uncompressed Size
// fields of the Index; Index Padding and CRC32 are handled by the caller.
func (s *stream) decIndex(b *buf) xzRet {
	for {
		ret := s.decVLI(b.in, &b.inPos, b.inSize)
		if ret != xzStreamEnd {
			s.indexUpdate(b)
			return ret
		}

		switch s.index.seq {
		case seqIndexCount:
			s.index.count = s.vli
			if s.index.count != s.block.count {
				return xzError
			}
			s.index.seq = seqIndexUnpadded

		case seqIndexUnpadded:
			s.index.hash.unpadded += s.vli
			s.index.seq = seqIndexUncompressed

		case seqIndexUncompressed:
			s.index.hash.uncompressed += s.vli
			s.index.count--
			s.index.seq = seqIndexUnpadded
		}

		if s.index.count == 0 {
			return xzStreamEnd
		}
	}
}

// crcValidate checks the next four or eight input bytes against s.crc.
func (s *stream) crcValidate(b *buf, bits int) xzRet {
	for {
		if b.inPos == b.inSize {
			return xzOK
		}
		if byte(s.crc>>uint(s.pos)) != b.in[b.inPos] {
			return xzError
		}
		b.inPos++
		s.pos += 8
		if s.pos >= bits {
			break
		}
	}

	s.crc = 0
	s.pos = 0

	return xzStreamEnd
}

// checkSkip skips a Check field the decoder does not verify.
func (s *stream) checkSkip(b *buf) bool {
	for s.pos < checkSizes[s.checkType] {
		if b.inPos == b.inSize {
			return false
		}
		b.inPos++
		s.pos++
	}
	s.pos = 0
	return true
}

func (s *stream) decStreamHeader() xzRet {
	for i := range headerMagic {
		if s.temp.buf[i] != headerMagic[i] {
			return xzNotXZ
		}
	}

	if crc32.ChecksumIEEE(s.temp.buf[headerMagicSize:headerMagicSize+2]) !=
		uint32(s.temp.buf[8])|uint32(s.temp.buf[9])<<8|
			uint32(s.temp.buf[10])<<16|uint32(s.temp.buf[11])<<24 {
		return xzError
	}

	// Reserved flags must be zero.
	if s.temp.buf[headerMagicSize] != 0 {
		return xzOptionsError
	}

	s.checkType = int(s.temp.buf[headerMagicSize+1])
	if s.checkType > checkMax {
		return xzOptionsError
	}

	return xzOK
}

func (s *stream) decStreamFooter() xzRet {
	if s.temp.buf[10] != 'Y' || s.temp.buf[11] != 'Z' {
		return xzError
	}

	if crc32.ChecksumIEEE(s.temp.buf[4:10]) !=
		uint32(s.temp.buf[0])|uint32(s.temp.buf[1])<<8|
			uint32(s.temp.buf[2])<<16|uint32(s.temp.buf[3])<<24 {
		return xzError
	}

	// Backward Size: the Index CRC32 field was not counted in index.size.
	backward := uint32(s.temp.buf[4]) | uint32(s.temp.buf[5])<<8 |
		uint32(s.temp.buf[6])<<16 | uint32(s.temp.buf[7])<<24
	if s.index.size>>2 != uint64(backward) {
		return xzError
	}

	if s.temp.buf[8] != 0 || int(s.temp.buf[9]) != s.checkType {
		return xzError
	}

	return xzStreamEnd
}

func (s *stream) decBlockHeader() xzRet {
	// Validate the CRC32 of the header. A block header is at least four
	// bytes (size byte and CRC32), so anything shorter is corrupt.
	if s.temp.size < 4 {
		return xzError
	}
	s.temp.size -= 4
	if crc32.ChecksumIEEE(s.temp.buf[:s.temp.size]) !=
		uint32(s.temp.buf[s.temp.size])|uint32(s.temp.buf[s.temp.size+1])<<8|
			uint32(s.temp.buf[s.temp.size+2])<<16|uint32(s.temp.buf[s.temp.size+3])<<24 {
		return xzError
	}

	s.temp.pos = 2

	// Only one or two filters in the chain are supported, so any other
	// Block Flag bit is unsupported.
	if s.temp.buf[1]&0x3E != 0 {
		return xzOptionsError
	}

	if s.temp.buf[1]&0x40 != 0 {
		if s.decVLI(s.temp.buf[:], &s.temp.pos, s.temp.size) != xzStreamEnd {
			return xzError
		}
		s.blockHeader.compressed = s.vli
	} else {
		s.blockHeader.compressed = vliUnknown
	}

	if s.temp.buf[1]&0x80 != 0 {
		if s.decVLI(s.temp.buf[:], &s.temp.pos, s.temp.size) != xzStreamEnd {
			return xzError
		}
		s.blockHeader.uncompressed = s.vli
	} else {
		s.blockHeader.uncompressed = vliUnknown
	}

	// With two filters the first one must be a BCJ filter.
	s.bcjActive = s.temp.buf[1]&0x01 != 0
	if s.bcjActive {
		if s.temp.size-s.temp.pos < 2 {
			return xzOptionsError
		}
		if ret := s.bcj.reset(s.temp.buf[s.temp.pos]); ret != xzOK {
			return ret
		}
		s.temp.pos++

		// A custom start offset is not supported.
		if s.temp.buf[s.temp.pos] != 0 {
			return xzOptionsError
		}
		s.temp.pos++
	}

	// Valid Filter Flags always take at least two bytes.
	if s.temp.size-s.temp.pos < 2 {
		return xzError
	}

	// Filter ID must be LZMA2 with a one-byte property field.
	if s.temp.buf[s.temp.pos] != 0x21 {
		return xzOptionsError
	}
	s.temp.pos++

	if s.temp.buf[s.temp.pos] != 1 {
		return xzOptionsError
	}
	s.temp.pos++

	if s.temp.size-s.temp.pos < 1 {
		return xzError
	}

	if ret := s.lzma2.reset(s.temp.buf[s.temp.pos], s.dictLimit); ret != xzOK {
		return ret
	}
	s.temp.pos++

	// The rest must be Header Padding.
	for s.temp.pos < s.temp.size {
		if s.temp.buf[s.temp.pos] != 0 {
			return xzOptionsError
		}
		s.temp.pos++
	}

	s.temp.pos = 0
	s.block.compressed = 0
	s.block.uncompressed = 0

	return xzOK
}

// run decodes as much as the input and output buffers allow.
func (s *stream) run(b *buf) xzRet {
	s.inStart = b.inPos

	for {
		switch s.seq {
		case seqStreamHeader:
			if !s.fillTemp(b) {
				return xzOK
			}
			s.seq = seqBlockStart
			if ret := s.decStreamHeader(); ret != xzOK {
				return ret
			}

		case seqBlockStart:
			if b.inPos == b.inSize {
				return xzOK
			}

			// A zero byte starts the Index field.
			if b.in[b.inPos] == 0 {
				s.inStart = b.inPos
				b.inPos++
				s.seq = seqIndex
				break
			}

			// The size byte stays in the input: it is part of the
			// Block Header field.
			s.blockHeader.size = (int(b.in[b.inPos]) + 1) << 2

			s.temp.size = s.blockHeader.size
			s.temp.pos = 0
			s.seq = seqBlockHeader

		case seqBlockHeader:
			if !s.fillTemp(b) {
				return xzOK
			}
			if ret := s.decBlockHeader(); ret != xzOK {
				return ret
			}
			s.seq = seqBlockUncompress

		case seqBlockUncompress:
			if ret := s.decBlock(b); ret != xzStreamEnd {
				return ret
			}
			s.seq = seqBlockPadding

		case seqBlockPadding:
			for s.block.compressed&3 != 0 {
				if b.inPos == b.inSize {
					return xzOK
				}
				if b.in[b.inPos] != 0 {
					return xzError
				}
				b.inPos++
				s.block.compressed++
			}
			s.seq = seqBlockCheck

		case seqBlockCheck:
			switch s.checkType {
			case checkCRC32:
				if ret := s.crcValidate(b, 32); ret != xzStreamEnd {
					return ret
				}
			case checkCRC64:
				if ret := s.crcValidate(b, 64); ret != xzStreamEnd {
					return ret
				}
			default:
				if !s.checkSkip(b) {
					return xzOK
				}
			}
			s.seq = seqBlockStart

		case seqIndex:
			if ret := s.decIndex(b); ret != xzStreamEnd {
				return ret
			}
			s.seq = seqIndexPadding

		case seqIndexPadding:
			for (s.index.size+uint64(b.inPos-s.inStart))&3 != 0 {
				if b.inPos == b.inSize {
					s.indexUpdate(b)
					return xzOK
				}
				if b.in[b.inPos] != 0 {
					return xzError
				}
				b.inPos++
			}

			s.indexUpdate(b)

			if s.block.hash.unpadded != s.index.hash.unpadded ||
				s.block.hash.uncompressed != s.index.hash.uncompressed {
				return xzError
			}
			s.seq = seqIndexCRC32

		case seqIndexCRC32:
			if ret := s.crcValidate(b, 32); ret != xzStreamEnd {
				return ret
			}
			s.temp.size = streamHeaderSize
			s.temp.pos = 0
			s.seq = seqStreamFooter

		case seqStreamFooter:
			if !s.fillTemp(b) {
				return xzOK
			}
			return s.decStreamFooter()
		}
	}
}

// headerPrefixMatches reports whether the bytes of a partially read stream
// header still match the .xz magic. It lets the caller tell "trailing
// garbage" from "truncated stream" when the input ends mid-header.
func (s *stream) headerPrefixMatches() bool {
	if s.seq != seqStreamHeader {
		return true
	}
	for i := 0; i < s.temp.pos && i < headerMagicSize; i++ {
		if s.temp.buf[i] != headerMagic[i] {
			return false
		}
	}
	return true
}

// Reader decompresses a sequence of concatenated .xz streams. Stream padding
// (null bytes between streams) is accepted; trailing bytes that are not a
// stream are reported as ErrCorrupt.
type Reader struct {
	src       io.Reader
	dictLimit int

	dec     *stream
	pending []byte
	err     error

	inBuf   []byte
	inPos   int
	inSize  int
	outBuf  []byte
	eof     bool
	streams int
	padding int
}

var _ io.Reader = (*Reader)(nil)

// NewReader returns a Reader decompressing src with the default dictionary
// limit (DefaultDictLimit).
func NewReader(src io.Reader) *Reader {
	return NewReaderWithLimit(src, DefaultDictLimit)
}

// NewReaderWithLimit returns a Reader decompressing src, refusing streams
// whose LZMA2 dictionary is larger than dictLimit bytes.
func NewReaderWithLimit(src io.Reader, dictLimit int) *Reader {
	if dictLimit <= 0 {
		dictLimit = DefaultDictLimit
	}
	return &Reader{
		src:       src,
		dictLimit: dictLimit,
		inBuf:     make([]byte, 4096),
		outBuf:    make([]byte, 32*1024),
	}
}

func (r *Reader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for {
		if len(r.pending) > 0 {
			n := copy(p, r.pending)
			r.pending = r.pending[n:]
			return n, nil
		}
		if r.err != nil {
			return 0, r.err
		}
		r.step()
	}
}

// fill reads more input unless the source is exhausted.
func (r *Reader) fill() {
	if r.eof || r.inPos < r.inSize {
		return
	}
	n, err := r.src.Read(r.inBuf)
	r.inPos = 0
	r.inSize = n
	if n == 0 {
		if err != nil && !errors.Is(err, io.EOF) {
			r.err = err
			return
		}
		r.eof = true
	}
}

// step runs the decoder until it produces output, finishes a stream or
// reports an error.
func (r *Reader) step() {
	for {
		r.fill()
		if r.err != nil {
			return
		}

		if r.dec == nil {
			// Between streams only null padding may appear.
			r.skipPadding()

			if r.inPos == r.inSize {
				if !r.eof {
					continue
				}
				if r.streams == 0 {
					r.err = ErrNotXZ
					return
				}
				if r.padding%4 != 0 {
					r.err = ErrCorrupt
					return
				}
				r.err = io.EOF
				return
			}

			if r.padding%4 != 0 {
				r.err = ErrCorrupt
				return
			}
			r.padding = 0
			r.dec = newStream(r.dictLimit)
		}

		out := r.outBuf
		b := &buf{
			in:      r.inBuf,
			inPos:   r.inPos,
			inSize:  r.inSize,
			out:     out,
			outSize: len(out),
		}
		inBefore := b.inPos
		ret := r.dec.run(b)
		r.inPos = b.inPos

		if b.outPos > 0 {
			r.pending = out[:b.outPos]
		}

		switch ret {
		case xzStreamEnd:
			r.dec = nil
			r.streams++
			r.padding = 0
			// Trailers are validated inside the decoder; anything that
			// follows must be padding or another stream.
			if len(r.pending) > 0 {
				return
			}
		case xzNotXZ:
			if r.streams > 0 || r.padding > 0 {
				r.err = ErrCorrupt
			} else {
				r.err = ErrNotXZ
			}
			return
		case xzOptionsError:
			r.err = ErrOptions
			return
		case xzMemoryError:
			r.err = ErrMemoryLimit
			return
		case xzError:
			r.err = ErrCorrupt
			return
		default: // xzOK
			if b.inPos == inBefore && b.outPos == 0 && r.eof {
				// No progress and no more input. If what is there
				// already contradicts the stream magic this is not a
				// truncated stream but garbage after the last one.
				if !r.dec.headerPrefixMatches() {
					if r.streams == 0 {
						r.err = ErrNotXZ
					} else {
						r.err = ErrCorrupt
					}
					return
				}
				r.err = io.ErrUnexpectedEOF
				return
			}
			if len(r.pending) > 0 {
				return
			}
		}
	}
}

// skipPadding consumes the zero bytes between streams.
func (r *Reader) skipPadding() {
	for r.inPos < r.inSize && r.inBuf[r.inPos] == 0 {
		r.inPos++
		r.padding++
	}
}
