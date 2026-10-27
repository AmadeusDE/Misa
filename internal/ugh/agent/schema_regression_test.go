package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/tools"
)

// worktree_status has no required parameters and its schema said
// "required": null, which Codex rejects for the whole request ("null is not
// of type array"). No root tool schema may contain a null.
func TestRootToolSchemasHaveNoNulls(t *testing.T) {
	r := NewRuntime()
	a := New(Config{Provider: &funcProvider{}, Model: "m", Cwd: t.TempDir(), Tree: r.Tree, Spawn: r.Spawn,
		StartChild: r.StartChild, Resume: r.ResumeChild, Tools: tools.NewRegistry(tools.All()...)})
	specs := a.cfg.Tools.Specs()
	if len(specs) == 0 {
		t.Fatal("no tools")
	}
	seen := false
	for _, s := range specs {
		b, err := json.Marshal(s.Parameters)
		if err != nil {
			t.Fatalf("%s: %v", s.Name, err)
		}
		if strings.Contains(string(b), "null") {
			t.Errorf("%s schema contains null: %s", s.Name, b)
		}
		seen = seen || s.Name == "worktree_status"
	}
	if !seen {
		t.Fatal("worktree_status not offered to the root")
	}
}
