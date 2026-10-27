//go:build !unix

package privdir

import "os"

// Ownership needs ACLs off Unix (docs/plans/windows.md, UD2).
func ownedByMe(os.FileInfo) bool { return true }
