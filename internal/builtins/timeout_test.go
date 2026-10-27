//go:build unix

// Behaviour adapted from toybox timeout.c (0BSD) and uutils coreutils timeout (MIT).
package builtins

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestTimeoutDuration(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  time.Duration
	}{
		{"0", 0}, {"-0", 0}, {"0e-9999", 0}, {"0x0p-9999", 0}, {".1", 100 * time.Millisecond}, {"2s", 2 * time.Second},
		{"1.5m", 90 * time.Second}, {"2h", 2 * time.Hour}, {"1d", 24 * time.Hour},
		{"1e-2", 10 * time.Millisecond}, {"0x0.1d", 113281250 * time.Nanosecond},
		{"0x1p-2s", 250 * time.Millisecond}, {"0x1d", 29 * time.Second},
		{"1e1000", time.Duration(math.MaxInt64)}, {"9223372036854775808d", time.Duration(math.MaxInt64)},
		{".0000000001", time.Nanosecond}, {"1e-100", time.Nanosecond},
		{"1e-18172487393827593258", time.Nanosecond},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got, err := timeoutDuration(tc.input)
			if err != nil || got != tc.want {
				t.Fatalf("duration(%q) = %v, %v; want %v", tc.input, got, err, tc.want)
			}
		})
	}
	for _, input := range []string{"", "-1", "-1e-9999", "-0x1p-9999", "xyz", "1q", "10€", "NaN", "1_0", "1sfoo"} {
		if _, err := timeoutDuration(input); err == nil {
			t.Errorf("duration(%q) should fail", input)
		}
	}
}

func TestTimeoutSignals(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  syscall.Signal
		name  string
	}{
		{"TERM", syscall.SIGTERM, "TERM"}, {"sigTeRm", syscall.SIGTERM, "TERM"},
		{"15", syscall.SIGTERM, "TERM"}, {"0", 0, "0"}, {"cOnT", syscall.SIGCONT, "CONT"},
		{"RTMIN", 34, "RTMIN"}, {"SIGRTMAX", 64, "RTMAX"}, {"RTMIN+1", 35, "RTMIN+1"},
		{"RTMAX-1", 63, "RTMIN+29"},
	} {
		sig, ok := timeoutSignal(tc.input)
		if !ok || sig != tc.want || timeoutSignalName(sig) != tc.name {
			t.Errorf("signal(%q) = %v (%s), %v", tc.input, sig, timeoutSignalName(sig), ok)
		}
	}
	for _, input := range []string{"MONKEY", "SIGBREAK", "-1", "65", "RTMIN+31", "RTMAX-31"} {
		if _, ok := timeoutSignal(input); ok {
			t.Errorf("signal(%q) should fail", input)
		}
	}
}

func timeoutTestRun(t *testing.T, dir string, env []string, args ...string) (int, string, string) {
	t.Helper()
	var out, err bytes.Buffer
	code := runTimeout(&Context{Dir: dir, Env: withTimeoutPath(env), Args: args, Stdin: strings.NewReader(""), Stdout: &out, Stderr: &err})
	return code, out.String(), err.String()
}

// withTimeoutPath supplies the directories timeout searches when PATH is
// unset, so a child shell can find the same commands.
func withTimeoutPath(env []string) []string {
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			return env
		}
	}
	out := make([]string, len(env), len(env)+1)
	copy(out, env)
	return append(out, "PATH=/bin:/usr/bin:/system/bin")
}

func TestTimeoutCommand(t *testing.T) {
	for _, tc := range []struct {
		name     string
		args     []string
		code     int
		out, err string
	}{
		{"exit zero", []string{"1", "true"}, 0, "", ""},
		{"exit one", []string{"1", "false"}, 1, "", ""},
		{"arguments", []string{"1", "sh", "-c", "printf '%s' \"$1\"", "sh", "abcd"}, 0, "abcd", ""},
		{"child status", []string{"1", "sh", "-c", "exit 42"}, 42, "", ""},
		{"timeout", []string{".05", "sleep", "5"}, 124, "", ""},
		{"preserve", []string{"--preserve-status", ".05", "sleep", "5"}, 143, "", ""},
		{"kill", []string{"-s9", ".05", "sleep", "5"}, 137, "", ""},
		{"kill after", []string{"-s0", "-k.05", ".05", "sleep", "5"}, 137, "", ""},
		{"foreground", []string{"--foreground", ".05", "sleep", "5"}, 124, "", ""},
		{"foreground kill after", []string{"--foreground", "-s0", "-k.05", ".05", "sleep", "5"}, 137, "", ""},
		{"zero disables timer", []string{"-v", "0", "sleep", ".05"}, 0, "", ""},
		{"zero kill after disables escalation", []string{"-k0", ".05", "sleep", "5"}, 124, "", ""},
		{"continue preserve", []string{"-s", "SIGcont", "-p", ".02", "sleep", ".05"}, 0, "", ""},
		{"continue timeout", []string{"-sCONT", ".02", "sleep", ".05"}, 124, "", ""},
		{"verbose", []string{"-v", ".05", "sleep", "5"}, 124, "", "sending signal TERM to command 'sleep'"},
		{"verbose realtime", []string{"-v", "-sRTMIN", ".05", "sleep", "5"}, 124, "", "sending signal RTMIN to command 'sleep'"},
		{"nonexistent", []string{"1", "/this/does/not/exist"}, 127, "", ""},
		{"directory", []string{"1", "/"}, 126, "", ""},
		{"invalid arg", []string{"--definitely-invalid"}, 125, "", ""},
		{"missing args", nil, 125, "", ""},
		{"invalid time", []string{"", ""}, 125, "", "timeout: invalid time interval ''"},
		{"invalid kill after", []string{"-k", "xyz", "1", "true"}, 125, "", "invalid time interval 'xyz'"},
		{"invalid signal", []string{"-s", "invalid", "1", "true"}, 125, "", "'invalid': invalid signal"},
		{"negative", []string{"--", "-1", "true"}, 125, "", "invalid time interval '-1'"},
		{"overflow", []string{"9223372036854775808d", "true"}, 0, "", ""},
		{"help", []string{"--help"}, 0, "Usage: timeout", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, err := timeoutTestRun(t, t.TempDir(), []string{}, tc.args...)
			if code != tc.code || !strings.Contains(out, tc.out) || !strings.Contains(err, tc.err) {
				t.Fatalf("got code=%d stdout=%q stderr=%q; want code=%d stdout containing %q stderr containing %q", code, out, err, tc.code, tc.out, tc.err)
			}
		})
	}
}

func TestTimeoutScripts(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name, script string
		mode         os.FileMode
		args         []string
		code         int
		out          string
	}{
		{"no shebang", "printf '%s' \"$1\"", 0700, []string{"1", "./script", "argument"}, 0, "argument"},
		{"not executable", "echo nope", 0600, []string{"1", "./script"}, 126, ""},
		{"trap", shBang() + "trap 'exit 3' TERM\nwhile :; do :; done", 0700, []string{".1", "./script"}, 124, ""},
		{"trap preserve", shBang() + "trap 'exit 3' TERM\nwhile :; do :; done", 0700, []string{"-p", ".1", "./script"}, 3, ""},
		{"ignore then kill", shBang() + "trap '' TERM\nwhile :; do :; done", 0700, []string{"-v", "-k.05", ".1", "./script"}, 137, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := filepath.Join(dir, "script")
			if err := os.WriteFile(file, []byte(tc.script), tc.mode); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(file, tc.mode); err != nil {
				t.Fatal(err)
			}
			code, out, err := timeoutTestRun(t, dir, []string{}, tc.args...)
			if code != tc.code || out != tc.out {
				t.Fatalf("got code=%d stdout=%q stderr=%q", code, out, err)
			}
		})
	}
}

func TestTimeoutContext(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "child")
	if err := os.WriteFile(script, []byte(shBang()+"echo \"$VALUE\"\npwd\n"+hostCat()), 0700); err != nil {
		t.Fatal(err)
	}
	var out, err bytes.Buffer
	ctx := &Context{Dir: dir, Env: []string{"PATH=.", "VALUE=context"}, Args: []string{"1", "child"}, Stdin: strings.NewReader("input"), Stdout: &out, Stderr: &err}
	if code := runTimeout(ctx); code != 0 || out.String() != "context\n"+dir+"\ninput" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), err.String())
	}
}

func TestTimeoutMergedOutput(t *testing.T) {
	var output bytes.Buffer
	ctx := &Context{Dir: t.TempDir(), Env: withTimeoutPath(nil), Args: []string{"-v", ".1", "sh", "-c", "while :; do echo output; echo error >&2; sleep .01; done"}, Stdin: strings.NewReader(""), Stdout: &output, Stderr: &output}
	if code := runTimeout(ctx); code != 124 || !strings.Contains(output.String(), "sending signal TERM") {
		t.Fatalf("code=%d output=%q", code, output.String())
	}
}

func TestTimeoutProcessGroup(t *testing.T) {
	dir := t.TempDir()
	code, _, err := timeoutTestRun(t, dir, nil, ".15", "sh", "-c", "sleep 5 & echo $! > child.pid; wait")
	if code != 124 {
		t.Fatalf("code=%d stderr=%q", code, err)
	}
	data, e := os.ReadFile(filepath.Join(dir, "child.pid"))
	if e != nil {
		t.Fatal(e)
	}
	pid, e := strconv.Atoi(strings.TrimSpace(string(data)))
	if e != nil {
		t.Fatal(e)
	}
	// A killed grandchild can remain a zombie until the host's init reaps it.
	data, e = os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	// Z is a zombie; X is dead and not yet reaped. Either means the child did not survive.
	if e == nil && !strings.Contains(string(data), ") Z ") && !strings.Contains(string(data), ") X ") {
		t.Fatalf("grandchild survived timeout: %s", data)
	}
}

func TestTimeoutWaitDelay(t *testing.T) {
	start := time.Now()
	code, _, err := timeoutTestRun(t, t.TempDir(), nil, "2", "sh", "-c", "sleep .5 & exit 0")
	if code != 0 || time.Since(start) > 400*time.Millisecond {
		t.Fatalf("code=%d elapsed=%v stderr=%q", code, time.Since(start), err)
	}
}

func shBang() string {
	return "#!" + hostBin("sh") + "\n"
}

func hostCat() string { return hostBin("cat") }

func hostBin(name string) string {
	for _, dir := range []string{"/bin", "/usr/bin", "/system/bin"} {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return "/bin/" + name
}
