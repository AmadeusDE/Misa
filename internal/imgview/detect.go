package imgview

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// IsImageFile reports whether CheckImageFile finds an image, discarding errors.
func IsImageFile(path string, limits Limits) bool {
	ok, _ := CheckImageFile(path, limits)
	return ok
}

// CheckImageFile checks a supported extension and a bounded matching signature.
// Unsupported names and ordinary non-image contents return false without error;
// inaccessible, non-regular, changed, or oversized files return an error.
func CheckImageFile(path string, limits Limits) (bool, error) {
	if !isImageName(path) {
		return false, nil
	}
	limit := limits.normalized().MaxEncodedBytes
	before, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if !before.Mode().IsRegular() {
		return false, errors.New("not a regular file")
	}
	if before.Size() > limit {
		return false, ErrLimit
	}
	f, err := openImageCandidate(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	after, err := f.Stat()
	if err != nil {
		return false, err
	}
	if !after.Mode().IsRegular() || !os.SameFile(before, after) {
		return false, errors.New("file changed while opening")
	}
	if after.Size() > limit {
		return false, ErrLimit
	}
	var header [512]byte
	n, err := io.ReadFull(f, header[:])
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return false, fmt.Errorf("reading image header: %w", err)
	}
	end, err := f.Stat()
	if err != nil {
		return false, err
	}
	if end.Size() > limit {
		return false, ErrLimit
	}
	if end.Size() != after.Size() || !end.ModTime().Equal(after.ModTime()) {
		return false, errors.New("file changed while reading")
	}
	b := header[:n]
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")), nil
	case ".jpg", ".jpeg":
		return bytes.HasPrefix(b, []byte("\xff\xd8\xff")), nil
	case ".gif":
		return bytes.HasPrefix(b, []byte("GIF87a")) || bytes.HasPrefix(b, []byte("GIF89a")), nil
	case ".webp":
		return len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP", nil
	case ".bmp":
		return bytes.HasPrefix(b, []byte("BM")), nil
	case ".tif", ".tiff":
		return bytes.HasPrefix(b, []byte("II\x2a\x00")) || bytes.HasPrefix(b, []byte("MM\x00\x2a")), nil
	}
	return false, nil
}
