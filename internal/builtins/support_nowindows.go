//go:build !windows

package builtins

// platformLimits is empty: Unix implements every option of the filesystem
// builtins.
var platformLimits []Limit
