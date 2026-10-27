package provider

import "testing"

// Embedded, provider warnings go to the host's sink, never the terminal.
func TestWarnGoesToTheSink(t *testing.T) {
	var got []string
	SetWarn(func(s string) { got = append(got, s) })
	t.Cleanup(func() { SetWarn(nil) })
	warn("scope file unreadable")
	if len(got) != 1 || got[0] != "scope file unreadable" {
		t.Fatalf("sink got %v", got)
	}
}
