package tui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/ughbot/api"
)

// RepoBackend is the grant store the Repos screen needs; *client.Client satisfies it.
type RepoBackend interface {
	RepoGrants(ctx context.Context, bot string) ([]api.RepoGrant, error)
	RevokeRepo(ctx context.Context, bot, dir string) error
}

type reposMsg struct {
	list []api.RepoGrant
	err  error
}

type repoDoneMsg struct {
	note string
	err  error
}

type reposModel struct {
	list    []api.RepoGrant
	cursor  int
	confirm bool // revoke waits for y
	note    string
	err     error
	loaded  bool
}

func (m *Model) repoBackend() (RepoBackend, bool) {
	rb, ok := m.be.(RepoBackend)
	return rb, ok
}

func (m *Model) fetchRepos() tea.Cmd {
	rb, ok := m.repoBackend()
	if !ok {
		return nil
	}
	ctx := m.ctx
	return func() tea.Msg {
		list, err := rb.RepoGrants(ctx, "")
		return reposMsg{list, err}
	}
}

func (r *reposModel) load(msg reposMsg) {
	r.loaded, r.err = true, msg.err
	if msg.err == nil {
		r.list = msg.list
	}
	r.cursor = min(max(r.cursor, 0), max(len(r.list)-1, 0))
}

func (m *Model) reposKey(msg tea.KeyPressMsg) tea.Cmd {
	r := &m.repos
	if r.confirm {
		r.confirm = false
		if s := msg.String(); len(r.list) == 0 || s != "y" && s != "Y" {
			return nil
		}
		g := r.list[r.cursor]
		rb, _ := m.repoBackend()
		ctx := m.ctx
		return func() tea.Msg {
			return repoDoneMsg{fmt.Sprintf("%s may no longer run code tasks in %s", g.Bot, g.Dir), rb.RevokeRepo(ctx, g.Bot, g.Dir)}
		}
	}
	switch msg.String() {
	case "up", "k":
		r.cursor = max(r.cursor-1, 0)
	case "down", "j":
		r.cursor = min(r.cursor+1, max(len(r.list)-1, 0))
	case "x":
		if len(r.list) > 0 {
			r.confirm, r.note = true, ""
		}
	case "r":
		return m.fetchRepos()
	}
	return nil
}

func (m *Model) repoDone(msg repoDoneMsg) tea.Cmd {
	if msg.err != nil {
		m.repos.note = errStyle.Render(msg.err.Error())
	} else {
		m.repos.note = goodStyle.Render(msg.note)
	}
	return m.fetchRepos()
}

func (r *reposModel) hint() string {
	if r.confirm {
		return "revoke this repo? its code tasks stop; files stay on disk. y/N"
	}
	return "x revoke  r refresh"
}

func (r *reposModel) view(w, h int) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Repos") + dimStyle.Render("  code dirs bots were granted through approvals") + "\n\n")
	if r.err != nil {
		b.WriteString(errStyle.Render("cannot list repos: "+r.err.Error()) + "\n")
	}
	switch {
	case !r.loaded:
		b.WriteString(dimStyle.Render("loading..."))
	case len(r.list) == 0 && r.err == nil:
		b.WriteString(dimStyle.Render("none yet: a bot with code dirs, or a manager, asks with request_repo and you approve it"))
	}
	rows := max(h-4, 1)
	start := min(max(r.cursor-rows+1, 0), max(len(r.list)-rows, 0))
	for i := start; i < len(r.list) && i < start+rows; i++ {
		g := r.list[i]
		from := fmt.Sprintf("approval #%d, %s", g.Approval, g.Created.Local().Format("2006-01-02"))
		if g.Source != "" {
			from = "cloned from " + g.Source + ", " + from
		}
		head := fmt.Sprintf("%-12s ", clip(g.Bot, 12))
		rest := cut(g.Dir+"  "+dimStyle.Render(from), max(w-len(head)-2, 10))
		if i == r.cursor {
			b.WriteString(selectedStyle.Render("> "+head) + rest + "\n")
		} else {
			b.WriteString("  " + head + rest + "\n")
		}
	}
	if r.note != "" {
		b.WriteString("\n" + r.note)
	}
	return strings.TrimRight(b.String(), "\n")
}
