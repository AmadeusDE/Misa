package vi

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileF07PermissionsAndLinks(t *testing.T) {
	t.Run("regular mode", func(t *testing.T) {
		e, p := f07Editor(t, "abc\n")
		f07Keys(t, e, "x:w<CR>")
		st, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != 0640 {
			t.Errorf("mode = %o", st.Mode().Perm())
		}
	})
	t.Run("symlink", func(t *testing.T) {
		d := t.TempDir()
		target := filepath.Join(d, "target")
		link := filepath.Join(d, "link")
		if err := os.WriteFile(target, []byte("abc\n"), 0640); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, link); err != nil {
			t.Skip(err)
		}
		before, err := os.Stat(target)
		if err != nil {
			t.Fatal(err)
		}
		e := New(link, d)
		f07Keys(t, e, "x:w<CR>")
		st, err := os.Lstat(link)
		if err != nil || st.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("symlink lost: %v %v", st, err)
		}
		if got := f07Read(t, target); got != "bc\n" {
			t.Errorf("target = %q", got)
		}
		after, err := os.Stat(target)
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(before, after) {
			t.Error("symlink target inode changed")
		}
		if after.Mode().Perm() != 0640 {
			t.Errorf("target mode = %o", after.Mode().Perm())
		}
	})
	t.Run("hardlink", func(t *testing.T) {
		e, p := f07Editor(t, "abc\n")
		other := filepath.Join(filepath.Dir(p), "other")
		if err := os.Link(p, other); err != nil {
			t.Skip(err)
		}
		f07Keys(t, e, "x:w<CR>")
		if got := f07Read(t, other); got != "bc\n" {
			t.Errorf("hardlink = %q", got)
		}
		a, _ := os.Stat(p)
		b, _ := os.Stat(other)
		if !os.SameFile(a, b) {
			t.Error("hardlink inode changed")
		}
	})
}

func TestFileF07RegularAtomicRename(t *testing.T) {
	e, p := f07Editor(t, "abc\n")
	before, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	f07Keys(t, e, "x:w<CR>")
	after, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(before, after) {
		t.Error("regular single-link save did not replace inode atomically")
	}
	if got := f07Read(t, p); got != "bc\n" {
		t.Errorf("saved = %q", got)
	}
}

func TestFileF07ReadOnlyOwnerForce(t *testing.T) {
	e, p := f07Editor(t, "abc\n")
	if err := os.Chmod(p, 0444); err != nil {
		t.Fatal(err)
	}
	// Reopen after chmod to ensure the read snapshot reflects the protected file.
	e = New(p, filepath.Dir(p))
	e.Resize(80, 24)
	f07Keys(t, e, "x")
	saved := e.savedRevision
	f07Keys(t, e, ":w<CR>")
	if got := f07Read(t, p); got != "abc\n" {
		t.Errorf("ordinary write changed read-only file to %q", got)
	}
	if !e.modified || e.savedRevision != saved {
		t.Errorf("refused write changed saved revision: dirty=%v saved=%d want=%d", e.modified, e.savedRevision, saved)
	}
	before, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	f07Keys(t, e, ":w!<CR>")
	if got := f07Read(t, p); got != "bc\n" {
		t.Errorf("forced write = %q, status %q", got, e.statusMsg)
	}
	if e.modified {
		t.Error("forced write did not mark clean")
	}
	after, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if after.Mode().Perm() != 0444 {
		t.Errorf("forced write mode = %o, want 0444", after.Mode().Perm())
	}
	if os.SameFile(before, after) {
		t.Error("forced write did not atomically replace read-only regular file")
	}
}

func TestFileF07SpecialModePreserved(t *testing.T) {
	for _, tc := range []struct {
		name string
		bit  os.FileMode
	}{{"setuid", os.ModeSetuid}, {"setgid", os.ModeSetgid}} {
		t.Run(tc.name, func(t *testing.T) {
			e, p := f07Editor(t, "abc\n")
			if err := os.Chmod(p, tc.bit|0750); err != nil {
				t.Skipf("%s unsupported: %v", tc.name, err)
			}
			before, err := os.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			if before.Mode()&tc.bit == 0 {
				t.Skipf("filesystem did not retain %s", tc.name)
			}
			e = New(p, filepath.Dir(p))
			e.Resize(80, 24)
			f07Keys(t, e, "x:w<CR>")
			after, err := os.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			if got := f07Read(t, p); got != "bc\n" {
				t.Errorf("save = %q, status %q", got, e.statusMsg)
			}
			if after.Mode()&(tc.bit|os.ModePerm) != before.Mode()&(tc.bit|os.ModePerm) {
				t.Errorf("mode = %v, want %v", after.Mode(), before.Mode())
			}
		})
	}
}
