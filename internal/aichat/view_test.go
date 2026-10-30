package aichat

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func checkFrame(t *testing.T, r *rig, w, h int) {
	t.Helper()
	lines := strings.Split(r.chat.View(), "\n")
	if len(lines) != h {
		t.Errorf("%dx%d: %d rows", w, h, len(lines))
	}
	for i, l := range lines {
		if got := lipgloss.Width(l); got != w {
			t.Errorf("%dx%d: row %d is %d columns: %q", w, h, i, got, stripANSI(l))
		}
	}
}

func TestViewFitsEveryWindowAndMode(t *testing.T) {
	r := newRig(t, withThinking(withUsage(text("# Heading\n\nSome *text* with a long line "+strings.Repeat("word ", 40)+"\n\n```\ncode\n```\n| a | b |\n|---|---|\n| 1 | 2 |"), 1, 2, 0.5), "hmm"))
	r.drive(r.chat.Init())
	r.ask("question " + strings.Repeat("wide ", 30))
	for _, size := range [][2]int{{100, 30}, {80, 24}, {72, 10}, {71, 10}, {40, 8}, {20, 5}, {10, 3}, {1, 1}} {
		r.chat.Resize(size[0], size[1])
		checkFrame(t, r, size[0], size[1])
		r.drive(r.press("ctrl+o"))
		checkFrame(t, r, size[0], size[1])
		r.press("esc")
		r.press("ctrl+s")
		checkFrame(t, r, size[0], size[1])
		r.press("esc")
		r.press("tab")
		r.press("r")
		checkFrame(t, r, size[0], size[1])
		r.press("esc")
		r.press("esc")
	}
}

func TestSidebarHiddenWhenNarrowOrToggled(t *testing.T) {
	r := newRig(t, text("a"))
	r.drive(r.chat.Init())
	r.ask("hello")
	if !strings.Contains(r.view(), "+ new chat") {
		t.Fatal("no sidebar at 100 columns")
	}
	r.press("ctrl+b")
	if strings.Contains(r.view(), "+ new chat") {
		t.Error("sidebar still shown after ctrl+b")
	}
	r.press("ctrl+b")
	r.chat.Resize(60, 20)
	if strings.Contains(r.view(), "+ new chat") {
		t.Error("sidebar shown at 60 columns")
	}
	r.press("tab")
	if r.chat.focus != focusInput {
		t.Error("tab focused a hidden sidebar")
	}
}

func TestSidebarSwitchRenameDelete(t *testing.T) {
	r := newRig(t, text("a1"), text("b1"))
	r.drive(r.chat.Init())
	r.ask("alpha question")
	first := r.chat.Conversation().ID
	r.press("ctrl+n")
	if r.chat.saved || len(r.chat.Conversation().Messages) != 0 {
		t.Fatal("ctrl+n did not start an empty conversation")
	}
	r.ask("beta question")
	second := r.chat.Conversation().ID
	if first == second {
		t.Fatal("same id twice")
	}
	if got := titles(r.chat); got != "beta question|alpha question" {
		t.Fatalf("sidebar = %s", got)
	}
	r.press("tab", "down", "down", "enter")
	if r.chat.Conversation().ID != first || len(r.chat.Conversation().Messages) != 2 || r.chat.focus != focusInput {
		t.Fatalf("did not open the first conversation: %s", r.chat.Conversation().ID)
	}
	r.press("tab")
	if r.chat.sideCursor != 2 {
		t.Errorf("sidebar cursor starts at %d, want the open conversation (2)", r.chat.sideCursor)
	}
	r.press("r")
	r.press("ctrl+u")
	r.typeText("Renamed one")
	r.press("enter")
	if got := r.saved(first).Title; got != "Renamed one" {
		t.Errorf("saved title = %q", got)
	}
	r.press("up", "r")
	r.press("ctrl+u")
	r.typeText("Renamed two")
	r.press("enter")
	if got := r.saved(second).Title; got != "Renamed two" || titles(r.chat) != "Renamed two|Renamed one" {
		t.Errorf("title %q sidebar %s", got, titles(r.chat))
	}
	r.press("d")
	if !strings.Contains(r.view(), "delete this conversation") {
		t.Error("no confirmation")
	}
	r.press("n")
	if _, err := r.chat.store.Load(second); err != nil {
		t.Error("deleted without confirmation")
	}
	r.press("d", "y")
	if _, err := r.chat.store.Load(second); err == nil {
		t.Error("confirmed delete kept the file")
	}
	r.press("down")
	r.press("d", "y")
	if titles(r.chat) != "" || r.chat.saved || r.chat.Conversation().ID == first {
		t.Errorf("deleting the open conversation: sidebar %q, saved %v", titles(r.chat), r.chat.saved)
	}
	if got := r.chat.store.List(); len(got) != 0 {
		t.Errorf("files left: %v", got)
	}
}

func titles(c *Chat) string {
	var out []string
	for _, m := range c.metas {
		out = append(out, m.Title)
	}
	return strings.Join(out, "|")
}

func TestNewConversationKeepsModelAndSearchChoices(t *testing.T) {
	r := newRig(t)
	r.chat.Conversation().Model = "fake/other"
	r.chat.Conversation().Search = true
	r.press("ctrl+n")
	if c := r.chat.Conversation(); c.Model != "fake/other" || !c.Search {
		t.Errorf("new conversation = %+v", c)
	}
}

func TestSwitchingStopsTheAnswerInFlight(t *testing.T) {
	r := newRig(t, withThinking(turn{hold: true}, "x"), text("b"))
	r.typeText("one")
	r.press("enter")
	r.chat.Update(r.chat.job.wait()())
	r.press("ctrl+n")
	if r.chat.Generating() {
		t.Error("still generating in the abandoned conversation")
	}
	list := r.chat.store.List()
	if len(list) != 1 {
		t.Fatalf("saved %d conversations", len(list))
	}
	saved := r.saved(list[0].ID)
	if len(saved.Messages) != 2 || saved.Messages[1].Error != errStopped {
		t.Errorf("saved = %+v", saved.Messages)
	}
}

func TestMouseSidebarClickAndWheel(t *testing.T) {
	r := newRig(t, text("a1"), text("b1"))
	r.drive(r.chat.Init())
	r.ask("alpha")
	r.press("ctrl+n")
	r.ask("beta")
	first := r.chat.metas[1].ID
	// row 0 is the header, row 1 "+ new chat", row 2 the newest, row 3 the older one
	r.chat.Update(tea.MouseClickMsg{X: 3, Y: 3, Button: tea.MouseLeft})
	if r.chat.Conversation().ID != first {
		t.Fatalf("click did not open the older conversation")
	}
	r.chat.Update(tea.MouseClickMsg{X: 3, Y: 1, Button: tea.MouseLeft})
	if r.chat.saved || len(r.chat.Conversation().Messages) != 0 {
		t.Error("click on new chat did not start one")
	}
	r.chat.Update(tea.MouseClickMsg{X: 3, Y: 20, Button: tea.MouseLeft})
	r.chat.Update(tea.MouseClickMsg{X: 3, Y: 0, Button: tea.MouseLeft})
}

func TestMouseWheelScrollsTranscript(t *testing.T) {
	r := newRig(t, text(strings.Repeat("line\n\n", 60)))
	r.chat.Resize(100, 20)
	r.ask("long")
	if r.chat.scroll != 0 {
		t.Fatal("not following the bottom")
	}
	bottom := r.view()
	r.chat.Update(tea.MouseWheelMsg{X: 50, Y: 5, Button: tea.MouseWheelUp})
	if r.chat.scroll != wheelLines || r.view() == bottom {
		t.Errorf("wheel up: scroll %d", r.chat.scroll)
	}
	r.chat.Update(tea.MouseWheelMsg{X: 50, Y: 5, Button: tea.MouseWheelDown})
	if r.chat.scroll != 0 {
		t.Errorf("wheel down: scroll %d", r.chat.scroll)
	}
	r.chat.Update(tea.MouseWheelMsg{X: 50, Y: 5, Button: tea.MouseWheelDown})
	if r.chat.scroll != 0 {
		t.Error("scrolled below the bottom")
	}
	for range 100 {
		r.chat.Update(tea.MouseWheelMsg{X: 50, Y: 5, Button: tea.MouseWheelUp})
	}
	if r.chat.scroll != r.chat.maxScroll() || !strings.Contains(r.view(), "you") {
		t.Errorf("scroll %d max %d", r.chat.scroll, r.chat.maxScroll())
	}
	r.press("pgdown")
	if r.chat.scroll >= r.chat.maxScroll() {
		t.Error("pgdown did not scroll")
	}
	r.press("pgup")
	r.drive(r.press("ctrl+r"))
	if r.chat.scroll != 0 {
		t.Error("a new answer does not jump to the bottom")
	}
}

func TestMouseSelectsModelInPicker(t *testing.T) {
	r := newRig(t)
	r.drive(r.press("ctrl+o"))
	// header row 0, title row 1, filter row 2, blank row 3, items from row 4
	r.chat.Update(tea.MouseClickMsg{X: 40, Y: 5, Button: tea.MouseLeft})
	if r.chat.mode != modeChat || r.chat.Conversation().Model != "fake/other" {
		t.Errorf("mode %v model %q", r.chat.mode, r.chat.Conversation().Model)
	}
}

// ctrl+w is the mux prefix and alt+1-9, alt+h/j/k/l and alt+arrows move between panes: Misa never delivers
// them to a pane's program as chat keys, and the chat must not use them either.
func TestMuxKeysAreNotBound(t *testing.T) {
	r := newRig(t, text("a"))
	r.ask("hello")
	r.typeText("draft")
	before := r.view()
	keys := []string{"ctrl+w", "alt+0", "alt+1", "alt+9", "alt+h", "alt+j", "alt+k", "alt+l", "alt+left", "alt+right", "alt+up", "alt+down"}
	for _, k := range keys {
		if cmd := r.press(k); cmd != nil {
			if msg := cmd(); msg != nil {
				t.Errorf("%s produced %T", k, msg)
			}
		}
		if r.chat.IsQuit() || r.chat.Generating() || r.chat.mode != modeChat || r.chat.input.Value() != "draft" {
			t.Fatalf("%s changed the chat state (input %q)", k, r.chat.input.Value())
		}
	}
	if strings.Count(r.view(), "draft") != strings.Count(before, "draft") {
		t.Error("view changed")
	}
}

func TestQuitKeys(t *testing.T) {
	r := newRig(t)
	r.typeText("x")
	r.press("ctrl+d")
	if r.chat.IsQuit() {
		t.Error("ctrl+d quit with text in the input")
	}
	r.press("ctrl+q")
	if !r.chat.IsQuit() {
		t.Error("ctrl+q did not quit")
	}
	r = newRig(t)
	r.press("ctrl+d")
	if !r.chat.IsQuit() {
		t.Error("ctrl+d on an empty input did not quit")
	}
	r = newRig(t, turn{hold: true})
	r.typeText("x")
	r.press("enter")
	r.press("ctrl+c")
	if r.chat.IsQuit() || r.chat.Generating() {
		t.Error("ctrl+c while answering should only stop")
	}
	r.press("ctrl+c")
	if !r.chat.IsQuit() {
		t.Error("ctrl+c when idle did not quit")
	}
}

func TestReopenedConversationRestoresEverything(t *testing.T) {
	r := newRig(t, withUsage(text("answer"), 7, 8, 0))
	r.press("ctrl+s")
	r.typeText("be terse")
	r.press("enter")
	r.ask("question")
	id := r.chat.Conversation().ID
	c := r.chatWith(Options{ID: id})
	got := c.Conversation()
	if got.System != "be terse" || got.Usage.In != 7 || len(got.Messages) != 2 || !c.saved {
		t.Errorf("reopened = %+v", got)
	}
	if !strings.Contains(stripANSI(c.View()), "answer") {
		t.Error("transcript not shown")
	}
}

func TestStreamingRendersAreThrottled(t *testing.T) {
	r := newRig(t, withThinking(turn{hold: true}, "x"))
	r.typeText("go")
	r.press("enter")
	gen := r.chat.gen
	feed := func(s string) tea.Cmd {
		return r.chat.Update(genMsg{gen: gen, events: []genEvent{{kind: evText, text: s}}})
	}
	feed("one ")
	r.chat.lastRender = time.Now()
	r.chat.renderPending = false
	before := strings.Join(r.chat.streamLines, "\n")
	cmd := feed("two ")
	if !r.chat.renderPending || cmd == nil {
		t.Fatal("second burst did not schedule a render")
	}
	if got := strings.Join(r.chat.streamLines, "\n"); got != before {
		t.Error("rendered again inside the 50 ms window")
	}
	if r.chat.last().Text != "one two " {
		t.Errorf("text lost: %q", r.chat.last().Text)
	}
	r.chat.Update(renderTickMsg{gen: gen})
	if !strings.Contains(stripANSI(strings.Join(r.chat.streamLines, "\n")), "one two") || r.chat.renderPending {
		t.Error("tick did not render")
	}
	r.chat.Update(renderTickMsg{gen: gen - 1})
	r.press("esc")
}

func TestParseArgs(t *testing.T) {
	for _, tc := range []struct {
		in   []string
		want Args
		err  string
	}{
		{nil, Args{}, ""},
		{[]string{"-m", "ollama/llama3.2"}, Args{Model: "ollama/llama3.2"}, ""},
		{[]string{"--model", "a/b", "20260101"}, Args{Model: "a/b", ID: "20260101"}, ""},
		{[]string{"id1", "--model=x/y"}, Args{Model: "x/y", ID: "id1"}, ""},
		{[]string{"-m"}, Args{}, "needs"},
		{[]string{"-x"}, Args{}, "unknown option"},
		{[]string{"a", "b"}, Args{}, "one conversation id"},
		{[]string{"-h"}, Args{}, "usage"},
	} {
		got, err := ParseArgs(tc.in)
		if tc.err == "" && (err != nil || got != tc.want) || tc.err != "" && (err == nil || !strings.Contains(err.Error(), tc.err)) {
			t.Errorf("ParseArgs(%v) = %+v, %v", tc.in, got, err)
		}
	}
}
