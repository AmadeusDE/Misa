// Lookup-time access checks for locate. Unix asks the kernel with access(2) so
// ACLs, groups and capabilities are honoured exactly as they are for the
// caller's own open(2) calls.
//
//go:build unix

package builtins

import "syscall"

// locateSearchable reports whether this user may search dir, i.e. whether a
// path below it can be resolved to a name.
func locateSearchable(dir string) bool {
	const xOK = 1 // access(2)'s X_OK; the syscall package does not export it
	return syscall.Access(dir, xOK) == nil
}
