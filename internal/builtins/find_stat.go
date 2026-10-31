// File metadata used by find. Inspired by Toybox find.c, copyright 2014 Rob
// Landley, at 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb (0BSD).
package builtins

import (
	"io/fs"
	"syscall"
	"time"
)

func findStat(info fs.FileInfo) *syscall.Stat_t { st, _ := info.Sys().(*syscall.Stat_t); return st }
func findTime(info fs.FileInfo, kind byte) time.Time {
	st := findStat(info)
	if st == nil || kind == 'm' {
		return info.ModTime()
	}
	ts := atim(st)
	if kind == 'c' {
		ts = ctim(st)
	}
	return time.Unix(ts.Sec, int64(ts.Nsec))
}
func findMode(info fs.FileInfo) uint32 {
	m := uint32(info.Mode().Perm())
	if info.Mode()&fs.ModeSetuid != 0 {
		m |= 04000
	}
	if info.Mode()&fs.ModeSetgid != 0 {
		m |= 02000
	}
	if info.Mode()&fs.ModeSticky != 0 {
		m |= 01000
	}
	return m
}
func findType(info fs.FileInfo) byte {
	m := info.Mode()
	switch {
	case m.IsRegular():
		return 'f'
	case m.IsDir():
		return 'd'
	case m&fs.ModeSymlink != 0:
		return 'l'
	case m&fs.ModeNamedPipe != 0:
		return 'p'
	case m&fs.ModeSocket != 0:
		return 's'
	case m&fs.ModeDevice != 0 && m&fs.ModeCharDevice != 0:
		return 'c'
	case m&fs.ModeDevice != 0:
		return 'b'
	}
	return '?'
}

func findModeString(info fs.FileInfo) string {
	m := findMode(info)
	b := []byte("----------")
	b[0] = findType(info)
	if b[0] == 'f' {
		b[0] = '-'
	}
	const rwx = "rwxrwxrwx"
	for i := 0; i < 9; i++ {
		if m&(1<<uint(8-i)) != 0 {
			b[i+1] = rwx[i]
		}
	}
	for _, x := range []struct {
		bit          uint32
		at           int
		lower, upper byte
	}{{04000, 3, 's', 'S'}, {02000, 6, 's', 'S'}, {01000, 9, 't', 'T'}} {
		if m&x.bit != 0 {
			if b[x.at] == 'x' {
				b[x.at] = x.lower
			} else {
				b[x.at] = x.upper
			}
		}
	}
	return string(b)
}
