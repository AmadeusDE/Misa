// locate tests. Matching modes, exit statuses and the access checks are
// modelled on uutils/findutils tests/db_tests.rs, commit
// d71f3242b0bc3b21bb311f04b23ae62e5f3fed67 (MIT); the database is written by
// the Misa updatedb above so the round trip is exercised end to end.
package builtins

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// buildLocateTree indexes dir and returns the database path.
func buildLocateTree(t *testing.T, dir string, extra ...string) string {
	t.Helper()
	db := filepath.Join(dir, "locatedb")
	args := append([]string{"--localpaths=tree", "--output=" + db}, extra...)
	r := runLocateCommand(t, dir, locateEnv(dir, ""), "updatedb", args...)
	if r.code != 0 {
		t.Fatalf("updatedb failed: %d %s", r.code, r.stderr)
	}
	return db
}

func TestLocateSubstringAndGlob(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "tree/alpha.txt", "x")
	writeFile(t, dir, "tree/beta.txt", "x")
	writeFile(t, dir, "tree/sub/gamma.log", "x")
	db := buildLocateTree(t, dir)
	prefix := filepath.Join(dir, "tree")

	tests := []struct {
		name string
		args []string
		want []string
		code int
	}{
		{name: "substring", args: []string{"alpha"}, want: []string{prefix + "/alpha.txt"}},
		{name: "substring shared", args: []string{".t"}, want: []string{prefix + "/alpha.txt", prefix + "/beta.txt"}},
		{name: "glob whole path", args: []string{prefix + "/*.txt"}, want: []string{prefix + "/alpha.txt", prefix + "/beta.txt"}},
		// A glob is matched against the whole path and, as with findutils'
		// fnmatch(3) call, '*' spans directory separators.
		{name: "glob spans separators", args: []string{"*.log"}, want: []string{prefix + "/sub/gamma.log"}},
		{name: "glob not anchored", args: []string{"*alpha*"}, want: []string{prefix + "/alpha.txt"}},
		{name: "glob in a subdirectory", args: []string{prefix + "/*/gamma*"}, want: []string{prefix + "/sub/gamma.log"}},
		{name: "question mark", args: []string{"-b", "?eta.txt"}, want: []string{prefix + "/beta.txt"}},
		{name: "case sensitive by default", args: []string{"ALPHA"}, want: nil, code: 1},
		{name: "ignore case", args: []string{"-i", "ALPHA"}, want: []string{prefix + "/alpha.txt"}},
		{name: "basename mode skips directories", args: []string{"-b", "gamma.log"}, want: []string{prefix + "/sub/gamma.log"}},
		{name: "basename mode hidden by directory name", args: []string{"-b", "sub"}, want: []string{prefix + "/sub"}},
		{name: "basename mode ignores parent name", args: []string{"-b", "tree"}, want: []string{prefix}},
		{name: "wholename overrides basename", args: []string{"-b", "-w", "tree/alpha"}, want: []string{prefix + "/alpha.txt"}},
		{name: "regex", args: []string{"-r", `alpha\.txt$`}, want: []string{prefix + "/alpha.txt"}},
		{name: "regex case insensitive", args: []string{"-r", "-i", `^.*ALPHA`}, want: []string{prefix + "/alpha.txt"}},
		{name: "two patterns match either", args: []string{"alpha", "gamma"}, want: []string{prefix + "/alpha.txt", prefix + "/sub/gamma.log"}},
		{name: "all patterns required", args: []string{"-a", "tree", "alpha"}, want: []string{prefix + "/alpha.txt"}},
		{name: "all patterns with one miss", args: []string{"-a", "tree", "nope"}, want: nil, code: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"-d", db}, tc.args...)
			r := runLocateCommand(t, dir, locateEnv(dir, ""), "locate", args...)
			assertExitCode(t, r, tc.code)
			var want string
			if len(tc.want) > 0 {
				want = strings.Join(tc.want, "\n") + "\n"
			}
			assertStdout(t, r, want)
		})
	}
}

func TestLocateCountLimitAndNull(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"one", "two", "three"} {
		writeFile(t, dir, "tree/"+name+".dat", "x")
	}
	db := buildLocateTree(t, dir)
	prefix := filepath.Join(dir, "tree")

	r := runLocateCommand(t, dir, locateEnv(dir, ""), "locate", "-d", db, "-c", ".dat")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "3\n")

	r = runLocateCommand(t, dir, locateEnv(dir, ""), "locate", "-d", db, "-c", "nothing-matches")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "0\n")

	r = runLocateCommand(t, dir, locateEnv(dir, ""), "locate", "-l", "2", "-d", db, ".dat")
	assertExitCode(t, r, 0)
	// Paths come out in database (byte-sorted) order: one, three, two.
	assertStdout(t, r, prefix+"/one.dat\n"+prefix+"/three.dat\n")

	// A limit of zero means "no limit", not "no output".
	r = runLocateCommand(t, dir, locateEnv(dir, ""), "locate", "-l", "0", "-d", db, "-c", ".dat")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "3\n")

	r = runLocateCommand(t, dir, locateEnv(dir, ""), "locate", "-0", "-d", db, ".dat")
	assertExitCode(t, r, 0)
	assertStdout(t, r, prefix+"/one.dat\x00"+prefix+"/three.dat\x00"+prefix+"/two.dat\x00")
}

// TestLocateNewlineInFilename checks the NUL-impossible/newline-valid
// boundary: a name with a newline is printed verbatim by default and is
// unambiguous with -0.
func TestLocateNewlineInFilename(t *testing.T) {
	dir := t.TempDir()
	name := "weird\nname"
	writeFile(t, dir, "tree/"+name, "x")
	db := buildLocateTree(t, dir)
	want := filepath.Join(dir, "tree", name)

	r := runLocateCommand(t, dir, locateEnv(dir, ""), "locate", "-d", db, "weird")
	assertExitCode(t, r, 0)
	assertStdout(t, r, want+"\n")

	r = runLocateCommand(t, dir, locateEnv(dir, ""), "locate", "-0", "-d", db, "weird")
	assertExitCode(t, r, 0)
	assertStdout(t, r, want+"\x00")
}

func TestLocateSkipsDeletedPaths(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "tree/gone.txt", "x")
	writeFile(t, dir, "tree/stays.txt", "x")
	db := buildLocateTree(t, dir)
	if err := os.Remove(filepath.Join(dir, "tree", "gone.txt")); err != nil {
		t.Fatal(err)
	}

	r := runLocateCommand(t, dir, locateEnv(dir, ""), "locate", "-d", db, "gone")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")

	r = runLocateCommand(t, dir, locateEnv(dir, ""), "locate", "-d", db, "-e", "stays")
	assertExitCode(t, r, 0)
	assertStdout(t, r, filepath.Join(dir, "tree", "stays.txt")+"\n")
}

// TestLocateAccessCheck indexes a path through a directory the caller cannot
// search, as if another user had built the database, and expects locate to
// withhold it.
func TestLocateAccessCheck(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can search every directory")
	}
	dir := t.TempDir()
	hidden := filepath.Join(dir, "hidden")
	if err := os.MkdirAll(hidden, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(hidden, "secret.txt")
	if err := os.WriteFile(secret, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(dir, "locatedb")
	// Build the database while the directory is still reachable.
	r := runLocateCommand(t, dir, locateEnv(dir, ""), "updatedb", "--localpaths=hidden", "--output="+db)
	assertExitCode(t, r, 0)

	if err := os.Chmod(hidden, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(hidden, 0o755) })

	r = runLocateCommand(t, dir, locateEnv(dir, ""), "locate", "-d", db, "secret")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")

	// The file still exists; only the search permission was missing.
	if err := os.Chmod(hidden, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(secret); err != nil {
		t.Fatalf("fixture file vanished: %v", err)
	}
}

func TestLocateUnreadableDatabase(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "no-such-db")
	r := runLocateCommand(t, dir, locateEnv(dir, ""), "locate", "-d", missing, "x")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "cannot open database")

	bad := filepath.Join(dir, "bad-db")
	if err := os.WriteFile(bad, []byte("this is not a locate database at all, just text\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r = runLocateCommand(t, dir, locateEnv(dir, ""), "locate", "-d", bad, "x")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "not a Misa locate database")

	truncated := filepath.Join(dir, "truncated-db")
	if err := os.WriteFile(truncated, buildLocateDB(t, locateTestPaths)[:20], 0o644); err != nil {
		t.Fatal(err)
	}
	r = runLocateCommand(t, dir, locateEnv(dir, ""), "locate", "-d", truncated, "x")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "truncated")
}

func TestLocateUsageErrors(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "tree/anything.txt", "x")
	db := buildLocateTree(t, dir)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "no pattern", args: []string{"-d", db}, want: "missing pattern"},
		{name: "unknown option", args: []string{"-d", db, "--bogus", "x"}, want: "unknown option"},
		{name: "statistics unsupported", args: []string{"-d", db, "-S", "x"}, want: "--statistics is not supported"},
		{name: "non-existing unsupported", args: []string{"-d", db, "-E", "x"}, want: "--non-existing is not supported"},
		{name: "follow unsupported", args: []string{"-d", db, "-L", "x"}, want: "-L/--follow is not supported"},
		{name: "mmap unsupported", args: []string{"-d", db, "--mmap", "x"}, want: "--mmap is not supported"},
		{name: "regextype unsupported", args: []string{"-d", db, "--regextype=posix-basic", "x"}, want: "--regextype is not supported"},
		{name: "bad regex", args: []string{"-d", db, "-r", "[", "x"}, want: "invalid regular expression"},
		{name: "bad glob", args: []string{"-d", db, "[a-", "x"}, want: "invalid pattern"},
		{name: "bad limit", args: []string{"-d", db, "-l", "-3", "x"}, want: "invalid limit"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := runLocateCommand(t, dir, locateEnv(dir, ""), "locate", tc.args...)
			assertExitCode(t, r, 2)
			assertStderrContains(t, r, tc.want)
		})
	}
}

func TestLocateDefaultDatabaseFromEnvironment(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "cache")
	writeFile(t, dir, "tree/found.txt", "x")
	env := []string{"HOME=" + dir, "XDG_CACHE_HOME=" + cache}

	r := runLocateCommand(t, dir, env, "updatedb", "--localpaths=tree")
	assertExitCode(t, r, 0)
	r = runLocateCommand(t, dir, env, "locate", "found.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, filepath.Join(dir, "tree", "found.txt")+"\n")

	// LOCATE_PATH supplies the database when -d is absent.
	other := filepath.Join(dir, "elsewhere")
	if err := os.Rename(filepath.Join(cache, "misa", "locatedb"), other); err != nil {
		t.Fatal(err)
	}
	r = runLocateCommand(t, dir, append(env, "LOCATE_PATH="+other), "locate", "found.txt")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "found.txt")

	// A database list is not supported and says so instead of guessing.
	r = runLocateCommand(t, dir, append(env, "LOCATE_PATH="+other+string(filepath.ListSeparator)+other), "locate", "found.txt")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "LOCATE_PATH with several databases")
}

func TestLocateHelp(t *testing.T) {
	dir := t.TempDir()
	r := runLocateCommand(t, dir, locateEnv(dir, ""), "locate", "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Usage: locate")
	assertStdoutContains(t, r, "--basename")
}

// TestLocateCancellation checks that a cancelled invocation stops scanning a
// long database instead of printing everything.
func TestLocateCancellation(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "locatedb")
	var paths []string
	for i := 0; i < 4096; i++ {
		paths = append(paths, filepath.Join(dir, "tree", "f"+string(rune('a'+i%26))+strconv.Itoa(i)))
	}
	var buf bytes.Buffer
	lw, err := newLocateWriter(&buf, 0, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		if err := lw.Add(p); err != nil {
			t.Fatal(err)
		}
	}
	if err := lw.Finish(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(db, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd, _ := Lookup("locate")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	code := cmd.Run(&Context{
		Dir: dir, Args: []string{"-d", db, "f"}, Env: locateEnv(dir, ""),
		Stdout: &stdout, Stderr: &stderr, Cancellation: ctx,
	})
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "interrupted") {
		t.Errorf("stderr = %q, want an interruption notice", stderr.String())
	}
}

// TestLocateUpdatedbInvocationIsolation runs two invocations concurrently in
// different directories with different HOME/XDG_CACHE_HOME values: each must
// build and query its own database without touching process state.
func TestLocateUpdatedbInvocationIsolation(t *testing.T) {
	type invocation struct {
		dir     string
		cache   string
		env     []string
		marker  string
		out     bytes.Buffer
		stderr  bytes.Buffer
		results []string
	}
	runs := make([]*invocation, 2)
	for i := range runs {
		dir := t.TempDir()
		marker := "marker-" + strconv.Itoa(i)
		writeFile(t, dir, "tree/"+marker, "x")
		runs[i] = &invocation{
			dir:    dir,
			cache:  filepath.Join(dir, "cache"),
			marker: marker,
			env:    []string{"HOME=" + filepath.Join(dir, "home"), "XDG_CACHE_HOME=" + filepath.Join(dir, "cache")},
		}
	}
	var wg sync.WaitGroup
	for _, r := range runs {
		wg.Go(func() {
			if code := runUpdatedbCtx(r.dir, r.env, &r.out, &r.stderr, "--localpaths=tree"); code != 0 {
				t.Errorf("updatedb %d: code=%d stderr=%q", 0, code, r.stderr.String())
				return
			}
			cmd, _ := Lookup("locate")
			var out, errOut bytes.Buffer
			code := cmd.Run(&Context{Dir: r.dir, Args: []string{"marker"}, Env: r.env, CmdName: "locate", Stdout: &out, Stderr: &errOut})
			if code != 0 {
				t.Errorf("locate in %s: code=%d stderr=%q", r.dir, code, errOut.String())
				return
			}
			r.results = strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
		})
	}
	wg.Wait()
	for i, r := range runs {
		if len(r.results) != 1 {
			t.Fatalf("invocation %d matched %q, want exactly its own file", i, r.results)
		}
		if want := filepath.Join(r.dir, "tree", r.marker); r.results[0] != want {
			t.Errorf("invocation %d matched %q, want %q", i, r.results[0], want)
		}
	}
}

// runUpdatedbCtx runs updatedb with an explicit Context so concurrent runs can
// be started from one test.
func runUpdatedbCtx(dir string, env []string, stdout, stderr *bytes.Buffer, args ...string) int {
	cmd, _ := Lookup("updatedb")
	return cmd.Run(&Context{Dir: dir, Args: args, CmdName: "updatedb", Env: env, Stdout: stdout, Stderr: stderr})
}
