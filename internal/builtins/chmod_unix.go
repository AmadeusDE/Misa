//go:build unix

package builtins

import "os"

// chmodResult is the mode chmod reports after changing path: Unix stores what
// was asked.
func chmodResult(path string, requested os.FileMode) os.FileMode { return requested }
