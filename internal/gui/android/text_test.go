package android

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/gui"
	"github.com/AmadeusDE/Misa/internal/ui"
)

func TestEventsForTextKeepsPlainCommits(t *testing.T) {
	evs := EventsForText("hi", 0)
	if len(evs) != 1 || evs[0].Kind != gui.EventText || evs[0].Text != "hi" {
		t.Fatalf("plain %+v", evs)
	}
	if EventsForText("", MetaCtrlOn) != nil {
		t.Fatal("empty commit produced events")
	}
}

func TestEventsForTextAppliesLatchedModifiers(t *testing.T) {
	cases := []struct {
		text string
		meta int
		want string
	}{
		{"w", MetaCtrlOn, "\x17"},
		{"b", MetaCtrlOn, "\x02"},
		{"c", MetaCtrlOn, "\x03"},
		{"w", MetaAltOn, "\x1bw"},
		{"W", MetaCtrlOn, "\x17"},
		{"hello", MetaCtrlOn, "\x08\x05\x0c\x0c\x0f"},
		{"|", MetaCtrlOn, "\x1c"},
	}
	for _, c := range cases {
		if got := textBytes(t, c.text, c.meta); got != c.want {
			t.Errorf("EventsForText(%q, %d) = %q, want %q", c.text, c.meta, got, c.want)
		}
	}
	evs := EventsForText("é", MetaCtrlOn)
	if len(evs) != 1 || evs[0].Kind != gui.EventText || evs[0].Text != "é" {
		t.Fatalf("unmapped %+v", evs)
	}
	shift := EventsForText("A", MetaAltOn)
	if len(shift) != 1 || shift[0].Key.Text != "A" || shift[0].Key.Mod&(tea.ModAlt|tea.ModShift) != tea.ModAlt|tea.ModShift {
		t.Fatalf("alt+shift+a %+v", shift)
	}
}

func textBytes(t *testing.T, text string, meta int) string {
	t.Helper()
	var b []byte
	for _, ev := range EventsForText(text, meta) {
		if ev.Kind != gui.EventKey {
			t.Fatalf("event %+v", ev)
		}
		b = append(b, ui.KeyBytes(ev.Key, false)...)
	}
	return string(b)
}
