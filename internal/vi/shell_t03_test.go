package vi

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func t03Shell(t *testing.T) {
	t.Helper()
	shell := "sh"
	if runtime.GOOS != "windows" {
		shell = "/bin/sh"
	}
	if _, err := exec.LookPath(shell); err != nil {
		t.Skip("shell unavailable: ", err)
	}
}

// Drive the same command/message boundary as Bubble Tea, with a bounded wait so
// a broken shell lifecycle cannot hang the package test.
func t03Keys(t *testing.T, e *Editor, keys string) {
	t.Helper()
	for _, k := range replayKeys(t, keys) {
		cmd := e.Update(k)
		for cmd != nil {
			ch := make(chan tea.Msg, 1)
			go func(c tea.Cmd) { ch <- c() }(cmd)
			select {
			case msg := <-ch:
				cmd = e.Update(msg)
			case <-time.After(5 * time.Second):
				t.Fatal("shell command timed out")
			}
		}
	}
}

func TestT03ShellOutputAndExit(t *testing.T) {
	t03Shell(t)
	e, _ := f07Editor(t, "original\n")
	t03Keys(t, e, ":!printf 'stdout-marker\\n'; printf 'stderr-marker\\n' >&2; exit 7<CR>")
	view := e.View()
	for _, s := range []string{"stdout-marker", "stderr-marker", "shell returned 7"} {
		if !strings.Contains(view, s) {
			t.Errorf("output missing %q: %q", s, view)
		}
	}
	if !reflect.DeepEqual(e.lines, []string{"original"}) {
		t.Errorf("buffer changed: %q", e.lines)
	}
}

func TestT03ReadShellExactLines(t *testing.T) {
	t03Shell(t)
	for _, tc := range []struct {
		name, command string
		want          []string
	}{
		{"blanks", "printf 'first\\n\\nlast\\n'", []string{"anchor", "first", "", "last", "tail"}},
		{"no final newline", "printf 'first\\nlast'", []string{"anchor", "first", "last", "tail"}},
		{"crlf", "printf 'first\\r\\n\\r\\nlast\\r\\n'", []string{"anchor", "first", "", "last", "tail"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := "anchor\ntail\n"
			if tc.name == "crlf" {
				input = "anchor\r\ntail\r\n"
			}
			e, _ := f07Editor(t, input)
			t03Keys(t, e, ":r !"+tc.command+"<CR>")
			if !reflect.DeepEqual(e.lines, tc.want) {
				t.Errorf("got %q want %q", e.lines, tc.want)
			}
		})
	}
}

func TestT03ReadAddressedFile(t *testing.T) {
	e, p := f07Editor(t, "one\ntwo\nthree\n")
	if err := os.WriteFile(filepath.Join(filepath.Dir(p), "insert"), []byte("A\n\nB\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t03Keys(t, e, ":2r insert<CR>")
	if want := []string{"one", "two", "A", "", "B", "three"}; !reflect.DeepEqual(e.lines, want) {
		t.Errorf("got %q want %q", e.lines, want)
	}
}

func TestT03ShellInvocationDirectoryAndEnvironment(t *testing.T) {
	t03Shell(t)
	dir, other := t.TempDir(), t.TempDir()
	path := filepath.Join(other, "file")
	if err := os.WriteFile(path, []byte("base\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MISA_T03_VALUE", "captured-value")
	e := New(path, dir)
	t.Setenv("MISA_T03_VALUE", "changed-after-new")
	e.Resize(100, 24)
	t03Keys(t, e, ":r !printf '%s\\n' \"$PWD\" \"$MISA_T03_VALUE\"<CR>")
	if want := []string{"base", dir, "captured-value"}; !reflect.DeepEqual(e.lines, want) {
		t.Errorf("got %q want %q", e.lines, want)
	}
}

func TestT03FilterUndoRegisterAndDot(t *testing.T) {
	t03Shell(t)
	e, _ := f07Editor(t, "alpha\nbeta\ngamma\n")
	t03Keys(t, e, "x")
	reg, linewise := e.Register()
	t03Keys(t, e, ":2,3!printf 'replacement\\n'<CR>")
	if want := []string{"lpha", "replacement"}; !reflect.DeepEqual(e.lines, want) {
		t.Errorf("filtered %q want %q", e.lines, want)
	}
	r, l := e.Register()
	if !reflect.DeepEqual(r, reg) || l != linewise {
		t.Errorf("register changed %q/%v -> %q/%v", reg, linewise, r, l)
	}
	t03Keys(t, e, "u")
	if want := []string{"lpha", "beta", "gamma"}; !reflect.DeepEqual(e.lines, want) {
		t.Errorf("undo %q want %q", e.lines, want)
	}
	t03Keys(t, e, "<C-r>")
	if want := []string{"lpha", "replacement"}; !reflect.DeepEqual(e.lines, want) {
		t.Errorf("redo %q want %q", e.lines, want)
	}
	t03Keys(t, e, "gg0.")
	if want := []string{"pha", "replacement"}; !reflect.DeepEqual(e.lines, want) {
		t.Errorf("dot %q want %q", e.lines, want)
	}
}

func TestT03FailedFilterPreservesInput(t *testing.T) {
	t03Shell(t)
	e, _ := f07Editor(t, "one\ntwo\nthree\n")
	t03Keys(t, e, ":1,2!printf 'partial\\n'; printf 'bad-filter\\n' >&2; exit 9<CR>")
	if want := []string{"one", "two", "three"}; !reflect.DeepEqual(e.lines, want) {
		t.Errorf("failed filter changed input: %q", e.lines)
	}
	if v := e.View(); !strings.Contains(v, "bad-filter") || !strings.Contains(v, "shell returned 9") {
		t.Errorf("failure not readable: %q", v)
	}
}
