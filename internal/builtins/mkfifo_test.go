//go:build unix

// Behaviour adapted from toybox mkfifo.c (0BSD) and uutils coreutils mkfifo (MIT).
package builtins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/proceffect"
)

func TestMkfifoModes(t *testing.T) {
	skipWithoutFifos(t)
	for _, tc := range []struct {
		mode string
		want os.FileMode
	}{
		{"000", 0}, {"124", 0124}, {"600", 0600}, {"a=rwx,o-w", 0775},
		{"u-r,g-w,o+x", 0247}, {"u+x", 0766}, {"u=rw,g=u,o=", 0660},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			dir := t.TempDir()
			r := runBuiltinIn(t, dir, "mkfifo", "-m", tc.mode, "pipe")
			assertExitCode(t, r, 0)
			info, err := os.Stat(filepath.Join(dir, "pipe"))
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode()&os.ModeNamedPipe == 0 || info.Mode().Perm() != tc.want {
				t.Fatalf("mode = %v, want FIFO %03o", info.Mode(), tc.want)
			}
		})
	}
}

func TestMkfifoErrors(t *testing.T) {
	for _, mode := range []string{"invalid", "0999", "", "u", "u+z"} {
		r := runBuiltin(t, "mkfifo", "-m", mode, "pipe")
		assertExitCode(t, r, 1)
		if !strings.Contains(r.stderr, "invalid mode") {
			t.Errorf("stderr = %q", r.stderr)
		}
	}
	for _, mode := range []string{"1777", "u+s", "a+t", "10000"} {
		r := runBuiltin(t, "mkfifo", "-m", mode, "pipe")
		assertExitCode(t, r, 1)
		if r.stderr != "mkfifo: mode must specify only file permission bits\n" {
			t.Errorf("stderr = %q", r.stderr)
		}
	}
	skipWithoutFifos(t)
	dir := t.TempDir()
	p := writeFile(t, dir, "existing", "keep")
	if err := os.Chmod(p, 0600); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "mkfifo", "-m", "777", "existing", "pipe")
	assertExitCode(t, r, 1)
	if r.stderr != "mkfifo: cannot create fifo 'existing': File exists\n" {
		t.Errorf("stderr = %q", r.stderr)
	}
	info, _ := os.Stat(p)
	if info.Mode().Perm() != 0600 {
		t.Fatal("existing mode changed")
	}
	if _, err := os.Stat(filepath.Join(dir, "pipe")); err != nil {
		t.Fatal(err)
	}
	r = runBuiltinIn(t, dir, "mkfifo", "")
	if r.stderr != "mkfifo: cannot create fifo '': No such file or directory\n" {
		t.Errorf("stderr = %q", r.stderr)
	}
}

func TestMkfifoOmittedWho(t *testing.T) {
	for _, tc := range []struct {
		mode       string
		mask, want os.FileMode
	}{
		{"=rwx", 0022, 0755}, {"=rwx,o-w", 0022, 0755},
		{"+x", 0022, 0777}, {"-w", 0022, 0466},
		{"=rw", 0077, 0600}, {"a=rwx", 0077, 0777},
	} {
		got, err := parseMkfifoMode(tc.mode, tc.mask)
		if err != nil || got != tc.want {
			t.Errorf("mode(%q, %03o) = %03o, %v; want %03o", tc.mode, tc.mask, got, err, tc.want)
		}
	}
	skipWithoutFifos(t)
	mask, err := proceffect.Umask()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	assertExitCode(t, runBuiltinIn(t, dir, "mkfifo", "-m", "=rwx", "pipe"), 0)
	info, err := os.Stat(filepath.Join(dir, "pipe"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != os.FileMode(0777&^mask) {
		t.Errorf("mode = %03o, mask = %03o", info.Mode().Perm(), mask)
	}
	after, err := proceffect.Umask()
	if err != nil || after != mask {
		t.Fatalf("umask changed: %03o -> %03o (%v)", mask, after, err)
	}
}
