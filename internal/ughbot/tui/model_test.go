package tui

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/webdevtui"
	"github.com/charmbracelet/x/ansi"
)

type fakeBackend struct {
	mu     sync.Mutex
	bots   []api.BotInfo
	rows   []api.CostRow
	status api.Status
	sent   []string
	after  []uint64
	costQ  []string
	subErr error
}

func (f *fakeBackend) ListBots(context.Context) ([]api.BotInfo, error) { return f.bots, nil }
func (f *fakeBackend) Status(context.Context) (api.Status, error)      { return f.status, nil }
func (f *fakeBackend) Cost(_ context.Context, q api.CostQuery) ([]api.CostRow, error) {
	f.costQ = append(f.costQ, q.Bot)
	var out []api.CostRow
	for _, r := range f.rows {
		if q.Bot == "" || r.Bot == q.Bot {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeBackend) Send(_ context.Context, _, text string) (string, error) {
	f.sent = append(f.sent, text)
	return "1", nil
}
func (f *fakeBackend) Subscribe(_ context.Context, _ string, after uint64) (<-chan api.Event, error) {
	f.after = append(f.after, after)
	if f.subErr != nil {
		return nil, f.subErr
	}
	return make(chan api.Event), nil
}

func newFake() *fakeBackend {
	return &fakeBackend{
		bots:   []api.BotInfo{{Name: "a", State: "running", Model: "m1"}, {Name: "b", State: "idle", Model: "m2"}},
		rows:   []api.CostRow{{Bot: "a", Day: "2026-01-02", USD: 0.5}, {Bot: "b", Day: "2026-01-02", USD: 0.25}, {Bot: "a", Day: "2026-01-01", USD: 1}},
		status: api.Status{Version: "v9", UptimeSeconds: 3700, Bots: []api.BotInfo{{Name: "a", State: "running"}}},
	}
}

// run executes cmd and feeds its message back, ignoring ticks and blocking waits.
func run(m *Model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && strings.Contains(v.Type().Name(), "sequence") {
		for i := 0; i < v.Len(); i++ {
			if c, ok := v.Index(i).Interface().(tea.Cmd); ok {
				run(m, c)
			}
		}
		return
	}
	switch msg.(type) {
	case botsMsg, statusMsg, costMsg, streamMsg, failedMsg, sendErrMsg, botDefMsg, botSavedMsg, approvalsMsg, decidedMsg, routinesMsg, routineDoneMsg, reposMsg, repoDoneMsg:
	default:
		if !webdevtui.Owns(msg) {
			return
		}
	}
	_, next := m.Update(msg)
	if _, ok := msg.(streamMsg); !ok {
		run(m, next)
	}
}

func press(m *Model, s string) tea.Cmd {
	var k tea.KeyPressMsg
	switch s {
	case "enter":
		k = tea.KeyPressMsg{Code: tea.KeyEnter}
	case "tab":
		k = tea.KeyPressMsg{Code: tea.KeyTab}
	case "esc":
		k = tea.KeyPressMsg{Code: tea.KeyEscape}
	case "down":
		k = tea.KeyPressMsg{Code: tea.KeyDown}
	case "right":
		k = tea.KeyPressMsg{Code: tea.KeyRight}
	case "ctrl+c":
		k = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "ctrl+o":
		k = tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl}
	case "ctrl+s":
		k = tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	case "shift+tab":
		k = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	default:
		k = tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
	}
	_, cmd := m.Update(k)
	return cmd
}

func typeText(m *Model, s string) {
	for _, r := range s {
		press(m, string(r))
	}
}

func ready(t *testing.T, be Backend, bot string) *Model {
	t.Helper()
	m := New(context.Background(), be, bot)
	m.Update(tea.WindowSizeMsg{Width: 70, Height: 20})
	run(m, m.fetchBots())
	return m
}

func ev(m *Model, seq uint64, typ string, data any) {
	raw, _ := json.Marshal(data)
	m.Update(eventMsg{m.chat.id, api.Event{Seq: seq, Type: typ, Data: raw}})
}

func wantIn(t *testing.T, view string, wants ...string) {
	t.Helper()
	view = ansi.Strip(view)
	for _, w := range wants {
		if !strings.Contains(view, w) {
			t.Errorf("view missing %q:\n%s", w, view)
		}
	}
}

func TestPickerListsFiltersAndOpensChat(t *testing.T) {
	be := newFake()
	m := ready(t, be, "")
	wantIn(t, m.View().Content, "Bots", "running", "m1", "idle", "m2")

	press(m, "/")
	typeText(m, "idl")
	press(m, "enter")
	view := ansi.Strip(m.View().Content)
	if strings.Contains(view, "m1") || !strings.Contains(view, "m2") {
		t.Fatalf("filter failed:\n%s", view)
	}
	run(m, press(m, "enter"))
	if m.screen != screenChat || m.chat.bot != "b" {
		t.Fatalf("screen=%v chat=%+v", m.screen, m.chat)
	}
	wantIn(t, m.View().Content, "connecting to b")
	m.Update(streamMsg{m.chat.id, make(chan api.Event)})
	wantIn(t, m.View().Content, "attached to b")
}

func TestPickerToChatSendsMultilineMessage(t *testing.T) {
	be := newFake()
	m := ready(t, be, "")
	run(m, press(m, "enter"))
	typeText(m, "hi")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt})
	typeText(m, "there")
	cmd := press(m, "enter")
	if cmd == nil {
		t.Fatal("enter produced no command")
	}
	cmd()
	if len(be.sent) != 1 || be.sent[0] != "hi\nthere" || m.chat.input.Value() != "" {
		t.Fatalf("sent=%q input=%q", be.sent, m.chat.input.Value())
	}
}

func TestChatRendersStreamedConversationAndRemembersSeq(t *testing.T) {
	m := ready(t, newFake(), "a")
	ev(m, 5, "user", map[string]string{"text": "hi"})
	ev(m, 6, "text", map[string]string{"text": "hel"})
	ev(m, 7, "text", map[string]string{"text": "lo"})
	ev(m, 8, "tool_call", map[string]string{"name": "read", "args": `{"path":"x"}`})
	ev(m, 9, "budget", map[string]string{"message": "budget exhausted"})
	ev(m, 10, "error", map[string]string{"message": "boom"})
	wantIn(t, m.View().Content, "> hi", "hello", "* read", "! budget exhausted", "! error: boom")
	if m.chat.lastSeq != 10 {
		t.Errorf("lastSeq = %d", m.chat.lastSeq)
	}
}

func TestChatStreamingTextAppendsOnOneLine(t *testing.T) {
	m := ready(t, newFake(), "a")
	ev(m, 1, "text", map[string]string{"text": "par"})
	wantIn(t, m.View().Content, "par")
	ev(m, 2, "text", map[string]string{"text": "tial"})
	wantIn(t, m.View().Content, "partial")
	ev(m, 3, "turn_end", nil)
	ev(m, 4, "text", map[string]string{"text": "next"})
	wantIn(t, m.View().Content, "partial", "next")
}

func TestChatReconnectResumesFromLastSeq(t *testing.T) {
	be := newFake()
	m := ready(t, be, "a")
	run(m, m.chat.subscribe())
	ev(m, 42, "text", map[string]string{"text": "x"})
	_, cmd := m.Update(closedMsg{m.chat.id})
	if cmd == nil {
		t.Fatal("no retry scheduled")
	}
	wantIn(t, m.View().Content, "disconnected, retrying (attempt 1)")
	_, sub := m.Update(retryMsg{m.chat.id})
	wantIn(t, m.View().Content, "connecting to a")
	run(m, sub)
	if got := be.after[len(be.after)-1]; got != 42 {
		t.Fatalf("resubscribed after %d, want 42 (all: %v)", got, be.after)
	}
}

func TestChatFailedSubscribeShowsErrorAndRetries(t *testing.T) {
	be := newFake()
	be.subErr = errors.New("daemon gone")
	m := ready(t, be, "a")
	_, cmd := m.Update(failedMsg{m.chat.id, be.subErr})
	if cmd == nil {
		t.Fatal("no retry")
	}
	wantIn(t, m.View().Content, "retrying", "daemon gone")
}

func TestStaleChatMessagesAreDropped(t *testing.T) {
	m := ready(t, newFake(), "a")
	old := m.chat.id
	run(m, m.openChat("b"))
	m.Update(eventMsg{old, api.Event{Seq: 3, Type: "note", Data: json.RawMessage(`{"text":"stale"}`)}})
	if strings.Contains(ansi.Strip(m.View().Content), "stale") || m.chat.lastSeq != 0 {
		t.Fatal("stale event leaked into new chat")
	}
}

func TestCostViewRendersBarsAndFiltersByBot(t *testing.T) {
	be := newFake()
	m := ready(t, be, "")
	for m.screen != screenCost {
		run(m, press(m, "tab"))
	}
	view := m.View().Content
	wantIn(t, view, "Total per bot", "2026-01-02", "$1.5000", "$1.7500", "$0.2500", "█", "all bots")
	run(m, press(m, "right"))
	view = ansi.Strip(m.View().Content)
	wantIn(t, view, "bot: a", "$1.5000")
	if strings.Contains(view, "$0.2500") {
		t.Errorf("bot b spend leaked into filter a:\n%s", view)
	}
	if be.costQ[len(be.costQ)-1] != "a" {
		t.Errorf("cost queries = %v", be.costQ)
	}
}

func TestStatusViewShowsVersionUptimeAndBots(t *testing.T) {
	m := ready(t, newFake(), "")
	run(m, press(m, "tab"))
	run(m, press(m, "tab"))
	if m.screen != screenStatus {
		t.Fatalf("screen = %v", m.screen)
	}
	wantIn(t, m.View().Content, "v9", "1h 1m", "running")
}

func TestNavigationHelpAndQuit(t *testing.T) {
	m := ready(t, newFake(), "a")
	run(m, press(m, "ctrl+o"))
	if m.screen != screenCost {
		t.Fatalf("ctrl+o from chat -> %v", m.screen)
	}
	press(m, "?")
	wantIn(t, m.View().Content, "Help", "ctrl+o")
	press(m, "x")
	if m.help {
		t.Fatal("help not dismissed")
	}
	if cmd := press(m, "q"); cmd == nil {
		t.Fatal("q did not quit")
	}
	m2 := ready(t, newFake(), "a")
	if cmd := press(m2, "q"); cmd == nil {
		t.Fatal("expected typed q to go to input, got quit command")
	} else if _, ok := cmd().(tea.QuitMsg); ok {
		t.Fatal("q quit while typing in chat")
	}
	if _, ok := press(m2, "ctrl+c")().(tea.QuitMsg); !ok {
		t.Fatal("ctrl+c did not quit")
	}
}

func TestResizeKeepsViewWithinBounds(t *testing.T) {
	m := ready(t, newFake(), "a")
	for _, sz := range [][2]int{{40, 10}, {120, 40}, {20, 6}} {
		m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		lines := strings.Split(m.View().Content, "\n")
		if len(lines) > sz[1] {
			t.Errorf("%v: %d lines", sz, len(lines))
		}
	}
}

func TestBarScalesAndClamps(t *testing.T) {
	if got := bar(10, 1); got != strings.Repeat("█", 10) {
		t.Errorf("full bar = %q", got)
	}
	if bar(10, 0) != "" || bar(10, 0.5) != "█████" {
		t.Errorf("bars: %q %q", bar(10, 0), bar(10, 0.5))
	}
	if bar(4, 2) != "████" {
		t.Error("not clamped")
	}
}

func TestHelpFitsAnEightyColumnTerminalAndScrolls(t *testing.T) {
	for _, l := range helpLines {
		if len([]rune(l)) > 76 {
			t.Errorf("help line wraps at 80 columns: %q", l)
		}
	}
	m := ready(t, newFake(), "")
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	press(m, "?")
	wantIn(t, m.View().Content, "Everywhere", "scroll")
	for range helpLines {
		press(m, "down")
	}
	wantIn(t, m.View().Content, "web terminal")
	press(m, "x")
	if m.help {
		t.Fatal("another key should close the help")
	}
}
