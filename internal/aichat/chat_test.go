package aichat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ai"
	"github.com/AmadeusDE/Misa/internal/clipboard"
	"github.com/AmadeusDE/Misa/internal/ugh/llm"
)

func TestSendStreamsAnswerAndSaves(t *testing.T) {
	r := newRig(t, withUsage(text("Hello ", "**there**"), 12, 34, 0.0123))
	r.ask("hi bot")
	c := r.chat.Conversation()
	if len(c.Messages) != 2 || c.Messages[0].Text != "hi bot" || c.Messages[1].Text != "Hello **there**" {
		t.Fatalf("messages = %+v", c.Messages)
	}
	if r.chat.Generating() {
		t.Error("still generating after the answer")
	}
	if c.Title != "hi bot" || c.Usage.In != 12 || c.Usage.Out != 34 {
		t.Errorf("title %q usage %+v", c.Title, c.Usage)
	}
	req := r.p.requests()[0]
	if len(req.Messages) != 1 || req.Messages[0].Role != llm.RoleUser || req.Messages[0].Text != "hi bot" || req.Model != "model" {
		t.Errorf("request = %+v", req)
	}
	v := r.view()
	for _, want := range []string{"you", "hi bot", "assistant", "there", "12 in 34 out $0.0123", "hi bot  ·  fake/model"} {
		if !strings.Contains(v, want) {
			t.Errorf("view lacks %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "**") {
		t.Errorf("markdown not rendered:\n%s", v)
	}
	got := r.saved(c.ID)
	if got.Messages[1].Text != "Hello **there**" || got.Messages[1].Model != "fake/model" {
		t.Errorf("saved = %+v", got.Messages)
	}
}

func TestSecondTurnSendsHistory(t *testing.T) {
	r := newRig(t, text("one"), text("two"))
	r.ask("first")
	r.ask("second")
	reqs := r.p.requests()
	var got []string
	for _, m := range reqs[1].Messages {
		got = append(got, string(m.Role)+":"+m.Text)
	}
	if strings.Join(got, "|") != "user:first|assistant:one|user:second" {
		t.Errorf("history = %v", got)
	}
}

func TestMultilineInputAndEmptySend(t *testing.T) {
	r := newRig(t, text("ok"))
	r.drive(r.press("enter"))
	if len(r.chat.Conversation().Messages) != 0 {
		t.Fatal("empty input was sent")
	}
	r.typeText("line one")
	r.press("alt+enter")
	r.typeText("line two")
	r.press("ctrl+j")
	r.typeText("three")
	if got := r.chat.input.Value(); got != "line one\nline two\nthree" {
		t.Fatalf("input = %q", got)
	}
	if r.chat.input.Height() != 3 {
		t.Errorf("input height %d, want 3", r.chat.input.Height())
	}
	r.drive(r.press("enter"))
	if got := r.chat.Conversation().Messages[0].Text; got != "line one\nline two\nthree" {
		t.Errorf("sent %q", got)
	}
	if r.chat.input.Value() != "" {
		t.Error("input not cleared")
	}
}

func TestThinkingCollapsedUntilToggled(t *testing.T) {
	r := newRig(t, withThinking(text("answer"), "secret reasoning\nsecond line"))
	r.ask("q")
	v := r.view()
	if !strings.Contains(v, "▸ thinking (2 lines") || strings.Contains(v, "secret reasoning") {
		t.Errorf("collapsed view wrong:\n%s", v)
	}
	r.press("ctrl+t")
	v = r.view()
	if !strings.Contains(v, "secret reasoning") || !strings.Contains(v, "▾ thinking") {
		t.Errorf("expanded view wrong:\n%s", v)
	}
	if !strings.Contains(r.chat.transcript()[3], "\x1b[") {
		t.Log("thinking is not styled")
	}
}

func TestStopKeepsPartialAndDropsLateEvents(t *testing.T) {
	partial := text("partial ")
	partial.hold = true
	r := newRig(t, partial)
	r.typeText("go")
	cmd := r.press("enter")
	_ = cmd
	// the first batch carries the partial text
	r.chat.Update(r.chat.job.wait()())
	if got := r.chat.last().Text; got != "partial " {
		t.Fatalf("streamed %q", got)
	}
	old := r.chat.gen
	r.press("esc")
	if r.chat.Generating() {
		t.Fatal("esc did not stop")
	}
	if last := r.chat.last(); last.Text != "partial " || last.Error != errStopped {
		t.Errorf("last = %+v", last)
	}
	r.chat.Update(genMsg{gen: old, events: []genEvent{{kind: evText, text: "LATE"}, {kind: evDone}}})
	if strings.Contains(r.chat.last().Text, "LATE") {
		t.Error("event of a stopped generation was applied")
	}
	if !strings.Contains(r.view(), "[stopped]") {
		t.Errorf("no stopped marker:\n%s", r.view())
	}
	if got := r.saved(r.chat.Conversation().ID).Messages[1].Error; got != errStopped {
		t.Errorf("saved error = %q", got)
	}
}

func TestStopBeforeAnyOutputRemovesEmptyAnswer(t *testing.T) {
	r := newRig(t, turn{hold: true})
	r.typeText("go")
	r.press("enter")
	r.press("esc")
	if msgs := r.chat.Conversation().Messages; len(msgs) != 1 || msgs[0].Role != "user" {
		t.Errorf("messages = %+v", msgs)
	}
}

func TestSendWhileAnsweringIsRefused(t *testing.T) {
	r := newRig(t, turn{hold: true})
	r.typeText("one")
	r.press("enter")
	r.typeText("two")
	r.press("enter")
	if len(r.chat.Conversation().Messages) != 2 || !strings.Contains(r.chat.notice, "esc") {
		t.Errorf("messages %d, notice %q", len(r.chat.Conversation().Messages), r.chat.notice)
	}
	if r.chat.input.Value() != "two" {
		t.Errorf("input lost: %q", r.chat.input.Value())
	}
	r.press("esc")
}

func TestRegenerateReplacesLastAnswer(t *testing.T) {
	r := newRig(t, text("first try"), text("second try"))
	r.ask("q")
	r.drive(r.press("ctrl+r"))
	msgs := r.chat.Conversation().Messages
	if len(msgs) != 2 || msgs[1].Text != "second try" {
		t.Fatalf("messages = %+v", msgs)
	}
	for _, m := range r.p.requests()[1].Messages {
		if m.Text == "first try" {
			t.Error("the old answer was sent again")
		}
	}
}

func TestEditLastResendsThatTurn(t *testing.T) {
	r := newRig(t, text("a1"), text("a2"), text("a3"))
	r.ask("first")
	r.ask("second")
	r.press("up")
	if r.chat.input.Value() != "second" || r.chat.editing != 2 {
		t.Fatalf("input %q editing %d", r.chat.input.Value(), r.chat.editing)
	}
	r.typeText(" edited")
	r.drive(r.press("enter"))
	var texts []string
	for _, m := range r.chat.Conversation().Messages {
		texts = append(texts, m.Text)
	}
	if got := strings.Join(texts, "|"); got != "first|a1|second edited|a3" {
		t.Errorf("messages = %s", got)
	}
}

func TestEditLastCanBeCancelled(t *testing.T) {
	r := newRig(t, text("a1"))
	r.ask("first")
	r.press("up")
	r.press("esc")
	if r.chat.input.Value() != "" || r.chat.editing != -1 || len(r.chat.Conversation().Messages) != 2 {
		t.Errorf("input %q editing %d messages %d", r.chat.input.Value(), r.chat.editing, len(r.chat.Conversation().Messages))
	}
}

func TestUpWithTextMovesTheCursorInstead(t *testing.T) {
	r := newRig(t, text("a1"))
	r.ask("first")
	r.typeText("draft")
	r.press("up")
	if r.chat.editing != -1 || r.chat.input.Value() != "draft" {
		t.Errorf("up with a draft started an edit")
	}
}

func TestCopyLastAnswer(t *testing.T) {
	r := newRig(t, text("# Title\nbody"))
	r.press("ctrl+y")
	if len(r.copied) != 0 || !r.chat.noticeErr {
		t.Error("copied without an answer")
	}
	r.ask("q")
	r.press("ctrl+y")
	if len(r.copied) != 1 || r.copied[0] != "# Title\nbody" {
		t.Errorf("copied = %q", r.copied)
	}
}

func TestProviderErrorShownAndRetryable(t *testing.T) {
	r := newRig(t, turn{err: errors.New("HTTP 429: slow down")}, text("fine now"))
	r.ask("q")
	msgs := r.chat.Conversation().Messages
	if msgs[1].Error != "HTTP 429: slow down" || !strings.Contains(r.view(), "error: HTTP 429") {
		t.Errorf("error not shown: %+v\n%s", msgs, r.view())
	}
	r.drive(r.press("ctrl+r"))
	msgs = r.chat.Conversation().Messages
	if len(msgs) != 2 || msgs[1].Text != "fine now" || msgs[1].Error != "" {
		t.Errorf("after retry: %+v", msgs)
	}
}

func TestNoModelSaysRunAIConfig(t *testing.T) {
	r := newRig(t)
	r.chat.opt.ModelFor = func() (string, error) { return "", errors.New("no model chosen for chat: run `aiconfig` to pick one") }
	r.ask("hello")
	if !strings.Contains(r.chat.notice, "aiconfig") || !r.chat.noticeErr {
		t.Errorf("notice = %q", r.chat.notice)
	}
	if !strings.Contains(r.view(), "aiconfig") {
		t.Error("header does not point at aiconfig")
	}
	if len(r.chat.Conversation().Messages) != 1 {
		t.Error("an empty answer was added")
	}
}

func TestSystemPromptIsSentAndSaved(t *testing.T) {
	r := newRig(t, text("ok"))
	r.press("ctrl+s")
	r.typeText("Answer in French.")
	r.press("alt+enter")
	r.typeText("Briefly.")
	r.press("enter")
	if r.chat.mode != modeChat || r.chat.Conversation().System != "Answer in French.\nBriefly." {
		t.Fatalf("mode %v system %q", r.chat.mode, r.chat.Conversation().System)
	}
	r.ask("bonjour")
	if got := r.p.requests()[0].System; got != "Answer in French.\nBriefly." {
		t.Errorf("System = %q", got)
	}
	if got := r.saved(r.chat.Conversation().ID).System; got != "Answer in French.\nBriefly." {
		t.Errorf("saved system = %q", got)
	}
	r.press("ctrl+s")
	r.press("esc")
	if r.chat.Conversation().System == "" {
		t.Error("esc cleared the prompt")
	}
}

func TestModelPicker(t *testing.T) {
	r := newRig(t, text("ok"))
	r.drive(r.press("ctrl+o"))
	if r.chat.mode != modePicker || len(r.chat.picker.items) != 3 {
		t.Fatalf("picker not loaded: %+v", r.chat.picker)
	}
	v := r.view()
	if !strings.Contains(v, "fake/other") || !strings.Contains(v, "broken: HTTP 500") {
		t.Errorf("picker view:\n%s", v)
	}
	r.typeText("oth")
	if got := r.chat.picker.matches(); len(got) != 1 || got[0] != "fake/other" {
		t.Fatalf("matches = %v", got)
	}
	r.press("enter")
	if r.chat.mode != modeChat || r.chat.Conversation().Model != "fake/other" {
		t.Fatalf("mode %v model %q", r.chat.mode, r.chat.Conversation().Model)
	}
	r.ask("hi")
	if len(r.opened) != 1 || r.opened[0] != "fake/other" || !strings.Contains(r.view(), "fake/other") {
		t.Errorf("opened %v", r.opened)
	}
}

func TestModelPickerTypedRefAndEscape(t *testing.T) {
	r := newRig(t)
	r.drive(r.press("ctrl+o"))
	r.typeText("custom/model-x")
	r.press("enter")
	if got := r.chat.Conversation().Model; got != "custom/model-x" {
		t.Errorf("model = %q", got)
	}
	r.drive(r.press("ctrl+o"))
	r.typeText("nomatch")
	r.press("enter")
	if r.chat.mode != modePicker {
		t.Error("enter on a non-ref left the picker")
	}
	r.press("esc")
	if r.chat.mode != modeChat || r.chat.Conversation().Model != "custom/model-x" {
		t.Error("esc changed the model")
	}
}

func TestStaleModelListIsDropped(t *testing.T) {
	r := newRig(t)
	r.chat.openPicker()
	r.chat.Update(modelsMsg{gen: r.chat.picker.gen - 1, refs: []string{"stale/x"}})
	if len(r.chat.picker.items) != 0 {
		t.Error("stale list applied")
	}
}

func TestModelFlagOverridesSavedConversation(t *testing.T) {
	r := newRig(t, text("a"))
	r.ask("hello")
	id := r.chat.Conversation().ID
	c := r.chatWith(Options{ID: id, Model: "flag/model"})
	if c.Conversation().Model != "flag/model" || len(c.Conversation().Messages) != 2 {
		t.Errorf("conversation = %+v", c.Conversation())
	}
	if _, err := New(Options{Dir: r.chat.store.Dir, ID: "nope"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown id: %v", err)
	}
}

func fakeSearchTool(got *[]string) ai.Tool {
	return ai.Tool{
		Spec: llm.ToolSpec{Name: "web_search", Description: "search", Parameters: map[string]any{"type": "object"}},
		Run: func(_ context.Context, args json.RawMessage) (string, error) {
			var a struct{ Query string }
			_ = json.Unmarshal(args, &a)
			*got = append(*got, a.Query)
			return "1. Cats\n   https://cats.example", nil
		},
	}
}

func TestWebSearchToggleAndToolNotes(t *testing.T) {
	r := newRig(t,
		toolCall("c1", "web_search", `{"query":"cat facts"}`),
		withUsage(text("Cats purr."), 5, 6, 0),
	)
	var queries []string
	r.tools = []ai.Tool{fakeSearchTool(&queries)}
	if strings.Contains(r.view(), "web") && strings.Contains(strings.Split(r.view(), "\n")[0], "· web") {
		t.Error("search shown as on before toggling")
	}
	r.press("ctrl+g")
	if !r.chat.Conversation().Search || !strings.Contains(strings.Split(r.view(), "\n")[0], "web") {
		t.Fatalf("search not on: %q", r.chat.notice)
	}
	r.ask("tell me about cats")
	if len(queries) != 1 || queries[0] != "cat facts" {
		t.Fatalf("queries = %v", queries)
	}
	if got := r.p.requests()[0].Tools; len(got) != 1 || got[0].Name != "web_search" {
		t.Errorf("tools offered = %+v", got)
	}
	last := r.chat.last()
	if last.Text != "Cats purr." || len(last.Notes) != 1 || last.Notes[0] != "searched: cat facts" {
		t.Errorf("answer = %+v", last)
	}
	if v := r.view(); !strings.Contains(v, "· searched: cat facts") {
		t.Errorf("tool call not shown:\n%s", v)
	}
	r.press("ctrl+g")
	if r.chat.Conversation().Search {
		t.Error("search not toggled off")
	}
}

func TestSearchOffInSettingsCannotBeToggledOn(t *testing.T) {
	r := newRig(t)
	r.press("ctrl+g")
	if r.chat.Conversation().Search || !r.chat.noticeErr || !strings.Contains(r.chat.notice, "ai.yaml") {
		t.Errorf("search %v notice %q", r.chat.Conversation().Search, r.chat.notice)
	}
}

func TestToolRejectionRetriesOnceWithoutTools(t *testing.T) {
	r := newRig(t,
		turn{err: errors.New("provider: HTTP 400: this model does not support tools")},
		withUsage(text("plain answer"), 3, 4, 0),
	)
	var queries []string
	r.tools = []ai.Tool{fakeSearchTool(&queries)}
	r.press("ctrl+g")
	r.ask("hi")
	reqs := r.p.requests()
	if len(reqs) != 2 || len(reqs[0].Tools) == 0 || len(reqs[1].Tools) != 0 {
		t.Fatalf("requests = %+v", reqs)
	}
	last := r.chat.last()
	if last.Text != "plain answer" || last.Error != "" || len(last.Notes) != 1 || !strings.Contains(last.Notes[0], "without web search") {
		t.Errorf("answer = %+v", last)
	}
	if r.chat.Conversation().Search {
		t.Error("search stays on after the model rejected tools")
	}
}

func TestOtherErrorsAreNotRetried(t *testing.T) {
	r := newRig(t, turn{err: errors.New("HTTP 401: bad key")}, text("never"))
	var queries []string
	r.tools = []ai.Tool{fakeSearchTool(&queries)}
	r.press("ctrl+g")
	r.ask("hi")
	if n := len(r.p.requests()); n != 1 {
		t.Errorf("%d requests, want 1", n)
	}
	if !strings.Contains(r.chat.last().Error, "401") {
		t.Errorf("answer = %+v", r.chat.last())
	}
}

type fakeClipboard struct {
	err  error
	text string
}

func (f *fakeClipboard) Read() (string, error) { return "", f.err }
func (f *fakeClipboard) Write(s string) error  { f.text = s; return f.err }

func TestCopyFallsBackToOSC52OnlyWithoutHostClipboard(t *testing.T) {
	host := &fakeClipboard{}
	clipboard.Use(host)
	t.Cleanup(func() { clipboard.Use(nil) })
	if cmd := copyText("abc"); cmd != nil || host.text != "abc" {
		t.Errorf("host clipboard not used: cmd %v text %q", cmd, host.text)
	}
	host.err = clipboard.ErrUnavailable
	if cmd := copyText("abc"); cmd == nil {
		t.Error("no OSC 52 fallback")
	}
}
