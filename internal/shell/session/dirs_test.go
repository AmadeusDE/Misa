package session

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// newTestSession starts a session at dir with an explicitly empty environment
// unless env is non-nil, and returns its captured standard streams.
func newTestSession(t *testing.T, dir string, env []string) (*Session, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errb bytes.Buffer
	if env == nil {
		env = []string{}
	}
	s, err := New(Options{Dir: dir, Env: env, Stdout: &out, Stderr: &errb})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, &out, &errb
}

// tree describes root/{realdir/sub,cdp/target} plus root/link -> realdir/sub.
// The test is skipped when the platform cannot create a symlink.
type tree struct{ root, real, sub, cdp, target, link string }

func newTree(t *testing.T) tree {
	t.Helper()
	root := t.TempDir()
	tr := tree{
		root:   root,
		real:   filepath.Join(root, "realdir"),
		sub:    filepath.Join(root, "realdir", "sub"),
		cdp:    filepath.Join(root, "cdp"),
		target: filepath.Join(root, "cdp", "target"),
		link:   filepath.Join(root, "link"),
	}
	for _, d := range []string{tr.sub, tr.target} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(tr.sub, tr.link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	return tr
}

func TestCdLogicalAndPhysical(t *testing.T) {
	tr := newTree(t)
	cases := []struct {
		name, script, wantDir, wantOut string
	}{
		{"logical names the symlink", "cd link; pwd", tr.link, tr.link + "\n"},
		{"logical dot dot is textual", "cd link; cd ..; pwd", tr.root, tr.root + "\n"},
		{"physical resolves the symlink", "cd -P link; pwd", tr.sub, tr.sub + "\n"},
		{"physical dot dot is real", "cd -P link; cd ..; pwd", tr.real, tr.real + "\n"},
		{"physical mid-path operand", "cd -P link/..; pwd", tr.real, tr.real + "\n"},
		{"logical mid-path operand", "cd -L link/..; pwd", tr.root, tr.root + "\n"},
		{"physical parent of logical dir", "cd link; cd -P ..; pwd", tr.real, tr.real + "\n"},
		{"option terminator", "cd -- link; pwd", tr.link, tr.link + "\n"},
		{"absolute path", "cd " + tr.sub + "; pwd", tr.sub, tr.sub + "\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, out, errb := newTestSession(t, tr.root, nil)
			r := s.Run(context.Background(), c.script)
			if r.Status != 0 || r.Err != nil {
				t.Fatalf("%s: %+v, stderr %q", c.script, r, errb.String())
			}
			if got := out.String(); got != c.wantOut {
				t.Fatalf("stdout %q, want %q", got, c.wantOut)
			}
			if got := s.Dir(); got != c.wantDir {
				t.Fatalf("dir %q, want %q", got, c.wantDir)
			}
			if got := s.Vars()["PWD"].Str; got != c.wantDir {
				t.Fatalf("PWD %q, want %q", got, c.wantDir)
			}
		})
	}
}

func TestCdPWDAndOLDPWD(t *testing.T) {
	tr := newTree(t)
	s, out, _ := newTestSession(t, tr.root, nil)
	run := func(src string) {
		t.Helper()
		if r := s.Run(context.Background(), src); r.Status != 0 || r.Err != nil {
			t.Fatalf("%s: %+v", src, r)
		}
	}
	run("cd " + tr.sub)
	env := strings.Join(s.Env(), "\n")
	if !strings.Contains(env, "PWD="+tr.sub) || !strings.Contains(env, "OLDPWD="+tr.root) {
		t.Fatalf("PWD/OLDPWD not exported: %q", env)
	}
	out.Reset()
	run("cd -")
	if got := out.String(); got != tr.root+"\n" {
		t.Fatalf("cd - printed %q", got)
	}
	if s.Dir() != tr.root || s.Vars()["OLDPWD"].Str != tr.sub {
		t.Fatalf("dir %q OLDPWD %q", s.Dir(), s.Vars()["OLDPWD"].Str)
	}
	// PWD is exported from the start, before the first cd.
	fresh, _, _ := newTestSession(t, tr.root, nil)
	if !strings.Contains(strings.Join(fresh.Env(), "\n"), "PWD="+tr.root) {
		t.Fatal(fresh.Env())
	}
}

func TestCdDashWithoutOLDPWD(t *testing.T) {
	tr := newTree(t)
	s, out, errb := newTestSession(t, tr.root, nil)
	r := s.Run(context.Background(), "cd -")
	if r.Status != 1 || r.Err != nil {
		t.Fatalf("%+v", r)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout %q, want empty", out.String())
	}
	if !strings.Contains(errb.String(), "OLDPWD not set") {
		t.Fatalf("stderr %q", errb.String())
	}
	if s.Dir() != tr.root {
		t.Fatalf("dir changed to %q", s.Dir())
	}
}

func TestCdHome(t *testing.T) {
	tr := newTree(t)
	s, _, errb := newTestSession(t, tr.root, []string{"HOME=" + tr.real})
	if r := s.Run(context.Background(), "cd"); r.Status != 0 || s.Dir() != tr.real {
		t.Fatalf("%+v dir %q stderr %q", r, s.Dir(), errb.String())
	}
	s, _, errb = newTestSession(t, tr.root, nil)
	r := s.Run(context.Background(), "unset HOME; cd")
	if r.Status != 1 || r.Err != nil || !strings.Contains(errb.String(), "HOME not set") {
		t.Fatalf("%+v stderr %q", r, errb.String())
	}
	if s.Dir() != tr.root {
		t.Fatalf("dir changed to %q", s.Dir())
	}
}

func TestCdErrors(t *testing.T) {
	tr := newTree(t)
	file := filepath.Join(tr.root, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, script, wantErr string
		wantStatus            int
	}{
		{"missing", "cd missing", `cd: no such file or directory: "missing"`, 1},
		{"absolute missing", "cd /nonexistent-misa-test", "no such file or directory", 1},
		{"not a directory", "cd " + file, "not a directory", 1},
		{"too many operands", "cd a b", "cd: too many arguments", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, out, errb := newTestSession(t, tr.root, nil)
			r := s.Run(context.Background(), c.script)
			if r.Status != c.wantStatus || r.Err != nil {
				t.Fatalf("%+v", r)
			}
			if !strings.Contains(errb.String(), c.wantErr) {
				t.Fatalf("stderr %q, want %q", errb.String(), c.wantErr)
			}
			if out.Len() != 0 || s.Dir() != tr.root || s.Vars()["PWD"].Str != tr.root {
				t.Fatalf("failed cd changed state: dir %q PWD %q stdout %q", s.Dir(), s.Vars()["PWD"].Str, out.String())
			}
		})
	}
}

func TestCdPermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	tr := newTree(t)
	locked := filepath.Join(tr.root, "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })
	s, _, errb := newTestSession(t, tr.root, nil)
	r := s.Run(context.Background(), "cd locked")
	if r.Status != 1 || !strings.Contains(errb.String(), "permission denied") {
		t.Fatalf("%+v stderr %q", r, errb.String())
	}
}

func TestCdpath(t *testing.T) {
	tr := newTree(t)
	cases := []struct {
		name, script, wantDir, wantOut string
		wantStatus                     int
	}{
		{
			name: "search writes the new directory",
			// POSIX has cd itself write the directory a CDPATH match found.
			script:  "CDPATH=" + tr.cdp + "; cd target; pwd",
			wantDir: tr.target, wantOut: tr.target + "\n" + tr.target + "\n",
		},
		{
			name:    "empty entry means the current directory",
			script:  "CDPATH=:" + tr.cdp + "; cd " + filepath.Base(tr.real) + "; pwd",
			wantDir: tr.real, wantOut: tr.real + "\n",
		},
		{
			name:    "dot entry does not write",
			script:  "CDPATH=.; cd " + filepath.Base(tr.real) + "; pwd",
			wantDir: tr.real, wantOut: tr.real + "\n",
		},
		{
			name:    "dot slash operand skips the search",
			script:  "CDPATH=" + tr.cdp + "; cd ./" + filepath.Base(tr.real) + "; pwd",
			wantDir: tr.real, wantOut: tr.real + "\n",
		},
		{
			name:    "absolute operand skips the search",
			script:  "CDPATH=" + tr.cdp + "; cd " + tr.real + "; pwd",
			wantDir: tr.real, wantOut: tr.real + "\n",
		},
		{
			name: "no match falls back to the operand",
			// "target" exists only under cdp, so an unrelated CDPATH fails.
			script: "CDPATH=" + tr.real + "; cd target", wantDir: tr.root, wantStatus: 1,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, out, errb := newTestSession(t, tr.root, nil)
			r := s.Run(context.Background(), c.script)
			if r.Status != c.wantStatus || r.Err != nil {
				t.Fatalf("%+v stderr %q", r, errb.String())
			}
			if got := out.String(); got != c.wantOut {
				t.Fatalf("stdout %q, want %q", got, c.wantOut)
			}
			if got := s.Dir(); got != c.wantDir {
				t.Fatalf("dir %q, want %q", got, c.wantDir)
			}
		})
	}
}

func TestOnCwdChange(t *testing.T) {
	tr := newTree(t)
	var events [][2]string
	var dirSeen string
	var out, errb bytes.Buffer
	var s *Session
	var err error
	s, err = New(Options{
		Dir: tr.root, Env: []string{}, Stdout: &out, Stderr: &errb,
		OnCwdChange: func(oldDir, newDir string) {
			// The callback runs outside the run lock, so Dir is callable.
			dirSeen = s.Dir()
			events = append(events, [2]string{oldDir, newDir})
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	check := func(script string, want ...[2]string) {
		t.Helper()
		events, dirSeen = nil, ""
		if r := s.Run(context.Background(), script); r.Err != nil {
			t.Fatalf("%s: %+v", script, r)
		}
		if len(events) != len(want) {
			t.Fatalf("%s: events %v, want %v", script, events, want)
		}
		for i := range want {
			if events[i] != want[i] {
				t.Fatalf("%s: events %v, want %v", script, events, want)
			}
		}
		if len(want) > 0 && dirSeen != want[len(want)-1][1] {
			t.Fatalf("%s: Dir() in callback %q, want %q", script, dirSeen, want[len(want)-1][1])
		}
	}
	check("cd "+tr.sub, [2]string{tr.root, tr.sub})
	check("cd " + tr.sub)         // same directory, no event
	check("cd missing")           // failure, no event
	check("true")                 // no cd at all
	check("(cd " + tr.root + ")") // subshell only
	check("cd "+tr.root+"; cd "+tr.sub,
		[2]string{tr.sub, tr.root}, [2]string{tr.root, tr.sub}) // one event per change
	check("cd "+tr.root+"; cd missing", [2]string{tr.sub, tr.root}) // events survive a failing status
	check("cd -", [2]string{tr.root, tr.sub})
	check("cd " + tr.sub + "; unset OLDPWD; cd -") // cd - fails, no second event
}

func TestCdKeepsProcessState(t *testing.T) {
	tr := newTree(t)
	beforeDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	beforeEnv := os.Environ()
	s, _, _ := newTestSession(t, tr.root, nil)
	if r := s.Run(context.Background(), "cd "+tr.sub+"; export PWD; unset OLDPWD"); r.Status != 0 {
		t.Fatalf("%+v", r)
	}
	if after, err := os.Getwd(); err != nil || after != beforeDir {
		t.Fatalf("process directory %q, want %q", after, beforeDir)
	}
	if after := os.Environ(); !slices.Equal(after, beforeEnv) {
		t.Fatal("process environment changed")
	}
	if v, ok := os.LookupEnv("OLDPWD"); ok && v == tr.root {
		t.Fatal("process OLDPWD changed")
	}
}

func TestCdIgnoresModifiedPWD(t *testing.T) {
	tr := newTree(t)
	s, out, _ := newTestSession(t, tr.root, nil)
	// The session's logical directory is authoritative: a modified PWD must
	// not redirect relative operands, and both variables are repaired.
	if r := s.Run(context.Background(), "PWD=/nonexistent; cd realdir/sub; pwd"); r.Status != 0 {
		t.Fatalf("%+v", r)
	}
	if got := out.String(); got != tr.sub+"\n" {
		t.Fatalf("stdout %q", got)
	}
	if s.Dir() != tr.sub || s.Vars()["PWD"].Str != tr.sub || s.Vars()["OLDPWD"].Str != tr.root {
		t.Fatalf("dir %q PWD %q OLDPWD %q", s.Dir(), s.Vars()["PWD"].Str, s.Vars()["OLDPWD"].Str)
	}
}
