package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ughbot/config"
)

// ActionAddRepo gives a bot a code directory: one that exists, or a clone made on approval.
const ActionAddRepo = "add_repo"

// RepoRequest asks for a code directory for Bot: Dir names an existing directory, URL a git
// repository to clone into the repos dir (as Name, default the URL's last element).
type RepoRequest struct {
	Bot  string `json:"bot"`
	Dir  string `json:"dir,omitempty"`
	URL  string `json:"url,omitempty"`
	Name string `json:"name,omitempty"`
}

// codeDirs are the directories the bot may start code tasks in: its definition's and those
// granted since, read each time so a grant needs no restart.
func (b *Bot) codeDirs() []string {
	dirs := slices.Clone(b.d.Bot.Capabilities.Code)
	if b.d.Store == nil {
		return dirs
	}
	grants, _ := b.d.Store.CodeGrants(b.name)
	for _, g := range grants {
		if !slices.Contains(dirs, g.Dir) {
			dirs = append(dirs, g.Dir)
		}
	}
	return dirs
}

// Capabilities are the bot's definition's capabilities with its granted code directories.
func (b *Bot) Capabilities() config.Capabilities {
	c := b.d.Bot.Capabilities
	c.Code = b.codeDirs()
	return c
}

// HasCodeTools reports whether the bot was built with the code tools; a bot granted its first
// code directory has to be rebuilt to get them.
func (b *Bot) HasCodeTools() bool { return b.codeTools }

// StopCodeIn stops the bot's code tasks working in dir, after a person took dir back.
func (b *Bot) StopCodeIn(dir string) int {
	n := 0
	for _, t := range b.code.list() {
		if inside(t.root, dir) {
			t.cancel()
			n++
		}
	}
	return n
}

type requestRepo struct{ b *Bot }

func (requestRepo) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "request_repo",
		Description: "Ask a person to give a bot a code directory to run code tasks in: yourself, or a bot you manage. " +
			"Give dir for a repository already on this host (an absolute path), or url for a git repository to clone " +
			"(https or ssh; it lands in ughbot's repos folder, named by name or the URL)." + approvalNote,
		Parameters: objectSchema(nil, map[string]any{
			"bot":  str("who gets it; default yourself"),
			"dir":  str("an existing directory, absolute"),
			"url":  str("a git URL to clone instead"),
			"name": str("folder name for the clone; default from the URL"),
		})}
}

func (t requestRepo) Run(_ context.Context, _ string, raw json.RawMessage) (string, error) {
	var req RepoRequest
	if err := decode(raw, &req); err != nil {
		return "", err
	}
	b := t.b
	if req.Bot == "" {
		req.Bot = b.name
	}
	if req.Bot != b.name && !b.d.Bot.Capabilities.MayManage(req.Bot) {
		return "", fmt.Errorf("you may ask for a repo for yourself or a bot you manage, not %s", req.Bot)
	}
	if (req.Dir == "") == (req.URL == "") {
		return "", fmt.Errorf("give either dir or url")
	}
	if b.d.Proposer == nil {
		return "", fmt.Errorf("repo requests are not available")
	}
	id, err := b.d.Proposer.ProposeRepo(b.name, req)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("approval #%d requested: a code directory for %s.%s", id, req.Bot, approvalNote), nil
}
