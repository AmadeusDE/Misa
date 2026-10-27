package tui

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/ughbot/api"
)

type repoFake struct {
	*routineFake
	grants  []api.RepoGrant
	revoked []string
}

func (f *repoFake) RepoGrants(context.Context, string) ([]api.RepoGrant, error) { return f.grants, nil }
func (f *repoFake) RevokeRepo(_ context.Context, bot, dir string) error {
	for i, g := range f.grants {
		if g.Bot == bot && g.Dir == dir {
			f.grants = append(f.grants[:i], f.grants[i+1:]...)
			f.revoked = append(f.revoked, bot+" "+dir)
			return nil
		}
	}
	return api.ErrNoGrant
}

func TestReposScreen(t *testing.T) {
	be := &repoFake{routineFake: &routineFake{approvalFake: newApprovalFake()}}
	m := ready(t, be, "")
	m.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	run(m, m.goTo(screenRepos))
	wantIn(t, m.View().Content, "none yet")

	be.grants = []api.RepoGrant{
		{Bot: "coder", Dir: "/srv/app", Approval: 4, Created: time.Now()},
		{Bot: "coder", Dir: "/state/repos/tool", Source: "https://x/tool.git", Approval: 5, Created: time.Now()},
	}
	run(m, press(m, "r"))
	wantIn(t, m.View().Content, "/srv/app", "cloned from https://x/tool.git", "approval #5")

	press(m, "down")
	press(m, "x")
	wantIn(t, m.View().Content, "revoke this repo?")
	press(m, "n")
	if len(be.revoked) != 0 {
		t.Fatal("anything but y keeps the grant")
	}
	press(m, "x")
	run(m, press(m, "y"))
	if len(be.revoked) != 1 || be.revoked[0] != "coder /state/repos/tool" {
		t.Fatalf("revoked %v", be.revoked)
	}
	wantIn(t, m.View().Content, "may no longer run code tasks")
}
