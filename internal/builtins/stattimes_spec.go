//go:build freebsd || darwin || netbsd

package builtins

import "syscall"

func rawAtim(st *syscall.Stat_t) syscall.Timespec { return st.Atimespec }
func rawMtim(st *syscall.Stat_t) syscall.Timespec { return st.Mtimespec }
func rawCtim(st *syscall.Stat_t) syscall.Timespec { return st.Ctimespec }
