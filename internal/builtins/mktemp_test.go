package builtins

import (
	"os"
	"strings"
	"testing"
)

func TestMktempDefault(t *testing.T) {
	r := runBuiltin(t, "mktemp")
	assertExitCode(t, r, 0)
	path := strings.TrimSpace(r.stdout)
	if path == "" {
		t.Fatal("expected a path")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if info.IsDir() {
		t.Error("should be a file, not directory")
	}
	os.Remove(path)
}

func TestMktempDirectory(t *testing.T) {
	r := runBuiltin(t, "mktemp", "-d")
	assertExitCode(t, r, 0)
	path := strings.TrimSpace(r.stdout)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("should be a directory")
	}
	os.Remove(path)
}

func TestMktempCustomDir(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltin(t, "mktemp", "-p", dir)
	assertExitCode(t, r, 0)
	path := strings.TrimSpace(r.stdout)
	if !strings.HasPrefix(path, dir) {
		t.Errorf("path %q should be under %q", path, dir)
	}
	os.Remove(path)
}

func TestMktempUnique(t *testing.T) {
	r1 := runBuiltin(t, "mktemp")
	r2 := runBuiltin(t, "mktemp")
	assertExitCode(t, r1, 0)
	assertExitCode(t, r2, 0)
	p1 := strings.TrimSpace(r1.stdout)
	p2 := strings.TrimSpace(r2.stdout)
	if p1 == p2 {
		t.Error("two mktemp calls should produce different paths")
	}
	os.Remove(p1)
	os.Remove(p2)
}

func TestMktempTemplate(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltin(t, "mktemp", "-p", dir, "test.XXXXXX")
	assertExitCode(t, r, 0)
	path := strings.TrimSpace(r.stdout)
	if !strings.Contains(path, "test.") {
		t.Errorf("path %q should contain template prefix", path)
	}
	os.Remove(path)
}

func TestMktempDryRun(t *testing.T) {
	r := runBuiltin(t, "mktemp", "-u")
	assertExitCode(t, r, 0)
	path := strings.TrimSpace(r.stdout)
	if _, err := os.Stat(path); err == nil {
		t.Error("dry-run should not create file")
		os.Remove(path)
	}
}
