//go:build !unix && !windows

package privdir

import "os"

// Ownership is not checked on other platforms.
func ownedByMe(os.FileInfo) bool { return true }

// UserTag names the current user in shared directory names.
func UserTag() string { return "user" }
