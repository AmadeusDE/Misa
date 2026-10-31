package builtins

import (
	"os"
	"path/filepath"
	"testing"
)

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestSedInPlace(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		code    int
		stdout  string
		files   map[string]string
		missing []string
	}{
		{"edits", []string{"-i", "s/a/x/", "f"}, 0, "", map[string]string{"f": "x\nb\n"}, nil},
		{"backup suffix", []string{"-i.bak", "s/a/x/", "f"}, 0, "", map[string]string{"f": "x\nb\n", "f.bak": "a\nb\n"}, nil},
		{"long backup suffix", []string{"--in-place=.orig", "1d", "f"}, 0, "", map[string]string{"f": "b\n", "f.orig": "a\nb\n"}, nil},
		{"star names the backup", []string{"-iold_*", "1d", "f"}, 0, "", map[string]string{"f": "b\n", "old_f": "a\nb\n"}, nil},
		{"files are separate", []string{"-i", "1d;$s/$/!/", "f", "g"}, 0, "", map[string]string{"f": "b!\n", "g": "d!\n"}, nil},
		{"p goes to the file", []string{"-n", "-i", "1p", "f"}, 0, "", map[string]string{"f": "a\n"}, nil},
		{"w /dev/stdout stays on stdout", []string{"-i", "-e", "w /dev/stdout", "-e", "s/a/x/", "f"}, 0, "a\nb\n", map[string]string{"f": "x\nb\n"}, nil},
		{"quit drops the rest", []string{"-i", "1q", "f"}, 0, "", map[string]string{"f": "a\n"}, nil},
		{"missing file", []string{"-i", "s/c/x/", "nope", "g"}, 2, "", map[string]string{"g": "x\nd\n"}, nil},
		{"stdin cannot be edited", []string{"-i", "p", "-"}, 4, "", nil, nil},
		{"directory cannot be edited", []string{"-i", "p", "dir"}, 4, "", nil, nil},
		{"no leftovers on error", []string{"-i", "p", "dir"}, 4, "", nil, []string{"dir/sed*"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "f", "a\nb\n")
			writeFile(t, dir, "g", "c\nd\n")
			os.Mkdir(filepath.Join(dir, "dir"), 0o755)
			r := runBuiltinIn(t, dir, "sed", tt.args...)
			assertExitCode(t, r, tt.code)
			assertStdout(t, r, tt.stdout)
			for name, want := range tt.files {
				if got := readTestFile(t, filepath.Join(dir, name)); got != want {
					t.Errorf("%s = %q, want %q", name, got, want)
				}
			}
			for _, pattern := range tt.missing {
				if m, _ := filepath.Glob(filepath.Join(dir, pattern)); len(m) > 0 {
					t.Errorf("leftover files: %v", m)
				}
			}
		})
	}
}

func TestSedInPlaceKeepsMode(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "f", "a\n")
	os.Chmod(path, 0o640)
	r := runBuiltinIn(t, dir, "sed", "-i", "s/a/b/", "f")
	assertExitCode(t, r, 0)
	st, err := os.Stat(path)
	if err != nil || st.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %v, %v; want 0640", st.Mode().Perm(), err)
	}
}

func TestSedInPlaceSymlinks(t *testing.T) {
	dir := t.TempDir()
	target := writeFile(t, dir, "target", "a\n")
	link := filepath.Join(dir, "link")
	os.Symlink("target", link)

	r := runBuiltinIn(t, dir, "sed", "-i", "--follow-symlinks", "s/a/b/", "link")
	assertExitCode(t, r, 0)
	if got := readTestFile(t, target); got != "b\n" {
		t.Errorf("--follow-symlinks: target = %q, want %q", got, "b\n")
	}
	if st, err := os.Lstat(link); err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Errorf("--follow-symlinks must keep the link")
	}

	r = runBuiltinIn(t, dir, "sed", "-i", "s/b/c/", "link")
	assertExitCode(t, r, 0)
	if st, err := os.Lstat(link); err != nil || !st.Mode().IsRegular() {
		t.Errorf("without --follow-symlinks the link becomes a regular file")
	}
	if got := readTestFile(t, link); got != "c\n" {
		t.Errorf("link = %q, want %q", got, "c\n")
	}
	if got := readTestFile(t, target); got != "b\n" {
		t.Errorf("target changed to %q", got)
	}
}

func TestSedInPlaceUnwritableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	sub := filepath.Join(dir, "ro")
	os.Mkdir(sub, 0o755)
	path := writeFile(t, sub, "f", "a\n")
	os.Chmod(sub, 0o555)
	t.Cleanup(func() { os.Chmod(sub, 0o755) })
	r := runBuiltinIn(t, dir, "sed", "-i", "s/a/b/", "ro/f")
	assertExitCode(t, r, 4)
	assertStderrContains(t, r, "couldn't open temporary file")
	if got := readTestFile(t, path); got != "a\n" {
		t.Errorf("original changed to %q", got)
	}
}
