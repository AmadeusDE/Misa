//go:build unix

package privdir

import (
	"os"
	"strconv"
	"syscall"
)

func ownedByMe(fi os.FileInfo) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == os.Getuid()
}

// UserTag names the current user in shared directory names (the uid).
func UserTag() string { return strconv.Itoa(os.Getuid()) }
