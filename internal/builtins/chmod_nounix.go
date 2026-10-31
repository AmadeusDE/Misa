//go:build !unix

package builtins

import "os"

// chmodResult reports the mode path really has afterwards. Go on Windows only
// maps the owner write bit to the read-only attribute, so a request such as
// 0755 ends as 0666 (0777 for a directory) and 0444 is what -w leaves.
func chmodResult(path string, requested os.FileMode) os.FileMode {
	info, err := os.Stat(path)
	if err != nil {
		return requested
	}
	return requested&^os.ModePerm | info.Mode().Perm()
}
