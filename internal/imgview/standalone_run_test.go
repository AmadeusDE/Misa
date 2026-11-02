package imgview

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestStandaloneCommandPanic(t *testing.T) {
	ch := make(chan tea.Msg, 1)
	standaloneCommands(context.Background(), ch, func() tea.Msg { panic("bad") })
	select {
	case m := <-ch:
		if _, ok := m.(standalonePanic); !ok {
			t.Fatalf("got %#v", m)
		}
	case <-time.After(time.Second):
		t.Fatal("command did not return panic")
	}
}
func TestStandaloneSequence(t *testing.T) {
	ch := make(chan tea.Msg, 2)
	standaloneCommands(context.Background(), ch, tea.Sequence(func() tea.Msg { return "first" }, func() tea.Msg { return "second" }))
	for _, want := range []string{"first", "second"} {
		select {
		case got := <-ch:
			if got != want {
				t.Fatalf("got %v want %v", got, want)
			}
		case <-time.After(time.Second):
			t.Fatal("sequence stalled")
		}
	}
}
