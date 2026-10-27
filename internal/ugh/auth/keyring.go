package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/ugh/roots"

	"github.com/zalando/go-keyring"
)

// Keyring backend: one Secret Service item (service=ugh, user=credentials)
// holding the whole JSON document, so secrets stay out of plain files.
const (
	keyringService = "ugh"
	keyringUser    = "credentials"
)

type keyringBackend struct{}

func (keyringBackend) Name() string { return "keyring" }

func (keyringBackend) Load() (*Credentials, error) {
	secret, err := keyring.Get(keyringService, keyringUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return newCredentials(), nil
	}
	if err != nil {
		return nil, errors.New("auth: keyring unavailable")
	}
	var c Credentials
	if json.Unmarshal([]byte(secret), &c) != nil {
		return nil, errors.New("auth: keyring credentials corrupt")
	}
	if c.Version != 1 {
		return nil, errors.New("auth: credentials version unsupported")
	}
	if c.Providers == nil {
		c.Providers = map[string]Credential{}
	}
	return &c, nil
}

func (keyringBackend) Save(c *Credentials) error {
	if c.Version == 0 {
		c.Version = 1
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return errors.New("auth: credentials encoding failed")
	}
	if err := keyring.Set(keyringService, keyringUser, string(raw)); err != nil {
		return errors.New("auth: keyring write failed")
	}
	return nil
}

// probeKeyring reports whether the Secret Service is reachable, asking once
// per process: D-Bus can activate a Secret Service that never answers (a
// headless server, CI), and that must not hang every credential lookup.
// Test binaries never use the user's keyring; tests that need one replace
// this variable.
var probeKeyring = onceWithin(func() error {
	if testing.Testing() {
		return errors.New("auth: the keyring is not used in tests")
	}
	_, err := keyringBackend{}.Load()
	return err
}, 3*time.Second)

// onceWithin runs probe on first use and remembers its answer, or an error
// if it took longer than limit.
func onceWithin(probe func() error, limit time.Duration) func() error {
	var once sync.Once
	var result error
	return func() error {
		once.Do(func() {
			done := make(chan error, 1)
			go func() { done <- probe() }()
			select {
			case result = <-done:
			case <-time.After(limit):
				result = errors.New("auth: keyring did not answer")
			}
		})
		return result
	}
}

// Open picks the backend named by mode:
//
//	"keyring" — Secret Service only, error if unreachable
//	"file"    — $XDG_CONFIG_HOME/ugh/credentials.json
//	"auto"    — keyring when reachable, otherwise file (the default)
//
// With explicit roots the keyring is a single machine-wide "ugh" entry that
// instances would share, so "auto" resolves to the roots' own file. A headless
// server without a Secret Service should set `[auth] store = "file"` in
// config.toml (or rely on "auto", which falls back to the file when D-Bus is
// unreachable).
func Open(mode string) (*Store, error) { return OpenIn(roots.Roots{}, mode) }

// OpenIn is Open for the instance owning r.
func OpenIn(r roots.Roots, mode string) (*Store, error) {
	path, err := r.ConfigFile(FileName)
	if err != nil {
		return nil, err
	}
	if r.Config != "" && mode == "auto" {
		return openIsolated(path)
	}
	return open(mode, path)
}

// openIsolated keeps an embedded instance's own file store and lends it the
// user's ugh api keys read-only, so one `ugh auth set` serves every bot.
func openIsolated(path string) (*Store, error) {
	own := fileBackend{path: path}
	ughPath, err := roots.Roots{}.ConfigFile(FileName)
	if err != nil {
		return NewStore(own), nil
	}
	shared, err := open("auto", ughPath)
	if err != nil {
		return NewStore(own), nil
	}
	return NewStore(layeredBackend{primary: own, fallback: shared.backend}), nil
}

func open(mode, filePath string) (*Store, error) {
	switch mode {
	case "keyring":
		if err := probeKeyring(); err != nil {
			return nil, fmt.Errorf("auth: keyring backend selected but unavailable: %w", err)
		}
		return NewStore(keyringBackend{}), nil
	case "file":
		return NewStore(fileBackend{path: filePath}), nil
	case "auto":
		if err := probeKeyring(); err == nil {
			return NewStore(keyringBackend{}), nil
		}
		return NewStore(fileBackend{path: filePath}), nil
	default:
		return nil, fmt.Errorf("auth: unknown store backend %q", mode)
	}
}
