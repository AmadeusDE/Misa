# Plan: the XZ decoder (`internal/xz`)

`xzcat` needs an XZ decoder. Go's standard library has none (only `compress/bzip2`,
`compress/flate`, `compress/gzip`, `compress/lzw`, `compress/zstd`), so the choice was between adding
a dependency and porting a small decoder.

## Decision: an in-tree port

Misa decodes XZ with `internal/xz`, a native Go decoder ported from the public-domain XZ Embedded
decoder by Lasse Collin, as vendored by toybox in `toys/pending/xzcat.c` (0BSD, commit
`6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb`). No new module dependency, no cgo, no external `xz`
process is ever run.

Why not a dependency:

- **go.mod is a promise.** Misa builds for Linux, the BSDs, macOS, Windows and Android, without cgo
  and without a network at build time. A pure-Go XZ decoder exists
  (`github.com/ulikunitz/xz`), but it is a BSD-3 clause dependency with its own release cadence, and
  it would be the only compression dependency in an otherwise standard-library-only binary: the
  34 MB baseline is the embedded agent, not compression code.
- **The licence is the simplest possible.** The reference implementation is public domain / 0BSD. A
  port keeps the notice story identical to `gzip`'s: one file header, no bundled third-party tree.
- **The decoder is small.** About 1,700 lines of Go for LZMA2, the range coder, the dictionary and
  the stream container, plus about 400 lines for the seven BCJ branch filters. That is a fraction of
  what `gzip`'s plumbing in `internal/builtins` costs, and it is testable on its own.
- **It is the same reference as the tools the tests cross-check against.** `tests/toybox/xzcat.test`
  is written against this decoder's behaviour, so the port makes the vendored test suite meaningful.

The cost is maintenance: if the XZ format gains a filter, `internal/xz` needs the same work the
reference would need.

## What is supported

- Filters: LZMA2 (`0x21`) alone, or a BCJ filter followed by LZMA2. The BCJ set is exactly the one
  in the reference: x86, PowerPC, IA-64, ARM, ARM-Thumb, SPARC and ARM64, with no custom start
  offset. Any other filter ID (delta, for example) fails with
  `unsupported options in the .xz headers` rather than decoding wrongly.
- Integrity checks: none, CRC32 (ID 1) and CRC64 (ID 4) are verified. Every other check ID
  (SHA-256 = 10 and the reserved IDs) has its Check field skipped, exactly like the reference
  decoder. A stream claiming a check ID above 15 is rejected.
- Blocks: multiple blocks per stream, optional Compressed/Uncompressed Size fields, the Index and
  Index CRC32, block padding.
- Streams: concatenated streams and stream padding. Stream padding that is not a multiple of four
  bytes is corrupt. Trailing bytes that are not a stream are corrupt.

## Deliberate differences from the reference

- **The dictionary is capped.** `DefaultDictLimit` is 64 MiB, the same limit toybox uses (toybox
  exits with "Dictionary too big"). A block whose LZMA2 properties ask for more is refused with
  `ErrMemoryLimit` *before* anything is allocated, so memory use never depends on the input: the
  worst case is the cap itself, and `NewReaderWithLimit` lowers it further. Nothing else in the
  decoder is sized from the input at all.
- **Concatenated streams decode.** `toys/pending/xzcat.c` stops at the end of the first stream.
- **Truncation is distinguished from corruption.** `io.ErrUnexpectedEOF` versus `ErrCorrupt`, so a
  builtin can say "unexpected end of file" like `gunzip` does.
- **The Index is compared field by field.** The reference `memcmp()`s a struct that contains padding
  bytes; the two size fields are compared directly here.
- **Every read is bounds checked.** The reference relies on the caller's padding and on distances
  validated elsewhere. A malformed stream here yields zero bytes past the end of a buffer instead of
  reading past it, and `FuzzDecode` (see below) holds that line.

## Memory and malformed input

- The dictionary allocation is the only large allocation, it is capped (above), and it is reused
  between blocks.
- The decoder is multi-call: it consumes input in 4 KiB pieces and fills a 32 KiB output buffer, so
  file size does not affect memory use.
- `internal/xz/xz_test.go` feeds every prefix of four fixtures to the decoder (all must error
  without a panic), flips every bit of a stream, and carries a `FuzzDecode` target; `go test
  -fuzz=FuzzDecode ./internal/xz/` ran ~770,000 mutated inputs (and found the one bug that is now
  a regression seed in `internal/xz/testdata/`). No input panics or hangs.

## Tests

Fixtures are small `.xz` files embedded as base64, produced with XZ Utils 5.8.1 on the development
machine (the regeneration commands are in `internal/xz/fixtures_test.go`); the expected uncompressed
payloads are rebuilt in Go, so only the compressed bytes are checked in. They cover each check type,
a small dictionary, multiple blocks, a 128 MiB dictionary request, BCJ x86, the delta filter
(unsupported), two concatenated streams and stream padding. `tests/toybox/xzcat.test` runs the
toybox cases through the multicall binary.
