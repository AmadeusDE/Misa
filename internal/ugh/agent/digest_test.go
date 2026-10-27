package agent

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/config"
	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ugh/perm"
	"github.com/AmadeusDE/Misa/internal/ugh/tools"
)

// digestRun runs one bash call that prints lots of output and exits 1, with a
// digest model that answers as scripted. It returns the second request's
// tool result and the digest provider.
func digestRun(t *testing.T, digestTurns [][]llm.Event, mut func(*Config)) (result string, dp *fakeProvider, a *Agent) {
	t.Helper()
	old := resolveRoute
	t.Cleanup(func() { resolveRoute = old })
	cheapDigests(t)
	dp = &fakeProvider{turns: digestTurns}
	resolveRoute = func(string) (llm.Provider, string, error) { return dp, "cheap", nil }
	fp := &fakeProvider{turns: [][]llm.Event{
		{{Type: llm.EventToolCall, Call: &llm.ToolCall{ID: "b", Name: "bash", Args: `{"command":"seq 1 4000; echo FAILED-TEST-X >&2; exit 1"}`}}, {Type: llm.EventDone, Stop: "tool_calls"}},
		answer("ok"),
	}}
	cfg := Config{Provider: fp, Model: "m", Cwd: t.TempDir(), Trace: &strings.Builder{}, BlobDir: t.TempDir() + "/s.blobs",
		Tools: tools.NewRegistry(tools.All()...),
		Context: ContextConfig{Digest: "low", DigestTools: []string{"bash"}, DigestMinBytes: 4000,
			Jev: &fakeJev{choose: func(int, string) string { return "digest" }}}}
	if mut != nil {
		mut(&cfg)
	}
	a = New(cfg)
	if _, err := a.Run(context.Background(), "run the suite"); err != nil {
		t.Fatal(err)
	}
	for _, m := range fp.requests[1].Messages {
		if m.Role == llm.RoleTool {
			result = m.Text
		}
	}
	return result, dp, a
}

func TestFreshBigOutputBecomesDigestWithHarnessFacts(t *testing.T) {
	// The model's own claim about the exit status must not matter.
	res, dp, a := digestRun(t, [][]llm.Event{answer("Everything passed, exit 0.")}, nil)
	if !strings.Contains(res, "Everything passed, exit 0.") {
		t.Fatalf("digest text missing: %q", res)
	}
	if !strings.HasPrefix(res, "[c1 digest · exit 1 · ") || !strings.Contains(res, " raw") {
		t.Fatalf("facts must come from the harness: %q", res)
	}
	if len(dp.requests) != 1 || len(dp.requests[0].Tools) != 0 {
		t.Fatal("digest model must be called once, tool-less")
	}
	// The raw output is still recallable from the blob.
	out, err := a.recall("c1", 1, "FAILED-TEST-X")
	if err != nil || !strings.Contains(out, "FAILED-TEST-X") {
		t.Fatalf("raw output lost: %v %q", err, out)
	}
}

func TestUnusableDigestsFallBackToFull(t *testing.T) {
	cases := map[string][][]llm.Event{
		"empty":    {answer("   ")},
		"over cap": {answer(strings.Repeat("a", 9<<10))},
		"error":    nil, // no scripted turns: the call fails
	}
	for name, turns := range cases {
		res, dp, _ := digestRun(t, turns, nil)
		if strings.Contains(res, "digest") || !strings.Contains(res, "exit 1") || !strings.Contains(res, "stdout:") {
			t.Errorf("%s: expected the normal bounded result, got %.160q", name, res)
		}
		if len(dp.requests) != 1 {
			t.Errorf("%s: digest attempted %d times", name, len(dp.requests))
		}
	}
}

func TestDigestOffAndBelowThresholdChangeNothing(t *testing.T) {
	res, dp, _ := digestRun(t, [][]llm.Event{answer("x")}, func(c *Config) { c.Context.Digest = "off" })
	if strings.Contains(res, "digest") || len(dp.requests) != 0 {
		t.Fatalf("digest off but active: %.100q", res)
	}
	res, dp, _ = digestRun(t, [][]llm.Event{answer("x")}, func(c *Config) { c.Context.DigestMinBytes = 10 << 20 })
	if strings.Contains(res, "digest") || len(dp.requests) != 0 {
		t.Fatalf("below threshold but digested: %.100q", res)
	}
	// Jev says full: no digest call.
	res, dp, _ = digestRun(t, [][]llm.Event{answer("x")}, func(c *Config) {
		c.Context.Jev = &fakeJev{choose: func(int, string) string { return "full" }}
	})
	if strings.Contains(res, "digest") || len(dp.requests) != 0 {
		t.Fatalf("Jev chose full but digested: %.100q", res)
	}
}

func TestDigestInputIsScrubbedAndBounded(t *testing.T) {
	const secret = "ghp_SUPERSECRETTOKEN999"
	perm.RegisterSecret(secret)
	old := resolveRoute
	t.Cleanup(func() { resolveRoute = old })
	cheapDigests(t)
	dp := &fakeProvider{turns: [][]llm.Event{answer("summary")}}
	resolveRoute = func(string) (llm.Provider, string, error) { return dp, "cheap", nil }
	blob := t.TempDir() + "/c1.txt"
	big := "token " + secret + "\n" + strings.Repeat("filler line\n", 40000) + "THE-END-MARKER\n"
	if err := writeFile(blob, big); err != nil {
		t.Fatal(err)
	}
	a := New(Config{Provider: &fakeProvider{}, Model: "m", Cwd: t.TempDir(), Trace: &strings.Builder{},
		Context: ContextConfig{Digest: "low"}})
	a.query = "deploy with " + secret
	e := archived{Ord: 1, Tool: "bash", Args: `{"command":"echo ` + secret + `"}`, Blob: blob, Text: "exit 0 (1s)"}
	text, _, ok := a.digestChunk(context.Background(), e, "exit 0 (1s)\nstdout:\n")
	if !ok || !strings.Contains(text, "summary") {
		t.Fatalf("digest failed: %q", text)
	}
	sent := dp.requests[0].Messages[0].Text
	if strings.Contains(sent, secret) {
		t.Fatal("secret reached the digest model")
	}
	if len(sent) > digestInputCap+4<<10 || !strings.Contains(sent, "elided from the middle") || !strings.Contains(sent, "THE-END-MARKER") {
		t.Fatalf("input not head+tail bounded: %d bytes", len(sent))
	}
	// Cached: a second request for the same chunk makes no new call.
	a.digests = map[int]string{1: text}
	again, _, ok := a.digestChunk(context.Background(), e, "")
	if !ok || again != text || len(dp.requests) != 1 {
		t.Fatal("digest recomputed")
	}
}

func TestDigestFacts(t *testing.T) {
	got := digestFacts("exit 2 (0.4s)\nstdout:\nx\n[1048576 bytes of output discarded beyond the 1048576-byte capture limit]", 812<<10)
	if got != "exit 2 · 812 KiB raw · 1048576 bytes discarded" {
		t.Fatalf("facts = %q", got)
	}
	if got := digestFacts("timeout (120.0s)\nstdout:", 5<<10); got != "timeout · 5 KiB raw" {
		t.Fatalf("facts = %q", got)
	}
}

// cheapDigests makes the digest tier a single fake route, p/cheap.
func cheapDigests(t *testing.T) {
	t.Helper()
	old := digestRoutes
	digestRoutes = func(config.Config, string) []string { return []string{"p/cheap"} }
	t.Cleanup(func() { digestRoutes = old })
}

// Above the auto threshold no one is asked: the output is digested.
func TestHugeOutputIsDigestedWithoutAskingJev(t *testing.T) {
	res, dp, _ := digestRun(t, [][]llm.Event{answer("summary")}, func(c *Config) {
		c.Context.DigestAutoBytes = 8000
		c.Context.Jev = &fakeJev{choose: func(int, string) string { t.Error("Jev asked above the auto threshold"); return "full" }}
	})
	if !strings.Contains(res, "summary") || len(dp.requests) != 1 {
		t.Fatalf("not digested: %.120q", res)
	}
}

// The digest walks the tier: a failing route hands over to the next one.
func TestDigestFallsThroughTheTier(t *testing.T) {
	old := resolveRoute
	t.Cleanup(func() { resolveRoute = old })
	oldRoutes := digestRoutes
	digestRoutes = func(config.Config, string) []string { return []string{"p/broken", "q/cheap"} }
	t.Cleanup(func() { digestRoutes = oldRoutes })
	broken := &fakeProvider{} // no scripted turns: fails
	cheap := &fakeProvider{turns: [][]llm.Event{answer("from q")}}
	resolveRoute = func(ref string) (llm.Provider, string, error) {
		if strings.HasPrefix(ref, "p/") {
			return broken, "broken", nil
		}
		return cheap, "cheap", nil
	}
	a := New(Config{Provider: &fakeProvider{}, Model: "m", Cwd: t.TempDir(), Trace: &strings.Builder{}, Context: ContextConfig{Digest: "low"}})
	text, _, err := a.digestOnce(context.Background(), "summarize")
	if err != nil || text != "from q" || len(broken.requests) != 1 {
		t.Fatalf("text=%q err=%v broken requests=%d", text, err, len(broken.requests))
	}
}

// Root and orchestrators digest at half the thresholds.
func TestLeadsDigestAtLowerThresholds(t *testing.T) {
	tree := &Tree{}
	root := tree.Root("root", "m", "")
	cc := ContextConfig{Digest: "low", DigestMinBytes: 16 << 10, DigestAutoBytes: 64 << 10}
	lead := New(Config{Provider: &fakeProvider{}, Model: "m", Cwd: t.TempDir(), Tree: tree, Node: root, Context: cc})
	worker := New(Config{Provider: &fakeProvider{}, Model: "m", Cwd: t.TempDir(), Tree: tree, Node: tree.Child(root, "worker", "p/m", t.TempDir()), Context: cc})
	if lo, hi := lead.digestThresholds(); lo != 8<<10 || hi != 32<<10 {
		t.Fatalf("root thresholds %d/%d", lo, hi)
	}
	if lo, hi := worker.digestThresholds(); lo != 16<<10 || hi != 64<<10 {
		t.Fatalf("worker thresholds %d/%d", lo, hi)
	}
}

// grep and find keep no blob of their own; a digest of their output must
// still leave the full text recallable.
func TestUntrimmedDigestKeepsTheFullTextRecallable(t *testing.T) {
	cheapDigests(t)
	old := resolveRoute
	t.Cleanup(func() { resolveRoute = old })
	dp := &fakeProvider{turns: [][]llm.Event{answer("summary")}}
	resolveRoute = func(string) (llm.Provider, string, error) { return dp, "cheap", nil }
	blobs := t.TempDir() + "/s.blobs"
	a := New(Config{Provider: &fakeProvider{}, Model: "m", Cwd: t.TempDir(), Trace: &strings.Builder{}, BlobDir: blobs,
		Context: ContextConfig{Digest: "low", DigestTools: []string{"grep"}, DigestMinBytes: 100, DigestAutoBytes: 200}})
	out := strings.Repeat("src/x.go:1:match\n", 50) + "NEEDLE\n"
	got := a.maybeDigestFresh(context.Background(), llm.ToolCall{ID: "g", Name: "grep", Args: `{"pattern":"match"}`}, out)
	if !strings.Contains(got, "summary") {
		t.Fatalf("not digested: %.100q", got)
	}
	if b, err := os.ReadFile(blobs + "/c1.txt"); err != nil || string(b) != out {
		t.Fatalf("full text not kept: %v", err)
	}
}

var _ = errors.New
