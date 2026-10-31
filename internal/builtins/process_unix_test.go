//go:build unix

package builtins

import (
	"strings"
	"testing"
)

func TestKillListsTheUnixSignals(t *testing.T) {
	r := runBuiltin(t, "kill", "-l")
	assertExitCode(t, r, 0)
	for _, name := range []string{"HUP", "KILL", "USR1", "TERM", "STOP", "RTMAX"} {
		if !strings.Contains(r.stdout, name) {
			t.Errorf("kill -l lacks %s: %q", name, r.stdout)
		}
	}
}

func TestSignalByNameKnowsTheUnixSignals(t *testing.T) {
	for _, name := range []string{"HUP", "SIGUSR1", "STOP", "CONT", "WINCH"} {
		if _, ok := signalByName(name); !ok {
			t.Errorf("signalByName(%q) failed", name)
		}
	}
}
