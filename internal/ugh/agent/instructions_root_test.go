package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootInstructionsStayInsideTheCwd(t *testing.T) {
	cwd := t.TempDir()
	secret := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(secret, []byte("PRIVATE KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(cwd, "AGENTS.md")); err != nil {
		t.Skip(err)
	}
	if got := rootInstructions(cwd); strings.Contains(got, "PRIVATE KEY") {
		t.Fatalf("AGENTS.md linking outside the cwd was read: %q", got)
	}

	// A link that stays inside (AGENTS.md -> docs/AGENTS.md) still works.
	inside := t.TempDir()
	os.MkdirAll(filepath.Join(inside, "docs"), 0o755)
	os.WriteFile(filepath.Join(inside, "docs", "AGENTS.md"), []byte("use tabs"), 0o644)
	if err := os.Symlink(filepath.Join("docs", "AGENTS.md"), filepath.Join(inside, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	if got := rootInstructions(inside); !strings.Contains(got, "use tabs") {
		t.Fatalf("in-tree link not followed: %q", got)
	}
}
