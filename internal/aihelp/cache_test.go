package aihelp

import (
	"os"
	"strings"
	"testing"
)

func TestCacheKeysOnModelAndQuery(t *testing.T) {
	c := &Cache{Dir: t.TempDir() + "/nested/aihelp"}
	if _, ok := c.Get("m1", "q"); ok {
		t.Fatal("empty cache hit")
	}
	if err := c.Put("m1", "q", "answer"); err != nil {
		t.Fatal(err)
	}
	if got, ok := c.Get("m1", "q"); !ok || got != "answer" {
		t.Errorf("Get = %q, %v", got, ok)
	}
	if _, ok := c.Get("m2", "q"); ok {
		t.Error("another model hit the entry")
	}
	if _, ok := c.Get("m1", "q2"); ok {
		t.Error("another query hit the entry")
	}
	if _, ok := c.Get("m1q", ""); ok {
		t.Error("model and query run together")
	}
	entries, _ := os.ReadDir(c.Dir)
	if len(entries) != 1 || !strings.HasSuffix(entries[0].Name(), ".json") || len(entries[0].Name()) != 64+len(".json") {
		t.Errorf("entries = %v", entries)
	}
	if info, _ := entries[0].Info(); info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", info.Mode())
	}
	if err := c.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Get("m1", "q"); ok {
		t.Error("hit after Clear")
	}
}

func TestNilAndEmptyCacheStoreNothing(t *testing.T) {
	for _, c := range []*Cache{nil, {}} {
		if err := c.Put("m", "q", "a"); err != nil {
			t.Error(err)
		}
		if _, ok := c.Get("m", "q"); ok {
			t.Error("hit")
		}
		if err := c.Clear(); err != nil {
			t.Error(err)
		}
	}
}

func TestDefaultCacheUsesTheXDGCacheDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	if got := DefaultCache().Dir; got != dir+"/misa/aihelp" {
		t.Errorf("Dir = %q", got)
	}
}
