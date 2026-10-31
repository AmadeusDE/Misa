//go:build windows

package policy

// syncDir is a no-op: Windows cannot fsync a directory handle, and os.Rename
// replaces through MoveFileEx after the file's own data was synced.
func syncDir(string) error { return nil }
