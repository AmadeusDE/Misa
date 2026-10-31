package session

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInvocationParameters(t *testing.T) {
	var out bytes.Buffer
	args := []string{"-e", "two words", ""}
	opts := []string{"-u"}
	s, err := New(Options{Env: []string{}, Arg0: strptr("shell-name"), Args: args, ShellOptions: opts, Stdout: &out})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	args[0] = "changed"
	opts[0] = "-e"
	r := s.RunFile(context.Background(), "script.sh", `printf '<%s> <%s> %s\n' "$0" "$1" "$#"; for x in "$@"; do printf '[%s]' "$x"; done`)
	if r.Err != nil || r.Status != 0 {
		t.Fatalf("%+v", r)
	}
	if got := out.String(); got != "<shell-name> <-e> 3\n[-e][two words][]" {
		t.Fatal(got)
	}
	out.Reset()
	r = s.Run(context.Background(), `printf '%s' "$0"`)
	if r.Err != nil || out.String() != "shell-name" {
		t.Fatalf("%+v %q", r, out.String())
	}
	r = s.Run(context.Background(), `printf '%s' "$UNSET"`)
	if r.Status == 0 {
		t.Fatalf("initial -u option was not retained: %+v", r)
	}
}

func TestNoParameters(t *testing.T) {
	var out bytes.Buffer
	s, err := New(Options{Env: []string{}, Arg0: strptr("zero"), Stdout: &out})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := s.Run(context.Background(), `printf '%s:' "$#"; for x in "$@"; do printf bad; done`)
	if r.Err != nil || out.String() != "0:" {
		t.Fatalf("%+v %q", r, out.String())
	}
}

func TestInitialShellOptions(t *testing.T) {
	for _, tc := range []struct {
		option, source, wantOut, wantErr string
		status                           int
	}{
		{"-e", `false; printf bad`, "", "", 1},
		{"-u", `printf '%s' "$UNSET"; printf bad`, "", "UNSET", 1},
		{"-x", `printf hello`, "hello", "printf", 0},
	} {
		t.Run(tc.option, func(t *testing.T) {
			var out, stderr bytes.Buffer
			s, err := New(Options{Env: []string{}, ShellOptions: []string{tc.option}, Stdout: &out, Stderr: &stderr})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			r := s.Run(context.Background(), tc.source)
			if r.Status != tc.status || out.String() != tc.wantOut || !strings.Contains(stderr.String(), tc.wantErr) {
				t.Fatalf("%+v out=%q err=%q", r, out.String(), stderr.String())
			}
		})
	}
	if _, err := New(Options{ShellOptions: []string{"-Z"}}); err == nil {
		t.Fatal("invalid option accepted")
	}
}

func TestRunFileSourceName(t *testing.T) {
	s, err := New(Options{Env: []string{}, Arg0: strptr("zero")})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := s.RunFile(context.Background(), "source.sh", "if")
	if r.ParseError == nil || !strings.Contains(r.Err.Error(), "source.sh") || r.Status != 2 {
		t.Fatalf("%+v", r)
	}
	r = s.Run(context.Background(), "if")
	if r.ParseError == nil || strings.Contains(r.Err.Error(), "source.sh") {
		t.Fatalf("%+v", r)
	}
}

func strptr(s string) *string { return &s }

func TestExplicitEmptyArg0AndSubshell(t *testing.T) {
	var out bytes.Buffer
	zero := ""
	s, err := New(Options{Env: []string{}, Arg0: &zero, Stdout: &out})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	zero = "mutated"
	r := s.RunFile(context.Background(), "named.sh", `printf '<%s>' "$0"; (printf '[%s]' "$0"); f() { printf '{%s}' "$0"; }; f`)
	if r.Err != nil || out.String() != "<>[]{}" {
		t.Fatalf("%+v %q", r, out.String())
	}
}

func TestArg0InSource(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "nested"), []byte(`printf 'source=<%s>' "$0"`), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	s, err := New(Options{Dir: dir, Env: []string{}, Arg0: strptr("outer"), Stdout: &out})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := s.RunFile(context.Background(), "other", `. ./nested`)
	if r.Err != nil || r.Status != 0 || out.String() != "source=<outer>" {
		t.Fatalf("%+v %q", r, out.String())
	}
}

func TestDefaultArg0(t *testing.T) {
	var out bytes.Buffer
	s, err := New(Options{Env: []string{}, Stdout: &out})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.Run(context.Background(), `printf '%s:' "$0"`)
	s.RunFile(context.Background(), "file.sh", `printf '%s' "$0"`)
	if out.String() != "gosh:file.sh" {
		t.Fatal(out.String())
	}
}

func TestLangErrorDiagnostic(t *testing.T) {
	for _, tc := range []struct{ name, source string }{{"", "a=(1 2)"}, {"file.sh", "a=(1 2)"}} {
		s, err := New(Options{Env: []string{}})
		if err != nil {
			t.Fatal(err)
		}
		r := s.RunFile(context.Background(), tc.name, tc.source)
		s.Close()
		if r.Status != 2 || r.ParseError == nil || !strings.Contains(r.ParseError.Message, "arrays") {
			t.Fatalf("%q: %+v", tc.name, r)
		}
		if strings.HasPrefix(r.ParseError.Message, "1:") || strings.Contains(r.ParseError.Message, tc.name+":1:") {
			t.Fatalf("%q: duplicated position in %q", tc.name, r.ParseError.Message)
		}
		if tc.name != "" && !strings.HasPrefix(r.Err.Error(), tc.name+":1:3: arrays") {
			t.Fatalf("%q: diagnostic %q", tc.name, r.Err.Error())
		}
	}
}
