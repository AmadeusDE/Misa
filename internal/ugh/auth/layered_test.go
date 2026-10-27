package auth

import "testing"

type memBackend struct{ c *Credentials }

func (m *memBackend) Name() string { return "mem" }
func (m *memBackend) Load() (*Credentials, error) {
	out := &Credentials{Providers: map[string]Credential{}}
	for k, v := range m.c.Providers {
		out.Providers[k] = v
	}
	return out, nil
}
func (m *memBackend) Save(c *Credentials) error { m.c = c; return nil }

func TestLayeredBorrowsOnlyAPIKeysAndNeverWritesFallback(t *testing.T) {
	primary := &memBackend{c: &Credentials{Providers: map[string]Credential{"own": {Type: "api_key", Key: "own-key-123456"}}}}
	fallback := &memBackend{c: &Credentials{Providers: map[string]Credential{
		"openrouter": {Type: "api_key", Key: "borrowed-key-123456"},
		"own":        {Type: "api_key", Key: "shadowed-key-123456"},
		"codex":      {Type: "oauth"},
	}}}
	s := NewStore(layeredBackend{primary: primary, fallback: fallback})
	if c, ok, _ := s.Get("openrouter"); !ok || c.Key != "borrowed-key-123456" {
		t.Fatalf("openrouter not borrowed: %+v %v", c, ok)
	}
	if c, _, _ := s.Get("own"); c.Key != "own-key-123456" {
		t.Fatalf("primary must win, got %q", c.Key)
	}
	if _, ok, _ := s.Get("codex"); ok {
		t.Fatal("oauth credentials must not be borrowed")
	}
	if err := s.Put("other", Credential{Type: "api_key", Key: "another-key-123456"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := fallback.c.Providers["other"]; ok {
		t.Fatal("fallback must stay read-only")
	}
}
