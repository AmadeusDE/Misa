package roots

import (
	"path/filepath"
	"testing"
)

func TestDefaultsFollowXDG(t *testing.T) {
	t.Setenv("HOME", "/h")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	var r Roots
	if d, _ := r.ConfigDir(); d != "/h/.config/ugh" {
		t.Fatalf("config %q", d)
	}
	if d, _ := r.StateDir(); d != "/h/.local/state/ugh" {
		t.Fatalf("state %q", d)
	}
	t.Setenv("XDG_STATE_HOME", "/x")
	if f, _ := r.StateFile("cost.jsonl"); f != "/x/ugh/cost.jsonl" {
		t.Fatalf("file %q", f)
	}
}

func TestExplicitIgnoresEnvironment(t *testing.T) {
	t.Setenv("HOME", "/h")
	t.Setenv("XDG_CONFIG_HOME", "/xc")
	r := Roots{Config: "/bots/a/config", State: "/bots/a/state"}
	if !r.Explicit() {
		t.Fatal("not explicit")
	}
	if f, _ := r.ConfigFile("config.toml"); f != filepath.Join("/bots/a/config", "config.toml") {
		t.Fatalf("config %q", f)
	}
	if d, _ := r.StateDir(); d != "/bots/a/state" {
		t.Fatalf("state %q", d)
	}
}

func TestRelativeExplicitRootRejected(t *testing.T) {
	if _, err := (Roots{State: "rel"}).StateDir(); err == nil {
		t.Fatal("relative root accepted")
	}
}
