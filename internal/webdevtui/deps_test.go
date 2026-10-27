package webdevtui

import (
	"os/exec"
	"strings"
	"testing"
)

func TestDependsOnNoMisaPackage(t *testing.T) {
	mod, err := exec.Command("go", "list", "-m").Output()
	if err != nil {
		t.Skipf("go list: %v", err)
	}
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Skipf("go list: %v", err)
	}
	prefix := strings.TrimSpace(string(mod)) + "/"
	for _, dep := range strings.Fields(string(out)) {
		if strings.HasPrefix(dep, prefix) && !strings.HasSuffix(dep, "/internal/webdevtui") {
			t.Errorf("webdevtui depends on %s", dep)
		}
	}
}
