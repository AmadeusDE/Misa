package posixre

import (
	"reflect"
	"strings"
	"testing"
)

var bre = Options{Syntax: Basic}
var ere = Options{Syntax: Extended}
var gbre = Options{Syntax: Basic, GNU: true}
var gere = Options{Syntax: Extended, GNU: true}

func TestFind(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		opt     Options
		input   string
		want    []int
	}{
		{"literal", "abc", bre, "xxabcx", []int{2, 5}},
		{"no match", "abd", bre, "abc", nil},
		{"dot", "a.c", bre, "abc", []int{0, 3}},
		{"dot matches newline", "a.c", bre, "a\nc", []int{0, 3}},
		{"leftmost longest alternation", "a|ab", ere, "abc", []int{0, 2}},
		{"leftmost before longest", "b|abc", ere, "xabcd", []int{1, 4}},
		{"longest across branches", "x*|b", ere, "bxx", []int{0, 1}},
		{"star greedy", "a*", bre, "aaab", []int{0, 3}},
		{"empty match at start", "x*", bre, "abc", []int{0, 0}},
		{"bre literal plus", "a+", bre, "aa+", []int{1, 3}},
		{"ere plus", "a+", ere, "aa+", []int{0, 2}},
		{"bre gnu plus", `a\+`, gbre, "aa+", []int{0, 2}},
		{"bre gnu question", `ab\?c`, gbre, "ac", []int{0, 2}},
		{"bre gnu alternation", `cat\|dog`, gbre, "hotdog", []int{3, 6}},
		{"bre interval", `a\{2,3\}`, bre, "aaaa", []int{0, 3}},
		{"bre interval exact", `a\{2\}`, bre, "a aa", []int{2, 4}},
		{"bre interval open", `a\{2,\}`, bre, "aaaaa", []int{0, 5}},
		{"ere interval", "a{2,3}", ere, "aaaa", []int{0, 3}},
		{"ere gnu interval no min", "a{,2}", gere, "aaa", []int{0, 2}},
		{"ere brace literal", "a{x", ere, "a{x", []int{0, 3}},
		{"bre leading star literal", "*a", bre, "b*a", []int{1, 3}},
		{"bre star after group open literal", `\(*a\)`, bre, "*a", []int{0, 2, 0, 2}},
		{"bre star after caret literal", "^*a", bre, "*a", []int{0, 2}},
		{"ere leading star literal", "*a", ere, "b*a", []int{1, 3}},
		{"bre caret mid literal", "a^b", bre, "a^b", []int{0, 3}},
		{"bre dollar mid literal", "a$b", bre, "a$b", []int{0, 3}},
		{"bre dollar before group close", `\(a$\)`, bre, "ba", []int{1, 2, 1, 2}},
		{"bre caret after group open", `\(^a\)`, bre, "ab", []int{0, 1, 0, 1}},
		{"ere caret anchor", "^a", ere, "ba", nil},
		{"ere dollar anchor", "a$", ere, "ab a", []int{3, 4}},
		{"bre group", `\(ab\)*c`, bre, "ababc", []int{0, 5, 2, 4}},
		{"ere group", "(ab)*c", ere, "ababc", []int{0, 5, 2, 4}},
		{"unset group", "(a)|b", ere, "b", []int{0, 1, -1, -1}},
		{"backref bre", `\(a*\)b\1`, bre, "aabaa", []int{0, 5, 0, 2}},
		{"backref must equal", `\(ab\)\1`, bre, "abac abab", []int{5, 9, 5, 7}},
		{"backref ere gnu", `(a|b)\1`, gere, "abba", []int{1, 3, 1, 2}},
		{"backref longest", `\(a*\)\1`, bre, "aaaa", []int{0, 4, 0, 2}},
		{"bracket", "[abc]+", ere, "xxcab!", []int{2, 5}},
		{"bracket negated", "[^abc]+", ere, "abxyc", []int{2, 4}},
		{"bracket close first", "[]a]+", ere, "x]a]", []int{1, 4}},
		{"bracket negated close first", "[^]a]", ere, "]ab", []int{2, 3}},
		{"bracket dash end", "[a-]+", ere, "x-a-", []int{1, 4}},
		{"bracket range", "[a-c]+", ere, "dcbae", []int{1, 4}},
		{"bracket class", "[[:digit:]]+", ere, "ab123c", []int{2, 5}},
		{"bracket class mixed", "[[:upper:]_]+", ere, "aB_Cd", []int{1, 4}},
		{"bracket backslash literal", `[\]+`, ere, `a\\b`, []int{1, 3}},
		{"bracket equivalence", "[[=a=]]", ere, "ba", []int{1, 2}},
		{"bracket collating", "[[.-.]]", ere, "a-", []int{1, 2}},
		{"bracket newline negated", "[^a]", ere, "a\n", []int{1, 2}},
		{"escaped special bre", `a\.c`, bre, "abc a.c", []int{4, 7}},
		{"escaped special ere", `a\+`, ere, "aa+", []int{1, 3}},
		{"bre parens literal", "(a)", bre, "(a)", []int{0, 3}},
		{"ere unmatched close literal", "a)", ere, "a)", []int{0, 2}},
		{"empty pattern", "", bre, "abc", []int{0, 0}},
		{"ere empty group", "a()b", ere, "ab", []int{0, 2, 1, 1}},
		{"ere empty branch", "a|", ere, "b", []int{0, 0}},
		{"word start", `\<b`, gbre, "ab b", []int{3, 4}},
		{"word end", `a\>`, gbre, "ab a", []int{3, 4}},
		{"word boundary", `\bx\b`, gbre, "axa x", []int{4, 5}},
		{"not word boundary", `\Bx\B`, gbre, "x axa", []int{3, 4}},
		{"word class", `\w+`, gere, "!ab_1!", []int{1, 5}},
		{"non word class", `\W+`, gere, "ab !? c", []int{2, 6}},
		{"space class", `\s+`, gere, "a \t b", []int{1, 4}},
		{"non space class", `\S+`, gere, "  ab ", []int{2, 4}},
		{"buffer anchors", "\\`a\\'", gbre, "a", []int{0, 1}},
		{"gnu newline escape", `a\nb`, gbre, "a\nb", []int{0, 3}},
		{"ignore case literal", "abc", Options{IgnoreCase: true}, "xABC", []int{1, 4}},
		{"ignore case class", "[a-c]+", Options{Syntax: Extended, IgnoreCase: true}, "xAbC", []int{1, 4}},
		{"ignore case negated class", "[^k]", Options{Syntax: Extended, IgnoreCase: true}, "Kx", []int{1, 2}},
		{"ignore case backref", `\(a\)\1`, Options{IgnoreCase: true}, "aA", []int{0, 2, 0, 1}},
		{"utf8 dot", "a.c", bre, "aéc", []int{0, 4}},
		{"utf8 class", "[é]", ere, "aé", []int{1, 3}},
		{"invalid byte dot", "a.c", bre, "a\xffc", []int{0, 3}},
		{"invalid byte literal", "\xff", bre, "a\xff", []int{1, 2}},
		{"nested star", "(a*)*", ere, "aa", []int{0, 2, 0, 2}},
		{"nested star nullable", "(a*)+b", ere, "b", []int{0, 1, 0, 0}},
		{"double star", "a**", ere, "aa", []int{0, 2}},
		{"repeat group interval", "(ab){2}", ere, "ababab", []int{0, 4, 2, 4}},
		{"zero repeat", "ab{0}c", ere, "ac", []int{0, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re, err := Compile(tt.pattern, tt.opt)
			if err != nil {
				t.Fatalf("Compile(%q): %v", tt.pattern, err)
			}
			got, err := re.Find([]byte(tt.input), 0)
			if err != nil {
				t.Fatalf("Find: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Find(%q, %q) = %v, want %v", tt.pattern, tt.input, got, tt.want)
			}
		})
	}
}

func TestFindAtUsesContext(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		opt     Options
		input   string
		pos     int
		want    []int
	}{
		{"caret not at offset", "^a", bre, "aa", 1, nil},
		{"caret at zero", "^a", bre, "aa", 0, []int{0, 1}},
		{"word start sees previous byte", `\<b`, gbre, "ab b", 1, []int{3, 4}},
		{"boundary sees previous byte", `\bb`, gbre, "ab", 1, nil},
		{"plain offset", "b", bre, "abab", 2, []int{3, 4}},
		{"offset at end", "x*", bre, "ab", 2, []int{2, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re := MustCompile(tt.pattern, tt.opt)
			got, err := re.Find([]byte(tt.input), tt.pos)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Find(%q, %q, %d) = %v, want %v", tt.pattern, tt.input, tt.pos, got, tt.want)
			}
		})
	}
}

func TestLineAndWord(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		opt     Options
		input   string
		want    []int
	}{
		{"line matches whole", "ab", Options{Line: true}, "ab", []int{0, 2}},
		{"line rejects partial", "ab", Options{Line: true}, "abc", nil},
		{"line alternation groups", "a|b", Options{Syntax: Extended, Line: true}, "ab", nil},
		{"word skips embedded", "foo", Options{Word: true}, "foobar foo", []int{7, 10}},
		{"word at edges", "foo", Options{Word: true}, "foo", []int{0, 3}},
		{"word retries shorter", "a+", Options{Syntax: Extended, Word: true}, "aab a", []int{4, 5}},
		{"literal", "a.b", Options{Literal: true}, "axb a.b", []int{4, 7}},
		{"literal ignore case", "A.B", Options{Literal: true, IgnoreCase: true}, "a.b", []int{0, 3}},
		{"literal word", "a.b", Options{Literal: true, Word: true}, "xa.b a.b", []int{5, 8}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re := MustCompile(tt.pattern, tt.opt)
			got, err := re.Find([]byte(tt.input), 0)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Find(%q, %q) = %v, want %v", tt.pattern, tt.input, got, tt.want)
			}
		})
	}
}

func TestCompileAll(t *testing.T) {
	re, err := CompileAll([]string{`\(a\)\1`, `\(b\)\1`}, bre)
	if err != nil {
		t.Fatal(err)
	}
	got, err := re.Find([]byte("xbbaa"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{1, 3}; !reflect.DeepEqual(got[:2], want) {
		t.Errorf("got %v, want prefix %v", got, want)
	}
	re = MustCompileAll([]string{"ab", "abcd", "b"}, bre)
	got, _ = re.Find([]byte("xabcd"), 0)
	if want := []int{1, 5}; !reflect.DeepEqual(got, want) {
		t.Errorf("leftmost-longest across patterns = %v, want %v", got, want)
	}
	re = MustCompileAll([]string{"ab", "cd"}, Options{Line: true})
	if ok, _ := re.Match([]byte("cd")); !ok {
		t.Error("line option should apply to each pattern")
	}
	if ok, _ := re.Match([]byte("abcd")); ok {
		t.Error("line option must anchor the alternation as a whole")
	}
}

func TestFindAll(t *testing.T) {
	tests := []struct {
		pattern, input string
		want           [][]int
	}{
		{"x*", "abc", [][]int{{0, 0}, {1, 1}, {2, 2}, {3, 3}}},
		{"a*", "baaac", [][]int{{0, 0}, {1, 4}, {5, 5}}},
		{"ab", "abab", [][]int{{0, 2}, {2, 4}}},
		{"^a", "aaa", [][]int{{0, 1}}},
		{".", "é!", [][]int{{0, 2}, {2, 3}}},
	}
	for _, tt := range tests {
		re := MustCompile(tt.pattern, bre)
		got, err := re.FindAll([]byte(tt.input), -1)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("FindAll(%q, %q) = %v, want %v", tt.pattern, tt.input, got, tt.want)
		}
	}
	re := MustCompile("a", bre)
	if got, _ := re.FindAll([]byte("aaa"), 2); len(got) != 2 {
		t.Errorf("limit 2 returned %d matches", len(got))
	}
}

func TestCompileErrors(t *testing.T) {
	tests := []struct {
		pattern string
		opt     Options
		want    string
	}{
		{`\(a`, bre, "Unmatched ( or \\("},
		{"(a", ere, "Unmatched ( or \\("},
		{`a\)`, bre, "Unmatched ) or \\)"},
		{"[a", ere, "Unmatched [, [^, [:, [., or [="},
		{"[[:foo:]]", ere, "Invalid character class name"},
		{"[z-a]", ere, "Invalid range end"},
		{`a\`, bre, "Trailing backslash"},
		{`\1`, bre, "Invalid back reference"},
		{`\(a\1\)`, bre, "Invalid back reference"},
		{`(a)\2`, gere, "Invalid back reference"},
		{`a\{1`, bre, "Unmatched \\{"},
		{`a\{2,1\}`, bre, "Invalid content of \\{\\}"},
		{`a\{x\}`, bre, "Invalid content of \\{\\}"},
		{"a{2,1}", ere, "Invalid content of \\{\\}"},
		{`\{1\}`, bre, "Invalid preceding regular expression"},
		{"a{99999}", ere, "Regular expression too big"},
		{"(((a{100}){100}){100})", ere, "Regular expression too big"},
		{"[[.ab.]]", ere, "Invalid collation character"},
	}
	for _, tt := range tests {
		_, err := Compile(tt.pattern, tt.opt)
		if err == nil || err.Error() != tt.want {
			t.Errorf("Compile(%q) error = %v, want %q", tt.pattern, err, tt.want)
		}
	}
}

func TestBackrefBudget(t *testing.T) {
	re := MustCompile(`\(a*\)*b\1`, bre)
	_, err := re.Find([]byte(strings.Repeat("a", 64)), 0)
	if err != ErrTooComplex {
		t.Fatalf("err = %v, want ErrTooComplex", err)
	}
}

func TestPathologicalWithoutBackrefsIsLinear(t *testing.T) {
	re := MustCompile(`\<\(a*\)*b`, gbre)
	m, err := re.Find([]byte(strings.Repeat("a", 5000)), 0)
	if err != nil || m != nil {
		t.Fatalf("got %v, %v; want no match and no error", m, err)
	}
}

func TestNumSubexp(t *testing.T) {
	if n := MustCompile(`\(a\)\(b\(c\)\)`, bre).NumSubexp(); n != 3 {
		t.Errorf("NumSubexp = %d, want 3", n)
	}
}

func BenchmarkFind(b *testing.B) {
	line := []byte(strings.Repeat("the quick brown fox jumps over the lazy dog ", 20) + "needle")
	for _, bc := range []struct {
		name    string
		pattern string
		opt     Options
	}{
		{"fast", "ne+dle$", ere},
		{"word", `\<needle\>`, gbre},
		{"backref", `\(n\)eedl\(e\)`, bre},
	} {
		re := MustCompile(bc.pattern, bc.opt)
		b.Run(bc.name, func(b *testing.B) {
			for b.Loop() {
				if m, _ := re.Find(line, 0); m == nil {
					b.Fatal("no match")
				}
			}
		})
	}
}
