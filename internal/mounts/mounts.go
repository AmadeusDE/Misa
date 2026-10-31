// Package mounts reads the platform mount table and the space and inode
// statistics of the filesystems behind it, for the df builtin. The parsers are
// pure functions over an io.Reader, so fixture text can be tested on any
// platform; List and Statfs are the only platform-specific parts.
package mounts

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Mount is one row of the platform mount table.
type Mount struct {
	Device  string // source as reported by the kernel: "/dev/sda1", "tmpfs", "overlay", ...
	Dir     string // mount point with octal escapes decoded ("\\040" -> " ")
	FSType  string // e.g. "ext4", "tmpfs", "nfs4"
	Options string // comma-separated per-mount option list ("" when the platform has none)
	Root    string // mountinfo root of the mount ("" for /proc/mounts and non-bind mounts)
}

// Stat holds the fields df needs. Counts are in units of Bsize-byte blocks.
type Stat struct {
	Blocks uint64
	Bfree  uint64
	Bavail uint64
	Files  uint64
	Ffree  uint64
	Bsize  uint64 // fundamental block size in bytes
}

// ErrUnsupported is returned by List and Statfs on platforms without a mount
// table or statfs implementation in this package.
var ErrUnsupported = errors.New("mount table is not supported on this platform")

// mountLineLimit is far above bufio.Scanner's 64 KiB default: a mountinfo line
// carries every optional and super-block option and can grow past 64 KiB.
const mountLineLimit = 1 << 20

// Unescape decodes the octal escapes the kernel uses in mount tables
// ("\040" space, "\011" tab, "\012" newline, "\134" backslash, any \NNN).
// Only a backslash followed by exactly three octal digits is decoded; every
// other backslash, including a trailing one or a short "\0", is kept as is.
func Unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) && isOctal(s[i+1]) && isOctal(s[i+2]) && isOctal(s[i+3]) {
			b.WriteByte(byte(int(s[i+1]-'0')<<6 | int(s[i+2]-'0')<<3 | int(s[i+3]-'0')))
			i += 3
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isOctal(c byte) bool { return c >= '0' && c <= '7' }

// ParseMountInfo parses Linux /proc/self/mountinfo text. Device, Dir and Root
// are unescaped; Options is the per-mount option field verbatim, escapes
// included, since the kernel escapes only the path and source fields.
func ParseMountInfo(r io.Reader) ([]Mount, error) {
	var mounts []Mount
	err := scanLines(r, func(line string, number int) error {
		m, err := parseMountInfoLine(line)
		if err != nil {
			return fmt.Errorf("mountinfo line %d: %w", number, err)
		}
		mounts = append(mounts, m)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return mounts, nil
}

// mountInfoFields is the fixed prefix of a mountinfo line plus the separator
// and the three trailing fields: id parent major:minor root dir options - fstype source superopts.
const mountInfoFields = 10

func parseMountInfoLine(line string) (Mount, error) {
	fields := strings.Split(line, " ")
	if len(fields) < mountInfoFields {
		return Mount{}, fmt.Errorf("%d fields, want at least %d", len(fields), mountInfoFields)
	}
	sep := -1
	for i := 6; i < len(fields); i++ {
		if fields[i] == "-" {
			sep = i
			break
		}
	}
	if sep < 0 || sep+3 >= len(fields) {
		return Mount{}, errors.New(`missing "-" separator before fstype, source and super options`)
	}
	return Mount{
		Device:  Unescape(fields[sep+2]),
		Dir:     Unescape(fields[4]),
		FSType:  fields[sep+1],
		Options: fields[5],
		Root:    Unescape(fields[3]),
	}, nil
}

// ParseMounts parses Linux /proc/mounts (fstab-like) text:
// "device mountpoint fstype options dump pass". Device and Dir are unescaped;
// Options is kept verbatim and Root is empty.
func ParseMounts(r io.Reader) ([]Mount, error) {
	var mounts []Mount
	err := scanLines(r, func(line string, number int) error {
		fields := strings.Split(line, " ")
		if len(fields) < 6 {
			return fmt.Errorf("mounts line %d: %d fields, want 6", number, len(fields))
		}
		mounts = append(mounts, Mount{
			Device:  Unescape(fields[0]),
			Dir:     Unescape(fields[1]),
			FSType:  fields[2],
			Options: fields[3],
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return mounts, nil
}

// scanLines calls fn once per non-blank line, with its 1-based line number.
func scanLines(r io.Reader, fn func(line string, number int) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64<<10), mountLineLimit)
	for number := 1; scanner.Scan(); number++ {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if err := fn(line, number); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// Match returns the mount whose Dir is the longest prefix of the absolute,
// symlink-resolved path (Dir == path, or path under Dir+"/"). Ties (the same
// Dir mounted twice) pick the later table entry, which is the live overmount.
func Match(mounts []Mount, path string) (Mount, bool) {
	var (
		best    Mount
		bestLen = -1
		matched bool
	)
	for _, m := range mounts {
		if len(m.Dir) < bestLen || !under(path, m.Dir) {
			continue
		}
		best, bestLen, matched = m, len(m.Dir), true
	}
	return best, matched
}

// under reports whether path is dir itself or lies beneath it.
func under(path, dir string) bool {
	if dir == "" {
		return false
	}
	dir = strings.TrimSuffix(dir, "/")
	if dir == "" || dir == "/" {
		return strings.HasPrefix(path, "/")
	}
	return path == dir || strings.HasPrefix(path, dir+"/")
}

// clamp converts a statfs count to uint64, turning negative values (some BSDs
// report -1 for a full disk or an unknown inode count) into zero.
func clamp[T int32 | int64 | uint32 | uint64](v T) uint64 {
	if v < 0 {
		return 0
	}
	return uint64(v)
}
