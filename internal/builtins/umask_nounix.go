//go:build !unix

package builtins

// currentUmask is zero: there is no umask outside Unix, so a symbolic mode
// without a "who" applies to everyone.
func currentUmask() int { return 0 }
