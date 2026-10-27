package ui

import (
	"errors"
	"testing"

	"github.com/AmadeusDE/Misa/internal/clipboard"
)

type clipFake struct {
	text string
	err  error
}

func (f *clipFake) Read() (string, error) { return f.text, f.err }
func (f *clipFake) Write(s string) error {
	if f.err != nil {
		return f.err
	}
	f.text = s
	return nil
}

func TestWriteClipboardKeepsHostTool(t *testing.T) {
	fake := &clipFake{}
	clipboard.Use(fake)
	t.Cleanup(func() { clipboard.Use(nil) })
	if cmd := writeClipboard("kept"); cmd != nil {
		t.Fatal("host write also emitted OSC 52")
	}
	if fake.text != "kept" {
		t.Fatalf("stored %q", fake.text)
	}
	if readClipboard() != "kept" {
		t.Fatalf("read %q", readClipboard())
	}
}

func TestWriteClipboardFallsBackToOSC52(t *testing.T) {
	clipboard.Use(&clipFake{err: errors.New("no tool")})
	t.Cleanup(func() { clipboard.Use(nil) })
	if writeClipboard("via osc") == nil {
		t.Fatal("missing host clipboard did not fall back to OSC 52")
	}
	if writeClipboard("") != nil {
		t.Fatal("empty copy emitted a command")
	}
	if readClipboard() != "" {
		t.Fatal("failed read returned text")
	}
}
