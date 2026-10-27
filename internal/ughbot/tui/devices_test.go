package tui

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
)

type deviceFake struct {
	*fakeBackend
	calls   []string
	disable bool
}

func (d *deviceFake) gate() error {
	if d.disable {
		return &api.StatusError{Code: http.StatusServiceUnavailable, Message: api.ErrWebDisabled.Error()}
	}
	return nil
}
func (d *deviceFake) WebDevices(context.Context) ([]api.WebDevice, error) {
	return []api.WebDevice{{ID: "d1", Name: "phone"}}, d.gate()
}
func (d *deviceFake) WebSessions(context.Context) ([]api.WebSession, error) { return nil, nil }
func (d *deviceFake) WebEvents(context.Context, int) ([]api.WebEvent, error) {
	return nil, nil
}
func (d *deviceFake) WebNotices(context.Context) ([]api.WebNotice, error) {
	return []api.WebNotice{{ID: "n1", Text: "New device enrolled: phone"}}, nil
}
func (d *deviceFake) WebRevoke(_ context.Context, id string) error {
	d.calls = append(d.calls, "revoke "+id)
	return nil
}
func (d *deviceFake) WebRename(context.Context, string, string) error { return nil }
func (d *deviceFake) WebKillSession(context.Context, string) error    { return nil }
func (d *deviceFake) WebDismissNotice(context.Context, string) error  { return nil }

func openDevices(t *testing.T, be Backend) *Model {
	t.Helper()
	m := ready(t, be, "")
	run(m, press(m, "4"))
	if m.screen != screenDevices {
		t.Fatalf("screen = %v", m.screen)
	}
	return m
}

func TestDevicesScreenListsAndRevokesThroughClient(t *testing.T) {
	be := &deviceFake{fakeBackend: newFake()}
	m := openDevices(t, be)
	wantIn(t, m.View().Content, "Devices", "phone", "d1", "active", "New device enrolled", "r rename")
	press(m, "x")
	wantIn(t, m.View().Content, "Revoke phone")
	run(m, press(m, "y"))
	if len(be.calls) != 1 || be.calls[0] != "revoke d1" {
		t.Fatalf("calls = %v", be.calls)
	}
}

func TestDevicesScreenCapturesKeysWhileConfirming(t *testing.T) {
	m := openDevices(t, &deviceFake{fakeBackend: newFake()})
	press(m, "x")
	press(m, "q")
	if m.screen != screenDevices {
		t.Fatal("q left the screen during a confirmation")
	}
	if m.devices.model.Capturing() {
		t.Fatal("any other key must cancel the confirmation")
	}
	press(m, "x")
	press(m, "]")
	if m.screen != screenDevices {
		t.Fatal("] navigated during a confirmation")
	}
}

func TestDevicesScreenExplainsDisabledWeb(t *testing.T) {
	m := openDevices(t, &deviceFake{fakeBackend: newFake(), disable: true})
	wantIn(t, m.View().Content, "web UI is not enabled", "ughbot serve -web", "[web]")
}

func TestDevicesScreenWithoutDeviceAPI(t *testing.T) {
	m := openDevices(t, newFake())
	if v := m.View().Content; !strings.Contains(v, "cannot manage devices") {
		t.Fatalf("view:\n%s", v)
	}
}

func TestNavigationKeysCycleScreens(t *testing.T) {
	m := ready(t, newFake(), "")
	run(m, press(m, "]"))
	if m.screen != screenCost {
		t.Fatalf("] -> %v", m.screen)
	}
	run(m, press(m, "["))
	if m.screen != screenPicker {
		t.Fatalf("[ -> %v", m.screen)
	}
	run(m, press(m, "shift+tab"))
	if m.screen != screenDevices {
		t.Fatalf("shift+tab wraps to %v", m.screen)
	}
	run(m, press(m, "3"))
	if m.screen != screenStatus {
		t.Fatalf("3 -> %v", m.screen)
	}
}

func TestNumberKeysAndBracketsAreTypedInChat(t *testing.T) {
	m := ready(t, newFake(), "a")
	typeText(m, "1[]")
	if m.screen != screenChat || m.chat.input.Value() != "1[]" {
		t.Fatalf("screen=%v input=%q", m.screen, m.chat.input.Value())
	}
	run(m, press(m, "ctrl+o"))
	if m.screen == screenChat {
		t.Fatal("ctrl+o did not leave the chat")
	}
}

func TestNumberKeysHonourChatTab(t *testing.T) {
	m := ready(t, newFake(), "a")
	run(m, press(m, "esc"))
	run(m, press(m, "2"))
	if m.screen != screenChat {
		t.Fatalf("2 with an open chat -> %v", m.screen)
	}
}
