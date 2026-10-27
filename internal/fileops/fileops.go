package fileops

import (
	"os"
	"path/filepath"
	"sort"
)

type Entry struct {
	Name  string
	IsDir bool
}

func List(dir string) ([]Entry, error) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, len(des))
	for _, de := range des {
		entries = append(entries, Entry{Name: de.Name(), IsDir: de.IsDir()})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

func CreateFile(dir, name string) (string, error) {
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	return path, f.Close()
}

func Delete(path string) error {
	return os.Remove(path)
}
