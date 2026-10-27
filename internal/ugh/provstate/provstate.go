// Package provstate records providers that should not be used right now,
// shared by every ugh process through ~/.local/state/ugh/providers.json:
//
//   - exhausted: the provider refused with a usage/quota limit; skip it for
//     every agent and every new spawn until the reset time;
//   - reserved: its usage window passed the configured reserve; new spawns
//     avoid it, agents already on it keep going;
//   - disabled: turned off by hand (/provider); nothing uses it until it is
//     turned back on, and no automatic mark replaces it.
package provstate

import (
	"encoding/json"
	"github.com/AmadeusDE/Misa/internal/ugh/roots"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// State is one provider/model restriction.
type State struct {
	Kind       string    `json:"kind"` // exhausted | reserved | rejected | outage | disabled
	Until      time.Time `json:"until"`
	Reason     string    `json:"reason"`
	Source     string    `json:"source,omitempty"`      // api_error | quota | reserve | config
	Scope      string    `json:"scope,omitempty"`       // account | model | transport | provider
	AccountID  string    `json:"account_id,omitempty"`  // account identity
	ModelID    string    `json:"model_id,omitempty"`    // model id if scoped to model
	Transport  string    `json:"transport,omitempty"`   // transport if scoped to transport
	RecordedAt time.Time `json:"recorded_at,omitempty"` // evidence freshness timestamp
}

var now = time.Now

// Book is one instance's provider health file. The zero roots give the
// user's shared ugh file; explicit roots give a private one.
type Book struct {
	mu       sync.Mutex
	cache    map[string]State
	cachedAt time.Time
	path     func() string
}

var (
	booksMu sync.Mutex
	books   = map[roots.Roots]*Book{}
)

// For returns the one Book for r, so every user in the process shares its
// cache and lock. The zero Roots is the user's shared ugh file.
func For(r roots.Roots) *Book {
	booksMu.Lock()
	defer booksMu.Unlock()
	if b, ok := books[r]; ok {
		return b
	}
	b := &Book{path: func() string {
		p, err := r.StateFile("providers.json")
		if err != nil {
			return ""
		}
		return p
	}}
	books[r] = b
	return b
}

var std = For(roots.Roots{})

func (b *Book) load() map[string]State {
	if b.cache != nil && now().Sub(b.cachedAt) < 3*time.Second {
		return b.cache
	}
	m := map[string]State{}
	if raw, err := os.ReadFile(b.path()); err == nil {
		_ = json.Unmarshal(raw, &m)
	}
	for p, s := range m {
		if !s.Until.IsZero() && now().After(s.Until) {
			delete(m, p)
		}
	}
	b.cache, b.cachedAt = m, now()
	return m
}

// Get returns the provider's active restriction, if any.
func (b *Book) Get(provider string) (State, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	s, ok := b.load()[provider]
	if ok && !s.Until.IsZero() && now().After(s.Until) {
		return State{}, false
	}
	return s, ok
}

// CheckRoute returns active restrictions for a specific provider, model, transport, and account.
func (b *Book) CheckRoute(provider, model, transport, accountID string) (State, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	m := b.load()
	matches := func(s State) bool {
		return (s.Until.IsZero() || now().Before(s.Until)) &&
			(s.AccountID == "" || s.AccountID == accountID) &&
			(s.ModelID == "" || s.ModelID == model) &&
			(s.Transport == "" || s.Transport == transport)
	}
	// 1. Check account-scoped provider exhaustion.
	if accountID != "" {
		if s, ok := m[provider+"@"+accountID]; ok && matches(s) {
			return s, true
		}
	}
	// 2. Check model-scoped rejection.
	if model != "" {
		if s, ok := m[provider+"/"+model]; ok && matches(s) {
			return s, true
		}
	}
	// 3. Check general provider-wide restriction.
	if s, ok := m[provider]; ok && matches(s) {
		return s, true
	}
	return State{}, false
}

// InvalidateAccount clears cached restrictions for the given accountID.
func (b *Book) InvalidateAccount(accountID string) {
	if accountID == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cache = nil
	m := b.load()
	changed := false
	for k, s := range m {
		if s.AccountID == accountID || strings.HasSuffix(k, "@"+accountID) {
			delete(m, k)
			changed = true
		}
	}
	if changed {
		b.writeLocked(m)
	}
}

// MarkExhausted records an account-level provider quota exhaustion.
func (b *Book) MarkExhausted(provider, accountID string, until time.Time, reason, source string) {
	key := provider
	scope := "provider"
	if accountID != "" {
		key = provider + "@" + accountID
		scope = "account"
	}
	b.Mark(key, State{
		Kind:       "exhausted",
		Until:      until,
		Reason:     reason,
		Source:     source,
		Scope:      scope,
		AccountID:  accountID,
		RecordedAt: now(),
	})
}

// MarkModelRejected records a model rejection (e.g. 404 or protocol unsupported).
func (b *Book) MarkModelRejected(provider, model string, until time.Time, reason, source string) {
	key := provider + "/" + model
	b.Mark(key, State{
		Kind:       "rejected",
		Until:      until,
		Reason:     reason,
		Source:     source,
		Scope:      "model",
		ModelID:    model,
		RecordedAt: now(),
	})
}

// MarkOutage records a transient provider endpoint outage.
func (b *Book) MarkOutage(provider string, until time.Time, reason, source string) {
	b.Mark(provider, State{
		Kind:       "outage",
		Until:      until,
		Reason:     reason,
		Source:     source,
		Scope:      "provider",
		RecordedAt: now(),
	})
}

// Usable reports whether new work may start on the provider (neither
// exhausted nor reserved).
func (b *Book) Usable(provider string) bool {
	_, ok := b.Get(provider)
	return !ok
}

// Exhausted reports whether the provider refuses requests right now.
func (b *Book) Exhausted(provider string) bool {
	s, ok := b.Get(provider)
	return ok && s.Kind == "exhausted"
}

// Mark records a restriction (an exhausted mark is never downgraded to
// reserved before it expires).
func (b *Book) Mark(provider string, s State) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cache = nil
	m := b.load()
	if old, ok := m[provider]; ok && (old.Kind == "exhausted" && s.Kind == "reserved" || old.Kind == "disabled" && s.Kind != "disabled") {
		return
	}
	if s.RecordedAt.IsZero() {
		s.RecordedAt = now()
	}
	m[provider] = s
	b.writeLocked(m)
}

// Disable turns a provider off until Enable, for every ugh process.
func (b *Book) Disable(provider string) {
	b.Mark(provider, State{Kind: "disabled", Reason: "turned off with /provider", Source: "user", Scope: "provider"})
}

// Enable lifts a Disable.
func (b *Book) Enable(provider string) { b.Clear(provider, "disabled") }

// Disabled reports whether the provider was turned off by hand.
func (b *Book) Disabled(provider string) bool {
	s, ok := b.Get(provider)
	return ok && s.Kind == "disabled"
}

// Clear removes a reserved mark (usage fell back under the reserve).
func (b *Book) Clear(provider, kind string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cache = nil
	m := b.load()
	if s, ok := m[provider]; ok && s.Kind == kind {
		delete(m, provider)
		b.writeLocked(m)
	}
}

func (b *Book) writeLocked(m map[string]State) {
	p := b.path()
	if p == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	data, _ := json.MarshalIndent(m, "", "  ")
	tmp := p + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil {
		_ = os.Rename(tmp, p)
	}
	b.cache, b.cachedAt = m, now()
}

// Summary describes every active restriction in one line, e.g.
// "codex exhausted until 22:47 (usage_limit_reached)".
func (b *Book) Summary() string {
	b.mu.Lock()
	m := b.load()
	b.mu.Unlock()
	var parts []string
	for p, s := range m {
		if !s.Until.IsZero() && now().After(s.Until) {
			continue
		}
		if s.Until.IsZero() {
			parts = append(parts, p+" "+s.Kind+" ("+s.Reason+")")
			continue
		}
		parts = append(parts, p+" "+s.Kind+" until "+s.Until.Local().Format("15:04")+" ("+s.Reason+")")
	}
	sort.Strings(parts)
	return strings.Join(parts, " · ")
}

var resetsIn = regexp.MustCompile(`"?resets_in_seconds"?\s*:\s*(\d+)`)
var resetsAt = regexp.MustCompile(`"?resets?_at"?\s*:\s*"?(\d{9,13})`)

// QuotaError reports whether err is a provider usage/quota refusal and when
// it resets (default: an hour from now when the error does not say).
func QuotaError(err error) (until time.Time, reason string, ok bool) {
	if err == nil {
		return time.Time{}, "", false
	}
	s := err.Error()
	low := strings.ToLower(s)
	switch {
	case strings.Contains(low, "usage_limit_reached"):
		reason = "usage_limit_reached"
	case strings.Contains(low, "insufficient_quota"), strings.Contains(low, "quota exceeded"), strings.Contains(low, "exceeded your current quota"):
		reason = "quota exceeded"
	case strings.Contains(low, "http 402"), strings.Contains(low, "insufficient credits"):
		reason = "out of credits"
	default:
		return time.Time{}, "", false
	}
	until = now().Add(time.Hour)
	if m := resetsIn.FindStringSubmatch(s); m != nil {
		if n, e := strconv.Atoi(m[1]); e == nil {
			until = now().Add(time.Duration(n) * time.Second)
		}
	} else if m := resetsAt.FindStringSubmatch(s); m != nil {
		if n, e := strconv.ParseInt(m[1], 10, 64); e == nil {
			if n > 1e11 {
				until = time.UnixMilli(n)
			} else {
				until = time.Unix(n, 0)
			}
		}
	}
	return until, reason, true
}

// SetPathForTest points the shared state file elsewhere.
func SetPathForTest(p string) {
	std.mu.Lock()
	std.path = func() string { return p }
	std.cache = nil
	std.mu.Unlock()
}
