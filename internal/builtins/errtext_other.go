//go:build !windows

package builtins

func platformErrText(error) (string, bool) { return "", false }
