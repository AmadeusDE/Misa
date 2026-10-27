package agent

import (
	"strings"
	"testing"
)

// /model rebuilds the root agent on the same tree; the root node kept the
// first model (shown as a Sol root while MiMo Flash ran) and a bare id, so
// stream limits and history adaptation read the wrong provider and model.
func TestRebuiltRootUpdatesNodeModel(t *testing.T) {
	tree := &Tree{}
	New(Config{Provider: &funcProvider{}, ProviderName: "codex", Model: "gpt-6.1-sol", Tree: tree, Cwd: t.TempDir(), Trace: &strings.Builder{}})
	a := New(Config{Provider: &funcProvider{}, ProviderName: "openrouter", Model: "xiaomi/mimo-v2.6-flash", Tree: tree, Cwd: t.TempDir(), Trace: &strings.Builder{}})
	if got := a.Node.Model; got != "openrouter/xiaomi/mimo-v2.6-flash" {
		t.Fatalf("root node model = %q", got)
	}
}
