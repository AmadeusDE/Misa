package aichat

import (
	"context"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/AmadeusDE/Misa/internal/ai"
)

// Options configure a Chat. Every function is optional; the defaults use internal/ai and the real clipboard.
type Options struct {
	Model    string // -m: model for this session, over the conversation's own
	ID       string // conversation to open; empty starts a new one
	Dir      string // where conversations are kept; default DefaultDir()
	ModelFor func() (string, error)
	Open     func(ref string) (*ai.Client, error)
	Models   func(ctx context.Context) (refs, problems []string)
	Tools    func() ([]ai.Tool, error) // the web tools; nil, nil when search is off
	Copy     func(text string) tea.Cmd
	Now      func() time.Time
}

type mode int

const (
	modeChat mode = iota
	modePicker
	modeSystem
	modeRename
)

const (
	sidebarWidth    = 28
	sidebarMinWidth = 72
	renderEvery     = 50 * time.Millisecond
	maxInputRows    = 8
)

// Chat is the chat screen. It satisfies procmon.Viewer, so Misa shows it in a pane like `mines`.
type Chat struct {
	opt   Options
	store Store

	w, h  int
	quit  bool
	mode  mode
	focus focus

	conv    *Conversation
	saved   bool
	metas   []Meta
	editing int // index of the user message being rewritten, or -1

	input    textarea.Model
	system   textarea.Model // the system prompt editor
	rename   string         // the title being typed
	renameID string
	picker   picker

	showSidebar  bool
	sideCursor   int
	sideScroll   int
	confirmDel   string // id awaiting y/n
	showThinking bool
	scroll       int // transcript lines scrolled up from the bottom

	client        *ai.Client
	job           *job
	gen           int
	needBreak     bool // a note came in since the last text: start a new paragraph
	renderPending bool
	lastRender    time.Time
	streamLines   []string

	md      *glamour.TermRenderer
	mdWidth int
	blocks  []block

	notice    string
	noticeErr bool
}

type focus int

const (
	focusInput focus = iota
	focusSidebar
)

// New opens the conversation opt.ID, or starts an empty one.
func New(opt Options) (*Chat, error) {
	if opt.Dir == "" {
		opt.Dir = DefaultDir()
	}
	if opt.ModelFor == nil {
		opt.ModelFor = func() (string, error) { return ai.ModelFor(ai.FeatureChat) }
	}
	if opt.Open == nil {
		opt.Open = ai.Open
	}
	if opt.Models == nil {
		opt.Models = providerModels
	}
	if opt.Tools == nil {
		opt.Tools = webTools
	}
	if opt.Copy == nil {
		opt.Copy = copyText
	}
	if opt.Now == nil {
		opt.Now = time.Now
	}
	m := &Chat{opt: opt, store: Store{Dir: opt.Dir}, showSidebar: true, editing: -1, w: 100, h: 30}
	m.input = newInput("message")
	m.system = newInput("system prompt: how the model should behave in this conversation")
	if opt.ID != "" {
		c, err := m.store.Load(opt.ID)
		if err != nil {
			return nil, err
		}
		m.conv, m.saved = c, true
	} else {
		m.conv = NewConversation(opt.Now())
	}
	if opt.Model != "" {
		m.conv.Model = opt.Model
	}
	m.layout()
	return m, nil
}

func newInput(placeholder string) textarea.Model {
	in := textarea.New()
	in.Prompt = "› "
	in.Placeholder = placeholder
	in.ShowLineNumbers = false
	in.CharLimit = 0
	in.MaxWidth = 0
	in.MaxHeight = maxInputRows
	in.MaxContentHeight = 20000
	in.DynamicHeight = true
	in.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("alt+enter", "ctrl+j", "shift+enter"))
	in.KeyMap.LineNext = key.NewBinding(key.WithKeys("down"))
	in.KeyMap.LinePrevious = key.NewBinding(key.WithKeys("up"))
	in.KeyMap.CharacterBackward = key.NewBinding(key.WithKeys("left"))
	in.KeyMap.CharacterForward = key.NewBinding(key.WithKeys("right"))
	in.KeyMap.TransposeCharacterBackward = key.NewBinding()
	in.KeyMap.DeleteWordBackward = key.NewBinding(key.WithKeys("alt+backspace"))
	st := textarea.DefaultStyles(true)
	st.Focused.CursorLine = lipgloss.NewStyle()
	st.Focused.Prompt = lipgloss.NewStyle().Foreground(colAccent)
	st.Blurred.Prompt = lipgloss.NewStyle().Foreground(colDim)
	in.SetStyles(st)
	in.Focus()
	return in
}

type listMsg struct{ metas []Meta }

// Init lists the saved conversations off the Update goroutine.
func (m *Chat) Init() tea.Cmd {
	store := m.store
	return func() tea.Msg { return listMsg{store.List()} }
}

func (m *Chat) IsQuit() bool { return m.quit }

// Resize gives the screen its size.
func (m *Chat) Resize(w, h int) {
	m.w, m.h = max(w, 1), max(h, 1)
	m.blocks = nil
	m.layout()
	m.renderStreamNow()
}

// Generating reports whether an answer is streaming.
func (m *Chat) Generating() bool { return m.job != nil }

// Conversation is the one on screen.
func (m *Chat) Conversation() *Conversation { return m.conv }

// Update handles one message and returns the commands it starts.
func (m *Chat) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Resize(msg.Width, msg.Height)
	case tea.KeyPressMsg:
		return m.key(msg)
	case tea.PasteMsg:
		return m.paste(msg)
	case tea.MouseClickMsg:
		return m.click(msg.Mouse())
	case tea.MouseWheelMsg:
		m.wheel(msg.Mouse())
	case listMsg:
		m.metas = msg.metas
		if m.saved {
			m.touchMeta()
		}
	case genMsg:
		return m.generated(msg)
	case renderTickMsg:
		if msg.gen == m.gen {
			m.renderPending = false
			m.renderStreamNow()
		}
	case modelsMsg:
		m.gotModels(msg)
	}
	return nil
}
