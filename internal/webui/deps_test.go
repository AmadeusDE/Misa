package webui

import (
	"os/exec"
	"strings"
	"testing"
)

func TestWebuiDoesNotImportFrontends(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Skipf("go list: %v", err)
	}
	for _, dep := range strings.Fields(string(out)) {
		for _, banned := range []string{"/internal/ui", "/internal/gui", "/internal/ugh", "/internal/ughbot"} {
			if strings.HasSuffix(dep, banned) || strings.Contains(dep, banned+"/") {
				t.Errorf("webui depends on %s", dep)
			}
		}
	}
}
