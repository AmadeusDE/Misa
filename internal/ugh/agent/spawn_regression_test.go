package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ugh/session"
	"github.com/AmadeusDE/Misa/internal/ugh/tools"
	"github.com/AmadeusDE/Misa/internal/ugh/worktree"
)

func TestSpawnRejectsIncompleteBatchBeforeStarting(t *testing.T) {
	a := New(Config{Cwd: t.TempDir(), Spawn: func(context.Context, *Agent, []Task) string { t.Fatal("invalid batch launched"); return "" }})
	for _, brief := range []string{"", " \n\t", "fix", "a          b", "placeholder", "PLACEHOLDER", "TODO", "TBD", "...", "<task>", "lorem ipsum dolor sit amet", "Implement the utility.\n=== YOUR SECTION ===\n", "Implement the utility.\n## Acceptance checks", "Implement the utility.\nRequirements:", "Implement the utility.\nRequirements\n===", "Implement the utility.\n###"} {
		t.Run(fmt.Sprintf("%q", brief), func(t *testing.T) {
			raw, _ := json.Marshal(map[string]any{"tasks": []Task{{Task: "Implement the first utility."}, {Task: brief}}})
			if _, err := (spawnTool{agent: a}).Run(context.Background(), "", raw); err == nil || !strings.Contains(err.Error(), "task 2:") {
				t.Fatalf("error = %v", err)
			}
		})
	}
	for _, brief := range []string{"Implement the utility.\n=== YOUR SECTION ===\nWrite tests.", "Read foo.go and report findings.", "Task: implement the diff utility.",
		"Review this snippet and report.\n    if ready:", "Review this snippet and report.\n\tcase 1:", "Check the helper that computes sums.\nfunc total() int:",
		"Do the following and report each outcome in detail, including failures, then summarize everything you saw:"} {
		if err := validateTask(Task{Task: brief}); err != nil {
			t.Fatalf("valid brief: %v", err)
		}
	}
	for _, path := range []string{"", "../escape", "/absolute"} {
		if err := validateTask(Task{Task: "Implement the utility.", Outputs: []string{path}}); err == nil {
			t.Errorf("accepted output %q", path)
		}
	}
}

func TestSpawnNumbersAreConsecutiveAndMatchIDs(t *testing.T) {
	r := NewRuntime()
	r.runChild = func(ctx context.Context, a *Agent, task Task) ChildResult {
		n := r.Tree.childNumber(a.Node, "worker", "m", a.cfg.Cwd, task.number)
		return ChildResult{ID: n.ID, Status: "ok"}
	}
	a := New(Config{Cwd: t.TempDir(), Tree: r.Tree, Runtime: r, Spawn: r.Spawn, StartChild: r.StartChild})
	for batch := 0; batch < 2; batch++ {
		raw := json.RawMessage(`{"tasks":[{"task":"Implement utility one."},{"task":"Implement utility two."},{"task":"Implement utility three."}]}`)
		out, err := (spawnTool{agent: a}).Run(context.Background(), "", raw)
		if err != nil {
			t.Fatal(err)
		}
		kids := a.kidsSnapshot()
		for i, k := range kids {
			number := batch*3 + i + 1
			if !strings.Contains(out, fmt.Sprintf("- task %d:", number)) {
				t.Fatalf("labels: %s", out)
			}
			<-k.done
			if k.res.ID != fmt.Sprintf("root/worker-%d", number) {
				t.Fatalf("task %d id=%s", number, k.res.ID)
			}
		}
		a.collectFinished()
	}
}

func TestSpawnNeedsParentGrantAndRespectsConfiguredDepth(t *testing.T) {
	for _, role := range []string{"worker", "scout", "reviewer", "orchestrator"} {
		for _, grant := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/grant=%v", role, grant), func(t *testing.T) {
				r := NewRuntime()
				root := r.Tree.Root("root", "m", "")
				n := r.Tree.Child(root, role, "m", "")
				a := New(Config{Node: n, Tree: r.Tree, Runtime: r, AllowSpawn: grant, Spawn: r.Spawn, Tools: tools.NewRegistry()})
				want := grant || role == "orchestrator"
				if _, ok := a.cfg.Tools.Get("spawn"); ok != want {
					t.Fatalf("spawn offered=%v want %v", ok, want)
				}
				if !want {
					_, err := (spawnTool{agent: a}).Run(context.Background(), "", json.RawMessage(`{"tasks":[{"task":"Read the source files."}]}`))
					if err == nil || !strings.Contains(err.Error(), "delegation not allowed") {
						t.Fatalf("direct tool call: %v", err)
					}
					if res := r.startChild(context.Background(), a, Task{Task: "Read the source files."}); res.Status != "error" {
						t.Fatalf("runtime bypassed grant: %+v", res)
					}
				}
				r.Config.Agent.MaxSpawnDepth = 1
				if a.canSpawn() {
					t.Fatal("grant bypassed configured depth")
				}
			})
		}
	}
	r := NewRuntime()
	r.Config.Agent.MaxSpawnDepth = 0
	a := New(Config{Tree: r.Tree, Runtime: r, Spawn: r.Spawn})
	if _, ok := a.cfg.Tools.Get("spawn"); ok {
		t.Fatal("depth zero should disable root spawn")
	}
}

func TestExpectedOutputsMakeFalseCompletionAnErrorAndSurviveResume(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	r := NewRuntime()
	cwd := t.TempDir()
	parent := New(Config{Tree: r.Tree, Cwd: cwd})
	n := r.Tree.Child(parent.Node, "worker", "m", cwd)
	store, err := session.Create(cwd, session.Record{Run: r.RunID, Agent: n.ID, Parent: parent.ID})
	if err != nil {
		t.Fatal(err)
	}
	sw := &sessWriter{s: store}
	child := New(Config{Tree: r.Tree, Node: n, Cwd: cwd, Provider: &funcProvider{script: func(int, llm.Request) []llm.Event { return []llm.Event{say("Everything is done.")} }}, Outputs: []string{"tests/diffutils/diff/result.txt"}})
	res := r.execChild(context.Background(), parent, child, sw, "Implement the diff utility.")
	if res.Status != "error" || !strings.Contains(res.Report, "Incomplete: expected outputs") || !strings.Contains(res.Report, "tests/diffutils/diff/result.txt") || n.status() != "error" {
		t.Fatalf("false completion: %+v status=%s", res, n.status())
	}
	data, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range strings.Split(string(data), "\n") {
		var rec session.Record
		if json.Unmarshal([]byte(line), &rec) == nil && rec.Type == "result" {
			found = true
			if rec.Status != "error" || !strings.Contains(rec.Error, "expected outputs") {
				t.Fatalf("persisted result: %+v", rec)
			}
		}
	}
	if !found {
		t.Fatal("missing result record")
	}
	path := filepath.Join(cwd, "tests/diffutils/diff/result.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("diff output"), 0600); err != nil {
		t.Fatal(err)
	}
	res = r.ResumeChild(context.Background(), parent, n.ID, "Finish and check the output.", 5)
	if res.Status != "ok" {
		t.Fatalf("existing output: %+v", res)
	}
	// A turn-out is not a completion claim, so missing outputs do not mask it.
	turnedOut := ChildResult{Status: "turn-out", Cwd: cwd}
	verifyOutputs(&turnedOut, []string{"missing"})
	if turnedOut.Status != "turn-out" {
		t.Fatal("output check masked turn-out")
	}
}

func TestRootCanRequestManagedWriterAndDirtySnapshot(t *testing.T) {
	for _, snapshot := range []bool{false, true} {
		t.Run(fmt.Sprintf("snapshot=%v", snapshot), func(t *testing.T) {
			r, _, repo := writerEnv(t)
			root := New(Config{Tree: r.Tree, Node: r.Tree.root, Cwd: repo, Runtime: r, Spawn: r.Spawn})
			head := gitIn(t, repo, "rev-parse", "HEAD")
			res := r.startChild(context.Background(), root, Task{Task: "Implement the feature.", Role: "worker", Worktree: true, Snapshot: snapshot, AllowSpawn: true, Outputs: []string{"feature.txt"}})
			wt := r.writer(res.ID)
			if res.Status != "ok" || wt == nil || res.Cwd == repo {
				t.Fatalf("root writer: %+v wt=%v", res, wt)
			}
			if wt.Meta.BaseCommit != head || !strings.Contains(res.Report, head) {
				t.Fatalf("base not disclosed: %+v", res)
			}
			if _, ok := r.parkedKids[res.ID].agent.cfg.Tools.Get("spawn"); ok {
				t.Fatal("a feature writer must not spawn into its own worktree, even with allow_spawn")
			}
			_, err := os.Stat(filepath.Join(res.Cwd, "parent-dirty.txt"))
			if snapshot && err != nil {
				t.Fatal("dirty parent file missing from snapshot")
			}
			if !snapshot && !os.IsNotExist(err) {
				t.Fatalf("dirty state leaked: %v", err)
			}
			if snapshot && (wt.Meta.DirtyBasePolicy != worktree.PolicySnapshotAuthorized || !strings.Contains(res.Report, "dirty files carried")) {
				t.Fatalf("snapshot not disclosed: %+v", res)
			}
			status, err := (worktreeStatusTool{agent: root}).Run(context.Background(), "", nil)
			if err != nil || !strings.Contains(status, head) || !strings.Contains(status, "parent-dirty.txt") {
				t.Fatalf("status=%s err=%v", status, err)
			}
		})
	}
}

func TestMissingManagedOutputMarksWriterFailed(t *testing.T) {
	r, orch, repo := writerEnv(t)
	// A parent-side file must not satisfy an output expected in the child's cwd.
	if err := os.WriteFile(filepath.Join(repo, "expected.txt"), []byte("parent only"), 0600); err != nil {
		t.Fatal(err)
	}
	res := r.startChild(context.Background(), orch, Task{Task: "Implement the expected feature.", Role: "worker", Outputs: []string{"expected.txt"}})
	wt := r.writer(res.ID)
	if res.Status != "error" || wt == nil || wt.Meta.Lifecycle != worktree.LifecycleFailed || !strings.Contains(res.Report, "expected.txt") {
		t.Fatalf("false managed completion: %+v worktree=%v", res, wt)
	}
}

func TestAllowSpawnNeverLetsFeatureWritersDelegate(t *testing.T) {
	r, orch, _ := writerEnv(t)
	res := r.startChild(context.Background(), orch, Task{Task: "Implement the feature.", Role: "worker", AllowSpawn: true})
	writer := r.parkedKids[res.ID].agent
	if writer.canSpawn() {
		t.Fatal("orchestrator-spawned writer delegates despite allow_spawn")
	}
	if _, ok := writer.cfg.Tools.Get("spawn"); ok {
		t.Fatal("spawn offered to a feature writer")
	}
	// The grant alone is also refused from a directly constructed writer node.
	n := r.Tree.Child(orch.Node, "worker", "m", "")
	direct := New(Config{Node: n, Tree: r.Tree, Runtime: r, AllowSpawn: true, Spawn: r.Spawn, Tools: tools.NewRegistry()})
	if direct.canSpawn() {
		t.Fatal("worker under an orchestrator may not spawn")
	}
}
