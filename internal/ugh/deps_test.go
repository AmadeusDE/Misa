package ugh_test

import (
	"os/exec"
	"strings"
	"testing"
)

const module = "github.com/AmadeusDE/Misa/"

func depsOf(t *testing.T, pattern string) []string {
	t.Helper()
	out, err := exec.Command("go", "list", "-deps", "-test=false", pattern).Output()
	if err != nil {
		t.Fatalf("go list %s: %v", pattern, err)
	}
	return strings.Fields(string(out))
}

func TestDependencyDirection(t *testing.T) {
	forbidden := map[string][]string{
		module + "internal/ugh/...":   {"internal/ughbot", "internal/ui", "internal/webui"},
		module + "internal/term":      {"internal/ugh/", "internal/ughbot", "internal/ui"},
		module + "internal/sshclient": {"internal/ugh/", "internal/ughbot", "internal/ui"},
		module + "internal/builtins":  {"internal/ugh/", "internal/ughbot"},
		module + "internal/webui":     {"internal/ui", "internal/gui", "internal/ugh", "internal/ughbot"},
		module + "internal/termkeys":  {"internal/ui", "internal/gui", "internal/ugh", "internal/ughbot"},
		module + "internal/filelock":  {"internal/ugh", "internal/ughbot", "internal/webui", "internal/ui", "internal/gui"},
		module + "internal/lscore":    {"internal/ugh/", "internal/ughbot"},
	}
	for pattern, banned := range forbidden {
		for _, dep := range depsOf(t, pattern) {
			for _, b := range banned {
				if strings.HasPrefix(dep, module+b) {
					t.Errorf("%s must not depend on %s", pattern, dep)
				}
			}
		}
	}
}

func TestMisaDoesNotLinkTheBotStore(t *testing.T) {
	banned := []string{"modernc.org/sqlite", module + "internal/ughbot/store"}
	for _, pattern := range []string{module[:len(module)-1], module + "internal/ui"} {
		for _, dep := range depsOf(t, pattern) {
			for _, b := range banned {
				if strings.HasPrefix(dep, b) {
					t.Errorf("%s must not depend on %s", pattern, dep)
				}
			}
		}
	}
}
