package term

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// testBin finds name under the usual Unix and Android roots, then PATH.
func testBin(t *testing.T, name string) string {
	t.Helper()
	for _, dir := range []string{"/bin", "/usr/bin", "/system/bin"} {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	p, err := exec.LookPath(name)
	if err != nil {
		t.Skip(name + " not available")
	}
	return p
}
