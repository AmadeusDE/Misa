//go:build !unix && !windows

package lscore

import "os"

func readMeta(path string, info os.FileInfo, lstat bool) sysMeta {
	return sysMeta{Missing: allFields}
}

// Identify reports no identity on platforms without a provider.
func Identify(path string) (DirIdent, bool) { return DirIdent{}, false }
