package vi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestT02RangeWrites(t *testing.T) {
	for _, tc := range []struct{ name, command, want string }{
		{"numeric", "2,3w part", "two\nthree\n"},
		{"full explicit", "%w part", "Xone\ntwo\nthree\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, p := f07Editor(t, "one\ntwo\nthree\n")
			f07Keys(t, e, "iX<Esc>")
			f07Keys(t, e, ":"+tc.command+"<CR>")
			if got := f07Read(t, filepath.Join(filepath.Dir(p), "part")); got != tc.want {
				t.Errorf("bytes %q want %q", got, tc.want)
			}
			if e.filename != p || !e.modified {
				t.Errorf("identity/dirty = %q/%v", e.filename, e.modified)
			}
		})
	}
}

func TestT02PartialCurrentForce(t *testing.T) {
	e, p := f07Editor(t, "one\ntwo\nthree\n")
	f07Keys(t, e, "iX<Esc>:2,3w<CR>")
	if !strings.Contains(e.statusMsg, "E140") || f07Read(t, p) != "one\ntwo\nthree\n" || !e.modified {
		t.Errorf("partial write should fail: status=%q dirty=%v", e.statusMsg, e.modified)
	}
	f07Keys(t, e, ":2,3w!<CR>")
	if got := f07Read(t, p); got != "two\nthree\n" || !e.modified {
		t.Errorf("forced range = %q dirty=%v", got, e.modified)
	}
}

func TestT02Append(t *testing.T) {
	for _, tc := range []struct {
		name, initial, cmd, want string
		current                  bool
	}{
		{"new", "", "2,3w! >> out", "two\nthree\n", false},
		{"existing", "old\n", "2,3w >> out", "old\ntwo\nthree\n", false},
		{"current", "", "2,3w >> file", "one\ntwo\nthree\ntwo\nthree\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, p := f07Editor(t, "one\ntwo\nthree\n")
			out := filepath.Join(filepath.Dir(p), "out")
			if tc.current {
				out = p
			} else if tc.initial != "" {
				if err := os.WriteFile(out, []byte(tc.initial), 0600); err != nil {
					t.Fatal(err)
				}
			}
			f07Keys(t, e, "iX<Esc>:"+tc.cmd+"<CR>")
			if got := f07Read(t, out); got != tc.want {
				t.Errorf("append = %q want %q status %q", got, tc.want, e.statusMsg)
			}
			if e.filename != p || !e.modified {
				t.Errorf("filename=%q dirty=%v", e.filename, e.modified)
			}
		})
	}
}

func TestT02AppendMissingRefused(t *testing.T) {
	e, p := f07Editor(t, "one\ntwo\n")
	f07Keys(t, e, "iX<Esc>:w >> out<CR>")
	if _, err := os.Stat(filepath.Join(filepath.Dir(p), "out")); !os.IsNotExist(err) {
		t.Errorf("unforced append created output: %v", err)
	}
	if e.filename != p || !e.modified || e.statusMsg == "" {
		t.Errorf("filename=%q dirty=%v status=%q", e.filename, e.modified, e.statusMsg)
	}
}

func TestT02OtherOverwriteAndIdentity(t *testing.T) {
	e, p := f07Editor(t, "one\n")
	other := filepath.Join(filepath.Dir(p), "other")
	if err := os.WriteFile(other, []byte("old\n"), 0600); err != nil {
		t.Fatal(err)
	}
	f07Keys(t, e, "iX<Esc>:w other<CR>")
	if !strings.Contains(e.statusMsg, "E13") || f07Read(t, other) != "old\n" || e.filename != p || !e.modified {
		t.Errorf("unforced status=%q filename=%q dirty=%v", e.statusMsg, e.filename, e.modified)
	}
	f07Keys(t, e, ":w! other<CR>")
	if got := f07Read(t, other); got != "Xone\n" || e.filename != p || !e.modified {
		t.Errorf("forced bytes=%q filename=%q dirty=%v", got, e.filename, e.modified)
	}
	if alt := e.alternateFilename; filepath.Base(alt) != "other" {
		t.Errorf("alternate=%q want other", alt)
	}
}

func TestT02UnnamedWrite(t *testing.T) {
	d := t.TempDir()
	e := New("", d)
	e.Resize(80, 24)
	f07Keys(t, e, "ihello<Esc>:w new<CR>")
	if got := f07Read(t, filepath.Join(d, "new")); got != "hello\n" || filepath.Base(e.filename) != "new" || e.modified {
		t.Errorf("bytes=%q filename=%q dirty=%v", got, e.filename, e.modified)
	}
}

func TestT02SaveAsAndFile(t *testing.T) {
	for _, tc := range []struct {
		name, keys    string
		dirty, exists bool
	}{
		{"saveas", "iX<Esc>:saveas new<CR>", false, true},
		{"sav force", "iX<Esc>:sav! new<CR>", false, true},
		{"file clean", ":file new<CR>", false, false},
		{"file dirty", "iX<Esc>:file new<CR>", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, p := f07Editor(t, "one\n")
			newpath := filepath.Join(filepath.Dir(p), "new")
			if tc.name == "sav force" {
				if err := os.WriteFile(newpath, []byte("old\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			f07Keys(t, e, tc.keys)
			if filepath.Base(e.filename) != "new" || e.modified != tc.dirty {
				t.Errorf("filename=%q dirty=%v status=%q", e.filename, e.modified, e.statusMsg)
			}
			if alt := e.alternateFilename; alt != p {
				t.Errorf("alternate filename=%q want %q", alt, p)
			}
			if tc.exists {
				if got := f07Read(t, newpath); got != "Xone\n" {
					t.Errorf("saved %q", got)
				}
			} else {
				if _, err := os.Stat(newpath); !os.IsNotExist(err) {
					t.Errorf("rename wrote file: %v", err)
				}
				if !strings.Contains(e.statusMsg, "[Not edited]") && !strings.Contains(e.View(), "[Not edited]") {
					t.Errorf("missing [Not edited]: %q", e.statusMsg)
				}
			}
		})
	}
}

func TestT02FailedWriteState(t *testing.T) {
	for _, command := range []string{"w ", "2w ", "saveas "} {
		t.Run(strings.TrimSpace(command), func(t *testing.T) {
			e, p := f07Editor(t, "one\ntwo\n")
			f07Keys(t, e, "iX<Esc>")
			filename, dirty, saved, snapshot := e.filename, e.modified, e.savedRevision, e.file
			alternate := e.alternateFilename
			e.executeCommand(command + filepath.Join(p, "child"))
			if e.filename != filename || e.modified != dirty || e.savedRevision != saved || e.file != snapshot || e.alternateFilename != alternate || f07Read(t, p) != "one\ntwo\n" {
				t.Errorf("failure changed state: filename=%q dirty=%v status=%q", e.filename, e.modified, e.statusMsg)
			}
		})
	}
}

func TestT02WriteQuit(t *testing.T) {
	for _, tc := range []struct {
		name, keys, want string
		quit             bool
	}{
		{"wq name", "iX<Esc>:wq out<CR>", "Xone\ntwo\n", true},
		{"wq range name", "iX<Esc>:2wq out<CR>", "two\n", true},
		{"x dirty", "iX<Esc>:x out<CR>", "Xone\ntwo\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, p := f07Editor(t, "one\ntwo\n")
			f07Keys(t, e, tc.keys)
			out := filepath.Join(filepath.Dir(p), "out")
			b, err := os.ReadFile(out)
			if err != nil || string(b) != tc.want || e.Quit != tc.quit {
				t.Errorf("bytes=%q err=%v quit=%v status=%q", b, err, e.Quit, e.statusMsg)
			}
		})
	}
	t.Run("x clean does not write", func(t *testing.T) {
		e, p := f07Editor(t, "one\n")
		f07Keys(t, e, ":x out<CR>")
		if !e.Quit {
			t.Errorf("did not quit: %q", e.statusMsg)
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(p), "out")); !os.IsNotExist(err) {
			t.Errorf("clean x wrote target: %v", err)
		}
	})
}

func TestT02VisualRangeReplay(t *testing.T) {
	e, p := f07Editor(t, "alpha\nbeta\ngamma\ndelta\n")
	out := filepath.Join(filepath.Dir(p), "selection")
	f07Keys(t, e, "jVj:w "+out+"<CR>")
	b, err := os.ReadFile(out)
	if err != nil || string(b) != "beta\ngamma\n" || e.filename != p {
		t.Errorf("visual output=%q err=%v filename=%q status=%q", b, err, e.filename, e.statusMsg)
	}
}

func TestT02FullExplicitRangeStaysDirty(t *testing.T) {
	e, p := f07Editor(t, "alpha\nbeta\n")
	f07Keys(t, e, "iX<Esc>:%w<CR>")
	if got := f07Read(t, p); got != "Xalpha\nbeta\n" || !e.modified {
		t.Errorf("bytes=%q dirty=%v status=%q", got, e.modified, e.statusMsg)
	}
}

func TestT02ShellWriteRange(t *testing.T) {
	e, p := f07Editor(t, "alpha\nbeta\ngamma\n")
	out := filepath.Join(filepath.Dir(p), "shell-output")
	f07Keys(t, e, ":2,3w !cat > "+out+"<CR>")
	b, err := os.ReadFile(out)
	if err != nil || string(b) != "beta\ngamma\n" || e.filename != p {
		t.Errorf("shell output=%q err=%v filename=%q status=%q", b, err, e.filename, e.statusMsg)
	}
}

func TestT02UndoAfterCopyAndSaveAs(t *testing.T) {
	for _, tc := range []struct {
		name, command        string
		undoDirty, redoDirty bool
	}{
		{"copy", "w copy", false, true},
		{"saveas", "saveas copy", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, p := f07Editor(t, "alpha\n")
			f07Keys(t, e, "iX<Esc>:"+tc.command+"<CR>")
			f07Keys(t, e, "u")
			if e.modified != tc.undoDirty {
				t.Errorf("undo dirty=%v want %v", e.modified, tc.undoDirty)
			}
			f07Keys(t, e, "<C-r>")
			if e.modified != tc.redoDirty {
				t.Errorf("redo dirty=%v want %v", e.modified, tc.redoDirty)
			}
			if got := f07Read(t, filepath.Join(filepath.Dir(p), "copy")); got != "Xalpha\n" {
				t.Errorf("copy bytes=%q", got)
			}
		})
	}
}

func TestT02RangeLineEndings(t *testing.T) {
	for _, tc := range []struct{ name, input, command, want string }{
		{"crlf", "one\r\ntwo\r\nthree\r\n", "2,3w out", "two\r\nthree\r\n"},
		{"noeol ending", "one\ntwo\nthree", "2,3w out", "two\nthree"},
		{"noeol middle", "one\ntwo\nthree", "1,2w out", "one\ntwo\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, p := f07Editor(t, tc.input)
			f07Keys(t, e, ":"+tc.command+"<CR>")
			got := f07Read(t, filepath.Join(filepath.Dir(p), "out"))
			if got != tc.want {
				t.Errorf("bytes=%q want %q", got, tc.want)
			}
		})
	}
}

func TestT02AppendCurrentImplicitName(t *testing.T) {
	e, p := f07Editor(t, "one\ntwo\n")
	f07Keys(t, e, "iX<Esc>:2w >> <CR>")
	if got := f07Read(t, p); got != "one\ntwo\ntwo\n" || !e.modified || e.filename != p {
		t.Errorf("bytes=%q dirty=%v filename=%q status=%q", got, e.modified, e.filename, e.statusMsg)
	}
}
