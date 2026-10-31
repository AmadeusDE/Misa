package vipat

import (
	"errors"
	"strings"
	"testing"
)

func find(t *testing.T, pat, line string, ic bool) (string, bool) {
	t.Helper()
	re, err := Compile(pat, ic)
	if err != nil {
		t.Fatalf("Compile(%q): %v", pat, err)
	}
	s, e, ok, err := re.Find(line, 0)
	if err != nil {
		t.Fatalf("Find(%q, %q): %v", pat, line, err)
	}
	if !ok {
		return "", false
	}
	return line[s:e], true
}

func TestMatchTable(t *testing.T) {
	for _, c := range []struct {
		name, pat, line string
		ic              bool
		want            string
		found           bool
	}{
		{"literal", "foo", "a foo b", false, "foo", true},
		{"dot any rune", "f.o", "xfåo", false, "få" + "o", true},
		{"dot is not literal", "a.c", "abc", false, "abc", true},
		{"star greedy", "a*", "aaab", false, "aaa", true},
		{"star leading literal", "*a", "x*a", false, "*a", true},
		{"star after caret literal", "^*a", "*a", false, "*a", true},
		{"plus", `a\+`, "baaa", false, "aaa", true},
		{"plus needs one", `x\+`, "abc", false, "", false},
		{"question", `colou\=r`, "color", false, "color", true},
		{"question synonym", `colou\?r`, "colour", false, "colour", true},
		{"brace exact", `a\{2}`, "aaaa", false, "aa", true},
		{"brace range greedy", `a\{1,3}`, "aaaaa", false, "aaa", true},
		{"brace open", `a\{2,}`, "aaaaa", false, "aaaaa", true},
		{"brace upper only", `a\{,2}`, "aaaaa", false, "aa", true},
		{"brace any", `a\{}`, "aaab", false, "aaa", true},
		{"brace lazy any", `a\{-}`, "aaab", false, "", true},
		{"brace lazy min", `a\{-2,}`, "aaaa", false, "aa", true},
		{"brace lazy range", `a\{-1,3}b`, "aaab", false, "aaab", true},
		{"brace closing backslash", `a\{2\}`, "aaa", false, "aa", true},
		{"lazy then anchor", `<.\{-}>`, "<a><b>", false, "<a>", true},
		{"greedy dotstar", `<.*>`, "<a><b>", false, "<a><b>", true},
		{"leftmost first alternation", `a\|ab`, "ab", false, "a", true},
		{"alternation second", `x\|b`, "abc", false, "b", true},
		{"leftmost beats longer later", `ab\|b`, "abb", false, "ab", true},
		{"group repeat", `\(ab\)\+`, "xababab", false, "ababab", true},
		{"noncapturing group", `\%(ab\)*c`, "ababc", false, "ababc", true},
		{"group alternation", `gr\(a\|e\)y`, "grey", false, "grey", true},
		{"nested group backtrack", `\(a*\)\(ab\)`, "aaab", false, "aaab", true},
		{"caret start", "^a", "ab", false, "a", true},
		{"caret not at start", "^a", "ba", false, "", false},
		{"caret in middle literal", "a^b", "a^b", false, "a^b", true},
		{"caret after alt", `x\|^a`, "ab", false, "a", true},
		{"caret after group open", `\(^a\)`, "ab", false, "a", true},
		{"dollar end", "a$", "baa", false, "a", true},
		{"dollar middle literal", "a$b", "a$b", false, "a$b", true},
		{"dollar before alt", `a$\|z`, "xa", false, "a", true},
		{"dollar before close", `\(a$\)`, "ba", false, "a", true},
		{"only caret", "^", "abc", false, "", true},
		{"only dollar", "$", "abc", false, "", true},
		{"escaped caret", `\^a`, "^a", false, "^a", true},
		{"word start", `\<foo`, "xfoo foo", false, "foo", true},
		{"word end", `foo\>`, "foob foo", false, "foo", true},
		{"word both", `\<foo\>`, "foo.foo", false, "foo", true},
		{"word boundary punctuation", `\<b`, "a.b", false, "b", true},
		{"word boundary unicode", `\<été\>`, "xété été", false, "été", true},
		{"word not inside unicode word", `\<b`, "ab é.b", false, "b", true},
		{"class digit", `\d\+`, "ab123c", false, "123", true},
		{"class non digit", `\D\+`, "12ab3", false, "ab", true},
		{"class space", `\s\+`, "a \t b", false, " \t ", true},
		{"class nonspace", `\S\+`, "  ab  ", false, "ab", true},
		{"class word", `\w\+`, "-ab_1-", false, "ab_1", true},
		{"class nonword", `\W`, "ab-", false, "-", true},
		{"class alpha", `\a\+`, "12ab_", false, "ab", true},
		{"class lower upper", `\u\l\+`, "xx Word", false, "Word", true},
		{"class hex", `\x\+`, "zzBEEFg", false, "BEEF", true},
		{"class octal", `\o\+`, "89017", false, "017", true},
		{"class head", `\h\w*`, "9ab1", false, "ab1", true},
		{"collection", "[abc]\\+", "xxcabx", false, "cab", true},
		{"collection range", "[a-c]\\+", "xxcabx", false, "cab", true},
		{"collection negated", "[^a-c]\\+", "abxyc", false, "xy", true},
		{"collection bracket first", "[]a]\\+", "x]a]", false, "]a]", true},
		{"collection dash last", "[a-]\\+", "x-a", false, "-a", true},
		{"collection posix class", "[[:digit:]]\\+", "ab12", false, "12", true},
		{"collection mixed", "[[:alpha:]_]\\+", "1ab_c2", false, "ab_c", true},
		{"collection escape", `[\t ]\+`, "a \t", false, " \t", true},
		{"collection escaped bracket", `[\]]`, "a]", false, "]", true},
		{"collection missing close literal", "a[b", "xa[b", false, "a[b", true},
		{"collection unicode range", "[α-ω]\\+", "xαβγ", false, "αβγ", true},
		{"collection negated unicode", "[^a]", "aé", false, "é", true},
		{"tab escape", `a\tb`, "a\tb", false, "a\tb", true},
		{"esc escape", `\e`, "a\x1b", false, "\x1b", true},
		{"cr escape", `\r`, "a\r", false, "\r", true},
		{"literal brace in magic", "a{2}", "a{2}", false, "a{2}", true},
		{"literal plus in magic", "a+", "a+", false, "a+", true},
		{"literal paren in magic", "(a)", "(a)", false, "(a)", true},
		{"escaped dot", `a\.c`, "abc a.c", false, "a.c", true},
		{"escaped star", `a\*`, "aa a*", false, "a*", true},
		{"escaped slash", `a\/b`, "a/b", false, "a/b", true},
		{"escaped backslash", `a\\b`, `a\b`, false, `a\b`, true},
		{"escaped tilde", `a\~`, "a~", false, "a~", true},
		{"ignore case", "foo", "FOO", true, "FOO", true},
		{"case sensitive default", "foo", "FOO", false, "", false},
		{"backslash c", `foo\c`, "FOO", false, "FOO", true},
		{"backslash c anywhere", `f\coo`, "FOO", false, "FOO", true},
		{"backslash C overrides ignorecase", `foo\C`, "FOO", true, "", false},
		{"ignore case collection", "[a-c]\\+", "ABC", true, "ABC", true},
		{"ignore case unicode", "åå", "ÅÅ", true, "ÅÅ", true},
		{"empty pattern matches empty", "", "abc", false, "", true},
		{"empty alternative", `a\|`, "b", false, "", true},
		{"zero width repeat terminates", `\(x*\)*`, "ab", false, "", true},
		{"multibyte positions", "é+", "aé+", false, "é+", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, ok := find(t, c.pat, c.line, c.ic)
			if ok != c.found || got != c.want {
				t.Errorf("%q on %q = %q,%v want %q,%v", c.pat, c.line, got, ok, c.want, c.found)
			}
		})
	}
}

func TestMagicModes(t *testing.T) {
	for _, c := range []struct {
		name, pat, line, want string
		found                 bool
	}{
		{"very magic plus", `\va+`, "baaa", "aaa", true},
		{"very magic group alt", `\v(ab|cd)+`, "xabcd", "abcd", true},
		{"very magic brace", `\va{2}`, "aaa", "aa", true},
		{"very magic lazy brace", `\va{-1,}`, "aaa", "a", true},
		{"very magic word", `\v<foo>`, "xfoo foo", "foo", true},
		{"very magic question", `\vcolou?r`, "color", "color", true},
		{"very magic escaped plus literal", `\va\+`, "a+", "a+", true},
		{"very magic noncapture", `\v%(ab)+c`, "ababc", "ababc", true},
		{"very magic literal letters", `\vab`, "ab", "ab", true},
		{"very magic equals", `\vcolou=r`, "color", "color", true},
		{"very magic dot", `\va.c`, "abc", "abc", true},
		{"very magic caret", `\v^a`, "ab", "a", true},
		{"very magic less than escaped literal", `\v\<`, "a<", "<", true},
		{"magic switch back", `\v(a)\mb*`, "abb", "abb", true},
		{"nomagic dot literal", `\Ma.c`, "abc a.c", "a.c", true},
		{"nomagic escaped dot any", `\Ma\.c`, "abc", "abc", true},
		{"nomagic star literal", `\Ma*`, "aa a*", "a*", true},
		{"nomagic escaped star", `\Ma\*`, "aaa", "aaa", true},
		{"nomagic bracket literal", `\Ma[b]`, "ab a[b]", "a[b]", true},
		{"nomagic escaped bracket", `\Ma\[b]`, "ab", "ab", true},
		{"nomagic caret", `\M^a`, "ab", "a", true},
		{"nomagic dollar", `\Ma$`, "ba", "a", true},
		{"nomagic escaped plus", `\Ma\+`, "aaa", "aaa", true},
		{"verynomagic dot literal", `\Va.c`, "abc a.c", "a.c", true},
		{"verynomagic star literal", `\Va*`, "aa a*", "a*", true},
		{"verynomagic caret start", `\V^a`, "ab a", "a", true},
		{"verynomagic caret anchors", `\V^a`, "ba", "", false},
		{"verynomagic dollar end", `\Va$`, "ab a", "a", true},
		{"verynomagic escaped dot any", `\Va\.c`, "abc", "abc", true},
		{"verynomagic group", `\V\(ab\)\+`, "abab", "abab", true},
		{"verynomagic slash", `\Va/b`, "a/b", "a/b", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, ok := find(t, c.pat, c.line, false)
			if ok != c.found || got != c.want {
				t.Errorf("%q on %q = %q,%v want %q,%v", c.pat, c.line, got, ok, c.want, c.found)
			}
		})
	}
}

func TestFindFromAndAnchors(t *testing.T) {
	re, _ := Compile("^a", false)
	if _, _, ok, _ := re.Find("aaa", 1); ok {
		t.Error("^ must not match at a later start offset")
	}
	re, _ = Compile(`\<a`, false)
	if _, _, ok, _ := re.Find("aa", 1); ok {
		t.Error(`\< must see the previous character`)
	}
	re, _ = Compile("é", false)
	s, e, ok, _ := re.Find("aéé", 2)
	if !ok || s != 3 || e != 5 {
		t.Errorf("multibyte from offset: %d %d %v", s, e, ok)
	}
}

func TestSyntaxErrors(t *testing.T) {
	for pat, want := range map[string]string{
		`\(a`:    "E54",
		`a\)`:    "E55",
		`\+a`:    "E64",
		`a**`:    "E61",
		`a*\+`:   "E61",
		`a\{x}`:  "E554",
		`a\{2`:   "E554",
		`[z-a]`:  "E944",
		`\(a\|b`: "E54",
	} {
		_, err := Compile(pat, false)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Compile(%q) error = %v want %s", pat, err, want)
		}
	}
}

func TestUnsupportedConstructs(t *testing.T) {
	for pat, want := range map[string]string{
		`a\zsb`:      `\zs`,
		`a\zeb`:      `\ze`,
		`a\&b`:       `\&`,
		`a\@=`:       `\@=`,
		`\va@!`:      `\@!`,
		`a\@<=b`:     `\@<=`,
		`\_s`:        `\_s`,
		`\_.`:        `\_.`,
		`\%[ab]`:     `\%[`,
		`\%d65`:      `\%d`,
		`\%^`:        `\%^`,
		`\(a\)\1`:    `\1`,
		`a\nb`:       `\n`,
		`a~`:         `~`,
		`\Ma\~`:      `~`,
		`\i`:         `\i`,
		`\k\+`:       `\k`,
		`\f`:         `\f`,
		`\p`:         `\p`,
		`\q`:         `\q`,
		`[[=a=]]`:    `[=a=]`,
		`[[.a.]]`:    `[.a.]`,
		`[\d65]`:     `character code`,
		`[\n]`:       `\n in a collection`,
		`\v(a)&(b)`:  `\&`,
		`\va%[bc]`:   `\%[`,
		`\v%d65`:     `\%d`,
		`\v(a)@<=b`:  `\@<=`,
		`\%V`:        `\%V`,
		`\%23l`:      `\%2`,
		`\_^a`:       `\_^`,
		`a\{-}\zs`:   `\zs`,
		`\va{-}\zsb`: `\zs`,
		`\vx|y\zs`:   `\zs`,
		`\v(x)%#=1y`: `\%#`,
		`\vx\_[a]`:   `\_[`,
	} {
		_, err := Compile(pat, false)
		var u *UnsupportedError
		if !errors.As(err, &u) || !strings.Contains(u.Construct, want) {
			t.Errorf("Compile(%q) error = %v, want unsupported %q", pat, err, want)
		}
	}
}

func TestUnsupportedMessage(t *testing.T) {
	_, err := Compile(`a\zsb`, false)
	if err == nil || err.Error() != `Unsupported pattern construct: \zs` {
		t.Fatalf("message = %v", err)
	}
}

func TestNeverReadsGoSyntax(t *testing.T) {
	for _, pat := range []string{`(?i)a`, `a{2}`, `\p{L}`, `a|b`, `a+`, `a?`} {
		re, err := Compile(pat, false)
		if err != nil {
			continue
		}
		if _, _, ok, _ := re.Find("aab", 0); ok {
			t.Errorf("%q matched as if Go syntax", pat)
		}
	}
}

func TestTooComplexIsBounded(t *testing.T) {
	re, err := Compile(`\(a*\)*b`, false)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = re.Find(strings.Repeat("a", 40), 0)
	if !errors.Is(err, ErrTooComplex) {
		t.Fatalf("err = %v want ErrTooComplex", err)
	}
	re, _ = Compile(`\(\a\+\)\+$`, false)
	if _, _, _, err = re.Find(strings.Repeat("a", 40)+"!", 0); !errors.Is(err, ErrTooComplex) {
		t.Fatalf("err = %v want ErrTooComplex", err)
	}
}

func TestCommonPatternsStayLinear(t *testing.T) {
	line := strings.Repeat("word ", 400)
	for _, pat := range []string{`.*zzz`, `\w\+ zzz`, `\(word \)*zzz`, `\<w\w*d\> $`} {
		re, err := Compile(pat, false)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := re.Find(line, 0); err != nil {
			t.Errorf("%q: %v", pat, err)
		}
	}
}

func BenchmarkFindDotStar(b *testing.B) {
	re, _ := Compile(`\<w.*d\>`, false)
	line := strings.Repeat("word ", 200)
	for i := 0; i < b.N; i++ {
		re.Find(line, 0)
	}
}

func BenchmarkFindPathological(b *testing.B) {
	re, _ := Compile(`\(a*\)*b`, false)
	line := strings.Repeat("a", 40)
	for i := 0; i < b.N; i++ {
		re.Find(line, 0)
	}
}
