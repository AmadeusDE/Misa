package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/ughbot/bot"
	"github.com/AmadeusDE/Misa/internal/ughbot/config"
	"github.com/AmadeusDE/Misa/internal/ughbot/store"
)

var _ api.Repos = (*Daemon)(nil)

const cloneTimeout = 30 * time.Minute

var (
	cloneName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	scpLike   = regexp.MustCompile(`^[A-Za-z0-9._-]+@[A-Za-z0-9.-]+:[^:]`)
)

// ProposeRepo checks a request for a code directory and queues it; the stored request names
// the final directory, so the person approves exactly where a clone lands.
func (d *Daemon) ProposeRepo(from string, req bot.RepoRequest) (int64, error) {
	req, err := d.planRepo(req)
	if err != nil {
		return 0, err
	}
	var summary string
	if req.URL != "" {
		summary = fmt.Sprintf("clone %s into %s and let bot %s run code tasks there", req.URL, req.Dir, req.Bot)
	} else {
		summary = fmt.Sprintf("let bot %s run code tasks in %s", req.Bot, req.Dir)
	}
	if from != req.Bot {
		summary += " (asked by " + from + ")"
	}
	return d.queue(from, bot.ActionAddRepo, req, summary)
}

// planRepo resolves and checks a request: the bot exists and lacks the directory, an existing
// directory passes the code-dir checks, and a clone gets a fresh folder under the repos dir.
func (d *Daemon) planRepo(req bot.RepoRequest) (bot.RepoRequest, error) {
	b, ok := d.Lookup(req.Bot)
	if !ok {
		return req, fmt.Errorf("%w: %q", api.ErrUnknownBot, req.Bot)
	}
	if req.URL != "" {
		if err := checkCloneURL(req.URL); err != nil {
			return req, err
		}
		if req.Name == "" {
			req.Name = nameFromURL(req.URL)
		}
		if !cloneName.MatchString(req.Name) {
			return req, fmt.Errorf("clone name %q: use letters, digits, '.', '-' and '_'", req.Name)
		}
		req.Dir = filepath.Join(d.cfg.ReposDir(), req.Name)
		if _, err := os.Lstat(req.Dir); err == nil {
			return req, fmt.Errorf("%s already exists; ask for it as dir, or pick another name", req.Dir)
		}
	} else {
		req.Name = ""
		req.Dir = filepath.Clean(config.ExpandHome(req.Dir))
		if fi, err := os.Stat(req.Dir); err != nil || !fi.IsDir() {
			return req, fmt.Errorf("%s is not a directory on this host", req.Dir)
		}
	}
	if err := d.cfg.CheckCodeDir(req.Bot, req.Dir); err != nil {
		return req, err
	}
	if slices.Contains(b.Capabilities().Code, req.Dir) {
		return req, fmt.Errorf("bot %s can already run code tasks in %s", req.Bot, req.Dir)
	}
	return req, nil
}

func checkCloneURL(u string) error {
	switch {
	case strings.HasPrefix(u, "-"):
	case strings.HasPrefix(u, "https://"), strings.HasPrefix(u, "http://"), strings.HasPrefix(u, "ssh://"),
		strings.HasPrefix(u, "git://"), scpLike.MatchString(u):
		return nil
	}
	return fmt.Errorf("%q: only https, http, ssh and git URLs (or user@host:path) can be cloned", u)
}

func nameFromURL(u string) string {
	u = strings.TrimSuffix(strings.TrimRight(u, "/"), ".git")
	if i := strings.LastIndexAny(u, "/:"); i >= 0 {
		u = u[i+1:]
	}
	return u
}

// addRepo carries out an approved request: an existing directory is granted now, a clone
// starts in the background and its outcome is recorded and mailed when it ends.
func (d *Daemon) addRepo(a store.Approval, req bot.RepoRequest) (string, error) {
	planned, err := d.planRepo(req)
	if err != nil {
		return "", err
	}
	if planned.Dir != req.Dir {
		return "", fmt.Errorf("the request resolves to %s now, not %s; ask again", planned.Dir, req.Dir)
	}
	if req.URL == "" {
		return d.grantRepo(a, req, "")
	}
	return fmt.Sprintf("cloning %s into %s; the outcome follows as mail", req.URL, req.Dir), nil
}

// startClone begins an approved clone once the approval's first result is recorded, so the
// clone's own result always comes last.
func (d *Daemon) startClone(a store.Approval) {
	var req bot.RepoRequest
	if json.Unmarshal([]byte(a.Args), &req) != nil || req.URL == "" {
		return
	}
	d.mu.RLock()
	ctx := d.runCtx
	d.mu.RUnlock()
	if ctx == nil {
		ctx = context.Background()
	}
	go d.clone(ctx, a, req)
}

func (d *Daemon) clone(ctx context.Context, a store.Approval, req bot.RepoRequest) {
	result, err := d.cloneInto(ctx, req)
	if err == nil {
		result, err = d.grantRepo(a, req, req.URL)
	}
	if err != nil {
		result = "clone failed: " + err.Error()
	}
	_ = d.st.SetApprovalResult(a.ID, result, err != nil)
	d.tell(a, fmt.Sprintf("%s\n%s", a.Summary, result))
}

func (d *Daemon) cloneInto(ctx context.Context, req bot.RepoRequest) (string, error) {
	if err := os.MkdirAll(d.cfg.ReposDir(), 0o700); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(d.cfg.ReposDir(), "."+req.Name+".partial-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	ctx, cancel := context.WithTimeout(ctx, cloneTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-c", "protocol.allow=never", "-c", "protocol.https.allow=always",
		"-c", "protocol.http.allow=always", "-c", "protocol.ssh.allow=always", "-c", "protocol.git.allow=always",
		"clone", "--no-recurse-submodules", "--", req.URL, filepath.Join(tmp, "repo"))
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "SSH_ASKPASS=")
	cmd.Stdin = nil
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(lastLines(string(out), 5)))
	}
	if err := os.Rename(filepath.Join(tmp, "repo"), req.Dir); err != nil {
		return "", err
	}
	return "", nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.Join(lines[max(len(lines)-n, 0):], "\n")
}

// grantRepo records the grant. A bot without the code tools yet is rebuilt to get them, which
// ends its current turn; the bot is told when someone else asked for it.
func (d *Daemon) grantRepo(a store.Approval, req bot.RepoRequest, source string) (string, error) {
	if err := d.st.AddCodeGrant(store.CodeGrant{Bot: req.Bot, Dir: req.Dir, Source: source, Approval: a.ID}); err != nil {
		return "", err
	}
	result := fmt.Sprintf("bot %s may now run code tasks in %s", req.Bot, req.Dir)
	b, ok := d.Lookup(req.Bot)
	if !ok {
		return result, nil
	}
	if !b.HasCodeTools() {
		if err := d.rebuild(req.Bot); err != nil {
			return "", fmt.Errorf("granted, but rebuilding %s for its code tools failed: %w", req.Bot, err)
		}
		result += " (it was restarted to get the code tools)"
	}
	if a.Bot != req.Bot {
		if nb, ok := d.Lookup(req.Bot); ok {
			_, _ = nb.Deliver(fmt.Sprintf("approval #%d", a.ID),
				fmt.Sprintf("%s asked for a code directory for you and a person approved it: you may now start code tasks in %s.", a.Bot, req.Dir))
		}
	}
	return result, nil
}

// rebuild replaces a running bot with a fresh one from the same definition.
func (d *Daemon) rebuild(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	old, ok := d.bots[name]
	if !ok {
		return fmt.Errorf("%w: %q", api.ErrUnknownBot, name)
	}
	old.Stop()
	defer d.publishPeers()
	nb, err := d.build(d.cfg.Bots[name])
	if err != nil {
		delete(d.bots, name)
		log.Printf("rebuild %s: %v", name, err)
		return err
	}
	d.bots[name] = nb
	return nil
}

func (d *Daemon) RepoGrants(_ context.Context, name string) ([]api.RepoGrant, error) {
	grants, err := d.st.CodeGrants(name)
	if err != nil {
		return nil, err
	}
	out := make([]api.RepoGrant, len(grants))
	for i, g := range grants {
		out[i] = api.RepoGrant{Bot: g.Bot, Dir: g.Dir, Source: g.Source, Approval: g.Approval, Created: g.Created}
	}
	return out, nil
}

func (d *Daemon) RevokeRepo(_ context.Context, name, dir string) error {
	if err := d.st.DeleteCodeGrant(name, dir); errors.Is(err, store.ErrNoGrant) {
		return fmt.Errorf("%w: %s for %s", api.ErrNoGrant, dir, name)
	} else if err != nil {
		return err
	}
	if b, ok := d.Lookup(name); ok && !slices.Contains(b.Capabilities().Code, dir) {
		note := fmt.Sprintf("a person took back code directory %s; you can no longer start code tasks there.", dir)
		if n := b.StopCodeIn(dir); n > 0 {
			note += fmt.Sprintf(" %d code task(s) working there were stopped.", n)
		}
		_, _ = b.Deliver("repos", note)
	}
	return nil
}
