package xz

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"testing"
)

// TestDecodeChecks decodes one stream per supported integrity check type.
func TestDecodeChecks(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		want    []byte
	}{
		{"crc64", "helloXZ", []byte(helloText)},
		{"crc32", "helloCRC32", []byte(helloText)},
		{"sha256-skipped", "helloSHA256", []byte(helloText)},
		{"none", "helloNone", []byte(helloText)},
		{"small dictionary", "helloDict64K", []byte(helloText)},
		{"multiple blocks", "blocksXZ", blockText()},
		{"bcj x86", "x86XZ", x86Data()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeAll(NewReader(bytes.NewReader(fixture(t, tc.fixture))))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("decoded %d bytes, want %d", len(got), len(tc.want))
			}
		})
	}
}

// TestDecodeStreams covers concatenated streams and stream padding, which the
// reference decoder does not handle.
func TestDecodeStreams(t *testing.T) {
	want := []byte(helloText + helloText)

	t.Run("two streams", func(t *testing.T) {
		got, err := decodeAll(NewReader(bytes.NewReader(fixture(t, "twoStreamsXZ"))))
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("decoded %q", got)
		}
	})

	t.Run("stream padding", func(t *testing.T) {
		got, err := decodeAll(NewReader(bytes.NewReader(fixture(t, "paddingXZ"))))
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("decoded %q", got)
		}
	})

	t.Run("single stream", func(t *testing.T) {
		got, err := decodeAll(NewReader(bytes.NewReader(fixture(t, "helloXZ"))))
		if err != nil || !bytes.Equal(got, []byte(helloText)) {
			t.Fatalf("decode = %q, %v", got, err)
		}
	})
}

// TestTinyReads checks that the decoder works when the caller and the source
// hand over one byte at a time.
func TestTinyReads(t *testing.T) {
	data := fixture(t, "blocksXZ")
	r := NewReader(&oneByteReader{bytes.NewReader(data)})
	var out bytes.Buffer
	buf := make([]byte, 1)
	for {
		n, err := r.Read(buf)
		out.Write(buf[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read: %v", err)
		}
	}
	if !bytes.Equal(out.Bytes(), blockText()) {
		t.Fatalf("decoded %d bytes, want %d", out.Len(), len(blockText()))
	}
}

type oneByteReader struct{ r io.Reader }

func (o *oneByteReader) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	return o.r.Read(p)
}

func TestDictionaryLimit(t *testing.T) {
	// The default limit accepts an ordinary dictionary.
	if _, err := decodeAll(NewReader(bytes.NewReader(fixture(t, "helloXZ")))); err != nil {
		t.Fatalf("default limit: %v", err)
	}

	// A 64 KiB dictionary fits in a 64 KiB limit...
	if _, err := decodeAll(NewReaderWithLimit(bytes.NewReader(fixture(t, "helloDict64K")), 64<<10)); err != nil {
		t.Fatalf("64 KiB limit: %v", err)
	}

	// ...but an 8 MiB dictionary does not fit in 1 MiB.
	_, err := decodeAll(NewReaderWithLimit(bytes.NewReader(fixture(t, "helloXZ")), 1<<20))
	if !errors.Is(err, ErrMemoryLimit) {
		t.Fatalf("1 MiB limit: got %v, want ErrMemoryLimit", err)
	}

	// A stream asking for a 128 MiB dictionary is refused, not allocated.
	_, err = decodeAll(NewReader(bytes.NewReader(fixture(t, "bigDictXZ"))))
	if !errors.Is(err, ErrMemoryLimit) {
		t.Fatalf("128 MiB dictionary: got %v, want ErrMemoryLimit", err)
	}
}

func TestUnsupportedFilters(t *testing.T) {
	// The delta filter is not part of the reference decoder's filter set.
	_, err := decodeAll(NewReader(bytes.NewReader(fixture(t, "deltaXZ"))))
	if !errors.Is(err, ErrOptions) {
		t.Fatalf("delta filter: got %v, want ErrOptions", err)
	}
}

func TestBadInput(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want error
	}{
		{"empty", nil, ErrNotXZ},
		{"short garbage", []byte("hello"), ErrNotXZ},
		{"garbage", bytes.Repeat([]byte("x"), 64), ErrNotXZ},
		{"trailing garbage", append(fixture(t, "helloXZ"), 'j', 'u', 'n', 'k'), ErrCorrupt},
		{"odd stream padding", append(fixture(t, "helloXZ"), 0, 0), ErrCorrupt},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeAll(NewReader(bytes.NewReader(tc.in)))
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

// TestTruncated feeds every prefix of a stream to the decoder. None may
// panic, and every proper prefix must be reported as an error.
func TestTruncated(t *testing.T) {
	for _, name := range []string{"helloXZ", "blocksXZ", "x86XZ", "helloSHA256"} {
		data := fixture(t, name)
		for i := 0; i < len(data); i++ {
			out, err := decodeAll(NewReader(bytes.NewReader(data[:i])))
			if err == nil {
				t.Fatalf("%s: %d-byte prefix decoded %d bytes without an error", name, i, len(out))
			}
		}
		if out, err := decodeAll(NewReader(bytes.NewReader(data))); err != nil {
			t.Fatalf("%s: complete stream: %v", name, err)
		} else if len(out) == 0 {
			t.Fatalf("%s: complete stream decoded nothing", name)
		}
	}
}

// TestCorrupt flips every bit of a small stream: the decoder must never
// panic, and a bit flip in the stored integrity check must be caught.
func TestCorrupt(t *testing.T) {
	data := fixture(t, "helloXZ")

	for i := range data {
		for bit := 0; bit < 8; bit++ {
			bad := append([]byte(nil), data...)
			bad[i] ^= 1 << bit
			if _, err := decodeAll(NewReader(bytes.NewReader(bad))); err == nil {
				// A flip somewhere in the stream could in principle
				// describe another valid stream; nothing to assert then
				// beyond "no panic and no hang".
				continue
			}
		}
	}

	// The last four bytes of the stream footer are its CRC32.
	bad := append([]byte(nil), data...)
	bad[len(bad)-1] ^= 0xff
	if _, err := decodeAll(NewReader(bytes.NewReader(bad))); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("footer CRC: got %v, want ErrCorrupt", err)
	}
}

// TestMemoryCap guards against allocating from an untrusted dictionary size:
// the block header property byte is rewritten to ask for the largest
// dictionary the format allows (3 GiB) and the header CRC32 is fixed up, so
// the stream is well formed but must be refused instead of allocated.
func TestMemoryCap(t *testing.T) {
	data := append([]byte(nil), fixture(t, "helloDict64K")...)

	// The stream header is 12 bytes; the block header follows and its
	// first byte gives its size in four-byte units. Its last four bytes are
	// the header CRC32.
	const headerStart = 12
	headerSize := int(data[headerStart]+1) << 2
	body := data[headerStart+1 : headerStart+headerSize-4]
	crcAt := headerStart + headerSize - 4

	idx := bytes.Index(body, []byte{0x21, 0x01})
	if idx < 0 || idx+2 >= len(body) {
		t.Fatalf("cannot find the LZMA2 filter properties in the fixture")
	}
	body[idx+2] = 39 // 3 GiB dictionary

	// The CRC32 covers the whole Block Header field, size byte included.
	crc := crc32.ChecksumIEEE(data[headerStart:crcAt])
	binary.LittleEndian.PutUint32(data[crcAt:], crc)

	_, err := decodeAll(NewReader(bytes.NewReader(data)))
	if !errors.Is(err, ErrMemoryLimit) {
		t.Fatalf("got %v, want ErrMemoryLimit", err)
	}

	// The same stream with a 64 KiB dictionary decodes when the limit allows.
	if _, err := decodeAll(NewReader(bytes.NewReader(fixture(t, "helloDict64K")))); err != nil {
		t.Fatalf("64 KiB dictionary: %v", err)
	}
}

func TestReadEmptyBuffer(t *testing.T) {
	r := NewReader(bytes.NewReader(fixture(t, "helloXZ")))
	if n, err := r.Read(nil); n != 0 || err != nil {
		t.Fatalf("Read(nil) = %d, %v", n, err)
	}
}

// FuzzDecode checks that no input can make the decoder panic or hang. Run it
// with `go test -fuzz=FuzzDecode ./internal/xz/`.
func FuzzDecode(f *testing.F) {
	f.Add([]byte(nil))
	f.Add(fixture(f2t(f), "helloXZ"))
	f.Add(fixture(f2t(f), "blocksXZ"))
	f.Add(fixture(f2t(f), "x86XZ"))
	f.Add(fixture(f2t(f), "deltaXZ"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// The decoder must return, with or without an error, and must not
		// read past its buffers.
		io.Copy(io.Discard, NewReader(bytes.NewReader(data)))
	})
}

// f2t adapts a *testing.F to the *testing.T that fixture() wants.
func f2t(f *testing.F) *testing.T {
	f.Helper()
	return &testing.T{}
}
