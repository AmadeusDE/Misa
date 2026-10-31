//go:build !unix

package walk

import "io/fs"

type fileID struct{}

// idOf has no inode to compare here; Walk caps the depth instead.
func idOf(fs.FileInfo) (fileID, bool) { return fileID{}, false }

// Device IDs are unavailable on this platform: SameDevice cannot restrict descent.
func sameDevice(a, b fileID) bool { return true }
