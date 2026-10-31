//go:build !unix

package builtins

// defaultPSOwnTTY has no device number to report on this platform, so the
// default selection matches processes without a controlling terminal.
func defaultPSOwnTTY() int { return 0 }
