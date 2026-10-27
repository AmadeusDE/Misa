package daemon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/ughbot/client"
	"github.com/AmadeusDE/Misa/internal/ughbot/config"
	"github.com/AmadeusDE/Misa/internal/ughbot/fakellm"
	"github.com/AmadeusDE/Misa/internal/ughbot/store"
)

// repoDaemon has "boss" (manages coder), "coder" (no code yet) and "dev" (code in its own dir).
func repoDaemon(t *testing.T) (*Daemon, *client.Client, string) {
	t.Helper()
	h := newHarness(t)
	devDir := filepath.Join(h.dir, "src", "dev")
	if err := os.MkdirAll(devDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse([]byte(`
state_dir = "` + filepath.Join(h.dir, "state") + `"
socket = "` + filepath.Join(h.dir, "u.sock") + `"
[bots.boss]
model = "fake/m"
capabilities = { manage = ["coder"] }
[bots.coder]
model = "fake/m"
[bots.dev]
model = "fake/m"
capabilities = { code = ["` + devDir + `"] }
`))
	if err != nil {
		t.Fatal(err)
	}
	d, err := New(Options{Config: cfg, Provider: fakellm.Echo(0)})
	if err != nil {
		t.Fatal(err)
	}
	srv := api.NewServer(d)
	ctx, cancel := context.WithCancel(context.Background())
	go srv.Serve(ctx, d.ln)
	c, err := client.Dial(cfg.Socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); c.Close(); d.closeAll() })
	return d, c, h.dir
}

func hasTool(d *Daemon, botName, name string) bool {
	b, _ := d.Lookup(botName)
	_, err := b.RunTool(context.Background(), name, []byte(`{}`))
	return err == nil || !strings.Contains(err.Error(), "has no "+name+" tool")
}

func TestManagerAsksForARepoForItsBot(t *testing.T) {
	d, c, home := repoDaemon(t)
	repo := filepath.Join(home, "src", "app")
	os.MkdirAll(repo, 0o755)
	if hasTool(d, "coder", "request_repo") || hasTool(d, "coder", "code_task") {
		t.Fatal("a bot with neither code nor manage gets no repo tools")
	}
	if !hasTool(d, "dev", "request_repo") || !hasTool(d, "boss", "request_repo") {
		t.Fatal("bots with code or manage may ask")
	}
	if _, err := tool(t, d, "boss", "request_repo", `{"bot":"dev","dir":"`+repo+`"}`); err == nil {
		t.Fatal("boss does not manage dev")
	}
	for _, bad := range []string{home, filepath.Join(home, "missing"), filepath.Join(home, "state")} {
		if _, err := tool(t, d, "boss", "request_repo", `{"bot":"coder","dir":"`+bad+`"}`); err == nil {
			t.Errorf("%s accepted", bad)
		}
	}
	if _, err := tool(t, d, "dev", "request_repo", `{"dir":"`+filepath.Join(home, "src", "dev")+`"}`); err == nil {
		t.Fatal("a directory the bot already has")
	}
	out, err := tool(t, d, "boss", "request_repo", `{"bot":"coder","dir":"`+repo+`"}`)
	if err != nil || !strings.Contains(out, "approval #") {
		t.Fatalf("request: %q, %v", out, err)
	}
	p := pendingSoon(t, c, 1)[0]
	if p.Bot != "boss" || !strings.Contains(p.Summary, "coder") || !strings.Contains(p.Summary, repo) || !strings.Contains(p.Summary, "asked by boss") {
		t.Fatalf("approval %+v", p)
	}
	if b, _ := d.Lookup("coder"); slices.Contains(b.Capabilities().Code, repo) {
		t.Fatal("granted before approval")
	}
	info, err := c.Approve(context.Background(), p.ID, "test")
	if err != nil || info.State != store.ApprovalApproved || !strings.Contains(info.Result, "restarted") {
		t.Fatalf("approve: %+v %v", info, err)
	}
	b, _ := d.Lookup("coder")
	if !b.HasCodeTools() || !slices.Contains(b.Capabilities().Code, repo) {
		t.Fatalf("coder after approval: tools %v, caps %v", b.HasCodeTools(), b.Capabilities())
	}
	if m := mailFrom(t, d, "coder", "approval #"); !strings.Contains(m.Body, repo) {
		t.Fatalf("coder told: %q", m.Body)
	}
	if m := mailFrom(t, d, "boss", "approval #"); !strings.Contains(m.Body, "may now run code tasks") {
		t.Fatalf("boss told: %q", m.Body)
	}
	bots, _ := c.ListBots(context.Background())
	for _, bi := range bots {
		if bi.Name == "coder" && !strings.Contains(bi.Capabilities, repo) {
			t.Fatalf("coder capabilities line %q", bi.Capabilities)
		}
	}

	grants, err := c.RepoGrants(context.Background(), "coder")
	if err != nil || len(grants) != 1 || grants[0].Approval != p.ID {
		t.Fatalf("grants %+v %v", grants, err)
	}
	if err := c.RevokeRepo(context.Background(), "coder", repo); err != nil {
		t.Fatal(err)
	}
	if err := c.RevokeRepo(context.Background(), "coder", repo); err == nil {
		t.Fatal("second revoke")
	}
	if b, _ := d.Lookup("coder"); slices.Contains(b.Capabilities().Code, repo) {
		t.Fatal("still granted after revoke")
	}
	if m := mailFrom(t, d, "coder", "repos"); !strings.Contains(m.Body, "took back") {
		t.Fatalf("revoke note %q", m.Body)
	}
}

// gitServer serves a one-commit repository over git's dumb HTTP protocol.
func gitServer(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git missing")
	}
	work, bare := t.TempDir(), filepath.Join(t.TempDir(), "tool.git")
	run := func(dir string, args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run(work, "init", "-q")
	os.WriteFile(filepath.Join(work, "README"), []byte("hi\n"), 0o644)
	run(work, "add", ".")
	run(work, "commit", "-qm", "one")
	run(work, "clone", "-q", "--bare", work, bare)
	run(bare, "update-server-info")
	srv := httptest.NewServer(http.FileServer(http.Dir(filepath.Dir(bare))))
	t.Cleanup(srv.Close)
	return srv.URL + "/tool.git"
}

func TestApprovedCloneLandsInTheReposDir(t *testing.T) {
	d, c, _ := repoDaemon(t)
	url := gitServer(t)
	for _, bad := range []string{"file:///etc", "/srv/x", "ext::sh -c id", "-uhttps://x"} {
		if _, err := tool(t, d, "dev", "request_repo", `{"url":"`+bad+`"}`); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if _, err := tool(t, d, "dev", "request_repo", `{"url":"`+url+`","name":"../x"}`); err == nil {
		t.Fatal("a name with a path")
	}
	if _, err := tool(t, d, "dev", "request_repo", `{"url":"`+url+`"}`); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(d.cfg.ReposDir(), "tool")
	p := pendingSoon(t, c, 1)[0]
	if !strings.Contains(p.Summary, url) || !strings.Contains(p.Summary, dir) {
		t.Fatalf("summary %q", p.Summary)
	}
	info, err := c.Approve(context.Background(), p.ID, "test")
	if err != nil || !strings.Contains(info.Result, "cloning") {
		t.Fatalf("approve %+v %v", info, err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		a, _, _ := d.st.Approval(p.ID)
		if strings.Contains(a.Result, "may now run") {
			break
		}
		if a.State == store.ApprovalFailed || time.Now().After(deadline) {
			t.Fatalf("clone: %s %s", a.State, a.Result)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(filepath.Join(dir, "README")); err != nil {
		t.Fatal(err)
	}
	b, _ := d.Lookup("dev")
	if !slices.Contains(b.Capabilities().Code, dir) {
		t.Fatalf("caps %v", b.Capabilities())
	}
	if grants, _ := c.RepoGrants(context.Background(), ""); len(grants) != 1 || grants[0].Source != url {
		t.Fatalf("grants %+v", grants)
	}
	if _, err := tool(t, d, "dev", "request_repo", `{"url":"`+url+`"}`); err == nil {
		t.Fatal("a second clone into the same folder")
	}
	leftovers, _ := filepath.Glob(filepath.Join(d.cfg.ReposDir(), ".tool.partial-*"))
	if len(leftovers) != 0 {
		t.Fatalf("partial clone left: %v", leftovers)
	}
}
