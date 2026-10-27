package webui

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileStore keeps State as JSON (0600) in a single file.
type FileStore struct {
	mu    sync.RWMutex
	path  string
	state State
	seen  fileStamp
}

// fileStamp identifies the file contents last loaded or written, so another
// process (the admin CLI) editing the file is noticed.
type fileStamp struct {
	mod  time.Time
	size int64
}

func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f := &FileStore{path: filepath.Join(dir, "webui.json")}
	if err := f.reload(); err != nil {
		return nil, err
	}
	return f, nil
}

func (f *FileStore) stamp() fileStamp {
	info, err := os.Stat(f.path)
	if err != nil {
		return fileStamp{}
	}
	return fileStamp{mod: info.ModTime(), size: info.Size()}
}

func (f *FileStore) reload() error {
	stamp := f.stamp()
	data, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		f.state, f.seen = State{}, fileStamp{}
		return nil
	}
	if err != nil {
		return err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return err
	}
	f.state, f.seen = st, stamp
	return nil
}

// refresh picks up changes made to the file by another process.
func (f *FileStore) refresh() {
	if f.stamp() != f.seen {
		_ = f.reload()
	}
}

func (f *FileStore) View(fn func(*State)) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refresh()
	fn(&f.state)
	return nil
}

// Update runs fn on the current state under a lock shared with other
// processes (the admin CLI and the server), so neither undoes the other.
func (f *FileStore) Update(fn func(*State) error) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	unlock, err := lockFile(f.path + ".lock")
	if err != nil {
		return err
	}
	defer unlock()
	if err := f.reload(); err != nil {
		return err
	}
	before, err := json.Marshal(&f.state)
	if err != nil {
		return err
	}
	if err := fn(&f.state); err != nil {
		f.restore(before)
		return err
	}
	if err := f.save(); err != nil {
		f.restore(before)
		return err
	}
	return nil
}

func (f *FileStore) restore(data []byte) {
	f.state = State{}
	_ = json.Unmarshal(data, &f.state)
}

func (f *FileStore) save() error {
	data, err := json.Marshal(&f.state)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(f.path), ".webui-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, werr := tmp.Write(data)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return werr
	}
	if err := os.Rename(tmp.Name(), f.path); err != nil {
		return err
	}
	f.seen = f.stamp()
	return nil
}
