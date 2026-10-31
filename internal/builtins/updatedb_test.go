// updatedb tests. Case list inspired by uutils/findutils tests/db_tests.rs,
// commit d71f3242b0bc3b21bb311f04b23ae62e5f3fed67 (MIT): an invalid flag, an
// empty prune list, an un-creatable output path and a full updatedb -> locate
// round trip.
package builtins

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// locateEnv is the environment the locate/updatedb tests hand to a builtin:
// no ambient HOME or XDG_CACHE_HOME, so nothing touches the developer's cache.
func locateEnv(home, cache string) []string {
	return []string{"HOME=" + home, "XDG_CACHE_HOME=" + cache}
}

func runLocateCommand(t *testing.T, dir string, env []string, name string, args ...string) runResult {
	t.Helper()
	cmd, ok := Lookup(name)
	if !ok {
		t.Fatalf("builtin %q not registered", name)
	}
	var stdout, stderr bytes.Buffer
	ctx := &Context{Dir: dir, Args: args, CmdName: name, Env: env, Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")}
	code := cmd.Run(ctx)
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func TestUpdatedbBuildsDatabase(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "tree/a.txt", "x")
	writeFile(t, dir, "tree/sub/b.txt", "x")
	db := filepath.Join(dir, "locatedb")

	r := runLocateCommand(t, dir, locateEnv(dir, ""), "updatedb", "--localpaths=tree", "--output="+db)
	assertExitCode(t, r, 0)
	if r.stderr != "" {
		t.Errorf("stderr = %q, want none", r.stderr)
	}

	data, err := os.ReadFile(db)
	if err != nil {
		t.Fatalf("database not written: %v", err)
	}
	hdr, paths := readLocateDB(t, data)
	if len(hdr.Roots) != 1 || hdr.Roots[0] != filepath.Join(dir, "tree") {
		t.Errorf("roots = %q, want the absolute tree path", hdr.Roots)
	}
	joined := strings.Join(paths, "\n")
	for _, want := range []string{"tree", filepath.Join("tree", "a.txt"), filepath.Join("tree", "sub", "b.txt")} {
		if !strings.Contains(joined, want) {
			t.Errorf("database %q does not index %q", joined, want)
		}
	}
	for _, p := range paths {
		if !filepath.IsAbs(p) {
			t.Errorf("indexed a non-absolute path %q", p)
		}
	}
}

func TestUpdatedbPrunePaths(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "tree/keep.txt", "x")
	writeFile(t, dir, "tree/drop/deep.txt", "x")
	writeFile(t, dir, "tree/droplet/still-here.txt", "x")
	db := filepath.Join(dir, "locatedb")
	prune := filepath.Join(dir, "tree", "drop")

	r := runLocateCommand(t, dir, locateEnv(dir, ""), "updatedb",
		"--localpaths=tree", "--prunepaths="+prune, "--output="+db)
	assertExitCode(t, r, 0)
	_, paths := readLocateDB(t, mustReadFile(t, db))
	joined := strings.Join(paths, "\n")
	if strings.Contains(joined, "drop/deep.txt") || strings.Contains(joined, "/drop\n") {
		t.Errorf("pruned subtree leaked into the database: %q", joined)
	}
	// A sibling whose name merely starts with the pruned name must survive.
	if !strings.Contains(joined, "droplet") {
		t.Errorf("pruning was too eager: %q", joined)
	}
}

func TestUpdatedbDoesNotFollowSymlinks(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	writeFile(t, outside, "hidden/secret.txt", "x")
	if err := os.Symlink(outside, filepath.Join(dir, "tree", "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	writeFile(t, dir, "tree/plain.txt", "x")
	db := filepath.Join(dir, "locatedb")
	r := runLocateCommand(t, dir, locateEnv(dir, ""), "updatedb", "--localpaths=tree", "--output="+db)
	assertExitCode(t, r, 0)
	_, paths := readLocateDB(t, mustReadFile(t, db))
	joined := strings.Join(paths, "\n")
	if !strings.Contains(joined, "link") {
		t.Errorf("the symlink itself should be indexed: %q", joined)
	}
	if strings.Contains(joined, "secret.txt") {
		t.Errorf("updatedb followed a symlink: %q", joined)
	}
}

func TestUpdatedbUnreadableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read every directory")
	}
	dir := t.TempDir()
	writeFile(t, dir, "tree/ok.txt", "x")
	locked := filepath.Join(dir, "tree", "locked")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "tree/locked/hidden.txt", "x")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	db := filepath.Join(dir, "locatedb")
	r := runLocateCommand(t, dir, locateEnv(dir, ""), "updatedb", "--localpaths=tree", "--output="+db)
	// The unreadable directory is reported and makes the run unsuccessful,
	// but the database is still written with everything else.
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "locked")
	_, paths := readLocateDB(t, mustReadFile(t, db))
	joined := strings.Join(paths, "\n")
	if !strings.Contains(joined, "ok.txt") {
		t.Errorf("readable entries missing: %q", joined)
	}
	if strings.Contains(joined, "hidden.txt") {
		t.Errorf("unreadable directory contents leaked: %q", joined)
	}
}

func TestUpdatedbAtomicReplacement(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "locatedb")
	old := buildLocateDB(t, []string{"/old/entry"})
	if err := os.WriteFile(db, old, 0o644); err != nil {
		t.Fatal(err)
	}
	injected := errors.New("walk exploded")
	err := writeLocateDBFile(db, 1, []string{"/"}, func(add func(string) error) error {
		if err := add("/new/entry"); err != nil {
			return err
		}
		return injected
	})
	if err == nil || !strings.Contains(err.Error(), injected.Error()) {
		t.Fatalf("err = %v, want it to report %v", err, injected)
	}
	got, err := os.ReadFile(db)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, old) {
		t.Errorf("the previous database was replaced by a failed run")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("temporary files left behind: %v", entries)
	}
}

func TestUpdatedbOutputCreateError(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "does-not-exist", "locatedb")
	r := runLocateCommand(t, dir, locateEnv(dir, ""), "updatedb", "--localpaths=tree", "--output="+bad)
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "cannot create")
	assertStderrContains(t, r, bad)
}

func TestUpdatedbUnsavedDatabaseOnCancel(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "tree/a.txt", "x")
	db := filepath.Join(dir, "locatedb")
	cmd, _ := Lookup("updatedb")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	code := cmd.Run(&Context{
		Dir: dir, Args: []string{"--localpaths=tree", "--output=" + db}, Env: locateEnv(dir, ""),
		Stdout: &stdout, Stderr: &stderr, Cancellation: ctx,
	})
	if code == 0 {
		t.Errorf("a cancelled run must not report success")
	}
	if _, err := os.Stat(db); err == nil {
		t.Errorf("a cancelled run left a database behind")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".locatedb-") {
			t.Errorf("temporary file left behind: %s", e.Name())
		}
	}
}

func TestUpdatedbUnsupportedOptions(t *testing.T) {
	dir := t.TempDir()
	for _, opt := range []string{"--prunefs=ext4", "--findoptions=-xdev", "--dbformat=LOCATE02", "--netpaths=/nfs", "--localuser=root", "--netuser=root"} {
		r := runLocateCommand(t, dir, locateEnv(dir, ""), "updatedb", opt)
		assertExitCode(t, r, 2)
		assertStderrContains(t, r, "not supported")
	}
	r := runLocateCommand(t, dir, locateEnv(dir, ""), "updatedb", "--unknown-option")
	assertExitCode(t, r, 2)
	assertStderrContains(t, r, "unknown option")
	r = runLocateCommand(t, dir, locateEnv(dir, ""), "updatedb", "extra-operand")
	assertExitCode(t, r, 2)
	assertStderrContains(t, r, "extra operand")
}

func TestUpdatedbDefaultDatabasePath(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "cache")
	home := filepath.Join(dir, "home")
	writeFile(t, dir, "tree/a.txt", "x")

	r := runLocateCommand(t, dir, []string{"HOME=" + home, "XDG_CACHE_HOME=" + cache}, "updatedb", "--localpaths=tree")
	assertExitCode(t, r, 0)
	want := filepath.Join(cache, "misa", "locatedb")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("no database at %s: %v", want, err)
	}

	// Without XDG_CACHE_HOME the database lives under ~/.cache.
	r = runLocateCommand(t, dir, []string{"HOME=" + home, "XDG_CACHE_HOME="}, "updatedb", "--localpaths=tree")
	assertExitCode(t, r, 0)
	want = filepath.Join(home, ".cache", "misa", "locatedb")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("no database at %s: %v", want, err)
	}

	// With neither variable set the path cannot be derived: fail honestly.
	r = runLocateCommand(t, dir, []string{}, "updatedb", "--localpaths=tree")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "XDG_CACHE_HOME")
}

func TestUpdatedbHelp(t *testing.T) {
	dir := t.TempDir()
	r := runLocateCommand(t, dir, locateEnv(dir, ""), "updatedb", "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Usage: updatedb")
	assertStdoutContains(t, r, "--localpaths")
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

// TestUpdatedbUnwritableDestinationKeepsOldDB covers the other half of atomic
// replacement: if the temporary file cannot even be created, the previous
// database is untouched.
func TestUpdatedbUnwritableDestinationKeepsOldDB(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write into a read-only directory")
	}
	dir := t.TempDir()
	cache := filepath.Join(dir, "cache")
	db := filepath.Join(cache, "locatedb")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	old := buildLocateDB(t, []string{"/old/entry"})
	if err := os.WriteFile(db, old, 0o644); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "tree/new.txt", "x")
	if err := os.Chmod(cache, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(cache, 0o755) })

	r := runLocateCommand(t, dir, locateEnv(dir, ""), "updatedb", "--localpaths=tree", "--output="+db)
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "cannot create")

	got, err := os.ReadFile(db)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, old) {
		t.Errorf("the previous database was damaged by a failed run")
	}
}
