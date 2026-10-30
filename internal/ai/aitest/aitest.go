// Package aitest isolates tests of code that uses internal/ai from the real user's ugh and Misa configuration.
package aitest

import (
	"os"
	"strings"
	"testing"
)

// Isolate points HOME and the XDG directories at fresh temp directories, clears every *_API_KEY variable so no
// ambient key or Console login is found, and returns the config home (ugh's files go in <dir>/ugh, Misa's in
// <dir>/misa). The OS keyring is never used under `go test`.
func Isolate(t testing.TB) string {
	t.Helper()
	home, cfg, state := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv("XDG_DATA_HOME", home+"/data")
	t.Setenv("XDG_CACHE_HOME", home+"/cache")
	for _, kv := range os.Environ() {
		if name, _, _ := strings.Cut(kv, "="); strings.HasSuffix(name, "_API_KEY") {
			t.Setenv(name, "")
		}
	}
	return cfg
}
