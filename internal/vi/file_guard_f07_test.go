package vi

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Each mutation occurs after New, so a normal write must neither destroy the
// external change nor advance the editor's saved revision.
func TestFileF07ExternalIdentityGuard(t *testing.T) {
	for _, kind := range []string{"replacement", "deletion", "created after ENOENT"} {
		t.Run(kind, func(t *testing.T) {
			var e *Editor
			var p string
			if kind == "created after ENOENT" {
				d := t.TempDir()
				p = filepath.Join(d, "file")
				e = New(p, d)
				e.Resize(80, 24)
			} else {
				e, p = f07Editor(t, "abc\n")
			}
			f07Keys(t, e, "iX<Esc>")
			saved := e.savedRevision
			switch kind {
			case "replacement":
				before, err := os.Stat(p)
				if err != nil {
					t.Fatal(err)
				}
				replacement := filepath.Join(filepath.Dir(p), "replacement")
				if err := os.WriteFile(replacement, []byte("xyz\n"), 0640); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(replacement, before.ModTime(), before.ModTime()); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(replacement, p); err != nil {
					t.Fatal(err)
				}
				after, err := os.Stat(p)
				if err != nil {
					t.Fatal(err)
				}
				if os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
					t.Fatal("replacement fixture did not isolate inode change")
				}
			case "deletion":
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
			case "created after ENOENT":
				if err := os.WriteFile(p, []byte("other\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			f07Keys(t, e, ":w<CR>")
			if !e.modified || e.savedRevision != saved {
				t.Errorf("refused write changed saved revision: dirty=%v saved=%d want=%d", e.modified, e.savedRevision, saved)
			}
			if kind == "deletion" {
				if _, err := os.Stat(p); !os.IsNotExist(err) {
					t.Errorf("deleted file recreated, err=%v", err)
				}
			} else {
				want := "xyz\n"
				if kind == "created after ENOENT" {
					want = "other\n"
				}
				if got := f07Read(t, p); got != want {
					t.Errorf("external bytes overwritten: %q", got)
				}
			}
			f07Keys(t, e, ":w!<CR>")
			want := "Xabc\n"
			if kind == "created after ENOENT" {
				want = "X\n"
			}
			if got := f07Read(t, p); got != want {
				t.Errorf("forced write=%q want=%q, status %q", got, want, e.statusMsg)
			}
			if e.modified {
				t.Error("forced write should mark clean")
			}
		})
	}
}

func TestFileF07ForceWriteQuit(t *testing.T) {
	for _, cmd := range []string{":wq!<CR>", ":x!<CR>"} {
		t.Run(cmd, func(t *testing.T) {
			e, p := f07Editor(t, "abc\n")
			f07Keys(t, e, "x")
			if err := os.WriteFile(p, []byte("external longer\n"), 0640); err != nil {
				t.Fatal(err)
			}
			plain := ":wq<CR>"
			if cmd == ":x!<CR>" {
				plain = ":x<CR>"
			}
			f07Keys(t, e, plain)
			if e.Quit || !e.modified {
				t.Errorf("%s with external change: Quit=%v dirty=%v", plain, e.Quit, e.modified)
			}
			if got := f07Read(t, p); got != "external longer\n" {
				t.Errorf("%s overwrote external file: %q", plain, got)
			}
			f07Keys(t, e, cmd)
			if !e.Quit {
				t.Errorf("%s did not quit, status %q", cmd, e.statusMsg)
			}
			if got := f07Read(t, p); got != "bc\n" {
				t.Errorf("%s saved %q", cmd, got)
			}
		})
	}
}

func TestFileF07DirectoryReadFailure(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "directory")
	if err := os.Mkdir(p, 0755); err != nil {
		t.Fatal(err)
	}
	e := New(p, d)
	e.Resize(80, 24)
	if e.statusMsg == "" {
		t.Error("directory read failure not reported")
	}
	saved := e.savedRevision
	f07Keys(t, e, "iX<Esc>:w<CR>")
	if !e.modified || e.savedRevision != saved {
		t.Error("failed write changed saved revision")
	}
	f07Keys(t, e, "u")
	if e.modified {
		t.Error("undo after failed save should return to the clean revision")
	}
	f07Keys(t, e, "<C-r>:w!<CR>")
	if !e.modified || e.savedRevision != saved {
		t.Error("forced write to directory changed saved revision")
	}
	if st, err := os.Stat(p); err != nil || !st.IsDir() {
		t.Errorf("directory replaced: %v %v", st, err)
	}
}

func TestFileF07ExternalGuardAndUndo(t *testing.T) {
	for _, kind := range []string{"size", "mtime"} {
		t.Run(kind, func(t *testing.T) {
			e, p := f07Editor(t, "abc\n")
			f07Keys(t, e, "x")
			saved := e.savedRevision
			external := "other\n"
			if kind == "mtime" {
				external = "xyz\n"
			}
			original, err := os.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(external), 0640); err != nil {
				t.Fatal(err)
			}
			future := original.ModTime()
			if kind == "mtime" {
				future = time.Now().Add(10 * time.Second)
			}
			if err := os.Chtimes(p, future, future); err != nil {
				t.Fatal(err)
			}
			f07Keys(t, e, ":w<CR>")
			if got := f07Read(t, p); got != external {
				t.Errorf("ordinary write overwrote external change: %q", got)
			}
			if !e.modified || e.savedRevision != saved {
				t.Errorf("failed write changed saved revision: dirty=%v saved=%d want=%d", e.modified, e.savedRevision, saved)
			}
			f07Keys(t, e, "u")
			if e.modified {
				t.Error("undo to original revision should be clean")
			}
			f07Keys(t, e, "<C-r>")
			if !e.modified {
				t.Error("redo should be dirty")
			}
			f07Keys(t, e, ":w!<CR>")
			if got := f07Read(t, p); got != "bc\n" {
				t.Errorf("forced save = %q, status %q", got, e.statusMsg)
			}
			if e.modified {
				t.Error("forced save still dirty")
			}
			f07Keys(t, e, "u")
			if !e.modified {
				t.Error("undo after save should be dirty")
			}
			f07Keys(t, e, "<C-r>")
			if e.modified {
				t.Error("redo to saved revision should be clean")
			}
			f07Keys(t, e, "x:w<CR>")
			if got := f07Read(t, p); got != "c\n" {
				t.Errorf("subsequent save after snapshot refresh = %q, status %q", got, e.statusMsg)
			}
		})
	}
}

func TestFileF07UnreadableRequiresForce(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read mode 000 files")
	}
	d := t.TempDir()
	p := filepath.Join(d, "unreadable")
	if err := os.WriteFile(p, []byte("original\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0); err != nil {
		t.Fatal(err)
	}
	e := New(p, d)
	e.Resize(80, 24)
	if e.statusMsg == "" {
		t.Error("read failure not reported")
	}
	if err := os.Chmod(p, 0600); err != nil {
		t.Fatal(err)
	}
	f07Keys(t, e, "iX<Esc>:w<CR>")
	if got := f07Read(t, p); got != "original\n" {
		t.Errorf("write after read failure overwrote %q", got)
	}
	if !e.modified {
		t.Error("refused save marked clean")
	}
	f07Keys(t, e, ":w!<CR>")
	if got := f07Read(t, p); got != "X\n" {
		t.Errorf("forced write = %q, status %q", got, e.statusMsg)
	}
	if e.modified {
		t.Error("forced save marked dirty")
	}
}
