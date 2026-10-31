//go:build !unix

package vi

import "os"

func mustWriteInPlace(info os.FileInfo) bool { return true }
