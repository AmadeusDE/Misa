//go:build windows

package builtins

func machineName() (string, error) { return nativeMachine(), nil }
