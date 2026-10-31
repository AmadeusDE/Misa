//go:build unix

package vi

import (
	"os"
	"syscall"
)

func mustWriteInPlace(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return !ok || stat.Nlink > 1 || stat.Uid != uint32(os.Geteuid()) || stat.Gid != uint32(os.Getegid())
}
