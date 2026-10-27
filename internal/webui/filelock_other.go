//go:build !unix

package webui

// lockFile is a no-op where flock is unavailable; one process per store.
func lockFile(string) (func(), error) { return func() {}, nil }
