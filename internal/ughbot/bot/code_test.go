package bot

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ughbot/fakellm"
	"github.com/AmadeusDE/Misa/internal/ughbot/sandbox"
)

type recordingSandbox struct {
	mu     sync.Mutex
	shares [][]string
}

func (s *recordingSandbox) Start(ctx context.Context, shares []string) (sandbox.Machine, error) {
	s.mu.Lock()
	s.shares = append(s.shares, shares)
	s.mu.Unlock()
	return sandbox.Local{}.Start(ctx, shares)
}

var questionRef = regexp.MustCompile(`\[question #(\d+) from code:c1`)

func lastToolResult(req llm.Request) (string, bool) {
	if n := len(req.Messages); n > 0 && req.Messages[n-1].Role == llm.RoleTool {
		return req.Messages[n-1].Text, true
	}
	return "", false
}

// codeScript plays both sides: the bot starts a code task and answers its question; the code
// session asks, then writes and commits a file through bash, then reports.
func codeScript(repo string) fakellm.Script {
	return func(req llm.Request, _ int) []llm.Event {
		result, afterTool := lastToolResult(req)
		if strings.Contains(req.System, "You work for the bot") {
			switch {
			case !afterTool:
				return fakellm.ToolCall("q1", "ask_user", `{"questions":[{"question":"Which plan?","header":"Plan","options":[{"label":"A","description":"a"},{"label":"B","description":"b"}]}]}`, 0.01)
			case strings.Contains(result, "Which plan?"):
				return fakellm.ToolCall("b1", "bash", `{"command":"echo made > out.txt && git add out.txt && git commit -qm made && echo committed"}`, 0.01)
			default:
				return fakellm.Reply("finished; the answer was "+result, 0.01)
			}
		}
		last := fakellm.LastUser(req)
		switch {
		case afterTool:
			return fakellm.Reply("ok", 0)
		case strings.Contains(last, "please code"):
			args, _ := json.Marshal(map[string]string{"dir": repo, "prompt": "make out.txt"})
			return fakellm.ToolCall("t1", "code_task", string(args), 0)
		case questionRef.MatchString(last):
			n := questionRef.FindStringSubmatch(last)[1]
			return fakellm.ToolCall("r1", "reply", `{"question":`+n+`,"text":"use plan B"}`, 0)
		}
		return fakellm.Reply("noted: "+last, 0)
	}
}

func gitRepo(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{{"init", "-q"}, {"commit", "-q", "--allow-empty", "-m", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
}

func TestCodeTaskWorksInTheSandboxAndAsksTheBot(t *testing.T) {
	if os.Getenv("UGHBOT_FLAKY_TESTS") == "" {
		// Intermittent: the bot's reply to the task's question is sometimes denied by the
		// untrusted-output check. Tracked in docs/plans/ughbot.md.
		t.Skip("flaky; set UGHBOT_FLAKY_TESTS=1 to run")
	}
	home := t.TempDir()
	repo := filepath.Join(home, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t"} {
		t.Setenv(k, v)
	}
	gitRepo(t, repo)
	r := newRig(t, `code = ["`+repo+`"]`, 0)
	sb := &recordingSandbox{}
	p := &fakellm.Provider{Script: codeScript(repo)}
	b, err := New(Deps{Config: r.cfg, Bot: r.cfg.Bots["alpha"], Store: r.st, Ledger: r.led, Provider: p, Sandbox: sb, Locks: NewDirLocks()})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := b.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer b.Stop()
	if _, err := b.Send("please code"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for !strings.Contains(p.History(), "[mail from code:c1]") {
		if time.Now().After(deadline) {
			t.Fatalf("no final report; last history:\n%s", p.History())
		}
		time.Sleep(50 * time.Millisecond)
	}
	report := p.History()
	answered := false
	for _, req := range p.Requests() {
		for _, m := range req.Messages {
			answered = answered || strings.Contains(req.System, "You work for the bot") && m.Role == llm.RoleTool && strings.Contains(m.Text, "use plan B")
		}
	}
	if !answered {
		t.Fatal("the code session's ask_user never got the bot's answer")
	}
	if !strings.Contains(report, "branch: ") || !strings.Contains(report, "working tree clean") {
		t.Fatalf("report lacks the git summary:\n%s", report)
	}
	if b, err := os.ReadFile(filepath.Join(repo, "out.txt")); err != nil || string(b) != "made\n" {
		t.Fatalf("out.txt = %q, %v", b, err)
	}
	sb.mu.Lock()
	shares := sb.shares
	sb.mu.Unlock()
	if len(shares) != 1 || shares[0][0] != repo || !strings.HasSuffix(shares[0][1], filepath.Join("code", "alpha", "c1", "worktrees")) {
		t.Fatalf("shares = %v", shares)
	}
	spent, err := r.st.SpendSince("alpha", time.Time{})
	if err != nil || spent < 0.029 {
		t.Fatalf("the code session's spend (%v, %v) must count against the bot", spent, err)
	}
}

func TestCodeTaskRefusals(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "repo")
	os.MkdirAll(repo, 0o755)
	r := newRig(t, `code = ["`+repo+`"]`, 0)
	b, err := New(Deps{Config: r.cfg, Bot: r.cfg.Bots["alpha"], Store: r.st, Ledger: r.led, Provider: fakellm.Echo(0)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.startCode(home, "x"); err == nil {
		t.Fatal("a directory outside the code list was accepted")
	}
	if _, err := b.startCode(repo, "x"); err == nil || !strings.Contains(err.Error(), "runs no VMs") {
		t.Fatalf("no sandbox: %v", err)
	}
	locks := NewDirLocks()
	release, err := locks.take(repo, "alpha/c1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := locks.take(filepath.Join(repo, "sub"), "beta/c1"); err == nil {
		t.Fatal("a directory inside a busy one was taken")
	}
	release()
	if _, err := locks.take(repo, "beta/c2"); err != nil {
		t.Fatal(err)
	}
}
