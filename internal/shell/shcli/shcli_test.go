package shcli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func run(t *testing.T, name string, args []string, opts Options) (int, string, string) {
	t.Helper()
	var out, err bytes.Buffer
	opts.Stdout = &out
	opts.Stderr = &err
	status := Run(context.Background(), name, args, opts)
	return status, out.String(), err.String()
}
func TestModesAndPositionals(t *testing.T) {
	dir := t.TempDir()
	script := `printf '%s|%s|%s|%s' "$0" "$#" "$1" "$2"`
	if err := os.WriteFile(filepath.Join(dir, "file"), []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name        string
		args        []string
		input, want string
	}{
		{"command", []string{"-c", script, "custom", "a b", ""}, "", "custom|2|a b|"},
		{"command default", []string{"-c", script}, "", "shell|0||"},
		{"command empty name", []string{"-c", script, "", "x"}, "", "|1|x|"},
		{"command after end options", []string{"-c", "--", script, "n"}, "", "n|0||"},
		{"file", []string{"file", "a b", ""}, "", "file|2|a b|"},
		{"stdin", nil, script, "shell|0||"},
		{"stdin args", []string{"-s", "a b", ""}, script, "shell|2|a b|"},
		{"end options", []string{"--", "file", "x"}, "", "file|1|x|"},
		{"dash operand", []string{"-"}, script, "shell|0||"},
		{"dash as parameter", []string{"-s", "-"}, script, "shell|1|-|"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, out, err := run(t, "shell", tc.args, Options{Dir: dir, Env: []string{}, Stdin: strings.NewReader(tc.input)})
			if code != 0 || out != tc.want || err != "" {
				t.Fatalf("got %d %q %q; want %q", code, out, err, tc.want)
			}
		})
	}
	code, out, _ := run(t, "", []string{"-c", `printf %s "$0"`}, Options{})
	if code != 0 || out != "sh" {
		t.Fatalf("fallback: %d %q", code, out)
	}
}
func TestFlagsAndStatus(t *testing.T) {
	cases := []struct {
		args   []string
		status int
		out    string
	}{
		{[]string{"-ec", "false; printf bad"}, 1, ""},
		{[]string{"-uc", `printf %s "$unset"`}, 1, ""},
		{[]string{"-c", "false; true"}, 0, ""},
		{[]string{"-c", "true; false"}, 1, ""},
		{[]string{"-c", "exit 23; printf bad"}, 23, ""},
		{[]string{"-c", ""}, 0, ""},
		{[]string{"-c", "-not_a_flag"}, 1, ""},
		{[]string{"-c", "--", "-not_a_flag"}, 127, ""},
		{[]string{"-x", "-c", "true"}, 0, ""},
		{[]string{"-z"}, 1, ""}, {[]string{"-c"}, 1, ""}, {[]string{"--bad"}, 1, ""}, {[]string{"--help"}, 1, ""}, {[]string{"-cs", "true"}, 1, ""},
	}
	for _, tc := range cases {
		code, out, err := run(t, "sh", tc.args, Options{})
		if code != tc.status || out != tc.out {
			t.Errorf("%q: %d %q %q", tc.args, code, out, err)
		}
		if (tc.args[0] == "-z" || tc.args[0] == "--bad" || tc.args[0] == "--help" || len(tc.args) == 1 && tc.args[0] == "-c") && err == "" {
			t.Errorf("%q: missing diagnostic", tc.args)
		}
		if tc.args[0] == "--help" && !strings.Contains(err, "--help") {
			t.Errorf("%q: diagnostic %q", tc.args, err)
		}
		if tc.args[0] == "-x" && err == "" {
			t.Error("-x missing trace")
		}
	}
}
func TestSyntaxLocations(t *testing.T) {
	dir := t.TempDir()
	source := "printf before\nif\n"
	if err := os.WriteFile(filepath.Join(dir, "bad"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args         []string
		input, label string
	}{{[]string{"-c", source}, "", "sh"}, {[]string{"bad"}, "", "bad"}, {nil, source, "sh"}} {
		code, out, err := run(t, "sh", tc.args, Options{Dir: dir, Stdin: strings.NewReader(tc.input)})
		if code != 2 || out != "" || !strings.Contains(err, tc.label+":2:") {
			t.Errorf("%q: %d %q %q", tc.args, code, out, err)
		}
	}
}
func TestScriptErrorsAndTTY(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		args   []string
		tty    bool
		status int
	}{{[]string{"missing"}, false, 127}, {[]string{"."}, false, 126}, {nil, true, 1}, {[]string{"-s"}, true, 1}, {[]string{"-c", "true"}, true, 0}} {
		code, _, err := run(t, "sh", tc.args, Options{Dir: dir, StdinTTY: tc.tty})
		if code != tc.status || (code != 0 && err == "") {
			t.Errorf("%v: %d %q", tc.args, code, err)
		}
	}
}

type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, errors.New("input failed") }
func TestInputError(t *testing.T) {
	code, _, err := run(t, "sh", nil, Options{Stdin: failReader{}})
	if code != 1 || !strings.Contains(err, "input failed") {
		t.Fatalf("%d %q", code, err)
	}
}
func TestIsolationAndAdapter(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHCLI_INHERITED", "outside")
	cwd, _ := os.Getwd()
	code, out, err := run(t, "sh", []string{"-c", `printf '%s:%s:' "$SHCLI_INHERITED" "$PWD"; SHCLI_INHERITED=changed; cd /; basename /abc; nonexistent_shcli_command`}, Options{Dir: dir, Env: []string{"PATH="}})
	if code != 127 || !strings.HasPrefix(out, ":"+dir+":abc\n") {
		t.Fatalf("%d %q %q", code, out, err)
	}
	now, _ := os.Getwd()
	if cwd != now || os.Getenv("SHCLI_INHERITED") != "outside" {
		t.Fatal("process state changed")
	}
	code, out, _ = run(t, "sh", []string{"-c", `printf %s "$SHCLI_INHERITED"`}, Options{})
	if code != 0 || out != "outside" {
		t.Fatalf("inherit %d %q", code, out)
	}
}

type ownedReader struct {
	io.Reader
	closed bool
}

func (r *ownedReader) Close() error { r.closed = true; return nil }

type ownedWriter struct {
	bytes.Buffer
	closed bool
}

func (w *ownedWriter) Close() error { w.closed = true; return nil }
func TestStreamOwnership(t *testing.T) {
	r := &ownedReader{Reader: strings.NewReader("true")}
	w := &ownedWriter{}
	code := Run(context.Background(), "sh", nil, Options{Stdin: r, Stdout: w, Stderr: w})
	if code != 0 || r.closed || w.closed {
		t.Fatalf("%d closed %v %v", code, r.closed, w.closed)
	}
}

func TestUnreadableScript(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private")
	if err := os.WriteFile(path, []byte("true"), 0000); err != nil {
		t.Fatal(err)
	}
	if f, err := os.Open(path); err == nil {
		f.Close()
		t.Skip("current user can read mode 0000")
	}
	code, _, diag := run(t, "sh", []string{path}, Options{})
	if code != 126 || diag == "" {
		t.Fatalf("%d %q", code, diag)
	}
}
func TestScriptWithTTY(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "script"), []byte("true"), 0600); err != nil {
		t.Fatal(err)
	}
	code, _, diag := run(t, "sh", []string{"script"}, Options{Dir: dir, StdinTTY: true})
	if code != 0 {
		t.Fatalf("%d %q", code, diag)
	}
}
func TestCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var err bytes.Buffer
	code := Run(ctx, "sh", []string{"-c", "true"}, Options{Stderr: &err})
	if code != 130 || err.Len() != 0 {
		t.Fatalf("%d %q", code, err.String())
	}
}

type blockingReader struct{ release chan struct{} }

func (r blockingReader) Read([]byte) (int, error) {
	<-r.release
	return 0, io.EOF
}

func runBlocked(t *testing.T, ctx context.Context) int {
	t.Helper()
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	done := make(chan int, 1)
	go func() { done <- Run(ctx, "sh", nil, Options{Stdin: blockingReader{release: release}, Env: []string{}}) }()
	select {
	case code := <-done:
		return code
	case <-time.After(2 * time.Second):
		t.Fatal("stdin read ignored cancellation")
		return -1
	}
}

func TestStdinReadCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if code := runBlocked(t, ctx); code != 130 {
		t.Fatalf("status %d", code)
	}
}

func TestStdinReadDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if code := runBlocked(t, ctx); code != 124 {
		t.Fatalf("status %d", code)
	}
}
