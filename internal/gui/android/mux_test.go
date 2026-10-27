package android

import (
	"os"
	"regexp"
	"testing"

	"github.com/AmadeusDE/Misa/internal/gui"
	"github.com/AmadeusDE/Misa/internal/ui"
)

func TestMuxChordsMatchUIBindings(t *testing.T) {
	cases := []struct {
		action string
		want   string
	}{
		{"prefix", "\x17"},
		{"split", "\x17v"},
		{"next", "\x17w"},
		{"left", "\x17h"},
		{"down", "\x17j"},
		{"up", "\x17k"},
		{"right", "\x17l"},
		{"close", "\x17q"},
		{"layout", "\x17 "},
		{"resize", "\x17r"},
		{"agent", "\x17a"},
		{"reattach", "\x17A"},
		{"copy", "\x19"},
		{"paste", "\x16"},
		{"tag1", "\x1b1"},
		{"tag0", "\x1b0"},
	}
	for _, c := range cases {
		if got := chordBytes(t, c.action); got != c.want {
			t.Errorf("%s = %q, want %q", c.action, got, c.want)
		}
	}
	if MuxChords("missing") != nil || MuxChords("zoom-in") != nil {
		t.Fatal("unknown action returned a chord")
	}
}

func TestOnTextAndMuxActionReachTheSession(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	m := NewManager(func(be gui.Backend, dir string, argv []string) (int, error) {
		<-release
		return 0, nil
	})
	if err := m.OnCreate(nil, "/bin/misa"); err != nil {
		t.Fatal(err)
	}
	id := m.NewSession()
	m.OnText("w", MetaCtrlOn)
	key := lastOf(m.backend(id).eventsLogged(), gui.EventKey)
	if string(ui.KeyBytes(key.Key, false)) != "\x17" {
		t.Fatalf("latched ctrl+w %+v", key.Key)
	}
	m.OnText("ok", 0)
	if lastOf(m.backend(id).eventsLogged(), gui.EventText).Text != "ok" {
		t.Fatal("plain text")
	}
	m.MuxAction("split")
	logged := m.backend(id).eventsLogged()
	got := ""
	for _, ev := range logged {
		if ev.Kind == gui.EventKey {
			got += string(ui.KeyBytes(ev.Key, false))
		}
	}
	if got != "\x17\x17v" {
		t.Fatalf("keys %q", got)
	}
	m.MuxAction("zoom-in")
	if lastOf(m.backend(id).eventsLogged(), gui.EventFont).Scale != ZoomIn {
		t.Fatal("zoom")
	}
	m.MuxAction("nope")
	m.CloseSession(id)
}

func TestExtraKeysButtonsMatchMuxActions(t *testing.T) {
	src, err := os.ReadFile("../../../android/app/src/main/java/app/misa/ExtraKeys.java")
	if err != nil {
		t.Fatal(err)
	}
	ids := regexp.MustCompile(`\{"[^"]*", "([^"]+)"\}`).FindAllStringSubmatch(string(src), -1)
	if len(ids) == 0 {
		t.Fatal("no ExtraKeys mux buttons")
	}
	seen := map[string]bool{}
	for _, m := range ids {
		id := m[1]
		seen[id] = true
		switch id {
		case "zoom-in", "zoom-out":
			if MuxChords(id) != nil {
				t.Fatalf("%s should be a font scale, not a chord", id)
			}
		default:
			if len(MuxChords(id)) == 0 {
				t.Fatalf("button %s has no mux chords", id)
			}
		}
	}
	for name := range muxChords {
		if !seen[name] {
			t.Errorf("mux chord %s has no extra-keys button", name)
		}
	}
}

func chordBytes(t *testing.T, action string) string {
	t.Helper()
	chords := MuxChords(action)
	if len(chords) == 0 {
		t.Fatalf("no chords for %s", action)
	}
	var b []byte
	for _, c := range chords {
		ev, ok := TranslateKey(c.Code, c.Meta, true)
		if !ok {
			t.Fatalf("%s chord %+v", action, c)
		}
		b = append(b, ui.KeyBytes(ev.Key, false)...)
	}
	return string(b)
}
