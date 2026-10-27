// Package confine opens paths inside directories an untrusted party can also write, such as
// a repository shared with a VM, so a symlink planted there never leads this process out.
package confine

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	mu   sync.Mutex
	dirs = map[string]int{}
)

// Add marks dir as shared until release is called; nested calls are counted.
func Add(dir string) (release func()) {
	dir = filepath.Clean(dir)
	mu.Lock()
	dirs[dir]++
	mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			mu.Lock()
			if dirs[dir]--; dirs[dir] <= 0 {
				delete(dirs, dir)
			}
			mu.Unlock()
		})
	}
}

// Shared reports the shared directory path lies in, and path relative to it.
func Shared(path string) (dir, rel string, ok bool) {
	path = filepath.Clean(path)
	mu.Lock()
	defer mu.Unlock()
	for d := range dirs {
		if r, err := filepath.Rel(d, path); err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) {
			if !ok || len(d) > len(dir) {
				dir, rel, ok = d, r, true
			}
		}
	}
	return dir, rel, ok
}

func open(path string) (*os.Root, string, error) {
	dir, rel, ok := Shared(path)
	if !ok {
		return nil, "", nil
	}
	r, err := os.OpenRoot(dir)
	return r, rel, err
}

// ReadFile is os.ReadFile that stays inside the shared directory holding path.
func ReadFile(path string) ([]byte, error) {
	r, rel, err := open(path)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return os.ReadFile(path)
	}
	defer r.Close()
	return r.ReadFile(rel)
}

// OpenFile is os.OpenFile that stays inside the shared directory holding path.
func OpenFile(path string, flag int, perm os.FileMode) (*os.File, error) {
	r, rel, err := open(path)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return os.OpenFile(path, flag, perm)
	}
	defer r.Close()
	return r.OpenFile(rel, flag, perm)
}

// MkdirAll is os.MkdirAll that stays inside the shared directory holding path.
func MkdirAll(path string, perm os.FileMode) error {
	r, rel, err := open(path)
	if err != nil {
		return err
	}
	if r == nil {
		return os.MkdirAll(path, perm)
	}
	defer r.Close()
	return r.MkdirAll(rel, perm)
}
