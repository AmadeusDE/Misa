//go:build linux || openbsd

package builtins

import "syscall"

func atim(st *syscall.Stat_t) syscall.Timespec { return st.Atim }
func mtim(st *syscall.Stat_t) syscall.Timespec { return st.Mtim }
func ctim(st *syscall.Stat_t) syscall.Timespec { return st.Ctim }
