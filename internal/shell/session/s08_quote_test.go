package session

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// s08CheckArgv runs one source string against an isolated injected command.
func s08CheckArgv(t *testing.T, src string, params, want []string) {
	t.Helper()
	dir := t.TempDir()
	var got []string
	var stdout, stderr bytes.Buffer
	calls := 0
	s, err := New(Options{Dir: dir, Env: []string{"PATH=", "HOME=" + dir}, Args: params, Stdout: &stdout, Stderr: &stderr,
		Exec: func(_ context.Context, i Invocation) int {
			calls++
			got = append([]string{i.Name}, i.Args...)
			return 0
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := s.Run(context.Background(), src)
	if r.Err != nil || r.ParseError != nil || r.Status != 0 || calls != 1 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("source %q: result %+v, calls %d, argv %q, stdout %q, stderr %q", src, r, calls, got, stdout.String(), stderr.String())
	}
	if !slices.Equal(got, want) {
		t.Errorf("source %q: argv got %q, want %q", src, got, want)
	}
}

func TestS08QuoteRemoval(t *testing.T) {
	tests := []struct {
		name, src string
		want      []string
	}{
		{"single literal", "probe 'a $x `cmd` \\ b'", []string{"probe", "a $x `cmd` \\ b"}},
		{"double expansion", `x=world; probe "hello $x"`, []string{"probe", "hello world"}},
		{"unquoted escapes", `probe a\ b \$x \` + "`" + ` \" \\`, []string{"probe", "a b", "$x", "`", `"`, `\`}},
		{"unquoted continuation", "probe ab\\\ncd", []string{"probe", "abcd"}},
		{"double special escapes", "probe \"\\$x \\` \\\" \\\\\"", []string{"probe", "$x ` \" \\"}},
		{"double nonspecial escape", `probe "a\qb"`, []string{"probe", `a\qb`}},
		{"double continuation", "probe \"ab\\\ncd\"", []string{"probe", "abcd"}},
		{"empty and concatenate", `x=mid; probe '' "" a''b 'left'"$x"right "$unset"`, []string{"probe", "", "", "ab", "leftmidright", ""}},
		{"unquoted empty removed", `x=; probe $x a${x}b`, []string{"probe", "ab"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { s08CheckArgv(t, tt.src, nil, tt.want) })
	}
}

func TestS08DollarSingleQuotePOSIXParser(t *testing.T) {
	dir := t.TempDir()
	var got []string
	s, err := New(Options{Dir: dir, Env: []string{"PATH=", "HOME=" + dir}, Exec: func(_ context.Context, i Invocation) int {
		got = append([]string{i.Name}, i.Args...)
		return 0
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := s.Run(context.Background(), `probe $'a\nb'`)
	if r.ParseError != nil {
		t.Skipf("POSIX parser does not support dollar-single-quote: %v", r.ParseError)
	}
	// Older POSIX grammars parse this as a literal dollar followed by a
	// single-quoted word, rather than rejecting the syntax.
	if r.Err == nil && r.Status == 0 && slices.Equal(got, []string{"probe", `$a\nb`}) {
		t.Skip("POSIX parser treats dollar-single-quote as literal dollar plus single quotes")
	}
	if r.Err != nil || r.Status != 0 || !slices.Equal(got, []string{"probe", "a\nb"}) {
		t.Fatalf("result %+v: argv got %q, want %q", r, got, []string{"probe", "a\nb"})
	}
}

func TestS08EscapedGlobAndActiveGlob(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"*a.txt", "*b.txt", "other.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	var stdout, stderr bytes.Buffer
	calls := 0
	s, err := New(Options{Dir: dir, Env: []string{"PATH=", "HOME=" + dir}, Stdout: &stdout, Stderr: &stderr, Exec: func(_ context.Context, i Invocation) int {
		calls++
		got = append([]string{i.Name}, i.Args...)
		return 0
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := s.Run(context.Background(), `probe \**.txt`)
	want := []string{"probe", "*a.txt", "*b.txt"}
	if r.Err != nil || r.ParseError != nil || r.Status != 0 || calls != 1 || stdout.Len() != 0 || stderr.Len() != 0 || !slices.Equal(got, want) {
		t.Fatalf("result %+v, calls %d, argv got %q want %q, stdout %q, stderr %q", r, calls, got, want, stdout.String(), stderr.String())
	}
}
