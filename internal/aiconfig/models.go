package aiconfig

import (
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/ai"
)

const (
	modelsHelp    = "enter default  c chat  h help  / filter  r refresh  esc back"
	modelsFilterH = "type to filter  enter keep  esc clear"
)

type modelsView struct {
	provider  string
	ids       []string
	shown     []string
	filter    textinput.Model
	filtering bool
	loading   bool
	err       string
	list      cursorList
	gen       int
}

func (m *Model) openModels(p ai.ProviderInfo) tea.Cmd {
	gen := m.models.gen + 1
	f := textinput.New()
	f.Prompt = ""
	m.models = modelsView{provider: p.Name, filter: f, loading: true, gen: gen}
	m.models.list.cur = 0
	m.mode = modeModels
	m.fit()
	return modelsCmd(gen, p.Name)
}

func (m *Model) modelsLoaded(msg modelsMsg) {
	if msg.gen != m.models.gen {
		return
	}
	m.models.loading = false
	if msg.err != nil {
		m.models.err = safeError(msg.err)
		return
	}
	m.models.ids = msg.ids
	m.models.refilter()
}

func (v *modelsView) refilter() {
	query := strings.ToLower(strings.TrimSpace(v.filter.Value()))
	v.shown = v.shown[:0]
	for _, id := range v.ids {
		if strings.Contains(strings.ToLower(id), query) {
			v.shown = append(v.shown, id)
		}
	}
	v.list.setN(len(v.shown))
}

func (m *Model) modelsKey(msg tea.KeyPressMsg) tea.Cmd {
	v := &m.models
	k := msg.String()
	if v.filtering {
		switch k {
		case "enter":
			v.filtering = false
			v.filter.Blur()
		case "esc":
			v.filtering = false
			v.filter.Blur()
			v.filter.SetValue("")
			v.refilter()
		case "up", "down", "pgup", "pgdown":
			v.list.key(k)
		default:
			return m.forward(msg)
		}
		return nil
	}
	if v.list.key(k) {
		return nil
	}
	switch k {
	case "esc", "q":
		if v.filter.Value() != "" {
			v.filter.SetValue("")
			v.refilter()
			return nil
		}
		m.mode = modeProviders
	case "/":
		v.filtering = true
		v.filter.Focus()
		v.filter.CursorEnd()
	case "r":
		if p, ok := m.provider(v.provider); ok {
			return m.openModelsKeepingFilter(p)
		}
	case "enter":
		m.setModel("")
	case "c":
		m.setModel(ai.FeatureChat)
	case "h":
		m.setModel(ai.FeatureHelp)
	case "?":
		m.helpFrom, m.mode = modeModels, modeHelp
	}
	return nil
}

func (m *Model) openModelsKeepingFilter(p ai.ProviderInfo) tea.Cmd {
	filter := m.models.filter
	cmd := m.openModels(p)
	m.models.filter = filter
	return cmd
}

// setModel makes the selected model ugh's default (feature "") or the model for a Misa feature.
func (m *Model) setModel(feature string) {
	v := &m.models
	if v.list.cur >= len(v.shown) {
		return
	}
	ref := v.provider + "/" + v.shown[v.list.cur]
	var err error
	what := "default model"
	if feature == "" {
		err = ai.SetDefaultModel(ref)
	} else {
		what = feature + " model"
		err = setFeature(feature, ref)
	}
	if err != nil {
		m.fail(err)
		return
	}
	m.reload()
	m.ok("%s set to %s", what, ref)
}

func (m *Model) modelTags(ref string) string {
	var tags []string
	if ref == m.cur.Default {
		tags = append(tags, "default")
	}
	for _, f := range []string{ai.FeatureChat, ai.FeatureHelp} {
		if ref == m.cur.Settings.Models[f] {
			tags = append(tags, f)
		}
	}
	return strings.Join(tags, " ")
}

func (m *Model) modelsView() ([]string, string) {
	v := &m.models
	head := "Models of " + v.provider
	if !v.loading && v.err == "" {
		head += dimSt.Render("  " + countOf(len(v.shown), len(v.ids)))
	}
	filter := dimSt.Render("filter: ") + v.filter.View()
	if !v.filtering && v.filter.Value() == "" {
		filter = dimSt.Render("filter: press / to type")
	}
	lines := []string{title(head, m.w), filter}
	switch {
	case v.loading:
		lines = append(lines, dimSt.Render("  loading models…"))
	case v.err != "":
		lines = append(lines, errSt.Render("  "+v.err))
	case len(v.shown) == 0:
		lines = append(lines, dimSt.Render("  no models match"))
	}
	lo, hi := v.list.window()
	for i := lo; i < hi; i++ {
		id := v.shown[i]
		tags := m.modelTags(v.provider + "/" + id)
		if i == v.list.cur {
			lines = append(lines, selSt.Render(cell("▸ "+cell(id, max(10, m.w-24))+" "+tags, m.w)))
			continue
		}
		lines = append(lines, "  "+textSt.Render(cell(id, max(10, m.w-24)))+" "+tagSt.Render(tags))
	}
	if v.filtering {
		return lines, modelsFilterH
	}
	return lines, modelsHelp
}

func countOf(shown, total int) string {
	if shown == total {
		return strconv.Itoa(total) + " models"
	}
	return strconv.Itoa(shown) + " of " + strconv.Itoa(total)
}
