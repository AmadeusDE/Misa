package android

import (
	"sync"
	"testing"

	"github.com/AmadeusDE/Misa/internal/gui"
)

func queuedEvents(b *Backend) []gui.Event {
	var events []gui.Event
	for {
		select {
		case ev := <-b.Events():
			events = append(events, ev)
		default:
			return events
		}
	}
}

func TestOverflowPreservesLifecycleBeforeOrdinaryInput(t *testing.T) {
	b := newBackend(nil)
	b.emit(gui.Event{Kind: gui.EventInvalidate})
	b.emit(gui.Event{Kind: gui.EventFocus, Focused: false})
	b.emit(gui.Event{Kind: gui.EventClose})
	for i := 0; i < cap(b.events)*2; i++ {
		b.emit(gui.Event{Kind: gui.EventText, Text: "input"})
	}
	var invalidate, blur, close int
	for _, ev := range queuedEvents(b) {
		switch ev.Kind {
		case gui.EventInvalidate:
			invalidate++
		case gui.EventFocus:
			if ev.Focused {
				t.Fatal("overflow invented focus-in")
			}
			blur++
		case gui.EventClose:
			close++
		}
	}
	if invalidate != 1 || blur != 1 || close != 1 {
		t.Fatalf("retained invalidate/blur/close = %d/%d/%d, want 1/1/1", invalidate, blur, close)
	}
}

func TestOverflowCoalescesLatestFocusAndInvalidation(t *testing.T) {
	b := newBackend(nil)
	for i := 0; i < cap(b.events); i++ {
		b.emit(gui.Event{Kind: gui.EventFocus, Focused: true})
	}
	b.emit(gui.Event{Kind: gui.EventInvalidate})
	b.emit(gui.Event{Kind: gui.EventInvalidate})
	b.emit(gui.Event{Kind: gui.EventClose})
	for i := 0; i < cap(b.events); i++ {
		b.emit(gui.Event{Kind: gui.EventText, Text: "input"})
	}
	b.emit(gui.Event{Kind: gui.EventFocus, Focused: false})
	var focus, invalidate, close int
	for _, ev := range queuedEvents(b) {
		switch ev.Kind {
		case gui.EventFocus:
			focus++
			if ev.Focused {
				t.Fatal("stale focus-in survived overflow's newer focus-out")
			}
		case gui.EventInvalidate:
			invalidate++
		case gui.EventClose:
			close++
		}
	}
	if focus != 1 || invalidate != 1 || close != 1 {
		t.Fatalf("coalesced focus/invalidate/close = %d/%d/%d, want 1/1/1", focus, invalidate, close)
	}
}

func TestOverflowConcurrentProducersKeepFinalBlur(t *testing.T) {
	b := newBackend(nil)
	var wg sync.WaitGroup
	for n := 0; n < 4; n++ {
		wg.Go(func() {
			for i := 0; i < cap(b.events)*2; i++ {
				b.emit(gui.Event{Kind: gui.EventText, Text: "input"})
				b.emit(gui.Event{Kind: gui.EventFocus, Focused: true})
				b.emit(gui.Event{Kind: gui.EventInvalidate})
			}
		})
	}
	wg.Wait()
	// Force overflow for the final state, rather than leaving old focus events
	// in a non-full queue that the GUI has yet to consume.
	for i := 0; i < cap(b.events); i++ {
		b.emit(gui.Event{Kind: gui.EventText, Text: "input"})
	}
	b.emit(gui.Event{Kind: gui.EventFocus, Focused: false})
	b.emit(gui.Event{Kind: gui.EventClose})
	b.emit(gui.Event{Kind: gui.EventText, Text: "after close"})
	if n := len(b.events); n > cap(b.events) {
		t.Fatalf("queue grew to %d", n)
	}
	var focus, invalidate, close int
	for _, ev := range queuedEvents(b) {
		switch ev.Kind {
		case gui.EventFocus:
			focus++
			if ev.Focused {
				t.Fatal("final blur was replaced by a stale focus-in")
			}
		case gui.EventInvalidate:
			invalidate++
		case gui.EventClose:
			close++
		}
	}
	if focus != 1 || invalidate != 1 || close != 1 {
		t.Fatalf("focus/invalidate/close after concurrent overflow = %d/%d/%d", focus, invalidate, close)
	}
}
