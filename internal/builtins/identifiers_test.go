// Behaviour tests for the mcookie, uuidgen, and hostid builtins.
package builtins

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestUUIDGen(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{nil, `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\n$`},
		{[]string{"-t"}, `^[0-9a-f]{8}-[0-9a-f]{4}-1[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\n$`},
		{[]string{"-m", "--namespace", "@dns", "--name", "example.com"}, "9073926b-929f-31c2-abc9-fad77ae3e8eb\n"},
	} {
		r := runBuiltin(t, "uuidgen", tc.args...)
		assertExitCode(t, r, 0)
		if matched, _ := regexp.MatchString(tc.want, r.stdout); !matched {
			t.Errorf("uuidgen %v stdout %q does not match %q", tc.args, r.stdout, tc.want)
		}
	}
	assertExitCode(t, runBuiltin(t, "uuidgen", "-r", "-t"), 1)
}

func TestMcookie(t *testing.T) {
	r := runBuiltin(t, "mcookie")
	assertExitCode(t, r, 0)
	if !regexp.MustCompile(`^[0-9a-f]{32}\n$`).MatchString(r.stdout) {
		t.Errorf("invalid cookie %q", r.stdout)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "seed"), []byte("seed data"), 0600); err != nil {
		t.Fatal(err)
	}
	r = runBuiltinIn(t, dir, "mcookie", "-v", "-f", "seed")
	assertExitCode(t, r, 0)
	if !strings.Contains(r.stderr, "Got 9 bytes from seed") {
		t.Errorf("verbose stderr %q", r.stderr)
	}
}

func TestHostid(t *testing.T) {
	r := runBuiltin(t, "hostid", "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Print the numeric identifier")
	r = runBuiltin(t, "hostid", "--invalid-argument")
	assertExitCode(t, r, 1)
}
