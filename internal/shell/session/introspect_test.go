package session

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"
)

// harnessOptions configures a session with fake resolution hooks so these tests
// never depend on the host's registry or PATH.
type harnessOptions struct {
	misa        []string          // names reported as Misa registry utilities
	paths       map[string]string // NAME -> absolute path for the fake PATH
	defaultPath string            // PATH value whose lookups resolve "external"
	status      int               // status returned by the fake handler
}

type harness struct {
	s              *Session
	outBuf, errBuf *bytes.Buffer
	out, err       string
	state          int
	// calls records handler invocations as Name followed by its arguments.
	calls [][]string
	argv0 []string
}

// newHarness builds a fresh session. Each test case gets its own session so
// functions and hash entries cannot leak between cases.
func newHarness(t *testing.T, opt harnessOptions) *harness {
	t.Helper()
	h := &harness{outBuf: &bytes.Buffer{}, errBuf: &bytes.Buffer{}}
	lookPath := func(name, _ string, env []string) (string, bool) {
		if opt.defaultPath != "" && pathValue(env) == opt.defaultPath {
			if name == "external" {
				return "/default/bin/external", true
			}
			return "", false
		}
		path, ok := opt.paths[name]
		return path, ok
	}
	s, err := New(Options{
		Dir: t.TempDir(), Env: []string{"PATH=/fake/bin"}, Stdout: h.outBuf, Stderr: h.errBuf,
		MisaBuiltin: func(name string) bool { return slices.Contains(opt.misa, name) },
		LookPath:    lookPath,
		DefaultPath: opt.defaultPath,
		Exec: func(_ context.Context, inv Invocation) int {
			h.calls = append(h.calls, append([]string{inv.Name}, inv.Args...))
			h.argv0 = append(h.argv0, inv.Argv0)
			return opt.status
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	h.s = s
	return h
}

func (h *harness) run(t *testing.T, source string) {
	t.Helper()
	h.outBuf.Reset()
	h.errBuf.Reset()
	h.calls, h.argv0 = nil, nil
	r := h.s.Run(context.Background(), source)
	if r.Err != nil {
		t.Fatalf("%q: %v", source, r.Err)
	}
	h.out, h.err, h.state = h.outBuf.String(), h.errBuf.String(), r.Status
}

func TestTypeReporting(t *testing.T) {
	cases := []struct {
		name, source, out, err string
		status                 int
	}{
		{"shell builtin", "type echo", "echo is a shell builtin\n", "", 0},
		{"keyword", "type if", "if is a shell keyword\n", "", 0},
		{"function", "f() { :; }; type f", "f is a function\n", "", 0},
		{"misa builtin", "type rev", "rev is a misa builtin\n", "", 0},
		{"path", "type tool", "tool is /fake/bin/tool\n", "", 0},
		{"not found", "type nope", "", "type: nope: not found\n", 1},
		{"function beats builtin", "echo() { :; }; type echo", "echo is a function\n", "", 0},
		{"function beats misa", "rev() { :; }; type rev", "rev is a function\n", "", 0},
		{"several resolutions", "type tool rev", "tool is /fake/bin/tool\nrev is a misa builtin\n", "", 0},
		{"partial miss", "type echo nope", "echo is a shell builtin\n", "type: nope: not found\n", 1},
		{"no operands", "type", "", "", 0},
		{"end of options", "type -- echo", "echo is a shell builtin\n", "", 0},
		{"dash operand", "type -", "", "type: -: not found\n", 1},
		{"token builtin", "type -t echo", "builtin\n", "", 0},
		{"token keyword", "type -t if", "keyword\n", "", 0},
		{"token function", "f() { :; }; type -t f", "function\n", "", 0},
		{"token misa", "type -t rev", "misa\n", "", 0},
		{"token path", "type -t tool", "file\n", "", 0},
		{"path only skips builtins", "type -p echo", "", "", 1},
		{"path only finds files", "type -P tool", "/fake/bin/tool\n", "", 0},
		{"unsupported -a", "type -a echo", "", "type: -a: unsupported option\n", 2},
		{"unsupported -f", "type -f echo", "", "type: -f: unsupported option\n", 2},
		{"invalid option", "type -z echo", "", "type: invalid option \"-z\"\n", 2},
		{"help is not supported", "type --help", "", "type: invalid option \"--help\"\n", 2},
		{"function named type", "type() { command echo function-ran; }; type echo", "function-ran\n", "", 0},
		{"exec reaches the same builtin", "exec type echo", "echo is a shell builtin\n", "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, harnessOptions{misa: []string{"rev"}, paths: map[string]string{"tool": "/fake/bin/tool"}})
			h.run(t, tc.source)
			if h.out != tc.out || h.err != tc.err || h.state != tc.status {
				t.Fatalf("%q: out %q, err %q, status %d; want %q %q %d", tc.source, h.out, h.err, h.state, tc.out, tc.err, tc.status)
			}
		})
	}
}

func TestCommandIntrospection(t *testing.T) {
	cases := []struct {
		name, source, out, err string
		status                 int
	}{
		{"builtin", "command -v echo", "echo\n", "", 0},
		{"function", "f() { :; }; command -v f", "f\n", "", 0},
		{"misa builtin", "command -v rev", "rev\n", "", 0},
		{"path", "command -v tool", "/fake/bin/tool\n", "", 0},
		{"not found", "command -v nope", "", "", 1},
		{"verbose builtin", "command -V echo", "echo is a shell builtin\n", "", 0},
		{"verbose function", "f() { :; }; command -V f", "f is a function\n", "", 0},
		{"verbose misa", "command -V rev", "rev is a misa builtin\n", "", 0},
		{"verbose path", "command -V tool", "tool is /fake/bin/tool\n", "", 0},
		{"verbose missing", "command -V nope", "", "", 1},
		{"several", "command -v echo nope tool", "echo\n/fake/bin/tool\n", "", 1},
		{"no operands", "command", "", "", 0},
		{"invalid option", "command -q echo", "", "command: invalid option \"-q\"\n", 2},
		{"help is not supported", "command --help", "", "command: invalid option \"--help\"\n", 2},
		{"keyword reported", "command -v if", "if\n", "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, harnessOptions{misa: []string{"rev"}, paths: map[string]string{"tool": "/fake/bin/tool"}})
			h.run(t, tc.source)
			if h.out != tc.out || h.err != tc.err || h.state != tc.status {
				t.Fatalf("%q: out %q, err %q, status %d; want %q %q %d", tc.source, h.out, h.err, h.state, tc.out, tc.err, tc.status)
			}
			if len(h.calls) != 0 {
				t.Fatalf("%q should not execute anything: %v", tc.source, h.calls)
			}
		})
	}
}

func TestCommandBypass(t *testing.T) {
	cases := []struct {
		name, source string
		calls        [][]string
		out          string
	}{
		{"function bypassed", "f() { echo function-ran; }; command f a b", [][]string{{"f", "a", "b"}}, ""},
		{"plain function runs", "f() { echo function-ran; }; f", nil, "function-ran\n"},
		{"builtin stays builtin", "echo() { :; }; command echo hi", nil, "hi\n"},
		{"misa builtin dispatched", "command rev x", [][]string{{"rev", "x"}}, ""},
		{"path dispatched", "command tool", [][]string{{"tool"}}, ""},
		{"introspection stays ours", "command type echo", nil, "echo is a shell builtin\n"},
		{"hash stays ours", "command hash -r", nil, ""},
		{"missing external", "command absent", [][]string{{"absent"}}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, harnessOptions{misa: []string{"rev"}, paths: map[string]string{"tool": "/fake/bin/tool"}})
			h.run(t, tc.source)
			if tc.out != h.out {
				t.Fatalf("%q: out %q, want %q", tc.source, h.out, tc.out)
			}
			if !slices.EqualFunc(tc.calls, h.calls, slices.Equal) {
				t.Fatalf("%q: calls %v, want %v", tc.source, h.calls, tc.calls)
			}
		})
	}
}

func TestCommandDefaultPath(t *testing.T) {
	const fixed = "/default/bin"
	cases := []struct {
		name, source, out, err string
		status                 int
		calls                  [][]string
		argv0                  string
	}{
		{"forces external", "external() { echo function-ran; }; command -p external", "", "", 0, [][]string{{"/default/bin/external"}}, "external"},
		{"skips builtins", "command -p echo hi", "", "command: echo: not found\n", 127, nil, ""},
		{"skips misa builtins", "command -p rev", "", "command: rev: not found\n", 127, nil, ""},
		{"ignores the session PATH", "command -p tool", "", "command: tool: not found\n", 127, nil, ""},
		{"introspection ignores -p", "command -p -v tool", "/fake/bin/tool\n", "", 0, nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, harnessOptions{misa: []string{"rev"}, defaultPath: fixed, paths: map[string]string{"tool": "/fake/bin/tool"}})
			h.run(t, tc.source)
			if h.out != tc.out || h.err != tc.err || h.state != tc.status {
				t.Fatalf("%q: out %q, err %q, status %d; want %q %q %d", tc.source, h.out, h.err, h.state, tc.out, tc.err, tc.status)
			}
			if !slices.EqualFunc(tc.calls, h.calls, slices.Equal) {
				t.Fatalf("%q: calls %v, want %v", tc.source, h.calls, tc.calls)
			}
			if tc.argv0 != "" && h.argv0[0] != tc.argv0 {
				t.Fatalf("argv0 %q, want %q", h.argv0[0], tc.argv0)
			}
		})
	}
}

func TestHashTable(t *testing.T) {
	cases := []struct {
		name, source, out, err string
		status                 int
	}{
		{"empty table", "hash", "", "", 0},
		{"record", "hash tool; hash", "tool=/fake/bin/tool\n", "", 0},
		{"record several sorted", "hash tool other; hash", "other=/fake/bin/other\ntool=/fake/bin/tool\n", "", 0},
		{"missing", "hash nope", "", "hash: nope: not found\n", 1},
		{"partial missing", "hash tool nope", "", "hash: nope: not found\n", 1},
		{"partial missing keeps entry", "hash tool nope; hash", "tool=/fake/bin/tool\n", "hash: nope: not found\n", 0},
		{"builtin is not hashed", "hash echo; hash", "", "", 0},
		{"misa builtin is not hashed", "hash rev; hash", "", "", 0},
		{"function is not hashed", "f() { :; }; hash f; hash", "", "", 0},
		{"invalid option", "hash -x", "", "hash: invalid option \"-x\"\n", 2},
		{"help is not supported", "hash --help", "", "hash: invalid option \"--help\"\n", 2},
		{"path operand", "hash /bin/tool", "", "hash: /bin/tool: not a command name\n", 1},
		{"reset", "hash tool; hash -r; hash", "", "", 0},
		{"reset with operand", "hash tool; hash -r tool; hash", "tool=/fake/bin/tool\n", "", 0},
		{"auto record external", "tool arg; hash", "tool=/fake/bin/tool\n", "", 0},
		{"explicit path not recorded", "./tool; hash", "", "", 0},
		{"misa name dispatched not recorded", "rev; hash", "", "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, harnessOptions{misa: []string{"rev"}, paths: map[string]string{"tool": "/fake/bin/tool", "other": "/fake/bin/other"}})
			h.run(t, tc.source)
			if h.out != tc.out || h.err != tc.err || h.state != tc.status {
				t.Fatalf("%q: out %q, err %q, status %d; want %q %q %d", tc.source, h.out, h.err, h.state, tc.out, tc.err, tc.status)
			}
		})
	}
}

func TestHashClearedOnPathAssignment(t *testing.T) {
	h := newHarness(t, harnessOptions{paths: map[string]string{"tool": "/fake/bin/tool"}})
	h.run(t, "hash tool; hash")
	if h.out != "tool=/fake/bin/tool\n" {
		t.Fatalf("recorded table %q", h.out)
	}
	h.run(t, "PATH=/other")
	h.run(t, "hash")
	if h.out != "" {
		t.Fatalf("PATH assignment did not clear the table: %q", h.out)
	}
	h.run(t, "export PATH=/fake/bin; tool; hash")
	if !strings.Contains(h.out, "tool=/fake/bin/tool") {
		t.Fatalf("re-recorded table %q", h.out)
	}
}
