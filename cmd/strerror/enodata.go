//go:build linux || darwin

package main

import "syscall"

func init() { names["ENODATA"] = syscall.ENODATA }
