package session

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestS08PathnameExpansion(t *testing.T) {
	tests := []struct {
		name, src string
		want      []string
	}{
		{"sorted", `probe *.txt`, []string{"probe", "a.txt", "b.txt"}},
		{"unmatched", `probe missing*.txt`, []string{"probe", "missing*.txt"}},
		{"quoted and escaped", `probe '*.txt' "*.txt" \*.txt`, []string{"probe", "*.txt", "*.txt", "*.txt"}},
		{"hidden excluded", `probe *`, []string{"probe", "a.txt", "b.txt", "c.go", "d1", "d2"}},
		{"hidden explicit", `probe .*.txt`, []string{"probe", ".hidden.txt"}},
		{"question", `probe ?.go`, []string{"probe", "c.go"}},
		{"bracket", `probe d[12]`, []string{"probe", "d1", "d2"}},
		{"variable pattern", `p='*.txt'; probe $p "$p"`, []string{"probe", "a.txt", "b.txt", "*.txt"}},
		{"command substitution pattern", `probe $(printf '%s' '*.txt')`, []string{"probe", "a.txt", "b.txt"}},
		{"noglob", `set -f; p='*.txt'; probe *.txt $p;`, []string{"probe", "*.txt", "*.txt"}},
		{"noglob reset", `set -f; set +f; probe *.txt`, []string{"probe", "a.txt", "b.txt"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, name := range []string{"b.txt", "a.txt", ".hidden.txt", "c.go", "d1", "d2"} {
				if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			var got []string
			calls := 0
			s, err := New(Options{Dir: dir, Env: []string{"PATH=", "HOME=" + dir}, Exec: func(_ context.Context, i Invocation) int {
				calls++
				got = append([]string{i.Name}, i.Args...)
				return 0
			}})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			r := s.Run(context.Background(), tt.src)
			if r.Err != nil || r.ParseError != nil || r.Status != 0 || calls != 1 || !slices.Equal(got, tt.want) {
				t.Errorf("%q: result %+v, calls %d, argv got %q; want %q", tt.src, r, calls, got, tt.want)
			}
		})
	}
}

// Expansion failures must stop before dispatch, even if the command itself is valid.
func TestS08ExpansionErrorDoesNotDispatch(t *testing.T) {
	tests := []struct{ name, src, stderr string }{
		{"parameter", `probe "${missing:?required}"`, "missing: required\n"},
		{"arithmetic", `probe "$((1 / 0))"`, "division by zero\n"},
		{"nounset", `set -u; probe "$missing"`, "missing: unbound variable\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			calls := 0
			var stderr bytes.Buffer
			s, err := New(Options{Dir: dir, Env: []string{"PATH=", "HOME=" + dir}, Stderr: &stderr, Exec: func(context.Context, Invocation) int { calls++; return 0 }})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			result := s.Run(context.Background(), tt.src)
			if result.Status != 1 || result.Err != nil || result.ParseError != nil || calls != 0 || stderr.String() != tt.stderr {
				t.Errorf("result %+v, calls %d, stderr %q; want status 1, no dispatch, stderr %q", result, calls, stderr.String(), tt.stderr)
			}
		})
	}
}
