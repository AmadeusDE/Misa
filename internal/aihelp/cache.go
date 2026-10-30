package aihelp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

// Cache keeps answers on disk, one JSON file per sha256(model + query). A nil or empty-Dir cache stores nothing.
type Cache struct{ Dir string }

// DefaultCache lives in the user cache directory, misa/aihelp.
func DefaultCache() *Cache {
	dir, err := os.UserCacheDir()
	if err != nil {
		return &Cache{}
	}
	return &Cache{Dir: filepath.Join(dir, "misa", "aihelp")}
}

type cacheEntry struct {
	Model    string `json:"model"`
	Query    string `json:"query"`
	Response string `json:"response"`
}

func (c *Cache) path(model, query string) string {
	sum := sha256.Sum256([]byte(model + "\x00" + query))
	return filepath.Join(c.Dir, hex.EncodeToString(sum[:])+".json")
}

// Get returns the cached answer for the pair.
func (c *Cache) Get(model, query string) (string, bool) {
	if c == nil || c.Dir == "" {
		return "", false
	}
	data, err := os.ReadFile(c.path(model, query))
	if err != nil {
		return "", false
	}
	var e cacheEntry
	if json.Unmarshal(data, &e) != nil || e.Response == "" {
		return "", false
	}
	return e.Response, true
}

// Put stores an answer; failures are ignored, a cache is only an optimisation.
func (c *Cache) Put(model, query, response string) error {
	if c == nil || c.Dir == "" || response == "" {
		return nil
	}
	if err := os.MkdirAll(c.Dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cacheEntry{model, query, response}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path(model, query), data, 0o600)
}

// Clear removes every cached answer.
func (c *Cache) Clear() error {
	if c == nil || c.Dir == "" {
		return nil
	}
	return os.RemoveAll(c.Dir)
}
