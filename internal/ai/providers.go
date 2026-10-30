package ai

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/AmadeusDE/Misa/internal/ugh/auth"
	"github.com/AmadeusDE/Misa/internal/ugh/config"
	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ugh/perm"
	"github.com/AmadeusDE/Misa/internal/ugh/provider"
)

// ProviderInfo describes one provider ugh can use.
type ProviderInfo struct {
	Name, BaseURL, KeyEnv string
	Kind                  string // "builtin" | "custom" | "console" | "codex"
	KeySource             string // "env" | "store" | "login" | "" (missing)
	KeyMasked             string
	Local                 bool            // a loopback base URL: works without a key
	Custom                config.Provider // the [providers.<name>] table, zero for a plain built-in
}

var builtinProviders = map[string]bool{"opencode-go": true, "opencode-zen": true, "openrouter": true}

// Providers lists the built-in, config-defined and Console-discovered providers, then codex.
func Providers() ([]ProviderInfo, error) {
	cfg, err := config.LoadIn(UghRoots)
	if err != nil {
		return nil, err
	}
	store, err := auth.OpenIn(UghRoots, cfg.Auth.Store)
	if err != nil {
		return nil, err
	}
	specs := provider.SpecsIn(UghRoots)
	sort.SliceStable(specs, func(i, j int) bool {
		bi, bj := builtinProviders[specs[i].Name], builtinProviders[specs[j].Name]
		return bi && !bj
	})
	var out []ProviderInfo
	for _, s := range specs {
		info := ProviderInfo{Name: s.Name, BaseURL: s.BaseURL, KeyEnv: auth.EnvVar(s.Name), Kind: "custom",
			Custom: cfg.Providers[s.Name], Local: isLoopbackURL(s.BaseURL)}
		switch {
		case s.Console:
			info.Kind = "console"
			if auth.HasConsole(store) {
				info.KeySource = "login"
			}
		case builtinProviders[s.Name]:
			info.Kind = "builtin"
		}
		if !s.Console {
			info.KeySource, info.KeyMasked = keyStatus(store, s.Name)
		}
		out = append(out, info)
	}
	codex := ProviderInfo{Name: "codex", Kind: "codex"}
	if cred, ok, _ := store.Get("codex"); ok && cred.Type == "oauth" {
		codex.KeySource = "login"
	}
	return append(out, codex), nil
}

func keyStatus(store *auth.Store, name string) (source, masked string) {
	key, err := auth.Resolve(name, os.LookupEnv, store)
	if err != nil {
		return "", ""
	}
	if v, ok := os.LookupEnv(auth.EnvVar(name)); ok && v != "" {
		return "env", auth.MaskKey(key)
	}
	return "store", auth.MaskKey(key)
}

func isLoopbackURL(u string) bool {
	parsed, err := url.Parse(u)
	if err != nil {
		return false
	}
	switch parsed.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

var configMu sync.Mutex

// editConfig applies a textual edit to ugh's config.toml. The result must still parse as a config, and check
// must hold for it, or nothing is written.
func editConfig(edit func(src string) string, check func(config.Config) error) error {
	configMu.Lock()
	defer configMu.Unlock()
	path, err := config.PathIn(UghRoots)
	if err != nil {
		return err
	}
	src, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	out := edit(string(src))
	var parsed config.Config
	if _, err := toml.Decode(out, &parsed); err != nil {
		return fmt.Errorf("config.toml: edit would not parse (%v); edit the file by hand", err)
	}
	if err := check(parsed); err != nil {
		return fmt.Errorf("config.toml: %v; edit the file by hand", err)
	}
	return writeFileAtomic(path, []byte(out), 0o644, 0o700)
}

// SaveProvider adds or replaces [providers.<name>] in ugh's config.toml.
func SaveProvider(name string, p config.Provider) error {
	if err := config.ValidateProvider(name, p); err != nil {
		return err
	}
	return editConfig(func(src string) string {
		return replaceTables(src, []string{"providers", name}, renderProvider(name, p))
	}, func(got config.Config) error {
		if !sameProvider(got.Providers[name], p) {
			return fmt.Errorf("providers.%s is defined in a form this editor cannot change", name)
		}
		return nil
	})
}

func sameProvider(a, b config.Provider) bool {
	return a.BaseURL == b.BaseURL && a.KeyEnv == b.KeyEnv && a.SessionHeader == b.SessionHeader && a.Usage == b.Usage &&
		strings.Join(a.NoThinking, "\x00") == strings.Join(b.NoThinking, "\x00") && sameHeaders(a.Headers, b.Headers)
}

func sameHeaders(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// RemoveProvider deletes [providers.<name>] and the provider's stored key. Built-ins cannot be removed.
func RemoveProvider(name string) error {
	cfg, err := config.LoadIn(UghRoots)
	if err != nil {
		return err
	}
	if _, ok := cfg.Providers[name]; !ok {
		if builtinProviders[name] {
			return fmt.Errorf("%s is built in and cannot be removed", name)
		}
		return fmt.Errorf("provider %q is not defined in config.toml", name)
	}
	err = editConfig(func(src string) string {
		return replaceTables(src, []string{"providers", name}, "")
	}, func(got config.Config) error {
		if _, still := got.Providers[name]; still {
			return fmt.Errorf("providers.%s is defined in a form this editor cannot remove", name)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if builtinProviders[name] {
		return nil // only its override went; the key still serves the built-in
	}
	if err := RemoveKey(name); err != nil && !errors.Is(err, errNoKey) {
		return err
	}
	return nil
}

var errNoKey = errors.New("no stored key to remove")

func openStore() (*auth.Store, error) {
	cfg, err := config.LoadIn(UghRoots)
	if err != nil {
		return nil, err
	}
	return auth.OpenIn(UghRoots, cfg.Auth.Store)
}

// SetKey stores an API key for a provider in ugh's credential store.
func SetKey(name, key string) error {
	key = strings.TrimSpace(key)
	if name == "codex" {
		return errors.New("codex uses a login, not a key: run `ugh login codex`")
	}
	if err := knownKeyProvider(name); err != nil {
		return err
	}
	store, err := openStore()
	if err != nil {
		return err
	}
	if err := store.Put(name, auth.Credential{Type: "api_key", Key: key}); err != nil {
		return err
	}
	perm.RegisterSecret(key)
	return nil
}

func knownKeyProvider(name string) error {
	for _, s := range provider.SpecsIn(UghRoots) {
		if s.Name == name {
			if s.Console {
				return fmt.Errorf("%s uses the OpenCode Console login: run `ugh login opencode-console`", name)
			}
			return nil
		}
	}
	return fmt.Errorf("unknown provider %q", name)
}

// RemoveKey deletes a provider's stored key. An environment variable is not touched.
func RemoveKey(name string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	if _, ok, err := store.Get(name); err != nil {
		return err
	} else if !ok {
		return errNoKey
	}
	return store.Remove(name)
}

// SetDefaultModel sets models.default in ugh's config.toml; an empty ref clears it.
func SetDefaultModel(ref string) error {
	if prov, model, ok := strings.Cut(ref, "/"); ref != "" && (!ok || prov == "" || model == "") {
		return fmt.Errorf("model %q must be provider/model", ref)
	}
	value := ""
	if ref != "" {
		value = tomlString(ref)
	}
	return editConfig(func(src string) string {
		return setKey(src, []string{"models"}, "default", value)
	}, func(got config.Config) error {
		if got.Models.Default != ref {
			return errors.New("models.default is defined in a form this editor cannot change")
		}
		return nil
	})
}

// Models lists the model ids a provider reports.
func Models(ctx context.Context, name string) ([]string, error) {
	ms, err := provider.ModelsIn(ctx, UghRoots, name)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(ms))
	for i, m := range ms {
		ids[i] = m.ID
	}
	sort.Strings(ids)
	return ids, nil
}

// Test makes a tiny completion and reports how long the provider took to answer.
func Test(ctx context.Context, ref string) (time.Duration, error) {
	c, err := Open(ref)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	start := time.Now()
	if _, _, err := c.Complete(ctx, "", []llm.Message{{Role: llm.RoleUser, Text: "Reply with the single word: ok"}}); err != nil {
		return 0, err
	}
	return time.Since(start), nil
}
