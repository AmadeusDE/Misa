package aiconfig

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/ai"
)

const keyHelp = "type or paste the key  enter store  esc cancel"

// keyPrompt asks for a provider's key. The input is masked, and the key is dropped from it once stored.
type keyPrompt struct {
	in     textinput.Model
	info   ai.ProviderInfo
	keyURL string
	added  bool // opened right after adding the provider
}

func (m *Model) startKey(p ai.ProviderInfo, added bool) {
	switch p.Kind {
	case "codex":
		m.failf("codex is a login, not a key: run `ugh login codex`")
		return
	case "console":
		m.failf("%s uses the OpenCode Console login: run `ugh login opencode-console`", p.Name)
		return
	}
	in := textinput.New()
	in.Prompt = ""
	in.EchoMode = textinput.EchoPassword
	in.EchoCharacter = '•'
	in.Focus()
	m.key = keyPrompt{in: in, info: p, added: added}
	for _, preset := range ai.Presets() {
		if preset.Name == p.Name {
			m.key.keyURL = preset.KeyURL
		}
	}
	m.mode = modeKey
	m.fit()
}

func (m *Model) keyPromptKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.mode = modeProviders
		m.key.in.SetValue("")
		if m.key.added {
			m.ok("saved %s without a key: press k to set one", m.key.info.Name)
		}
	case "enter":
		m.storeKey()
	default:
		return m.forward(msg)
	}
	return nil
}

func (m *Model) storeKey() {
	key := strings.TrimSpace(m.key.in.Value())
	if key == "" {
		m.failf("the key is empty")
		return
	}
	name := m.key.info.Name
	err := ai.SetKey(name, key)
	m.key.in.SetValue("")
	if err != nil {
		m.fail(err, key)
		return
	}
	m.mode = modeProviders
	m.reload()
	if p, ok := m.provider(name); ok {
		m.ok("key stored for %s (%s)", name, p.KeyMasked)
		if p.KeySource == "env" {
			m.ok("key stored for %s, but %s is set and takes precedence", name, p.KeyEnv)
		}
	}
}

func (m *Model) keyView() ([]string, string) {
	p := m.key.info
	lines := []string{title("Key for "+p.Name, m.w), ""}
	lines = append(lines, labelSt.Render("Key  ")+m.key.in.View(), "")
	lines = append(lines, dimSt.Render("The key is stored in ugh's credential store, never in config.toml."))
	if p.KeyEnv != "" {
		lines = append(lines, dimSt.Render("Setting "+p.KeyEnv+" in the environment overrides the stored key."))
	}
	if p.Local {
		lines = append(lines, dimSt.Render(p.Name+" runs locally and works without a key; esc to skip."))
	}
	if m.key.keyURL != "" {
		lines = append(lines, dimSt.Render("Get a key at "+m.key.keyURL))
	}
	if p.KeySource != "" {
		lines = append(lines, dimSt.Render("Current: "+p.KeySource+" "+p.KeyMasked))
	}
	return lines, keyHelp
}
