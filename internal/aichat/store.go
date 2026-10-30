// Package aichat is Misa's chat client for the AI providers ugh is set up with: a conversation list, a streaming
// markdown transcript, a model picker and optional web search, as a procmon.Viewer. Conversations are JSON files.
package aichat

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Message is one turn of a conversation. Notes are the dimmed lines for tool calls and retries.
type Message struct {
	Role     string   `json:"role"` // "user" or "assistant"
	Text     string   `json:"text"`
	Thinking string   `json:"thinking,omitempty"`
	Notes    []string `json:"notes,omitempty"`
	Model    string   `json:"model,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// Usage totals the tokens and reported cost of a conversation.
type Usage struct {
	In   int     `json:"in"`
	Out  int     `json:"out"`
	Cost float64 `json:"cost,omitempty"`
}

// Conversation is one saved chat.
type Conversation struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	System   string    `json:"system,omitempty"`
	Model    string    `json:"model,omitempty"`
	Search   bool      `json:"search,omitempty"`
	Created  time.Time `json:"created"`
	Updated  time.Time `json:"updated"`
	Usage    Usage     `json:"usage"`
	Messages []Message `json:"messages"`
}

// Meta is what the sidebar needs of a saved conversation.
type Meta struct {
	ID, Title string
	Updated   time.Time
}

const titleRunes = 40

var validID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// ErrNotFound is returned for an id with no saved conversation.
var ErrNotFound = errors.New("no such conversation")

// Store keeps conversations as <Dir>/<id>.json.
type Store struct{ Dir string }

// DefaultDir is misa/chats in the XDG data directory.
func DefaultDir() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "misa", "chats")
}

// NewConversation starts an empty conversation with a time-ordered id.
func NewConversation(now time.Time) *Conversation {
	return &Conversation{ID: now.UTC().Format("20060102-150405.000"), Created: now, Updated: now}
}

// TitleFrom is the first non-empty line of text, shortened.
func TitleFrom(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			return truncateRunes(line, titleRunes)
		}
	}
	return "New chat"
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}

func (s Store) path(id string) (string, error) {
	if !validID.MatchString(strings.ReplaceAll(id, ".", "-")) {
		return "", fmt.Errorf("%w: bad id %q", ErrNotFound, id)
	}
	return filepath.Join(s.Dir, id+".json"), nil
}

// Save writes c through a temp file and rename.
func (s Store) Save(c *Conversation) error {
	path, err := s.path(c.ID)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", " ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.Dir, c.ID+".tmp*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Load reads a conversation by id.
func (s Store) Load(id string) (*Conversation, error) {
	path, err := s.path(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w %q", ErrNotFound, id)
	}
	if err != nil {
		return nil, err
	}
	var c Conversation
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	c.ID = id
	return &c, nil
}

// Delete removes a conversation; a missing one is not an error.
func (s Store) Delete(id string) error {
	path, err := s.path(id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// List returns every readable conversation, most recently updated first. Unreadable files are skipped.
func (s Store) List() []Meta {
	entries, _ := os.ReadDir(s.Dir)
	var metas []Meta
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok || e.IsDir() {
			continue
		}
		c, err := s.Load(id)
		if err != nil {
			continue
		}
		metas = append(metas, Meta{ID: id, Title: c.Title, Updated: c.Updated})
	}
	sort.SliceStable(metas, func(i, j int) bool { return metas[i].Updated.After(metas[j].Updated) })
	return metas
}
