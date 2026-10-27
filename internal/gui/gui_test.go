package gui

import (
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/term"
)

func TestHoldKeepsFailedProgramOnScreen(t *testing.T) {
	a := &app{mapped: true, term: term.New(3, 60)}
	a.term.ProcessOutput([]byte("sshc: connection refused"))
	if !a.hold(255) {
		t.Fatal("hold refused on a mapped window")
	}
	if !a.holding || a.exitCode != 255 || a.termExited() != nil {
		t.Fatalf("holding %v code %d exited chan %v", a.holding, a.exitCode, a.termExited())
	}
	screen := strings.Join(a.term.PlainScreen(), "\n")
	if !strings.Contains(screen, "connection refused") || !strings.Contains(screen, "status 255") {
		t.Fatalf("screen = %q", screen)
	}
}

func TestHoldNeedsAMappedWindow(t *testing.T) {
	a := &app{term: term.New(3, 60)}
	if a.hold(1) {
		t.Fatal("held output in a window never shown")
	}
}
