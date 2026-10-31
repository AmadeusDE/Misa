package auth

// Path and permission helpers are salvaged from old/internal/auth/store.go by
// copy-and-trim: ConfigPath, checkDir and owned stay essentially as they were,
// while the old lock/JSON machinery was left behind. Writes are atomic
// (temp file + fsync + rename) and loads refuse group/world-accessible files.

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const (
	AppDir   = "ugh"
	FileName = "credentials.json"
	maxFile  = 64 * 1024
)

// ConfigPath resolves the credentials file from an explicit environment
// snapshot. An absolute XDG_CONFIG_HOME takes precedence; otherwise
// HOME/.config is used.
func ConfigPath(home, xdgConfigHome string) (string, error) {
	base := xdgConfigHome
	if !filepath.IsAbs(base) {
		if !filepath.IsAbs(home) {
			return "", errors.New("auth: absolute HOME required")
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, AppDir, FileName), nil
}

// DefaultFilePath resolves the credentials file from the current environment.
func DefaultFilePath() (string, error) {
	return ConfigPath(os.Getenv("HOME"), os.Getenv("XDG_CONFIG_HOME"))
}

// fileBackend stores the document at $XDG_CONFIG_HOME/ugh/credentials.json
// (dir 0700, file 0600, checked on load, atomic writes).
type fileBackend struct{ path string }

func (f fileBackend) Name() string { return "file" }

// Path exposes the store location for `ugh auth list`.
func (f fileBackend) Path() string { return f.path }

func (f fileBackend) checkPath() error {
	if !filepath.IsAbs(f.path) || filepath.Clean(f.path) != f.path || filepath.Base(f.path) != FileName {
		return errors.New("auth: absolute canonical credentials path required")
	}
	return nil
}

func (f fileBackend) Load() (*Credentials, error) {
	if err := f.checkPath(); err != nil {
		return nil, err
	}
	if err := checkLoadDir(filepath.Dir(f.path)); err != nil {
		return nil, err
	}
	file, e := openCredentials(f.path)
	if errors.Is(e, os.ErrNotExist) {
		return newCredentials(), nil
	}
	if e != nil {
		return nil, e
	}
	defer file.Close()
	raw, e := io.ReadAll(io.LimitReader(file, maxFile+1))
	if e != nil || len(raw) > maxFile {
		return nil, errors.New("auth: credentials read failed")
	}
	if len(raw) == 0 {
		return newCredentials(), nil
	}
	var c Credentials
	// Parser errors never echo the file contents (it holds secrets).
	if e := json.Unmarshal(raw, &c); e != nil {
		return nil, errors.New("auth: credentials file is corrupt")
	}
	if c.Version != 1 {
		return nil, errors.New("auth: credentials version unsupported")
	}
	if c.Providers == nil {
		c.Providers = map[string]Credential{}
	}
	for name, cred := range c.Providers {
		if cred.Type == "api_key" && checkKey(cred.Key) != nil {
			return nil, errors.New("auth: stored key invalid")
		}
		if name == "" {
			return nil, errors.New("auth: credentials file is corrupt")
		}
	}
	return &c, nil
}

func (f fileBackend) Save(c *Credentials) error {
	if err := f.checkPath(); err != nil {
		return err
	}
	if c.Version == 0 {
		c.Version = 1
	}
	if c.Providers == nil {
		c.Providers = map[string]Credential{}
	}
	dir := filepath.Dir(f.path)
	if e := checkDir(dir); e != nil {
		return e
	}
	raw, e := json.Marshal(c)
	if e != nil || len(raw)+1 > maxFile {
		return errors.New("auth: credentials encoding failed")
	}
	raw = append(raw, '\n')
	tmp, e := stageFile(dir)
	if e != nil {
		return errors.New("auth: staging failed")
	}
	defer os.Remove(tmp.Name())
	_, e = tmp.Write(raw)
	if e == nil {
		e = tmp.Sync()
	}
	if closeErr := tmp.Close(); e == nil {
		e = closeErr
	}
	if e != nil {
		return errors.New("auth: staging failed")
	}
	return replaceFile(tmp.Name(), f.path)
}
