//go:build !unix

package imgview

import "os"

func openImageCandidate(path string) (*os.File, error) { return os.Open(path) }
