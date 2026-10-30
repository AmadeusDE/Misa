package aichat

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func sample(id string, at time.Time) *Conversation {
	return &Conversation{
		ID: id, Title: "hello", System: "be brief", Model: "ollama/llama3.2", Search: true,
		Created: at, Updated: at, Usage: Usage{In: 12, Out: 34, Cost: 0.002},
		Messages: []Message{
			{Role: "user", Text: "hi\nthere"},
			{Role: "assistant", Text: "hello", Thinking: "hm", Notes: []string{"searched: x"}, Model: "ollama/llama3.2"},
		},
	}
}

func TestStoreRoundTrip(t *testing.T) {
	s := Store{Dir: filepath.Join(t.TempDir(), "chats")}
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	want := sample("one", at)
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load("one")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Load = %+v, %v; want %+v", got, err, want)
	}
	info, _ := os.Stat(filepath.Join(s.Dir, "one.json"))
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, want 0600 (CreateTemp)", info.Mode().Perm())
	}
	if leftovers, _ := filepath.Glob(filepath.Join(s.Dir, "*.tmp*")); len(leftovers) != 0 {
		t.Errorf("temp files left: %v", leftovers)
	}
}

func TestStoreListNewestFirstSkipsJunk(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, id := range []string{"a", "b", "c"} {
		c := sample(id, base.Add(time.Duration(i)*time.Hour))
		c.Title = "title " + id
		if err := s.Save(c); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(s.Dir, "broken.json"), []byte("{"), 0o600)
	os.WriteFile(filepath.Join(s.Dir, "notes.txt"), []byte("x"), 0o600)
	var ids []string
	for _, m := range s.List() {
		ids = append(ids, m.ID+":"+m.Title)
	}
	if got := strings.Join(ids, " "); got != "c:title c b:title b a:title a" {
		t.Errorf("List = %s", got)
	}
	if got := (Store{Dir: filepath.Join(s.Dir, "missing")}).List(); len(got) != 0 {
		t.Errorf("List of a missing dir = %v", got)
	}
}

func TestStoreDeleteAndMissing(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	s.Save(sample("x", time.Now()))
	if err := s.Delete("x"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("x"); err != nil {
		t.Errorf("deleting twice: %v", err)
	}
	if _, err := s.Load("x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Load after delete: %v", err)
	}
}

func TestStoreRejectsPathIDs(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	for _, id := range []string{"", "../x", "a/b", ".hidden", "a b", `a\b`} {
		if _, err := s.Load(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Load(%q) = %v", id, err)
		}
		if err := s.Save(&Conversation{ID: id}); err == nil {
			t.Errorf("Save(%q) succeeded", id)
		}
		if err := s.Delete(id); err == nil {
			t.Errorf("Delete(%q) succeeded", id)
		}
	}
}

func TestNewConversationIDsAreValidAndOrdered(t *testing.T) {
	a := NewConversation(time.Date(2026, 1, 2, 3, 4, 5, 6e6, time.UTC))
	b := NewConversation(time.Date(2026, 1, 2, 3, 4, 5, 7e6, time.UTC))
	if a.ID >= b.ID {
		t.Errorf("ids not ordered: %s, %s", a.ID, b.ID)
	}
	if _, err := (Store{Dir: t.TempDir()}).path(a.ID); err != nil {
		t.Errorf("new id rejected: %v", err)
	}
}

func TestTitleFrom(t *testing.T) {
	for in, want := range map[string]string{
		"":                              "New chat",
		"  \n\n  first   line \nsecond": "first line",
		strings.Repeat("é", 100):        strings.Repeat("é", titleRunes-1) + "…",
	} {
		if got := TitleFrom(in); got != want {
			t.Errorf("TitleFrom(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDefaultDirFollowsXDG(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/data")
	if got := DefaultDir(); got != "/data/misa/chats" {
		t.Errorf("DefaultDir = %s", got)
	}
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "/home/u")
	if got := DefaultDir(); got != "/home/u/.local/share/misa/chats" {
		t.Errorf("DefaultDir = %s", got)
	}
}

func TestNewConversationIDsDifferWithinOneMillisecond(t *testing.T) {
	now := time.Now()
	seen := map[string]bool{}
	for range 50 {
		seen[NewConversation(now).ID] = true
	}
	if len(seen) < 45 {
		t.Fatalf("only %d distinct ids out of 50 at one instant", len(seen))
	}
}
