//go:build freebsd || darwin || netbsd

package builtins

import "syscall"

func atim(st *syscall.Stat_t) syscall.Timespec { return st.Atimespec }
func mtim(st *syscall.Stat_t) syscall.Timespec { return st.Mtimespec }
func ctim(st *syscall.Stat_t) syscall.Timespec { return st.Ctimespec }
