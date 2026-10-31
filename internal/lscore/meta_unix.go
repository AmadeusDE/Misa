//go:build unix

package lscore

import (
	"os"
	"os/user"
	"strconv"
	"syscall"
)

// readMeta reads the Unix stat fields behind info. path and lstat are only
// needed by providers that must reopen the file.
func readMeta(path string, info os.FileInfo, lstat bool) sysMeta {
	sys, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return sysMeta{Missing: allFields}
	}
	m := sysMeta{
		UID:     sys.Uid,
		GID:     sys.Gid,
		Inode:   sys.Ino,
		Dev:     uint64(sys.Dev),
		Nlink:   uint64(sys.Nlink),
		Blocks:  sys.Blocks,
		Missing: FieldBirthTime,
	}
	m.Access, m.Change = statTimes(sys)
	if u, err := user.LookupId(strconv.Itoa(int(sys.Uid))); err == nil {
		m.Owner = u.Username
	} else {
		m.Owner = strconv.Itoa(int(sys.Uid))
	}
	if g, err := user.LookupGroupId(strconv.Itoa(int(sys.Gid))); err == nil {
		m.Group = g.Name
	} else {
		m.Group = strconv.Itoa(int(sys.Gid))
	}
	return m
}

// Identify returns the (dev, inode) pair of path, dereferencing symlinks.
func Identify(path string) (DirIdent, bool) {
	st, err := os.Stat(path)
	if err != nil {
		return DirIdent{}, false
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return DirIdent{}, false
	}
	return DirIdent{Dev: uint64(sys.Dev), Ino: sys.Ino}, true
}
