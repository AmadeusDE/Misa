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
	"github.com/AmadeusDE/Misa/internal/ugh/tools"
	"github.com/AmadeusDE/Misa/internal/ugh/worktree"
)

func criticalBash(command string) llm.ToolCall {
	raw, _ := json.Marshal(map[string]string{"command": command})
	return llm.ToolCall{Name: "bash", Args: string(raw)}
}

func TestCriticalStateSurvivesRepeatedCompactionAndReload(t *testing.T) {
	cwd, scratch := t.TempDir(), t.TempDir()
	task := "Implement cmp.\nHARD RULES: work ONLY here.\n" + strings.Repeat("old task detail\n", 800) +
		"MUST preserve this middle constraint\nNEVER invent a directory\nDO NOT change go.mod\n" +
		strings.Repeat("more detail\n", 800) + "Final task requirement."
	history := []llm.Message{
		{Role: llm.RoleUser, Text: task},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{criticalBash(`git worktree add -b cmp "$UGH_SCRATCH/cmp"; cd "$UGH_SCRATCH/cmp"`)}},
	}
	for range 20 {
		history = append(history, llm.Message{Role: llm.RoleAssistant, Text: strings.Repeat("irrelevant history ", 1000)})
	}
	a := New(Config{Provider: &growProvider{}, Model: "m", Cwd: cwd, Scratch: scratch, History: history})
	check := func(a *Agent) {
		t.Helper()
		text := a.hist[0].Text
		for _, want := range []string{criticalStart, cwd, scratch, filepath.Join(scratch, "cmp"), `"Branch":"cmp"`,
			"Implement cmp.", "HARD RULES: work ONLY here.", "MUST preserve this middle constraint",
			"NEVER invent a directory", "DO NOT change go.mod", "Final task requirement."} {
			if !strings.Contains(text, want) {
				t.Errorf("compaction lost %q", want)
			}
		}
	}
	if !a.compactFor(context.Background(), "m", 100000, 200000, nil) {
		t.Fatal("did not compact long history")
	}
	check(a)
	// A new harness instance must recover pins from the session seed, not from
	// the lossy summary (the fake summarizer mentions no paths or constraints).
	b := New(Config{Provider: &growProvider{}, Model: "m", Cwd: cwd, Scratch: scratch, History: a.History()})
	b.pinTask("Resume task: MUST keep cmp's branch.")
	for range 20 {
		b.push(llm.Message{Role: llm.RoleAssistant, Text: "later work"})
	}
	if !b.compactFor(context.Background(), "m", 100000, 200000, nil) {
		t.Fatal("did not compact again")
	}
	check(b)
	if !strings.Contains(b.hist[0].Text, "Resume task: MUST keep cmp's branch.") {
		t.Fatal("lost later task")
	}
	// Manual compaction uses exactly the same pins, even with an empty seed.
	b.ApplyCompaction(nil, llm.Usage{})
	check(b)
	b.pinTask("Newest instruction: NEVER discard earlier tasks.")
	b.ApplyCompaction(b.History(), llm.Usage{})
	check(b)
	if strings.Count(b.hist[0].Text, criticalStart) != 1 {
		t.Fatal("reapplying a manual seed duplicated pinned state")
	}
	c := New(Config{Cwd: cwd, Scratch: scratch, History: b.History()})
	if !strings.Contains(c.critical.Tasks, "Newest instruction: NEVER discard earlier tasks.") {
		t.Fatal("manual seed reload recovered stale critical state")
	}
}

func TestCriticalStatePinsRunResumeAndSendInstructions(t *testing.T) {
	a := New(Config{Provider: &growProvider{answerAt: 1}, Cwd: t.TempDir()})
	if _, err := a.Run(context.Background(), "Original assignment: NEVER leave cwd."); err != nil {
		t.Fatal(err)
	}
	a.Steer("Sent instruction: DO NOT touch generated files.")
	if _, err := a.Run(context.Background(), "Resume assignment: MUST add tests."); err != nil {
		t.Fatal(err)
	}
	a.ApplyCompaction([]llm.Message{{Role: llm.RoleUser, Text: "A lossy summary"}}, llm.Usage{})
	for _, want := range []string{"Original assignment: NEVER leave cwd.", "Sent instruction: DO NOT touch generated files.", "Resume assignment: MUST add tests."} {
		if !strings.Contains(a.hist[0].Text, want) {
			t.Errorf("lost %q", want)
		}
	}
}

func TestCriticalStateObservesWorktreeAndCDPaths(t *testing.T) {
	a := New(Config{Cwd: "/repo", Scratch: "/scratch/private"})
	for _, cmd := range []string{
		`cd /repo/sub && git worktree add "../../feature one" -b feature-one`,
		`git -C /other worktree add -Bfeature-two ../feature-two HEAD`,
		`git worktree add --detach /detached HEAD`,
		`git worktree add /automatic`,
		`cd "$UGH_SCRATCH/cmp"`,
		`git worktree add "$(echo /invented)"`,
		`(cd /subshell); git worktree add ../after-subshell`,
		`unused() { git worktree add /not-created; }`,
	} {
		a.noteCriticalCall(criticalBash(cmd))
	}
	want := []criticalWorktree{{"/feature one", "feature-one"}, {"/feature-two", "feature-two"}, {"/detached", "(detached)"}, {"/automatic", "automatic"}, {"/after-subshell", "after-subshell"}}
	got := a.critical.Paths.Worktrees
	if len(got) != len(want) {
		t.Fatalf("worktrees = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("worktree %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if !containsString(a.critical.Paths.Directories, "/scratch/private/cmp") {
		t.Fatal("lost cd path")
	}
	if a.cfg.Cwd != "/repo" {
		t.Fatal("shell cd changed the agent cwd")
	}
	// Managed writers need no git tool call to retain their path and branch.
	rt := NewRuntime()
	rt.writers = map[string]*worktree.Worktree{"root": {Dir: "/repo", Meta: worktree.Metadata{Branch: "managed-branch"}}}
	a.rt = rt
	if block := a.criticalBlock(); !strings.Contains(block, `"Path":"/repo","Branch":"managed-branch"`) {
		t.Fatalf("managed worktree missing: %s", block)
	}
}

type criticalMutationTool struct{ name string }

func (t criticalMutationTool) Spec() llm.ToolSpec { return llm.ToolSpec{Name: t.name} }
func (criticalMutationTool) Run(context.Context, string, json.RawMessage) (string, error) {
	return "mutation result", nil // never touch the external test paths
}

func TestMutationOutsideRootsGetsToolResultWarning(t *testing.T) {
	a := New(Config{Cwd: "/repo", Scratch: "/scratch/private", Tools: tools.NewRegistry(criticalMutationTool{"write"}, criticalMutationTool{"edit"})})
	a.pinWorktree("/work/actual", "feature")
	for _, name := range []string{"write", "edit"} {
		for _, tc := range []struct {
			path string
			warn bool
		}{
			{"/invented/work/cmp/file.go", true},
			{"/repo-sibling/file.go", true},
			{"../outside/file.go", true},
			{"local.go", false},
			{"/scratch/private/log", false},
			{"/work/actual/file.go", false},
			{filepath.Join(os.TempDir(), "ugh-test-output"), false},
		} {
			raw, _ := json.Marshal(map[string]string{"path": tc.path})
			out, _, err := a.execute(context.Background(), llm.ToolCall{Name: name, Args: string(raw)})
			if err != nil || strings.Contains(out, "WARNING:") != tc.warn || !strings.Contains(out, "mutation result") {
				t.Errorf("%s %s: out=%q err=%v", name, tc.path, out, err)
			}
		}
	}
	// A symlink in an allowed root must not hide a target outside all roots.
	dir := t.TempDir()
	if err := os.Symlink("/invented/work", filepath.Join(dir, "escape")); err != nil {
		t.Fatal(err)
	}
	a.cfg.Cwd = dir
	if note := a.mutationPathWarning(llm.ToolCall{Name: "write", Args: `{"path":"escape/file.go"}`}); note == "" {
		t.Fatal("symlink escape not warned")
	}
}

func TestPinTaskDedupesAndCaps(t *testing.T) {
	a := New(Config{Cwd: t.TempDir()})
	a.pinTask("First assignment: MUST stay.")
	a.pinTask("First assignment: MUST stay.")
	if n := strings.Count(a.critical.Tasks, "First assignment: MUST stay."); n != 1 {
		t.Fatalf("duplicate task pinned %d times", n)
	}
	for i := range 200 {
		a.pinTask(fmt.Sprintf("instruction %d: %s", i, strings.Repeat("x", 3000)))
	}
	if len(a.critical.Tasks) > criticalTasksMax+len("\n--- Task / instruction ---\n\n") {
		t.Fatalf("pinned tasks grew to %d bytes", len(a.critical.Tasks))
	}
	if !strings.Contains(a.critical.Tasks, "First assignment: MUST stay.") {
		t.Fatal("cap dropped the original assignment")
	}
	if !strings.Contains(a.critical.Tasks, "instruction 199:") {
		t.Fatal("cap dropped the newest instruction")
	}
}

func TestRestoreCriticalSkipsGeneratedMessagesAndDoesNotDuplicate(t *testing.T) {
	hist := []llm.Message{
		{Role: llm.RoleUser, Text: "Assignment: NEVER leave cwd."},
		{Role: llm.RoleAssistant, Text: "working"},
		{Role: llm.RoleUser, Text: "[Subagent results: 1 finished, 0 still running]\n\nbulky report"},
		{Role: llm.RoleUser, Text: turnOutNudge},
		{Role: llm.RoleUser, Text: "[Message from the user, sent while you were working]\nSteer: DO NOT touch go.mod."},
	}
	a := New(Config{Cwd: t.TempDir(), History: hist})
	if strings.Contains(a.critical.Tasks, "bulky report") || strings.Contains(a.critical.Tasks, "Turn budget") {
		t.Fatalf("generated messages pinned: %q", a.critical.Tasks)
	}
	a.pinTask("Steer: DO NOT touch go.mod.")
	a.pinTask("Assignment: NEVER leave cwd.")
	for _, want := range []string{"Assignment: NEVER leave cwd.", "Steer: DO NOT touch go.mod."} {
		if n := strings.Count(a.critical.Tasks, want); n != 1 {
			t.Errorf("%q pinned %d times", want, n)
		}
	}
}

func TestCompactionSeedHasTaskOnce(t *testing.T) {
	a := New(Config{Provider: &growProvider{}, Model: "m", Cwd: t.TempDir()})
	a.task = "UNIQUE-TASK-MARKER do the thing"
	a.pinTask(a.task)
	for range 30 {
		a.push(llm.Message{Role: llm.RoleAssistant, Text: strings.Repeat("filler ", 1000)})
	}
	if !a.compactFor(context.Background(), "m", 100000, 200000, nil) {
		t.Fatal("did not compact")
	}
	if n := strings.Count(a.hist[0].Text, "UNIQUE-TASK-MARKER"); n != 1 {
		t.Fatalf("task appears %d times in the seed", n)
	}
}

func TestMutationWarningTempDirFollowsEnvironment(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	a := New(Config{Cwd: t.TempDir()})
	if a.mutationPathWarning(llm.ToolCall{Name: "write", Args: `{"path":"` + filepath.Join(tmp, "x") + `"}`}) != "" {
		t.Fatal("os.TempDir() must be allowed")
	}
	if os.TempDir() != "/tmp" && a.mutationPathWarning(llm.ToolCall{Name: "write", Args: `{"path":"/tmp/ugh-hardcoded"}`}) == "" {
		t.Fatal("hardcoded /tmp must not be exempt when TMPDIR differs")
	}
}
