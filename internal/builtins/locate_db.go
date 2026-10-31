// Database container for locate/updatedb.
//
// The CLI behaviour of locate and updatedb follows uutils/findutils
// src/locate/mod.rs and src/updatedb/mod.rs, commit
// d71f3242b0bc3b21bb311f04b23ae62e5f3fed67 (MIT). The on-disk container is
// Misa's own versioned format (see docs/plans/locate.md); the GNU LOCATE02 /
// slocate layouts are neither read nor written.
//
// Layout (all integers little endian):
//
//	"MISA-LOCATE\0"            12-byte magic
//	uint32 version             locateFormatVersion
//	int64  created             unix seconds
//	uint32 rootCount
//	rootCount × (uint32 len, len bytes)   indexed roots, absolute
//	entry stream, front coded against the previous path:
//	  uint32 prefixLen, uint32 suffixLen, suffixLen bytes
//	  repeated; 0xFFFFFFFF in place of prefixLen ends the stream
//	uint64 entryCount
//	uint32 crc32 (IEEE) of the entry stream bytes
//	"MISA-LOCATE-END\0"        end magic
//
// Paths are byte strings: newlines are legal in a name, NUL is not, and the
// format never needs a NUL terminator because every field is length prefixed.
// A truncated, corrupt, or future-version database is refused with a clear
// error instead of being read optimistically.
package builtins

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"path/filepath"
	"strings"
)

const (
	locateMagic      = "MISA-LOCATE\x00"
	locateEndMagic   = "MISA-LOCATE-END\x00"
	locateVersion    = 1
	locateEndOfEntry = 0xFFFFFFFF
	locateMaxRoots   = 4096
	locateMaxName    = 1 << 20
)

// Locate database errors. locate and updatedb print these after the database
// path so a broken database is never mistaken for "no matches".
var (
	errLocateNotDB     = errors.New("not a Misa locate database (bad magic)")
	errLocateTruncated = errors.New("truncated database")
	errLocateCorrupt   = errors.New("corrupted database (checksum mismatch)")
	errLocateBadEnd    = errors.New("corrupted database (bad end marker)")
	errLocateTrailing  = errors.New("corrupted database (data after end marker)")
)

// locateHeader is the metadata every database starts with.
type locateHeader struct {
	Version uint32
	Created int64
	Roots   []string
}

// locateWriter streams a database to an io.Writer. Entries must be added as
// byte-sorted absolute paths for the front coding to pay off; Add refuses a
// path that is not absolute so a database never mixes relative names.
type locateWriter struct {
	w    io.Writer
	crc  hash.Hash32
	prev []byte
	n    uint64
	buf  []byte
}

func newLocateWriter(w io.Writer, created int64, roots []string) (*locateWriter, error) {
	if len(roots) > locateMaxRoots {
		return nil, fmt.Errorf("too many roots (%d)", len(roots))
	}
	var hdr []byte
	hdr = append(hdr, locateMagic...)
	hdr = binary.LittleEndian.AppendUint32(hdr, locateVersion)
	hdr = binary.LittleEndian.AppendUint64(hdr, uint64(created))
	hdr = binary.LittleEndian.AppendUint32(hdr, uint32(len(roots)))
	for _, root := range roots {
		if len(root) > locateMaxName {
			return nil, fmt.Errorf("root %q is too long", root)
		}
		hdr = binary.LittleEndian.AppendUint32(hdr, uint32(len(root)))
		hdr = append(hdr, root...)
	}
	if _, err := w.Write(hdr); err != nil {
		return nil, err
	}
	return &locateWriter{w: w, crc: crc32.NewIEEE()}, nil
}

// Add appends one path. Paths must be absolute and may contain any byte
// except NUL.
func (lw *locateWriter) Add(path string) error {
	if err := checkLocatePath(path, "indexing"); err != nil {
		return err
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("indexing: %q is not an absolute path", path)
	}
	prefix := 0
	for prefix < len(path) && prefix < len(lw.prev) && path[prefix] == lw.prev[prefix] {
		prefix++
	}
	suffix := path[prefix:]
	lw.buf = lw.buf[:0]
	lw.buf = binary.LittleEndian.AppendUint32(lw.buf, uint32(prefix))
	lw.buf = binary.LittleEndian.AppendUint32(lw.buf, uint32(len(suffix)))
	lw.buf = append(lw.buf, suffix...)
	lw.crc.Write(lw.buf)
	if _, err := lw.w.Write(lw.buf); err != nil {
		return err
	}
	lw.prev = append(lw.prev[:0], path...)
	lw.n++
	return nil
}

// Finish writes the trailer that makes truncation detectable.
func (lw *locateWriter) Finish() error {
	var tail []byte
	tail = binary.LittleEndian.AppendUint32(tail, locateEndOfEntry)
	tail = binary.LittleEndian.AppendUint64(tail, lw.n)
	tail = binary.LittleEndian.AppendUint32(tail, lw.crc.Sum32())
	tail = append(tail, locateEndMagic...)
	_, err := lw.w.Write(tail)
	return err
}

// locateReader streams the paths out of a database, validating the header, the
// entry records, the checksum and the end marker.
type locateReader struct {
	r      *bufio.Reader
	crc    hash.Hash32
	prev   []byte
	n      uint64
	Header locateHeader
	done   bool
}

// openLocateDB reads and validates the header of rd.
func openLocateDB(rd io.Reader) (*locateReader, error) {
	lr := &locateReader{r: bufio.NewReaderSize(rd, 64<<10), crc: crc32.NewIEEE()}
	var magic [len(locateMagic)]byte
	if _, err := io.ReadFull(lr.r, magic[:]); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%w: %w", errLocateTruncated, err)
		}
		return nil, err
	}
	if string(magic[:]) != locateMagic {
		return nil, errLocateNotDB
	}
	version, err := lr.readUint32(false)
	if err != nil {
		return nil, err
	}
	if version != locateVersion {
		return nil, fmt.Errorf("unsupported database format version %d (this locate reads version %d)", version, locateVersion)
	}
	created, err := lr.readUint64(false)
	if err != nil {
		return nil, err
	}
	rootCount, err := lr.readUint32(false)
	if err != nil {
		return nil, err
	}
	if rootCount > locateMaxRoots {
		return nil, fmt.Errorf("%w: implausible root count %d", errLocateCorrupt, rootCount)
	}
	roots := make([]string, 0, rootCount)
	for i := uint32(0); i < rootCount; i++ {
		root, err := lr.readString(false)
		if err != nil {
			return nil, err
		}
		roots = append(roots, root)
	}
	lr.Header = locateHeader{Version: version, Created: int64(created), Roots: roots}
	return lr, nil
}

// Next returns the next indexed path. ok is false after a validated trailer.
func (lr *locateReader) Next() (path string, ok bool, err error) {
	if lr.done {
		return "", false, nil
	}
	prefix, err := lr.readUint32(false)
	if err != nil {
		return "", false, err
	}
	if prefix == locateEndOfEntry {
		return "", false, lr.readTrailer()
	}
	lr.crc.Write(binary.LittleEndian.AppendUint32(nil, prefix))
	if int(prefix) > len(lr.prev) {
		return "", false, fmt.Errorf("%w: prefix %d exceeds the previous path", errLocateCorrupt, prefix)
	}
	suffix, err := lr.readString(true)
	if err != nil {
		return "", false, err
	}
	joined := make([]byte, 0, int(prefix)+len(suffix))
	joined = append(joined, lr.prev[:prefix]...)
	joined = append(joined, suffix...)
	lr.prev = joined
	lr.n++
	return string(joined), true, nil
}

func (lr *locateReader) readTrailer() error {
	count, err := lr.readUint64(false)
	if err != nil {
		return err
	}
	sum, err := lr.readUint32(false)
	if err != nil {
		return err
	}
	var end [len(locateEndMagic)]byte
	if _, err := io.ReadFull(lr.r, end[:]); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			return fmt.Errorf("%w: %w", errLocateTruncated, err)
		}
		return err
	}
	if string(end[:]) != locateEndMagic {
		return errLocateBadEnd
	}
	if count != lr.n {
		return fmt.Errorf("%w: %d entries counted, %d read", errLocateCorrupt, count, lr.n)
	}
	if sum != lr.crc.Sum32() {
		return errLocateCorrupt
	}
	if _, err := lr.r.ReadByte(); err == nil {
		return errLocateTrailing
	} else if !errors.Is(err, io.EOF) {
		return err
	}
	lr.done = true
	return nil
}

// read fills p and, when hashed, folds the bytes into the checksum that covers
// the entry stream only (the header and trailer are not checksummed).
func (lr *locateReader) read(p []byte, hashed bool) error {
	if _, err := io.ReadFull(lr.r, p); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			return fmt.Errorf("%w: %w", errLocateTruncated, err)
		}
		return err
	}
	if hashed {
		lr.crc.Write(p)
	}
	return nil
}

func (lr *locateReader) readUint32(hashed bool) (uint32, error) {
	var b [4]byte
	if err := lr.read(b[:], hashed); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b[:]), nil
}

func (lr *locateReader) readUint64(hashed bool) (uint64, error) {
	var b [8]byte
	if err := lr.read(b[:], hashed); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b[:]), nil
}

func (lr *locateReader) readString(hashed bool) (string, error) {
	n, err := lr.readUint32(hashed)
	if err != nil {
		return "", err
	}
	if n > locateMaxName {
		return "", fmt.Errorf("%w: implausible name length %d", errLocateCorrupt, n)
	}
	if n == 0 {
		return "", nil
	}
	b := make([]byte, n)
	if err := lr.read(b, hashed); err != nil {
		return "", err
	}
	return string(b), nil
}

// checkLocatePath rejects paths the format cannot represent (or should not
// store: relative names would be meaningless in a global database).
func checkLocatePath(path, what string) error {
	if path == "" {
		return fmt.Errorf("%s: empty path", what)
	}
	if strings.IndexByte(path, 0) >= 0 {
		return fmt.Errorf("%s: %q contains a NUL byte", what, path)
	}
	if len(path) > locateMaxName {
		return fmt.Errorf("%s: %q is too long", what, path)
	}
	return nil
}
