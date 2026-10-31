package uutils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupPathSafety(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"../escape", "/tmp/escape", "link/file", "link"} {
		if isPathSafe(root, p) {
			t.Errorf("accepted unsafe path %q", p)
		}
	}
	if !isPathSafe(root, "nested/file") {
		t.Error("rejected safe path")
	}
}

func TestFixtureReferenceSafety(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"../secret", "/etc/passwd", "link"} {
		if _, ok := resolveFixtureRef("\x00FIXTURE\x00"+p, root); ok {
			t.Errorf("accepted %q", p)
		}
	}
}

func TestKnownFailuresReasons(t *testing.T) {
	for id, reason := range knownFailures() {
		if reason == "" {
			t.Errorf("%s has no reason", id)
		}
	}
}

func TestKnownFailuresExist(t *testing.T) {
	seen := map[string]bool{}
	for name := range knownFailures() {
		seen[name] = false
	}
	entries, err := os.ReadDir("testdata/tests/by-util")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		n := e.Name()
		if !strings.HasPrefix(n, "test_") || !strings.HasSuffix(n, ".rs") {
			continue
		}
		cmd := strings.TrimSuffix(strings.TrimPrefix(n, "test_"), ".rs")
		s, err := parseTestFile(filepath.Join("testdata/tests/by-util", n), cmd)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range s.cases {
			key := fmt.Sprintf("%s/%s_L%d", cmd, c.name, c.line)
			if _, ok := seen[key]; ok {
				seen[key] = true
			}
		}
	}
	for key, found := range seen {
		if !found {
			t.Errorf("known exclusion %s is not a parsed case", key)
		}
	}
}

func TestSymlinkTargetLexicalSafety(t *testing.T) {
	for _, p := range []string{"1", "slink", "nested/file"} {
		if !safeSymlinkTarget(p) {
			t.Errorf("rejected %q", p)
		}
	}
	for _, p := range []string{"../escape", "/tmp/escape", "nested/../../escape"} {
		if safeSymlinkTarget(p) {
			t.Errorf("accepted %q", p)
		}
	}
}

func TestSetupSkipReasonRecorded(t *testing.T) {
	rec := &runRecord{}
	root := t.TempDir()
	t.Run("refused", func(t *testing.T) {
		executeSetup(t, root, []setupAction{{kind: setupTouch, path: "../outside"}}, rec)
	})
	if rec.reason == nil || rec.reason.Category != "unsafe_setup" || !strings.Contains(rec.reason.Detail, "../outside") {
		t.Fatalf("missing setup skip detail: %+v", rec.reason)
	}
}

func TestMutatorArgumentSafety(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	for _, v := range []struct {
		cmd  string
		args []string
	}{
		{"cp", []string{"file", "../outside"}}, {"rm", []string{"/tmp/outside"}},
		{"touch", []string{"--reference=/etc/passwd", "file"}}, {"tee", []string{"link/out"}},
		{"ln", []string{"target", "link"}},
	} {
		if detail := unsafeMutatorArgs(root, v.cmd, v.args); detail == "" {
			t.Errorf("accepted %s %q", v.cmd, v.args)
		}
	}
	if detail := unsafeMutatorArgs(root, "cp", []string{"-r", "src", "dest"}); detail != "" {
		t.Errorf("safe path rejected: %s", detail)
	}
	if detail := unsafeMutatorArgs(root, "echo", []string{"/tmp/text"}); detail != "" {
		t.Errorf("text treated as path: %s", detail)
	}
}

func TestFixtureCopyRejectsSymlinkRoot(t *testing.T) {
	source := t.TempDir()
	dest := t.TempDir()
	parent := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "secret"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "fixtures")
	if err := os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}
	copyFixtures(link, dest)
	if _, err := os.Stat(filepath.Join(dest, "secret")); !os.IsNotExist(err) {
		t.Fatalf("copied from symlink root: %v", err)
	}
	if _, ok := resolveFixtureRef("\x00FIXTURE\x00secret", link); ok {
		t.Fatal("read through symlink root")
	}
}

func TestContainedSymlinkPaths(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "real"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if !isPathSafe(root, "link/file") {
		t.Fatal("rejected contained symlink")
	}
	if err := os.Symlink("loop", filepath.Join(root, "loop")); err != nil {
		t.Fatal(err)
	}
	if isPathSafe(root, "loop/file") {
		t.Fatal("accepted unresolved loop")
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "outside")); err != nil {
		t.Fatal(err)
	}
	if detail := unsafeMutatorArgs(root, "rm", []string{"outside"}); detail != "" {
		t.Fatalf("removing link itself is safe: %s", detail)
	}
	if detail := unsafeMutatorArgs(root, "rm", []string{"outside/file"}); detail == "" {
		t.Fatal("accepted outside traversal")
	}
}

func TestMutatorNonPathOptions(t *testing.T) {
	root := t.TempDir()
	for _, v := range []struct {
		cmd  string
		args []string
	}{
		{"truncate", []string{"-s", "/4", "file"}}, {"truncate", []string{"--size=/4", "file"}},
		{"touch", []string{"--date", "2020/01/01", "file"}}, {"touch", []string{"-t", "2020/01/01", "file"}},
	} {
		if detail := unsafeMutatorArgs(root, v.cmd, v.args); detail != "" {
			t.Errorf("non-path option %s %q: %s", v.cmd, v.args, detail)
		}
	}
}
