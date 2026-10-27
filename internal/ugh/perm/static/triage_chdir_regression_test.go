package static

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/perm"
)

// A relative path is judged from every directory the command line may have
// moved to, not only from where it started.
func TestTriageFollowsDirectoryChanges(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, ".ssh"), 0o700)
	os.WriteFile(filepath.Join(home, ".ssh", "id_ed25519"), []byte("x"), 0o600)
	repo := filepath.Join(home, "src", "r")
	os.MkdirAll(repo, 0o755)
	fromRoot, _ := filepath.Rel("/", home)
	bash := func(cmd string) perm.Call {
		return perm.NewCall(repo, "bash", json.RawMessage(mustJSON(map[string]string{"command": cmd})))
	}
	for _, c := range []struct{ cmd, want string }{
		{"env --chdir=" + home + "/.ssh cat id_ed25519", High},
		{"env -C ~/.ssh cat id_ed25519", High},
		{"env -C" + home + "/.ssh cat id_ed25519", High},
		{"sudo -D ~/.ssh cat id_ed25519", High},
		{"cd / && cat " + fromRoot + "/.ssh/id_ed25519", High},
		{"cd .. && cd .. && cat .ssh/id_ed25519", High},
		{"cd /; cat < " + fromRoot + "/.ssh/id_ed25519", High},
		{"cat < ~/.ssh/id_ed25519", High},
		{"cd / && echo x > " + fromRoot + "/.bashrc", High},
		{`cd "$X" && cat id_ed25519`, Low},
		{"cd - && cat id_ed25519", Low},
		{"popd && cat id_ed25519", Low},
		{`env --chdir="$X" cat id_ed25519`, Low},
		{"cd sub && cat main.go", Trivial},
		{"cd .. && ls", Trivial},
		{"env -C sub cat main.go", Trivial},
	} {
		if got, why := Triage(bash(c.cmd)); got != c.want {
			t.Errorf("%s → %s (%s), want %s", c.cmd, got, why, c.want)
		}
	}
}

func TestGitBoundarySeesWrapperDirectoryChanges(t *testing.T) {
	owned := "/work/lane"
	for _, cmd := range []string{"env -C /work/other git commit -m x", "env --chdir=/work/other git status", `env -C "$X" git status`} {
		if v := GitOutsideOwnerViolations(cmd, owned); len(v) == 0 {
			t.Errorf("%s: no violation", cmd)
		}
	}
	if v := GitOutsideOwnerViolations("env -C sub git status", owned); len(v) != 0 {
		t.Errorf("env -C sub: %v", v)
	}
}
