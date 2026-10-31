//go:build !unix

package builtins

import "runtime"

func machineName() (string, error) { return machineFromGOARCH(runtime.GOARCH), nil }
