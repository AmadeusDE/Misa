//go:build !unix

package tarx

import "io/fs"

type fileID struct{}

func idOf(fs.FileInfo) (fileID, bool) { return fileID{}, false }
