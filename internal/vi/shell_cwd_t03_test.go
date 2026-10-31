package vi

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestT03AllShellFormsUseInvocationEnvironment(t *testing.T) {
	t03Shell(t)
	dir, other := t.TempDir(), t.TempDir()
	path := filepath.Join(other, "file")
	if err := os.WriteFile(path, []byte("base\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MISA_T03_CWD_VALUE", "before")
	command := `printf '%s\n' "$PWD" "$MISA_T03_CWD_VALUE"`
	for _, form := range []string{"!", "r !", "%!", "w !"} {
		t.Run(form, func(t *testing.T) {
			t.Setenv("MISA_T03_CWD_VALUE", "before")
			e := New(path, dir)
			e.Resize(100, 24)
			t.Setenv("MISA_T03_CWD_VALUE", "after")
			if form == "w !" {
				out := filepath.Join(dir, "capture")
				t03Keys(t, e, ":w !"+command+" > '"+out+"'<CR>")
				b, err := os.ReadFile(out)
				if err != nil || string(b) != dir+"\nbefore\n" {
					t.Errorf("write shell output=%q err=%v", b, err)
				}
				return
			}
			t03Keys(t, e, ":"+form+command+"<CR>")
			if form == "!" {
				view := e.View()
				if !strings.Contains(view, dir) || !strings.Contains(view, "before") || strings.Contains(view, "after") {
					t.Errorf("output %q", view)
				}
				return
			}
			want := []string{dir, "before"}
			if form == "r !" {
				want = append([]string{"base"}, want...)
			}
			if !reflect.DeepEqual(e.lines, want) {
				t.Errorf("lines=%q want %q", e.lines, want)
			}
		})
	}
}
