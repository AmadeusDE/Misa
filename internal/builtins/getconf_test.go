// Behaviour tests for the getconf builtin.
package builtins

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// wantConfstrPath is confstr(3)'s PATH per platform.
func wantConfstrPath() string {
	switch runtime.GOOS {
	case "darwin":
		return "/usr/bin:/bin:/usr/sbin:/sbin"
	case "android":
		return "/system/bin"
	}
	return "/usr/bin:/bin"
}

// wantArgMax is the platform's documented ARG_MAX.
func wantArgMax() string {
	switch runtime.GOOS {
	case "darwin":
		return "1048576"
	case "freebsd", "openbsd":
		return "262144"
	case "linux", "android":
		return "131072"
	}
	return "undefined"
}

// wantCharMax follows the target ABI's plain char signedness, which ARM,
// AArch64 and PowerPC make unsigned except on Apple platforms.
func wantCharMax() string {
	if runtime.GOOS != "darwin" {
		switch runtime.GOARCH {
		case "arm", "arm64", "ppc64", "ppc64le":
			return "255"
		}
	}
	return "127"
}

func TestGetconfValues(t *testing.T) {
	pageSize := strconv.Itoa(os.Getpagesize())
	longBit := strconv.Itoa(strconv.IntSize)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"PAGESIZE", []string{"PAGESIZE"}, pageSize},
		{"PAGE_SIZE", []string{"PAGE_SIZE"}, pageSize},
		{"LONG_BIT", []string{"LONG_BIT"}, longBit},
		{"WORD_BIT", []string{"WORD_BIT"}, "32"},
		{"CHAR_BIT", []string{"CHAR_BIT"}, "8"},
		{"CHAR_MAX", []string{"CHAR_MAX"}, wantCharMax()},
		{"INT_MAX", []string{"INT_MAX"}, "2147483647"},
		{"SSIZE_MAX", []string{"SSIZE_MAX"}, strconv.FormatInt(maxSSize(), 10)},
		{"_POSIX_VERSION", []string{"_POSIX_VERSION"}, "200809"},
		{"POSIX2_VERSION", []string{"POSIX2_VERSION"}, "200809"},
		{"_XOPEN_VERSION", []string{"_XOPEN_VERSION"}, "700"},
		{"PATH", []string{"PATH"}, wantConfstrPath()},
		{"CS_PATH", []string{"CS_PATH"}, wantConfstrPath()},
		{"ARG_MAX", []string{"ARG_MAX"}, wantArgMax()},
		{"NAME_MAX with path", []string{"NAME_MAX", "/"}, "255"},
		{"LINK_MAX with path", []string{"LINK_MAX", "/"}, wantLinkMax()},
		{"CHOWN_RESTRICTED with path", []string{"CHOWN_RESTRICTED", "/"}, "1"},
		{"NO_TRUNC with path", []string{"NO_TRUNC", "/"}, "1"},
		{"NPROCESSORS_ONLN", []string{"NPROCESSORS_ONLN"}, strconv.Itoa(runtime.NumCPU())},
		{"_NPROCESSORS_CONF", []string{"_NPROCESSORS_CONF"}, strconv.Itoa(runtime.NumCPU())},
		{"LFS_LDFLAGS", []string{"LFS_LDFLAGS"}, ""},
		{"LFS_LIBS", []string{"LFS_LIBS"}, ""},
	}
	for _, tc := range tests {
		r := runBuiltin(t, "getconf", tc.args...)
		assertExitCode(t, r, 0)
		if r.stderr != "" {
			t.Errorf("getconf %v stderr = %q, want empty", tc.args, r.stderr)
		}
		if want := tc.want + "\n"; r.stdout != want {
			t.Errorf("getconf %v stdout = %q, want %q", tc.args, r.stdout, want)
		}
	}
}

// wantLinkMax is the platform's documented LINK_MAX.
func wantLinkMax() string {
	switch runtime.GOOS {
	case "linux", "android":
		return "127"
	}
	return "32767"
}

func TestGetconfCharLimits(t *testing.T) {
	// x86-64 has a signed plain char everywhere, so this pins the common case
	// rather than only mirroring the ABI rule above.
	if runtime.GOARCH != "amd64" {
		t.Skip("plain char is unsigned on this architecture")
	}
	r := runBuiltin(t, "getconf", "CHAR_MAX")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "127\n")
	r = runBuiltin(t, "getconf", "CHAR_MIN")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "-128\n")
}

func TestGetconfOpenMax(t *testing.T) {
	var lim unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_NOFILE, &lim); err != nil {
		t.Skipf("getrlimit(RLIMIT_NOFILE): %v", err)
	}
	if lim.Cur == unix.RLIM_INFINITY {
		t.Skip("RLIMIT_NOFILE is unlimited")
	}
	r := runBuiltin(t, "getconf", "OPEN_MAX")
	assertExitCode(t, r, 0)
	assertStdout(t, r, strconv.FormatInt(int64(lim.Cur), 10)+"\n")
}

func TestGetconfUnknownName(t *testing.T) {
	r := runBuiltin(t, "getconf", "NOPE")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "getconf: bad 'NOPE'")
}

func TestGetconfUndefined(t *testing.T) {
	// Known name, but answering it needs a C library this build does not have.
	for _, name := range []string{"_POSIX_THREADS", "PTHREAD_KEYS_MAX", "ASYNC_IO"} {
		args := []string{name}
		if name == "ASYNC_IO" {
			args = append(args, "/") // pathconf(3) names need a path
		}
		r := runBuiltin(t, "getconf", args...)
		assertExitCode(t, r, 0)
		assertStdout(t, r, "undefined\n")
	}
}

func TestGetconfPathRules(t *testing.T) {
	r := runBuiltin(t, "getconf", "NAME_MAX")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "NAME_MAX needs a path")
	assertStderrContains(t, r, "usage: getconf")

	r = runBuiltin(t, "getconf", "PATH", "/")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "PATH does not take a path")
	assertStderrContains(t, r, "usage: getconf")

	// A sysconf(3) name does not take a path either.
	r = runBuiltin(t, "getconf", "PAGESIZE", "/")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "PAGESIZE does not take a path")

	// The path is never opened, so a missing one is still accepted.
	r = runBuiltin(t, "getconf", "NAME_MAX", "/nonexistent-path")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "255\n")
}

// getconfList parses -l output into group title → names.
func getconfList(t *testing.T) ([]string, map[string][]string) {
	t.Helper()
	r := runBuiltin(t, "getconf", "-l")
	assertExitCode(t, r, 0)
	if r.stderr != "" {
		t.Fatalf("getconf -l stderr = %q", r.stderr)
	}
	var titles []string
	groups := map[string][]string{}
	for _, line := range strings.Split(strings.TrimSuffix(r.stdout, "\n"), "\n") {
		if strings.HasPrefix(line, "  ") {
			if len(titles) == 0 {
				t.Fatalf("name %q before any group title", line)
			}
			name := strings.TrimPrefix(line, "  ")
			groups[titles[len(titles)-1]] = append(groups[titles[len(titles)-1]], name)
			continue
		}
		titles = append(titles, line)
	}
	return titles, groups
}

func TestGetconfList(t *testing.T) {
	titles, groups := getconfList(t)
	want := []string{"sysconf(3)", "pathconf(3)", "confstr(3)", "<limits.h>", "Misc"}
	if strings.Join(titles, ",") != strings.Join(want, ",") {
		t.Fatalf("getconf -l titles = %v, want %v", titles, want)
	}
	seen := map[string]string{}
	for _, title := range want {
		if len(groups[title]) == 0 {
			t.Errorf("group %q has no names", title)
		}
		for _, name := range groups[title] {
			if other, dup := seen[name]; dup {
				t.Errorf("name %q listed in both %q and %q", name, other, title)
			}
			seen[name] = title
		}
	}
	for _, name := range []string{"PAGESIZE", "PATH_MAX", "PATH", "CHAR_BIT", "LFS_CFLAGS"} {
		if _, ok := seen[name]; !ok {
			t.Errorf("getconf -l does not list %q", name)
		}
	}
	// Every listed name must also be accepted by a single lookup.
	for _, title := range want {
		for _, name := range groups[title] {
			args := []string{name}
			if title == "pathconf(3)" {
				args = append(args, "/")
			}
			r := runBuiltin(t, "getconf", args...)
			if r.code != 0 || r.stderr != "" || r.stdout == "" {
				t.Errorf("getconf %v = %+v, want a value", args, r)
			}
		}
	}
}

func TestGetconfAll(t *testing.T) {
	titles, groups := getconfList(t)
	var names []string
	for _, title := range titles {
		names = append(names, groups[title]...)
	}

	r := runBuiltin(t, "getconf", "-a")
	assertExitCode(t, r, 0)
	lines := strings.Split(strings.TrimSuffix(r.stdout, "\n"), "\n")
	if len(lines) != len(names) {
		t.Fatalf("getconf -a printed %d lines, want %d names from -l", len(lines), len(names))
	}
	for i, line := range lines {
		// %-34s then a space: a name longer than 34 columns (one exists)
		// pushes the value right instead of being cut.
		prefix := fmt.Sprintf("%-34s ", names[i])
		if !strings.HasPrefix(line, prefix) {
			t.Fatalf("line %q does not start with %q", line, prefix)
		}
		value := strings.TrimPrefix(line, prefix)
		if value == "undefined" || value == "" {
			continue
		}
		if _, err := strconv.ParseInt(value, 10, 64); err == nil {
			continue
		}
		if _, err := strconv.ParseUint(value, 10, 64); err == nil {
			continue
		}
		// Paths and LFS flags are the only non-numeric values.
		if strings.ContainsAny(value, "/:") || strings.HasPrefix(value, "-D_") {
			continue
		}
		t.Errorf("line %q has an unexpected value %q", line, value)
	}

	// -a wins over -l, as in Toybox.
	if both := runBuiltin(t, "getconf", "-a", "-l"); both.stdout != r.stdout || both.code != 0 {
		t.Errorf("getconf -a -l = %q (exit %d), want the -a output", both.stdout, both.code)
	}

	// The path is never opened.
	missing := runBuiltin(t, "getconf", "-a", "/nonexistent-path")
	assertExitCode(t, missing, 0)
	if got := strings.Split(strings.TrimSuffix(missing.stdout, "\n"), "\n"); len(got) != len(names) {
		t.Errorf("getconf -a /nonexistent-path printed %d lines, want %d", len(got), len(names))
	}
}

func TestGetconfCacheNames(t *testing.T) {
	// Cache geometry comes from sysfs on Linux; elsewhere (or on a machine
	// without sysfs) it is undefined, never a made-up number.
	r := runBuiltin(t, "getconf", "LEVEL1_DCACHE_SIZE")
	assertExitCode(t, r, 0)
	value := strings.TrimSuffix(r.stdout, "\n")
	if value == "undefined" {
		if runtime.GOOS == "linux" {
			t.Log("no sysfs cache geometry on this machine")
		}
		return
	}
	if n, err := strconv.ParseInt(value, 10, 64); err != nil || n <= 0 {
		t.Errorf("LEVEL1_DCACHE_SIZE = %q, want a positive number or undefined", value)
	}
	if runtime.GOOS != "linux" {
		t.Errorf("LEVEL1_DCACHE_SIZE = %q on %s, want undefined", value, runtime.GOOS)
	}
}

func TestGetconfErrors(t *testing.T) {
	r := runBuiltin(t, "getconf", "a", "b", "c")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "getconf: extra operand 'c'")

	// Toybox has no -v and no --version.
	for _, arg := range []string{"-v", "--version", "-av"} {
		r = runBuiltin(t, "getconf", arg)
		assertExitCode(t, r, 1)
		assertStdout(t, r, "")
		assertStderrContains(t, r, "getconf: unknown option")
	}

	r = runBuiltin(t, "getconf", "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "usage: getconf -a [PATH] | -l | NAME [PATH]")

	// No operands prints the usage and succeeds, as Toybox does.
	r = runBuiltin(t, "getconf")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "usage: getconf")
}
