package android

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/gui"
)

func TestTranslateKey(t *testing.T) {
	ev, ok := TranslateKey(keyA, 0, true)
	if !ok || ev.Key.Code != 'a' || ev.Key.Text != "a" || !ev.DismissHold {
		t.Fatalf("a: %+v %v", ev, ok)
	}
	ev, ok = TranslateKey(keyA, MetaShiftOn, true)
	if !ok || ev.Key.Code != 'a' || ev.Key.Text != "A" || ev.Key.Mod&tea.ModShift == 0 {
		t.Fatalf("shift+a: %+v", ev.Key)
	}
	ev, ok = TranslateKey(keyA, MetaCtrlOn|MetaShiftLeft, true)
	if !ok || ev.Key.Text != "" || ev.Key.Mod&(tea.ModCtrl|tea.ModShift) != (tea.ModCtrl|tea.ModShift) {
		t.Fatalf("ctrl+shift+a: %+v", ev.Key)
	}
	ev, ok = TranslateKey(key0+1, MetaShiftOn, true)
	if !ok || ev.Key.Code != '1' || ev.Key.Text != "!" {
		t.Fatalf("shift+1: %+v", ev.Key)
	}
	ev, ok = TranslateKey(keyEnter, 0, true)
	if !ok || ev.Key.Code != tea.KeyEnter {
		t.Fatalf("enter: %+v", ev.Key)
	}
	ev, ok = TranslateKey(keyDel, 0, true)
	if !ok || ev.Key.Code != tea.KeyBackspace {
		t.Fatalf("del: %+v", ev.Key)
	}
	ev, ok = TranslateKey(keyDpadUp, 0, true)
	if !ok || ev.Key.Code != tea.KeyUp {
		t.Fatalf("dpad: %+v", ev.Key)
	}
	ev, ok = TranslateKey(keyBack, 0, true)
	if !ok || ev.Key.Code != tea.KeyEscape {
		t.Fatalf("back: %+v", ev.Key)
	}
	ev, ok = TranslateKey(keyF1+2, 0, true)
	if !ok || ev.Key.Code != tea.KeyF3 {
		t.Fatalf("f3: %+v", ev.Key)
	}
	if _, ok := TranslateKey(keyA, 0, false); ok {
		t.Fatal("key up produced an event")
	}
	if _, ok := TranslateKey(keyShiftLeft, MetaShiftOn, true); ok {
		t.Fatal("bare shift produced an event")
	}
	ev, ok = TranslateKey(keyEquals, MetaCtrlOn|MetaShiftOn, true)
	if !ok || ev.Key.Code != '=' || ev.Key.Mod&(tea.ModCtrl|tea.ModShift) != (tea.ModCtrl|tea.ModShift) {
		t.Fatalf("zoom key: %+v", ev.Key)
	}
}

func TestPointerMouseScrollAndTouch(t *testing.T) {
	var p Pointer
	now := time.Unix(0, 0)
	evs := p.Feed(ActionDown, 4, 5, ButtonPrimary, now)
	if len(evs) != 1 || evs[0].Action != gui.PointerDown || evs[0].Button != tea.MouseLeft {
		t.Fatalf("mouse down %+v", evs)
	}
	evs = p.Feed(ActionUp, 4, 5, ButtonSecondary, now)
	if len(evs) != 1 || evs[0].Button != tea.MouseRight || evs[0].Action != gui.PointerUp {
		t.Fatalf("mouse up %+v", evs)
	}
	evs = p.Feed(ActionScroll, 0, 2, 0, now)
	if len(evs) != 1 || evs[0].Kind != gui.EventScroll || evs[0].Axis != 0 || evs[0].Steps != 2 {
		t.Fatalf("scroll %+v", evs)
	}

	p = Pointer{}
	if ev := p.Feed(ActionDown, 10, 10, 0, now); ev != nil {
		t.Fatalf("touch down leaked %+v", ev)
	}
	evs = p.Feed(ActionUp, 12, 11, 0, now.Add(10*time.Millisecond))
	if len(evs) != 2 || evs[0].Action != gui.PointerDown || evs[1].Action != gui.PointerUp || evs[0].Button != tea.MouseLeft {
		t.Fatalf("tap %+v", evs)
	}

	p = Pointer{}
	p.Feed(ActionDown, 10, 10, 0, now)
	evs = p.Feed(ActionMove, 10, 10+scrollStep+touchSlop, 0, now.Add(20*time.Millisecond))
	if len(evs) != 1 || evs[0].Kind != gui.EventScroll || evs[0].Steps >= 0 {
		t.Fatalf("drag %+v", evs)
	}
	if ev := p.Feed(ActionUp, 10, 40, 0, now.Add(30*time.Millisecond)); ev != nil {
		t.Fatalf("drag up leaked a click %+v", ev)
	}

	p = Pointer{}
	p.Feed(ActionDown, 3, 3, 0, now)
	evs = p.Feed(ActionUp, 3, 3, 0, now.Add(longPress))
	if len(evs) != 2 || evs[0].Button != tea.MouseRight {
		t.Fatalf("long press %+v", evs)
	}

	p = Pointer{}
	p.Feed(ActionDown, 8, 8, 0, now)
	p.Feed(ActionPointerDown, 20, 8, 0, now.Add(5*time.Millisecond))
	evs = p.Feed(ActionPointerUp, 21, 8, 0, now.Add(20*time.Millisecond))
	if len(evs) != 1 || evs[0].Kind != gui.EventKey || evs[0].Key.Code != 'v' || evs[0].Key.Mod&(tea.ModCtrl|tea.ModShift) == 0 {
		t.Fatalf("paste %+v", evs)
	}
}
