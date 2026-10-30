package aiconfig

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/ai"
	uc "github.com/AmadeusDE/Misa/internal/ugh/config"
	"github.com/AmadeusDE/Misa/internal/ugh/perm"
)

// current is what the defaults panel shows: ugh's default model and Misa's ai.yaml.
type current struct {
	Default  string
	Settings ai.Settings
}

func loadCurrent() (current, error) {
	s, err := ai.LoadSettings()
	if err != nil {
		return current{}, err
	}
	cfg, err := uc.LoadIn(ai.UghRoots)
	if err != nil {
		return current{Settings: s}, err
	}
	return current{Default: cfg.Models.Default, Settings: s}, nil
}

// fallback is the model a feature without its own entry uses.
func (c current) fallback() string {
	if ref := c.Settings.Models["default"]; ref != "" {
		return ref
	}
	return c.Default
}

// effective is the model a feature runs with.
func (c current) effective(feature string) string {
	if ref := c.Settings.Models[feature]; ref != "" {
		return ref
	}
	return c.fallback()
}

func providerOf(ref string) string {
	name, _, _ := strings.Cut(ref, "/")
	return name
}

// safeError renders an error for the screen with anything key-like scrubbed, so no key is ever shown.
func safeError(err error, secrets ...string) string {
	msg := perm.Scrub(err.Error())
	for _, s := range secrets {
		if s != "" {
			msg = strings.ReplaceAll(msg, s, "…")
		}
	}
	return strings.Join(strings.Fields(msg), " ")
}

type testDoneMsg struct {
	gen       int
	name, ref string
	took      time.Duration
	err       error
}

type modelsMsg struct {
	gen  int
	name string
	ids  []string
	err  error
}

const taskTimeout = 45 * time.Second

// testCmd makes a tiny completion in the background. With no ref it first asks the provider for a model.
func testCmd(gen int, name, ref string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), taskTimeout)
		defer cancel()
		if ref == "" {
			ids, err := ai.Models(ctx, name)
			if err == nil && len(ids) == 0 {
				err = errors.New("the provider lists no models")
			}
			if err != nil {
				return testDoneMsg{gen: gen, name: name, err: err}
			}
			ref = name + "/" + ids[0]
		}
		took, err := ai.Test(ctx, ref)
		return testDoneMsg{gen: gen, name: name, ref: ref, took: took, err: err}
	}
}

func modelsCmd(gen int, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), taskTimeout)
		defer cancel()
		ids, err := ai.Models(ctx, name)
		return modelsMsg{gen: gen, name: name, ids: ids, err: err}
	}
}

func setFeature(feature, ref string) error {
	s, err := ai.LoadSettings()
	if err != nil {
		return err
	}
	if ref == "" {
		delete(s.Models, feature)
	} else {
		if s.Models == nil {
			s.Models = map[string]string{}
		}
		s.Models[feature] = ref
	}
	return ai.SaveSettings(s)
}

func (m *Model) testProvider(p ai.ProviderInfo) tea.Cmd {
	m.testGen++
	ref := ""
	for _, r := range []string{m.cur.Default, m.cur.effective(ai.FeatureChat), m.cur.effective(ai.FeatureHelp)} {
		if providerOf(r) == p.Name {
			ref = r
			break
		}
	}
	m.ok("testing %s…", p.Name)
	return testCmd(m.testGen, p.Name, ref)
}

func (m *Model) testDone(msg testDoneMsg) {
	if msg.gen != m.testGen {
		return
	}
	if msg.err != nil {
		m.fail(fmt.Errorf("%s: test failed: %w", msg.name, msg.err))
		return
	}
	m.ok("%s: %s answered in %s", msg.name, msg.ref, msg.took.Round(time.Millisecond))
}
