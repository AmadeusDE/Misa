//go:build linux || openbsd

package builtins

import "syscall"

func rawAtim(st *syscall.Stat_t) syscall.Timespec { return st.Atim }
func rawMtim(st *syscall.Stat_t) syscall.Timespec { return st.Mtim }
func rawCtim(st *syscall.Stat_t) syscall.Timespec { return st.Ctim }
