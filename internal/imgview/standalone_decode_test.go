package imgview

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestStandaloneDecoder(t *testing.T) {
	d := standaloneDecoder{}
	if got := d.feed([]byte("\x1b[")); len(got) != 0 {
		t.Fatal(got)
	}
	got := d.feed([]byte("Aé\x1b[O\x1b[I\x1b[<64;4;5M"))
	if len(got) != 5 {
		t.Fatalf("messages: %#v", got)
	}
	if k, ok := got[0].(tea.KeyPressMsg); !ok || k.Code != tea.KeyUp {
		t.Fatalf("up: %#v", got[0])
	}
	if k, ok := got[1].(tea.KeyPressMsg); !ok || k.Text != "é" {
		t.Fatalf("unicode: %#v", got[1])
	}
	if _, ok := got[2].(tea.BlurMsg); !ok {
		t.Fatalf("blur: %#v", got[2])
	}
	if _, ok := got[3].(tea.FocusMsg); !ok {
		t.Fatalf("focus: %#v", got[3])
	}
	if m, ok := got[4].(tea.MouseWheelMsg); !ok || m.X != 3 || m.Y != 4 || m.Button != tea.MouseWheelUp {
		t.Fatalf("mouse: %#v", got[4])
	}
}
func TestStandaloneProbeReplies(t *testing.T) {
	d := standaloneDecoder{probeID: 42, probing: true}
	got := d.feed([]byte("q\x1b_Gi=41;OK\x1b\\\x1b_Gi=42;OK\x1b\\\x1b[?1;2c"))
	if len(got) != 1 || !d.kitty || !d.probeDone {
		t.Fatalf("got %#v kitty %v done %v", got, d.kitty, d.probeDone)
	}
	d = standaloneDecoder{probeID: 42, probing: true}
	d.feed([]byte("\x1b[?1;2c\x1b_Gi=42;OK\x1b\\"))
	if d.kitty {
		t.Fatal("late reply accepted")
	}
}

func TestStandaloneProbeRejectsOtherAPC(t *testing.T) {
	d := standaloneDecoder{probeID: 42, probing: true}
	d.feed([]byte("\x1b_i=42;OK\x1b\\"))
	if d.kitty {
		t.Fatal("non-Kitty APC accepted")
	}
	d.feed([]byte("\x1b_Gi=42;OK\x1b\\"))
	if !d.kitty {
		t.Fatal("valid reply without DA rejected")
	}
}
func TestStandaloneKittyKeyboardAndEscape(t *testing.T) {
	d := standaloneDecoder{}
	got := d.feed([]byte("\x1b[97;5u\x1b[1;2A\x1b"))
	if len(got) != 2 {
		t.Fatalf("got %#v", got)
	}
	if k, ok := got[0].(tea.KeyPressMsg); !ok || k.Code != 'a' || k.Mod != tea.ModCtrl {
		t.Fatalf("ctrl-a: %#v", got[0])
	}
	if k, ok := got[1].(tea.KeyPressMsg); !ok || k.Code != tea.KeyUp || k.Mod != tea.ModShift {
		t.Fatalf("shift-up: %#v", got[1])
	}
	got = d.flushEscape()
	if len(got) != 1 {
		t.Fatalf("escape: %#v", got)
	}
}

func TestStandaloneProbeRejectsMalformedHeader(t *testing.T) {
	for _, reply := range []string{"\x1b_Ggarbage,i=42;OK\x1b\\", "\x1b_Gi=1,i=42;OK\x1b\\", "\x1b_G,i=42;OK\x1b\\"} {
		d := standaloneDecoder{probeID: 42, probing: true}
		d.feed([]byte(reply))
		if d.kitty {
			t.Errorf("accepted malformed reply %q", reply)
		}
	}
}

func TestStandaloneOversizedControlDiscard(t *testing.T) {
	for _, prefix := range []string{"\x1b_G", "\x1b]10;", "\x1bP"} {
		d := standaloneDecoder{}
		if got := d.feed([]byte(prefix + strings.Repeat("x", 5000))); len(got) != 0 {
			t.Fatalf("initial %q: %#v", prefix, got)
		}
		if got := d.feed([]byte("q\x1b")); len(got) != 0 {
			t.Fatalf("fragment %q: %#v", prefix, got)
		}
		got := d.feed([]byte("\\n"))
		if len(got) != 1 {
			t.Fatalf("after ST %q: %#v", prefix, got)
		}
		k, ok := got[0].(tea.KeyPressMsg)
		if !ok || k.Text != "n" {
			t.Fatalf("after ST %q: %#v", prefix, got)
		}
	}
}
func TestStandaloneOSCAndCompleteOversized(t *testing.T) {
	d := standaloneDecoder{probeID: 42, probing: true}
	got := d.feed([]byte("\x1b]10;rgb:ffff/ffff/ffff\a" + "\x1b_Gi=42,x=" + strings.Repeat("x", 9000) + ";OK\x1b\\n"))
	if d.kitty {
		t.Fatal("oversized reply accepted")
	}
	if len(got) != 1 {
		t.Fatalf("got %#v", got)
	}
	if k, ok := got[0].(tea.KeyPressMsg); !ok || k.Text != "n" {
		t.Fatalf("got %#v", got)
	}
}
