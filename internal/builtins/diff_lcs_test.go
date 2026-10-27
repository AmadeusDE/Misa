// Utility behaviour tests ported from uutils diffutils (MIT/Apache-2.0).
package builtins

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDiffMyers(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for n := 0; n < 400; n++ {
		a, b := make([]diffLine, rng.Intn(20)), make([]diffLine, rng.Intn(20))
		for i := range a {
			a[i] = diffLine{text: string(rune('a' + rng.Intn(4))), newline: true}
		}
		for i := range b {
			b[i] = diffLine{text: string(rune('a' + rng.Intn(4))), newline: true}
		}
		es := diffMyers(a, b, diffOptions{})
		x, y, cost := 0, 0, 0
		for _, e := range es {
			if e.a != x || e.b != y {
				t.Fatalf("bad indices %+v; %d %d", e, x, y)
			}
			switch e.kind {
			case ' ':
				if a[x] != b[y] {
					t.Fatal("bad match")
				}
				x++
				y++
			case '-':
				x++
				cost++
			case '+':
				y++
				cost++
			default:
				t.Fatal("bad kind")
			}
		}
		if x != len(a) || y != len(b) {
			t.Fatal("incomplete")
		}
		dp := make([][]int, len(a)+1)
		for i := range dp {
			dp[i] = make([]int, len(b)+1)
		}
		for i := 1; i <= len(a); i++ {
			for j := 1; j <= len(b); j++ {
				if a[i-1] == b[j-1] {
					dp[i][j] = dp[i-1][j-1] + 1
				} else {
					dp[i][j] = max(dp[i-1][j], dp[i][j-1])
				}
			}
		}
		if cost != len(a)+len(b)-2*dp[len(a)][len(b)] {
			t.Fatal("nonminimal edit")
		}
	}
}

func TestDiffUtils(t *testing.T) {
	t.Run("expand_tabs/basics", func(t *testing.T) {
		for _, tc := range []struct {
			input, want string
			size        int
		}{
			{"foo barr   baz", "foo barr   baz", 8}, {"foo\tbarr\tbaz", "foo     barr    baz", 8},
			{"foo\tbarr\tbaz", "foo  barr baz", 5}, {"foo\tbarr\tbaz", "foo barr  baz", 2},
		} {
			if got := diffExpand(tc.input, tc.size); got != tc.want {
				t.Fatalf("%q != %q", got, tc.want)
			}
		}
	})
	t.Run("expand_tabs/multibyte_chars", func(t *testing.T) {
		for _, tc := range []struct {
			input, want string
			size        int
		}{
			{"foo\tépée\tbaz", "foo     épée    baz", 8}, {"foo\t😉\tbaz", "foo  😉   baz", 5},
			{"foo\t👩‍🔬\tbaz", "foo   👩‍🔬    baz", 6},
		} {
			if got := diffExpand(tc.input, tc.size); got != tc.want {
				t.Fatalf("%q != %q", got, tc.want)
			}
		}
	})
	t.Run("expand_tabs/invalid_utf8", func(t *testing.T) {
		input := string([]byte{240, 240, 152, 137, 9, 102, 111, 111})
		want := string([]byte{240, 240, 152, 137, 32, 32, 32, 32, 102, 111, 111})
		if got := diffExpand(input, 8); got != want {
			t.Fatalf("%q != %q", got, want)
		}
	})
	t.Run("write_line/basics", func(t *testing.T) {
		for _, tc := range []struct {
			input, want string
			expand      bool
		}{
			{"foo bar baz", "foo bar baz\n", false}, {"foo bar\tbaz", "foo bar\tbaz\n", false}, {"foo bar\tbaz", "foo bar baz\n", true},
		} {
			var out strings.Builder
			diffWriteLine(&out, "", diffLine{tc.input, true}, diffOptions{expandTabs: tc.expand, tabsize: 8})
			if out.String() != tc.want {
				t.Fatal(out.String())
			}
		}
	})
	t.Run("modification_time/set_time", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "file")
		stamp := time.Unix(123456789, 123456789)
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := diffHeader(path, info); got != path+"\t"+stamp.Format("2006-01-02 15:04:05.000000000 -0700") {
			t.Fatal(got)
		}
	})
	t.Run("modification_time/invalid_file", func(t *testing.T) {
		before := time.Now()
		got := diffHeader("missing", nil)
		_, stamp, _ := strings.Cut(got, "\t")
		parsed, err := time.Parse("2006-01-02 15:04:05.000000000 -0700", stamp)
		if err != nil || parsed.Before(before) || parsed.After(time.Now()) {
			t.Fatalf("%s %v", got, err)
		}
	})
}

func TestDiffMyersDeletionFirst(t *testing.T) {
	a, b := diffLines([]byte("a\nb\n")), diffLines([]byte("b\na\n"))
	edits := diffMyers(a, b, diffOptions{})
	var kinds strings.Builder
	for _, e := range edits {
		kinds.WriteByte(e.kind)
	}
	// uutils: ambiguous LCS ties can differ; Misa prefers deletion first.
	if got := kinds.String(); got != "- +" {
		t.Fatalf("edit kinds %q", got)
	}
}
