package webdevtui

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

type fakeAdmin struct {
	devices  []Device
	sessions []Session
	events   []Event
	notices  []Notice
	calls    []string
	failWith error
}

func (f *fakeAdmin) Devices() ([]Device, error)   { return f.devices, nil }
func (f *fakeAdmin) Sessions() ([]Session, error) { return f.sessions, nil }
func (f *fakeAdmin) Events(limit int) ([]Event, error) {
	return f.events, nil
}
func (f *fakeAdmin) Notices() ([]Notice, error) { return f.notices, nil }
func (f *fakeAdmin) Revoke(id string) error {
	f.calls = append(f.calls, "revoke "+id)
	return f.failWith
}
func (f *fakeAdmin) Rename(id, name string) error {
	f.calls = append(f.calls, "rename "+id+" "+name)
	return f.failWith
}
func (f *fakeAdmin) KillSession(p string) error {
	f.calls = append(f.calls, "kill "+p)
	return f.failWith
}
func (f *fakeAdmin) DismissNotice(id string) error {
	f.calls = append(f.calls, "dismiss "+id)
	return f.failWith
}

func newFake() *fakeAdmin {
	at := time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)
	return &fakeAdmin{
		devices:  []Device{{ID: "d1", Name: "phone", Created: at, LastSeen: at}, {ID: "d2", Name: "laptop", Created: at, Revoked: true}},
		sessions: []Session{{ID: "abcdef123456", DeviceID: "d1", DeviceName: "phone", IP: "10.0.0.2", Created: at, Expires: at.Add(time.Hour)}},
		events:   []Event{{At: at, Kind: "enroll", Name: "phone"}, {At: at.Add(time.Minute), Kind: "revoke", Name: "laptop"}},
		notices:  []Notice{{ID: "n1", Text: "New device enrolled: phone"}},
	}
}

// runCmd executes cmd, expanding batches and sequences, and feeds our results back.
func runCmd(m *Model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice {
		for i := 0; i < v.Len(); i++ {
			if c, ok := v.Index(i).Interface().(tea.Cmd); ok {
				runCmd(m, c)
			}
		}
		return
	}
	if Owns(msg) {
		runCmd(m, m.Update(msg))
	}
}

func send(m *Model, s string) tea.Cmd {
	var k tea.KeyPressMsg
	switch s {
	case "enter":
		k = tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		k = tea.KeyPressMsg{Code: tea.KeyEscape}
	case "down":
		k = tea.KeyPressMsg{Code: tea.KeyDown}
	case "right":
		k = tea.KeyPressMsg{Code: tea.KeyRight}
	default:
		k = tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
	}
	return m.Update(k)
}

func load(t *testing.T, a *fakeAdmin) *Model {
	t.Helper()
	m := New(a)
	m.Resize(90, 20)
	m.Update(m.reload()())
	return m
}

func text(m *Model) string { return ansi.Strip(m.View()) }

func wantIn(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in:\n%s", w, got)
		}
	}
}

func TestDevicesListShowsColumnsStateAndNoticeBanner(t *testing.T) {
	m := load(t, newFake())
	wantIn(t, text(m), "NAME", "phone", "d1", "2026-01", "active", "laptop", "revoked", "New device enrolled: phone", "dismiss")
}

func TestRevokeAsksConfirmationAndOnlyYRuns(t *testing.T) {
	a := newFake()
	m := load(t, a)
	send(m, "x")
	if !m.Capturing() {
		t.Fatal("confirmation must capture keys")
	}
	wantIn(t, text(m), "Revoke phone", "y/N")
	send(m, "n")
	if len(a.calls) != 0 || m.Capturing() {
		t.Fatalf("calls=%v capturing=%v", a.calls, m.Capturing())
	}
	send(m, "x")
	cmd := send(m, "y")
	runCmd(m, cmd)
	if len(a.calls) != 1 || a.calls[0] != "revoke d1" {
		t.Fatalf("calls = %v", a.calls)
	}
	wantIn(t, text(m), "revoked phone")
}

func TestRevokingRevokedDeviceIsRefused(t *testing.T) {
	a := newFake()
	m := load(t, a)
	send(m, "j")
	send(m, "x")
	if m.Capturing() || len(a.calls) != 0 {
		t.Fatal("revoked device must not prompt")
	}
	wantIn(t, text(m), "already revoked")
}

func TestRenameEditsPrefilledNameAndEscCancels(t *testing.T) {
	a := newFake()
	m := load(t, a)
	send(m, "r")
	if !m.Capturing() {
		t.Fatal("rename must capture keys")
	}
	for i := 0; i < len("phone"); i++ {
		m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	}
	for _, r := range "tablet" {
		send(m, string(r))
	}
	runCmd(m, send(m, "enter"))
	if len(a.calls) != 1 || a.calls[0] != "rename d1 tablet" {
		t.Fatalf("calls = %v", a.calls)
	}
	send(m, "r")
	send(m, "esc")
	if m.Capturing() || m.Quit() || len(a.calls) != 1 {
		t.Fatal("esc must only cancel the prompt")
	}
}

func TestSessionsViewKillsWithKAfterConfirm(t *testing.T) {
	a := newFake()
	m := load(t, a)
	send(m, "right")
	wantIn(t, text(m), "SESSION", "abcdef123456", "10.0.0.2")
	send(m, "k")
	wantIn(t, text(m), "End session")
	runCmd(m, send(m, "y"))
	if len(a.calls) != 1 || a.calls[0] != "kill abcdef123456" {
		t.Fatalf("calls = %v", a.calls)
	}
}

func TestEventsNewestFirst(t *testing.T) {
	m := load(t, newFake())
	send(m, "right")
	send(m, "right")
	v := text(m)
	wantIn(t, v, "WHEN", "enroll", "revoke")
	v = v[strings.Index(v, "WHEN"):]
	if strings.Index(v, "revoke") > strings.Index(v, "enroll") {
		t.Errorf("events not newest first:\n%s", v)
	}
}

func TestDismissNotice(t *testing.T) {
	a := newFake()
	m := load(t, a)
	runCmd(m, send(m, "d"))
	if len(a.calls) != 1 || a.calls[0] != "dismiss n1" {
		t.Fatalf("calls = %v", a.calls)
	}
}

func TestActionErrorIsShownAndSurvivesReload(t *testing.T) {
	a := newFake()
	a.failWith = errors.New("no device")
	m := load(t, a)
	send(m, "x")
	runCmd(m, send(m, "y"))
	wantIn(t, text(m), "no device")
}

func TestCursorClampsAndQuitKeys(t *testing.T) {
	m := load(t, newFake())
	for i := 0; i < 5; i++ {
		send(m, "j")
	}
	if m.cursor[viewDevices] != 1 {
		t.Fatalf("cursor = %d", m.cursor[viewDevices])
	}
	send(m, "q")
	if !m.Quit() {
		t.Fatal("q should quit")
	}
}

func TestViewStaysWithinSize(t *testing.T) {
	m := load(t, newFake())
	m.Resize(30, 8)
	for _, ln := range strings.Split(m.View(), "\n") {
		if w := len([]rune(ansi.Strip(ln))); w > 30 {
			t.Errorf("line too wide (%d): %q", w, ln)
		}
	}
}

func TestOwnsOnlyOwnMessages(t *testing.T) {
	if !Owns(loadedMsg{}) || !Owns(doneMsg{}) || Owns(tea.WindowSizeMsg{}) {
		t.Fatal("Owns misclassifies")
	}
}

func TestClipNeutralizesEscapes(t *testing.T) {
	if got := clip("evil\x1b]0;pwned\a", 40); strings.ContainsAny(got, "\x1b\a") {
		t.Errorf("clip kept control characters: %q", got)
	}
	if got := printable("detail\x1b[2J"); strings.Contains(got, "\x1b") {
		t.Errorf("printable kept ESC: %q", got)
	}
}
