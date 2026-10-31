// Format tests for the Misa locate database container (locate_db.go).
package builtins

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

// locateTestPaths contains the byte shapes the format must survive: newlines,
// tabs, invalid UTF-8 and a shared prefix for the front coding.
var locateTestPaths = []string{
	"/",
	"/home/user",
	"/home/user/a\nb",
	"/home/user/caf\xe9",
	"/home/user/dir/file",
	"/home/user/dir/file2",
	"/usr/bin/\ttab",
	"/usr/bin/sh",
}

func buildLocateDB(t *testing.T, paths []string) []byte {
	t.Helper()
	var buf bytes.Buffer
	lw, err := newLocateWriter(&buf, 1234567890, []string{"/home/user", "/usr"})
	if err != nil {
		t.Fatalf("newLocateWriter: %v", err)
	}
	for _, p := range paths {
		if err := lw.Add(p); err != nil {
			t.Fatalf("Add(%q): %v", p, err)
		}
	}
	if err := lw.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	return buf.Bytes()
}

func readLocateDB(t *testing.T, data []byte) (locateHeader, []string) {
	t.Helper()
	lr, err := openLocateDB(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("openLocateDB: %v", err)
	}
	var got []string
	for {
		p, ok, err := lr.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if !ok {
			break
		}
		got = append(got, p)
	}
	return lr.Header, got
}

func TestLocateDBRoundTrip(t *testing.T) {
	data := buildLocateDB(t, locateTestPaths)
	hdr, got := readLocateDB(t, data)
	if hdr.Version != locateVersion {
		t.Errorf("version = %d, want %d", hdr.Version, locateVersion)
	}
	if hdr.Created != 1234567890 {
		t.Errorf("created = %d", hdr.Created)
	}
	if strings.Join(hdr.Roots, ",") != "/home/user,/usr" {
		t.Errorf("roots = %q", hdr.Roots)
	}
	if strings.Join(got, "|") != strings.Join(locateTestPaths, "|") {
		t.Errorf("paths = %q, want %q", got, locateTestPaths)
	}
}

func TestLocateDBHeaderLayout(t *testing.T) {
	data := buildLocateDB(t, []string{"/a"})
	if !bytes.HasPrefix(data, []byte(locateMagic)) {
		t.Fatalf("missing magic: %q", data[:12])
	}
	if v := binary.LittleEndian.Uint32(data[len(locateMagic):]); v != locateVersion {
		t.Errorf("version field = %d", v)
	}
	if !bytes.HasSuffix(data, []byte(locateEndMagic)) {
		t.Errorf("missing end magic")
	}
}

// TestLocateDBFrontCoding checks that a shared prefix is stored once, not
// once per entry.
func TestLocateDBFrontCoding(t *testing.T) {
	shared := "/deeply/nested/directory/with/a/long/prefix/"
	var paths []string
	for i := 0; i < 200; i++ {
		paths = append(paths, fmt.Sprintf("%sfile-%03d", shared, i))
	}
	data := buildLocateDB(t, paths)
	naive := len(strings.Join(paths, ""))
	if len(data) >= naive {
		t.Errorf("front coded database is %d bytes, not smaller than the %d bytes of names", len(data), naive)
	}
	if _, got := readLocateDB(t, data); len(got) != len(paths) {
		t.Errorf("got %d paths, want %d", len(got), len(paths))
	}
}

func TestLocateDBEmptyDatabase(t *testing.T) {
	data := buildLocateDB(t, nil)
	hdr, got := readLocateDB(t, data)
	if len(got) != 0 {
		t.Errorf("paths = %q, want none", got)
	}
	if len(hdr.Roots) != 2 {
		t.Errorf("roots = %q", hdr.Roots)
	}
}

func TestLocateDBRejectsBadPaths(t *testing.T) {
	lw, err := newLocateWriter(io.Discard, 0, []string{"/"})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"", "a\x00b", "relative/path"} {
		if err := lw.Add(p); err == nil {
			t.Errorf("Add(%q) = nil, want an error", p)
		}
	}
}

func TestLocateDBReadErrors(t *testing.T) {
	mutate := func(fn func([]byte) []byte) []byte {
		return fn(buildLocateDB(t, locateTestPaths))
	}
	magicLen := len(locateMagic)
	tests := []struct {
		name string
		data []byte
		want error
		msg  string
	}{
		{name: "empty", data: nil, want: errLocateTruncated},
		{name: "one byte", data: []byte("X"), want: errLocateTruncated},
		{name: "short magic", data: []byte(locateMagic[:6]), want: errLocateTruncated},
		{name: "gzip instead", data: []byte("\x1f\x8b\x08\x00not a locate db"), want: errLocateNotDB},
		{name: "locate02", data: []byte("\x00LOCATE02\x00\x00/foo\x00"), want: errLocateNotDB},
		{
			name: "future version",
			data: mutate(func(d []byte) []byte {
				binary.LittleEndian.PutUint32(d[magicLen:], locateVersion+1)
				return d
			}),
			msg: "unsupported database format version",
		},
		{
			name: "truncated entries",
			data: mutate(func(d []byte) []byte { return d[:len(d)-40] }),
			want: errLocateTruncated,
		},
		{
			name: "truncated trailer",
			data: mutate(func(d []byte) []byte { return d[:len(d)-len(locateEndMagic)] }),
			want: errLocateTruncated,
		},
		{
			name: "checksum mismatch",
			data: mutate(func(d []byte) []byte {
				// Flip the stored checksum (the 4 bytes before the end magic).
				d[len(d)-len(locateEndMagic)-1] ^= 0xff
				return d
			}),
			want: errLocateCorrupt,
		},
		{
			name: "bad end marker",
			data: mutate(func(d []byte) []byte {
				copy(d[len(d)-len(locateEndMagic):], "MISA-LOCATE-XXX\x00")
				return d
			}),
			want: errLocateBadEnd,
		},
		{
			name: "trailing garbage",
			data: mutate(func(d []byte) []byte {
				return append(d, "extra"...)
			}),
			want: errLocateTrailing,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lr, err := openLocateDB(bytes.NewReader(tc.data))
			if err == nil {
				for {
					var ok bool
					if _, ok, err = lr.Next(); !ok || err != nil {
						break
					}
				}
			}
			if err == nil {
				t.Fatalf("read %q without an error, want %v", tc.name, tc.want)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
			if tc.msg != "" && !strings.Contains(err.Error(), tc.msg) {
				t.Errorf("error = %q, want it to mention %q", err, tc.msg)
			}
		})
	}
}

// TestLocateDBPrefixBeyondPrevious feeds a record whose prefix is longer than
// the previous path, which the front coding can never produce.
func TestLocateDBPrefixBeyondPrevious(t *testing.T) {
	// Hand-build a header plus one bad record.
	var hdr []byte
	hdr = append(hdr, locateMagic...)
	hdr = binary.LittleEndian.AppendUint32(hdr, locateVersion)
	hdr = binary.LittleEndian.AppendUint64(hdr, 0)
	hdr = binary.LittleEndian.AppendUint32(hdr, 0)
	body := binary.LittleEndian.AppendUint32(nil, 3) // prefix longer than ""
	body = binary.LittleEndian.AppendUint32(body, 1)
	body = append(body, 'x')
	data := append(hdr, body...)
	lr, err := openLocateDB(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("openLocateDB: %v", err)
	}
	if _, _, err := lr.Next(); !errors.Is(err, errLocateCorrupt) {
		t.Errorf("error = %v, want %v", err, errLocateCorrupt)
	}
}
