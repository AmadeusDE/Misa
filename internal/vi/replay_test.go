package vi

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

// These original fixtures target Vim 9.1 with 'nocompatible' and default
// editing options, not Neovim defaults. New does not read user configuration;
// fixtures do not require Vim or a vimrc. Each literal rune is one physical
// key. Special tokens are <Esc>, <CR>, <C-r>, <C-o>, <BS>, <Up>, <Down>, <Left>, <Right>;
// <lt> enters a literal less-than sign.
func replayKeys(t *testing.T, s string) []tea.KeyPressMsg {
	t.Helper()
	var keys []tea.KeyPressMsg
	for len(s) > 0 {
		if s[0] == '<' {
			end := strings.IndexByte(s, '>')
			if end < 0 {
				t.Fatalf("unterminated key notation %q", s)
			}
			token := s[1:end]
			name := map[string]string{"Esc": "esc", "CR": "enter", "C-r": "ctrl+r", "C-o": "ctrl+o", "BS": "backspace", "lt": "<", "Up": "up", "Down": "down", "Left": "left", "Right": "right"}[token]
			if name == "" {
				t.Fatalf("unknown key <%s>", token)
			}
			keys = append(keys, keyFromName(name))
			s = s[end+1:]
			continue
		}
		r, n := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError && n == 1 {
			t.Fatal("invalid UTF-8 in keys")
		}
		keys = append(keys, keyFromName(s[:n]))
		s = s[n:]
	}
	return keys
}

type replayState struct {
	lines                    []string
	row, col                 int
	mode                     Mode
	reg                      []string
	linewise, modified, quit bool
}

type replayFixture struct {
	name, input, keys string
	want              replayState
	skip              string
}

func runReplayFixture(t *testing.T, f replayFixture) {
	t.Helper()
	if f.skip != "" {
		t.Skip(f.skip)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "lesson.txt")
	if err := os.WriteFile(path, []byte(f.input), 0600); err != nil {
		t.Fatal(err)
	}
	e := New(path, dir)
	e.Resize(80, 24)
	for _, key := range replayKeys(t, f.keys) {
		e.Update(key)
	}
	reg, linewise := e.Register()
	got := replayState{e.lines, e.row, e.col, e.mode, reg, linewise, e.modified, e.Quit}
	if !reflect.DeepEqual(got, f.want) {
		t.Errorf("keys %q\ngot  %+v\nwant %+v", f.keys, got, f.want)
	}
}
