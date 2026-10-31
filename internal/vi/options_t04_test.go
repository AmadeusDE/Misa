package vi

import (
	"strings"
	"testing"
)

func TestSearchBooleanOptions(t *testing.T) {
	e := New("", t.TempDir())
	if e.ignoreCase || e.hlSearch || e.incSearch {
		t.Fatal("search options must default off")
	}
	for _, tc := range []struct {
		cmd          string
		ic, hls, inc bool
	}{
		{"set ic hls is", true, true, true},
		{"set noignorecase nohlsearch noincsearch", false, false, false},
		{"set ignorecase! invhlsearch incsearch!", true, true, true},
		{"set invic hls! invis", false, false, false},
	} {
		e.executeCommand(tc.cmd)
		if e.ignoreCase != tc.ic || e.hlSearch != tc.hls || e.incSearch != tc.inc {
			t.Errorf("%s: got %v %v %v", tc.cmd, e.ignoreCase, e.hlSearch, e.incSearch)
		}
	}
	e.executeCommand("set ic? hls? is?")
	for _, want := range []string{"noignorecase", "nohlsearch", "noincsearch"} {
		if !strings.Contains(e.statusMsg, want) {
			t.Errorf("query %q missing %q", e.statusMsg, want)
		}
	}
	e.executeCommand("set ic hls is")
	e.executeCommand("set")
	for _, want := range []string{"ignorecase", "hlsearch", "incsearch"} {
		if !strings.Contains(e.statusMsg, want) {
			t.Errorf("set listing %q missing %q", e.statusMsg, want)
		}
	}
}

func TestSearchOptionsErrorsAndNoHighlight(t *testing.T) {
	e := New("", t.TempDir())
	e.executeCommand("set hls")
	e.executeCommand("noh")
	if !e.hlSearch || !e.searchHighlightSuppressed {
		t.Fatal(":noh must suppress highlighting without changing option")
	}
	e.executeCommand("set hlsearch")
	if e.searchHighlightSuppressed {
		t.Fatal(":set hlsearch must restore highlighting")
	}
	e.executeCommand("nohlsearch")
	if !e.searchHighlightSuppressed || !e.hlSearch {
		t.Fatal(":nohlsearch must suppress highlighting")
	}
	e.executeCommand("set ic unknown is")
	if !e.ignoreCase || e.incSearch || !strings.Contains(e.statusMsg, "unknown") {
		t.Fatalf("unknown option must stop processing: ic=%v is=%v status=%q", e.ignoreCase, e.incSearch, e.statusMsg)
	}
	e.executeCommand("set nocp")
	if e.statusMsg != "nocompatible" {
		t.Fatalf("nocp compatibility: %q", e.statusMsg)
	}
}

func TestSearchOptionQueriesAndToggleSuppression(t *testing.T) {
	e := New("", t.TempDir())
	e.executeCommand("set\thls")
	e.executeCommand("noh")
	e.executeCommand("set hls?")
	if !e.hlSearch || !e.searchHighlightSuppressed || e.statusMsg != "hlsearch" {
		t.Fatalf("query changed option/suppression: enabled=%v suppressed=%v status=%q", e.hlSearch, e.searchHighlightSuppressed, e.statusMsg)
	}
	e.executeCommand("set hls!")
	if e.hlSearch {
		t.Fatal("toggle must disable hlsearch")
	}
	e.executeCommand("set invhls")
	if !e.hlSearch || e.searchHighlightSuppressed {
		t.Fatal("toggle on must restore highlighting")
	}
	e.executeCommand("set noic!")
	if !strings.Contains(e.statusMsg, "noic!") {
		t.Fatalf("malformed option must report error: %q", e.statusMsg)
	}
}
