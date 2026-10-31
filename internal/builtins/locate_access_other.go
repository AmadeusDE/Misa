// Lookup-time access checks for locate on platforms without access(2).
//
//go:build !unix

package builtins

// locateSearchable cannot ask the operating system on this platform, so it
// does not claim a check it cannot make; locate keeps the plain existence
// check in that case (see docs/plans/locate.md).
func locateSearchable(dir string) bool { return true }
