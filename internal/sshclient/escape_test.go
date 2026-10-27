package sshclient

import (
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

func filterInput(in string) (string, bool) {
	disconnected := false
	r := &escapeReader{r: iotest.OneByteReader(strings.NewReader(in)), atLineStart: true, disconnect: func() { disconnected = true }}
	out, _ := io.ReadAll(r)
	return string(out), disconnected
}

func TestEscape(t *testing.T) {
	cases := []struct {
		in, out      string
		disconnected bool
	}{
		{"ls\r", "ls\r", false},
		{"~.", "", true},
		{"ls\r~.rest", "ls\r", true},
		{"ls\n~.", "ls\n", true},
		{"a~.", "a~.", false},
		{"~~.", "~.", false},
		{"~x", "~x", false},
		{"\r~~\r~.", "\r~\r", true},
	}
	for _, c := range cases {
		out, disconnected := filterInput(c.in)
		if out != c.out || disconnected != c.disconnected {
			t.Errorf("%q: got %q %v, want %q %v", c.in, out, disconnected, c.out, c.disconnected)
		}
	}
}

func TestEscapeWithinOneRead(t *testing.T) {
	disconnected := false
	r := &escapeReader{r: strings.NewReader("ab\r~.cd"), atLineStart: true, disconnect: func() { disconnected = true }}
	out, _ := io.ReadAll(r)
	if string(out) != "ab\r" || !disconnected {
		t.Errorf("got %q %v", out, disconnected)
	}
}
