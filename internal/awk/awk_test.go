package awk

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/posixre"
)

type result struct {
	out, err string
	status   int
}

func run(t *testing.T, cfg Config, stdin string) result {
	t.Helper()
	var out, errb bytes.Buffer
	if cfg.Dir == "" {
		cfg.Dir = t.TempDir()
	}
	cfg.Stdin = strings.NewReader(stdin)
	cfg.Stdout, cfg.Stderr = &out, &errb
	status := Run(cfg)
	return result{out.String(), errb.String(), status}
}

func awkRun(t *testing.T, prog, stdin string, operands ...string) result {
	t.Helper()
	return run(t, Config{Program: prog, Operands: operands}, stdin)
}

func check(t *testing.T, name, prog, stdin, want string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		r := awkRun(t, prog, stdin)
		if r.status != 0 || r.out != want {
			t.Fatalf("awk %q <%q>\nstatus %d stderr %q\ngot  %q\nwant %q", prog, stdin, r.status, r.err, r.out, want)
		}
	})
}

func TestBasics(t *testing.T) {
	file1 := "abc def ghi 5\nghi jkl mno 10\nmno pqr stu 15\nstu vwx abc 20\n"
	cases := []struct{ name, prog, in, want string }{
		{"pattern", `/abc/`, file1, "abc def ghi 5\nstu vwx abc 20\n"},
		{"fields", `{print $2,$3}`, file1, "def ghi\njkl mno\npqr stu\nvwx abc\n"},
		{"field out of range", `{print $2,$8}`, file1, "def \njkl \npqr \nvwx \n"},
		{"begin end", "BEGIN { print \"H\" } { print $1 } END { print \"F\" }", "a\nb\n", "H\na\nb\nF\n"},
		{"numeric compare", `$4>0`, file1, file1},
		{"eq", `$4==15`, file1, "mno pqr stu 15\n"},
		{"match op", `$1~/abc/`, file1, "abc def ghi 5\n"},
		{"count", "BEGIN{c=0} $1~/abc/{c++} END{print \"Total Count =\", c}", file1, "Total Count = 1\n"},
		{"sum", "BEGIN{sum=0} {sum=sum+$4} END{print \"Sum is =\", sum}", file1, "Sum is = 50\n"},
		{"if", `{ if($2 == "jkl") print $1; }`, file1, "ghi\n"},
		{"for", `BEGIN { for(i=1;i<=3;i++) print "square of", i, "is", i*i; }`, "", "square of 1 is 1\nsquare of 2 is 4\nsquare of 3 is 9\n"},
		{"NF NR", `{print NF, NR}`, "a b\nc\n", "2 1\n1 2\n"},
		{"END NR", `END{print NR}`, file1, "4\n"},
		{"high field", `{print $99999}`, "a\n", "\n"},
		{"dollar expr", `{ print $(2*2) }`, "Pat 100 97 58\n", "58\n"},
		{"substr", `{if (substr($0,1,3) == "abc") { print $3 }}`, file1, "ghi\n"},
		{"printf noparen", `/ghi/ { printf $1 $2 $3 $4 }`, file1, "abcdefghi5ghijklmno10"},
		{"not", `! /foo/`, "foo\nbar\n", "bar\n"},
		{"and", `/2400/ && /foo/`, "foo bar 2500\nabc def 2400\n", ""},
		{"or", `/2400/ || /foo/`, "foo bar 2500\nabc def 2400\n", "foo bar 2500\nabc def 2400\n"},
		{"not match", `{ if (! ($0 ~ /foo/)) print }`, "foo\nbar\n", "bar\n"},
		{"sub", `{ sub(/a+/,"<A>"); print }`, "Pat 100\nSandy\nChris\n", "P<A>t 100\nS<A>ndy\nChris\n"},
		{"average", `BEGIN{l=0;t=0} {l++; t+=$1} END{print "Read:",l; print "Total:",t; if (l>0) {print "Average:", t/l} else {print "0"}}`, "10\n20\n30\n40\n50\n", "Read: 5\nTotal: 150\nAverage: 30\n"},
		{"avg float", `{ sum = $2 + $3 + $4 ; avg = sum / 3; print $1, avg }`, "Chris 72 92 89\n", "Chris 84.3333\n"},
		{"sqrt", `{ print "Square root of", $1, "is", sqrt($1) }`, "25", "Square root of 25 is 5\n"},
		{"printf pad", `{ printf "%5s %3d\n", $3, $4; }`, "abc def ghi 5\n", "  ghi   5\n"},
		{"printf paren dynfmt", `BEGIN { f2 ="%8s %6dn"; } { printf(f2, $1, $4); }`, "abc def ghi 5\n", "     abc      5n"},
		{"while", `BEGIN { i=1; while (i <= 5) { printf i "-" i*i " "; i=i+1; } }`, "", "1-1 2-4 3-9 4-16 5-25 "},
		{"do while", `{ i = 1; do { print $0; i++ } while (i <= 3) }`, "x\n", "x\nx\nx\n"},
		{"continue", `BEGIN { for (x = 0; x <= 8; x++) { if (x == 5) { continue; } printf "%d ",x } print "" }`, "", "0 1 2 3 4 6 7 8 \n"},
		{"break", `{ num = $1; for (div = 2; div*div <= num; div++) { if (num % div == 0) { break } } if (num % div == 0) { printf "divisor of %d is %d\n", num, div } else { printf "%d is prime\n", num } }`, "10\n15\n17\n", "divisor of 10 is 2\ndivisor of 15 is 3\n17 is prime\n"},
		{"NR assign", `NR == 2 { NR = 17 } { print NR }`, "a\nb\nc\nd\n", "1\n17\n18\n19\n"},
		{"length no arg", `{print length()}`, "a\n12\n\n6502", "1\n2\n0\n4\n"},
		{"length bare", `{print length}`, "abc\n", "3\n"},
		{"length bare div", `{print length / 2}`, "abcd\n", "2\n"},
		{"substr basic", `BEGIN{print substr("abac", 2, 2)}`, "", "ba\n"},
		{"atan2", `BEGIN{print substr(atan2(0, -1), 1, 5)}`, "", "3.141\n"},
		{"toupper", `BEGIN{print toupper("abABcD"), tolower("abABcD")}`, "", "ABABCD ababcd\n"},
		{"match", `BEGIN{print match("bcdab", "ab"), RSTART, RLENGTH}`, "", "4 4 2\n"},
		{"match none", `BEGIN{print match("x", "deadbeef"), RSTART, RLENGTH}`, "", "0 0 -1\n"},
		{"index", `BEGIN{print index("deadbeef","deadbeef"), index("x","deadbeef"), index("abc","")}`, "", "1 0 0\n"},
		{"srand default", `BEGIN{ print srand() }`, "", "1\n"},
		{"srand returns previous", `BEGIN{ srand(5); print srand(7) }`, "", "5\n"},
		{"printf %c", `BEGIN{a=255; printf "%c%c%c", a, b, 255}`, "", "\u00ff\x00\u00ff"},
		{"printf %c empty", `BEGIN{a=""; printf "(%c)", a}`, "", "(\x00)"},
		{"printf %c utf8 string", `BEGIN{a="ú"; printf "(%c)", a}`, "", "(ú)"},
	}
	for _, c := range cases {
		check(t, c.name, c.prog, c.in, c.want)
	}
}

func TestFieldAssignment(t *testing.T) {
	cases := []struct{ name, prog, in, want string }{
		{"assign field rebuilds", `BEGIN { FS=":"; OFS=":" } { $2=""; print }`, "abc:def:ghi:5\n", "abc::ghi:5\n"},
		{"assign field OFS", `BEGIN{OFS="-"} {$1=$1; print}`, "a b c\n", "a-b-c\n"},
		{"extend NF", `{$5="x"; print; print NF}`, "a b\n", "a b   x\n5\n"},
		{"NF=0", `BEGIN { $0 = "a b"; print NF, "x" $0 "y"; NF = 0; print NF, "x" $0 "y" }`, "", "2 xa by\n0 xy\n"},
		{"NF shrink", `{NF=2; print}`, "a b c d\n", "a b\n"},
		{"NF grow", `BEGIN{OFS=","} {NF=4; print}`, "a b\n", "a,b,,\n"},
		{"dollar zero assign", `{$0="x y z"; print NF, $2}`, "a\n", "3 y\n"},
		{"decrement NF in expr", `{print $NF; print $(NF-1)}`, "a b c\n", "c\nb\n"},
		{"FS change applies to next record", `{ print $2; FS=":"; print $2 }`, "a:b c\nd:e f\n", "c\nc\ne f\ne f\n"},
		{"OFS after field assign", `{ $1="x"; OFS="-"; print }`, "a b\n", "x b\n"},
		{"incr field", `{ $2++; print }`, "a 1\n", "a 2\n"},
		{"post incr field index", `{ i=1; $(i++) = "z"; print i, $0 }`, "a b\n", "2 z b\n"},
	}
	for _, c := range cases {
		check(t, c.name, c.prog, c.in, c.want)
	}
}

func TestFieldSeparators(t *testing.T) {
	data := "abc:def:ghi:5\n"
	cases := []struct{ name, prog, in, want string }{
		{"default blanks", `{print NF; print $1 "|" $3}`, "  a \t b\n c  \n", "2\na|\n1\nc|\n"},
		{"single char", `BEGIN{FS=":"} {print $2}`, data, "def\n"},
		{"single char empty fields", `BEGIN{FS=","} {print NF}`, "a,,b,\n", "4\n"},
		{"regex FS", `BEGIN{FS="[0-9]+"} {print $1 "|" $2 "|" $3}`, "a12b345c\n", "a|b|c\n"},
		{"regex FS leading", `BEGIN{FS="[ ]"} {print NF}`, " a b\n", "3\n"},
		{"empty FS chars", `BEGIN{FS=""} {print NF, $2}`, "aβc\n", "3 β\n"},
		{"unmatched FS", `BEGIN{FS="69793793"} {print $2}`, data, "\n"},
		{"tab FS", "BEGIN{FS=\"\\t\"} {print $2}", "a b\tc d\n", "c d\n"},
		{"pipe FS literal", `BEGIN{FS="|"} {print $2}`, "a|b|c\n", "b\n"},
		{"escaped pipe regex FS", `BEGIN{FS="\\|"} {print $2}`, "a|b|c\n", "b\n"},
		{"space regex is not blanks", `BEGIN{FS="  "} {print $2}`, "a  b\n", "b\n"},
		{"empty record", `{print NF}`, "\n", "0\n"},
		{"FS var undefined name", `BEGIN {FS=k;lksa;lkf;l} {print $2}`, "abc\nxyz\n", "b\ny\n"},
	}
	for _, c := range cases {
		check(t, c.name, c.prog, c.in, c.want)
	}
}

func TestRecordSeparators(t *testing.T) {
	multi := "abc defxy ghi\njkl mno\n\n\npqr stu\nvwxy abc\n"
	cases := []struct{ name, prog, in, want string }{
		{"paragraph default FS", `BEGIN{RS=""} {print NR ": " NF}`, multi, "1: 5\n2: 4\n"},
		{"paragraph leading and trailing blank lines", `BEGIN{RS=""} {print NR ": " $0}`, "\n\na\nb\n\n\n\nc\n\n", "1: a\nb\n2: c\n"},
		{"paragraph single char FS adds newline", `BEGIN{RS=""; FS="x"} {print NF}`, multi, "3\n3\n"},
		{"paragraph regex FS", `BEGIN{RS=""; FS="[ ]"} {print NF}`, multi, "4\n3\n"},
		{"paragraph chars", `BEGIN{RS=""; FS=""} {print NF}`, multi, "21\n16\n"},
		{"paragraph newline FS", "BEGIN { RS=\"\"; FS=\"\\n\" } { print $2,$3; }", "abc def ghi 5\nghi jkl mno 10\nmno pqr stu 15\nstu vwx abc 20\n", "ghi jkl mno 10 mno pqr stu 15\n"},
		{"single char RS keeps last newline", `BEGIN{RS=" "} {print}`, "a b\n", "a\nb\n\n"},
		{"regex RS", `BEGIN{RS="[0-9]+"} {print NR ":" $0}`, "a1b22c", "1:a\n2:b\n3:c\n"},
		{"null regex RS takes all", `BEGIN{RS="()"} {print NR, NF}`, "a b\nc\n", "1 3\n"},
		{"ORS", "BEGIN { ORS=\"\\r\\n\" } { print }", "a\nb\n", "a\r\nb\r\n"},
		{"no trailing newline", `{print}`, "a\nb", "a\nb\n"},
		{"RS change mid-stream", `NR==1{RS=";"} {print NR ":" $0}`, "a\nb;c;d", "1:a\n2:b\n3:c\n4:d\n"},
	}
	for _, c := range cases {
		check(t, c.name, c.prog, c.in, c.want)
	}
}

func TestCoercion(t *testing.T) {
	cases := []struct{ name, prog, in, want string }{
		{"string constants compare as strings", `BEGIN { print ("10" < "9"), (10 < 9) }`, "", "1 0\n"},
		{"fields compare numerically", `{ print ($1 < $2) }`, "10 9\n", "0\n"},
		{"fields non numeric compare as strings", `{ print ($1 < $2) }`, "10 9x\n", "1\n"},
		{"strnum with blanks", `{ print ($1 == 10) }`, " 10 \n", "1\n"},
		{"strnum vs string constant", `{ print ($1 == "10"), ($1 == "10.0") }`, "10.0\n", "0 1\n"},
		{"assignment keeps strnum", `{ x = $1; print (x < 9) }`, "10\n", "0\n"},
		{"concat result is string", `{ x = $1 ""; print (x < 9) }`, "10\n", "1\n"},
		{"uninit is both", `BEGIN { print (x == 0), (x == ""), length(x) }`, "", "1 1 0\n"},
		{"uninit field compare", `{ print ($5 == 0), ($5 == "") }`, "a\n", "1 1\n"},
		{"string to number prefix", `BEGIN { print "3x" + 4, " 2.5e1z" * 2, "x" + 1, "+5" + 0, ".5." + 0 }`, "", "7 50 1 5 0.5\n"},
		{"hex is not numeric", `BEGIN { print "0x1A" + 0 }`, "", "0\n"},
		{"integer output", `BEGIN { print 1e6, 100000 * 100000, 0.1 + 0.2, 3.0, -0 }`, "", "1000000 10000000000 0.3 3 0\n"},
		{"OFMT used by print", `BEGIN { OFMT="%.2f"; x = 3.14159; print x; print x "" }`, "", "3.14\n3.14159\n"},
		{"CONVFMT used by concat", `BEGIN { CONVFMT="%.2f"; x = 3.14159; y = x ""; print y; print 2 "" }`, "", "3.14\n2\n"},
		{"large integer", `BEGIN { print 2^53, 2^62 }`, "", "9007199254740992 4611686018427387904\n"},
		{"huge goes via OFMT", `BEGIN { print 1e30 }`, "", "1e+30\n"},
		{"truthiness of strnum", `{ if ($1) print "t"; else print "f" }`, "0\n0.0\nabc\n \n", "f\nf\nt\nf\n"},
		{"string constant zero is true", `BEGIN { if ("0") print "t"; else print "f" }`, "", "t\n"},
		{"inf nan", `BEGIN { x = log(-1); y = -log(0); print x, y }`, "", "nan inf\n"},
		{"unary minus string", `BEGIN { print -"3x", +"4y" }`, "", "-3 4\n"},
		{"comparison chain concat", `BEGIN { print 1 " " 2 < 3 }`, "", "1\n"},
		{"unary vs concat", `BEGIN { print 1 " " -1 }`, "", "1-1\n"},
	}
	for _, c := range cases {
		check(t, c.name, c.prog, c.in, c.want)
	}
}

func TestOperators(t *testing.T) {
	cases := []struct{ name, prog, want string }{
		{"precedence mul add", `BEGIN { print 1 + 2 * 3 - 4 }`, "3\n"},
		{"pow right assoc", `BEGIN { print 2 ^ 3 ^ 2 }`, "512\n"},
		{"unary minus pow", `BEGIN { print -2 ^ 2 }`, "-4\n"},
		{"pow negative exponent", `BEGIN { print 2 ^ -1 }`, "0.5\n"},
		{"star star", `BEGIN { print 2 ** 3; x = 2; x **= 3; print x }`, "8\n8\n"},
		{"mod", `BEGIN { print 7 % 3, -7 % 3, 7.5 % 2 }`, "1 -1 1.5\n"},
		{"div", `BEGIN { print 7 / 2, 1/4 }`, "3.5 0.25\n"},
		{"not binds tighter than compare", `BEGIN { print !0 + 1, !1 + 1 }`, "2 1\n"},
		{"ternary", `BEGIN { print 1 ? "y" : "n", 0 ? "y" : "n" }`, "y n\n"},
		{"ternary nested right", `BEGIN { x = 5; print x < 3 ? "low" : x < 10 ? "mid" : "high" }`, "mid\n"},
		{"ternary newline continuation", "BEGIN { print 1 ?\n\"y\" :\n\"n\" }", "y\n"},
		{"assign ops", `BEGIN { x = 10; x += 5; x -= 3; x *= 2; x /= 4; x %= 4; x ^= 2; print x }`, "4\n"},
		{"chained assign", `BEGIN { a = b = 3; print a, b }`, "3 3\n"},
		{"pre post incr", `BEGIN { x = 5; print x++, x, ++x, x--, --x }`, "5 6 7 7 5\n"},
		{"concat precedence over compare", `BEGIN { print ("a" "b" == "ab") }`, "1\n"},
		{"concat of expressions", `BEGIN { print 1+2 "x" 3*4 }`, "3x12\n"},
		{"and or short circuit", `BEGIN { x = 0; 0 && (x = 1); 1 || (x = 2); print x }`, "0\n"},
		{"and or precedence", `BEGIN { print 1 || 0 && 0 }`, "1\n"},
		{"match ops", `BEGIN { print ("abc" ~ "b"), ("abc" !~ /b/), ("abc" ~ /^b/) }`, "1 0 0\n"},
		{"dynamic regex", `BEGIN { r = "^a.c$"; print ("abc" ~ r), ("abcd" ~ r) }`, "1 0\n"},
		{"regex vs divide", `BEGIN { a = 8; b = 2; print a / b / 2; print a /b/ 2 }`, "2\n2\n"},
		{"regex after paren", `BEGIN { if ("a/b" ~ /\//) print "slash" }`, "slash\n"},
		{"regex slash in bracket", `BEGIN { if ("a/b" ~ /[/]/) print "slash" }`, "slash\n"},
		{"div assign vs regex", `BEGIN { x = 8; x /= 2; print x }`, "4\n"},
		{"regex leftmost longest", `BEGIN { match("xabcabcy", /(abc|abcabc)/); print RSTART, RLENGTH }`, "2 6\n"},
		{"regex escapes", `BEGIN { print ("a\tb" ~ /a\tb/), ("a.b" ~ /a\.b/), ("axb" ~ /a\.b/) }`, "1 1 0\n"},
		{"dynamic regex escapes", `BEGIN { print ("a.b" ~ "a\\.b"), ("axb" ~ "a\\.b"), ("a.b" ~ "a\.b") }`, "1 0 1\n"},
		{"empty regex", `BEGIN { print ("abc" ~ //) }`, "1\n"},
		{"newline after &&", "BEGIN { if (1 &&\n 1) print \"ok\" }", "ok\n"},
		{"newline after ||", "BEGIN { if (0 ||\n 1) print \"ok\" }", "ok\n"},
		{"newline after comma", "BEGIN { print 1,\n2 }", "1 2\n"},
		{"newline after brace", "BEGIN {\nprint 1\n}", "1\n"},
		{"newline after else and do", "BEGIN { if (0) print 1\nelse\n print 2\n do\n x++\n while (x < 3)\n print x }", "2\n3\n"},
		{"else after semicolon", `BEGIN { if (0) print 1; else print 2 }`, "2\n"},
		{"backslash continuation", "BEGIN { x = 1 + \\\n 2; print x }", "3\n"},
		{"comments", "# top\nBEGIN { print 1 # trailing\n}\n# end", "1\n"},
		{"semicolons between rules", `BEGIN { print 1 }; END { print 2 }`, "1\n2\n"},
		{"rules on one line", `BEGIN { print 1 } END { print 2 }`, "1\n2\n"},
		{"print parens", `BEGIN { print("a", "b") }`, "a b\n"},
		{"print parens concat", `BEGIN { print (1)(2) }`, "12\n"},
		{"print gt in parens", `BEGIN { print (2 > 1) }`, "1\n"},
		{"print empty", `BEGIN { print "" }`, "\n"},
		{"string escapes", `BEGIN { print "a\tb\\c\"d\101\/" }`, "a\tb\\c\"dA/\n"},
		{"empty action", `BEGIN { }`, ""},
		{"getline-free semicolon statements", `BEGIN { ; ; print 1 ; }`, "1\n"},
		{"numeric literals", `BEGIN { print .5, 5., 1e3, 1E-2, 011 }`, "0.5 5 1000 0.01 11\n"},
	}
	for _, c := range cases {
		check(t, c.name, c.prog, "", c.want)
	}
}

func TestPrintf(t *testing.T) {
	cases := []struct{ f, want string }{
		{`"%d|%i", 42.9, -3.9`, "42|-3"},
		{`"%5d|%-5d|%05d|%+d|% d", 42, 42, 42, 42, 42`, "   42|42   |00042|+42| 42"},
		{`"%.3d|%5.3d", 7, 7`, "007|  007"},
		{`"%o|%#o|%x|%X|%#x|%#X", 8, 8, 255, 255, 255, 255`, "10|010|ff|FF|0xff|0XFF"},
		{`"%u", -1`, "18446744073709551615"},
		{`"%e|%E", 1234.5, 0.000123`, "1.234500e+03|1.230000E-04"},
		{`"%.2e", 0`, "0.00e+00"},
		{`"%f|%.2f|%8.3f|%-8.1f|%08.2f", 3.14159, 3.14159, 3.14159, 3.14159, -3.14159`, "3.141590|3.14|   3.142|3.1     |-0003.14"},
		{`"%F", 1.5`, "1.500000"},
		{`"%g|%g|%g|%g|%g", 100000, 1000000, 0.0001, 0.00001, 123456789`, "100000|1e+06|0.0001|1e-05|1.23457e+08"},
		{`"%G", 0.000001234`, "1.234E-06"},
		{`"%.3g|%#g|%g", 3.14159, 1, 0`, "3.14|1.00000|0"},
		{`"%s|%10s|%-10s|%.2s", "abc", "abc", "abc", "abc"`, "abc|       abc|abc       |ab"},
		{`"%c|%c|%c", 65, "hello", 960`, "A|h|\u03c0"},
		{`"%5c|%-3c|", "x", "y"`, "    x|y  |"},
		{`"%*d|%-*d|%.*f", 5, 42, 4, 42, 2, 3.14159`, "   42|42  |3.14"},
		{`"%%|%5%|", 1`, "%|%|"},
		{`"%d %d", 1`, "1 0"},
		{`"%s", 0.1 + 0.2`, "0.3"},
		{`"%s %s", 1e6, 17`, "1000000 17"},
		{`"%5s|%-5s|", "αβ", "αβ"`, "   αβ|αβ   |"},
		{`"%.1s", "αβ"`, "α"},
		{`"%d", "0x11"`, "0"},
		{`"%d", 2^31`, "2147483648"},
		{`"%d", 1e30`, "1000000000000000019884624838656"},
		{`"%z"`, "%z"},
		{`"abc%"`, "abc%"},
		{`"%ld %5.2lf", 7, 2.5`, "7  2.50"},
		{`"%x", -1`, "ffffffffffffffff"},
		{`"%+.1e", 12345`, "+1.2e+04"},
		{`"%i", "12abc"`, "12"},
	}
	for _, c := range cases {
		check(t, c.f, "BEGIN { printf("+c.f+") }", "", c.want)
	}
	check(t, "sprintf", `BEGIN { s = sprintf("%03d-%s", 7, "x"); print s, length(s) }`, "", "007-x 5\n")
	check(t, "printf no newline default", `BEGIN { printf "a"; printf "b\n" }`, "", "ab\n")
}

func TestStringFunctions(t *testing.T) {
	cases := []struct{ name, prog, want string }{
		{"substr from", `BEGIN { print substr("hello", 2) }`, "ello\n"},
		{"substr zero start", `BEGIN { print substr("hello", 0, 2) "|" substr("hello", -1, 3) }`, "h|h\n"},
		{"substr past end", `BEGIN { print substr("hello", 4, 100) "|" substr("hello", 9) "|" substr("hello", 2, -1) "|" }`, "lo|||\n"},
		{"substr fractional", `BEGIN { print substr("hello", 1.5, 2) "|" substr("hello", 1.4, 1.6) }`, "el|he\n"},
		{"substr utf8", `BEGIN { print substr("aβcδ", 2, 2) }`, "βc\n"},
		{"length utf8", `BEGIN { print length("aβc") }`, "3\n"},
		{"length invalid utf8", `BEGIN { print length("a\377b") }`, "3\n"},
		{"index utf8", `BEGIN { print index("aβcスd", "ス") }`, "4\n"},
		{"match utf8", `BEGIN { print match("aβスc", "ス"), RSTART, RLENGTH }`, "3 3 1\n"},
		{"tolower utf8", `BEGIN { print tolower("ĞЖÞ") toupper("ğжþ") }`, "ğжþĞЖÞ\n"},
		{"upper invalid bytes preserved", `BEGIN { printf "%s", toupper("a\377b") }`, "A\xffB"},
		{"sub amp", `BEGIN { s = "hello"; sub(/l+/, "[&]", s); print s }`, "he[ll]o\n"},
		{"sub escaped amp", `BEGIN { s = "hello"; sub(/l+/, "\\&", s); print s }`, "he&o\n"},
		{"sub backslash", `BEGIN { s = "a.b"; sub(/\./, "\\\\", s); print s }`, "a\\b\n"},
		{"sub backslash amp", `BEGIN { s = "a.b"; sub(/\./, "\\\\&", s); print s }`, "a\\.b\n"},
		{"gsub count", `BEGIN { s = "banana"; n = gsub(/a/, "o", s); print n, s }`, "3 bonono\n"},
		{"gsub empty match", `BEGIN { s = "abc"; n = gsub(/x*/, "-", s); print n, s }`, "4 -a-b-c-\n"},
		{"gsub empty after match", `BEGIN { s = "axc"; n = gsub(/x*/, "-", s); print n, s }`, "3 -a-c-\n"},
		{"gsub anchored", `BEGIN { s = "aaa"; n = gsub(/^a/, "b", s); print n, s }`, "1 baa\n"},
		{"gsub dollar", `BEGIN { s = "abc"; gsub(/$/, "!", s); print s }`, "abc!\n"},
		{"gsub dynamic regex", `BEGIN { s = "a.b.c"; gsub(".", "-", s); print s; t = "a.b.c"; gsub("\\.", "-", t); print t }`, "-----\na-b-c\n"},
		{"gsub utf8 empty", `BEGIN { s = "aβ"; gsub(//, "-", s); print s }`, "-a-β-\n"},
		{"sub no match leaves target", `BEGIN { s = "abc"; print sub(/x/, "y", s), s }`, "0 abc\n"},
		{"sub numeric target", `BEGIN { x = 1234; sub(/2/, "x", x); print x }`, "1x34\n"},
		{"leftmost longest sub", `BEGIN { s = "abcd"; sub(/b|bc|bcd/, "X", s); print s }`, "aX\n"},
		{"int", `BEGIN { print int(3.9), int(-3.9), int("4.7x") }`, "3 -3 4\n"},
		{"math", `BEGIN { print exp(0), log(1), sin(0), cos(0), sqrt(16) }`, "1 0 0 1 4\n"},
		{"rand range", `BEGIN { r = rand(); print (r >= 0 && r < 1) }`, "1\n"},
		{"rand deterministic seed", `BEGIN { srand(3); a = rand(); srand(3); b = rand(); print (a == b) }`, "1\n"},
		{"concat number format", `BEGIN { x = 0.1; print x "" }`, "0.1\n"},
	}
	for _, c := range cases {
		check(t, c.name, c.prog, "", c.want)
	}
}

func TestStringFunctionsWithInput(t *testing.T) {
	check(t, "sub default $0", `{ n = gsub(/o/, "0"); print n, $0, $1 }`, "foo boo\n", "4 f00 b00 f00\n")
	check(t, "sub on field rebuilds", `{ sub(/b/, "X", $2); print }`, "a b c\n", "a X c\n")
	check(t, "sub on $0 resplits", `{ sub(/a/, "x y"); print NF }`, "a b\n", "3\n")
}

func TestByteMode(t *testing.T) {
	r := run(t, Config{Program: `{ print length(), length($0), substr($0, 1, 2) == "\343\201", index($0, "\201"), toupper("aé") }`, Bytes: true}, "あい\n")
	if r.status != 0 || r.out != "6 6 1 2 Aé\n" {
		t.Fatalf("got %q %q %d", r.out, r.err, r.status)
	}
	r = run(t, Config{Program: `BEGIN { FS = "" } { print NF }`, Bytes: true}, "aβ\n")
	if r.out != "3\n" {
		t.Fatalf("byte-mode empty FS: %q", r.out)
	}
}

func TestExit(t *testing.T) {
	cases := []struct {
		name, prog, in, out string
		status              int
	}{
		{"exit in BEGIN runs END", `BEGIN { print "b"; exit 3; print "no" } END { print "end" }`, "x\n", "b\nend\n", 3},
		{"exit in main runs END", `{ print; exit } END { print "end", NR }`, "a\nb\n", "a\nend 1\n", 0},
		{"exit expr", `BEGIN { exit 1 + 1 }`, "", "", 2},
		{"exit in END stops", `END { print 1; exit 4; print 2 }`, "", "1\n", 4},
		{"exit keeps earlier code", `BEGIN { exit 5 } END { exit }`, "", "", 5},
		{"exit code override in END", `BEGIN { exit 5 } END { exit 6 }`, "", "", 6},
		{"BEGIN only reads no input", `BEGIN { print "x" }`, "never\n", "x\n", 0},
		{"END sees last record", `END { print $0, NF }`, "a b\nc d e\n", "c d e 3\n", 0},
		{"next", `NR==1 { next } { print }`, "a\nb\nc\n", "b\nc\n", 0},
		{"next skips later rules", `/a/ { next } { print "second", $0 }`, "a\nb\n", "second b\n", 0},
		{"range", `/b/,/d/`, "a\nb\nc\nd\ne\n", "b\nc\nd\n", 0},
		{"range same line", `/b/,/b/`, "a\nb\nc\n", "b\n", 0},
		{"range reopens", `/a/,/b/ { print NR }`, "a\nb\nx\na\nb\n", "1\n2\n4\n5\n", 0},
		{"range unterminated", `/c/,/zzz/`, "a\nc\nd\n", "c\nd\n", 0},
		{"BEGIN multiple", `BEGIN { x = 1 } BEGIN { print x }`, "", "1\n", 0},
		{"pattern only newline", "NR==1\nNR==2", "a\nb\nc\n", "a\nb\n", 0},
		{"division by zero", `BEGIN { print 1/0 }`, "", "", 2},
		{"mod by zero", `BEGIN { print 1%0 }`, "", "", 2},
		{"bad regex", `BEGIN { print "a" ~ "(" }`, "", "", 2},
		{"negative field", `BEGIN { print $(-1) }`, "", "", 2},
		{"negative NF", `BEGIN { NF = -1 }`, "", "", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := awkRun(t, c.prog, c.in)
			if r.out != c.out || r.status != c.status {
				t.Fatalf("got out %q status %d (stderr %q), want %q %d", r.out, r.status, r.err, c.out, c.status)
			}
			if r.err != "" && !strings.HasPrefix(r.err, "awk: ") {
				t.Fatalf("stderr %q", r.err)
			}
		})
	}
}

func TestVariablesAndFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.txt", "a1\na2\n")
	write("b.txt", "b1\nb2")
	t.Run("two files", func(t *testing.T) {
		r := run(t, Config{Dir: dir, Program: `{ print FILENAME, NR, FNR, $0 }`, Operands: []string{"a.txt", "b.txt"}}, "")
		want := "a.txt 1 1 a1\na.txt 2 2 a2\nb.txt 3 1 b1\nb.txt 4 2 b2\n"
		if r.out != want {
			t.Fatalf("got %q", r.out)
		}
	})
	t.Run("operand assignment between files", func(t *testing.T) {
		r := run(t, Config{Dir: dir, Program: `{ print x $0 }`, Operands: []string{"x=1", "a.txt", "x=2\\t", "b.txt"}}, "")
		if want := "1a1\n1a2\n2\tb1\n2\tb2\n"; r.out != want {
			t.Fatalf("got %q", r.out)
		}
	})
	t.Run("-v is visible in BEGIN", func(t *testing.T) {
		r := run(t, Config{Program: `BEGIN { print v, w + 1 }`, Assign: []string{"v=a\\nb", "w=41"}}, "")
		if want := "a\nb 42\n"; r.out != want {
			t.Fatalf("got %q", r.out)
		}
	})
	t.Run("-v is strnum", func(t *testing.T) {
		r := run(t, Config{Program: `BEGIN { print (v < 9) }`, Assign: []string{"v=10"}}, "")
		if r.out != "0\n" {
			t.Fatalf("got %q", r.out)
		}
	})
	t.Run("-v bad", func(t *testing.T) {
		r := run(t, Config{Program: `BEGIN {}`, Assign: []string{"1x=3"}}, "")
		if r.status != 2 {
			t.Fatalf("status %d", r.status)
		}
	})
	t.Run("-F", func(t *testing.T) {
		fs := "\\t"
		r := run(t, Config{Program: `{ print $2 }`, FS: &fs}, "a\tb c\td\n")
		if r.out != "b c\n" {
			t.Fatalf("got %q", r.out)
		}
	})
	t.Run("-F regex and -v FS", func(t *testing.T) {
		fs := "[,;]"
		r := run(t, Config{Program: `{ print $2 }`, FS: &fs}, "a;b,c\n")
		if r.out != "b\n" {
			t.Fatalf("got %q", r.out)
		}
	})
	t.Run("stdin dash", func(t *testing.T) {
		r := run(t, Config{Dir: dir, Program: `{ print FILENAME ":" $0 }`, Operands: []string{"-", "a.txt"}}, "in\n")
		if want := "-:in\na.txt:a1\na.txt:a2\n"; r.out != want {
			t.Fatalf("got %q", r.out)
		}
	})
	t.Run("missing file", func(t *testing.T) {
		r := run(t, Config{Dir: dir, Program: `{ print } END { print "end" }`, Operands: []string{"nope.txt", "a.txt"}}, "")
		if r.status != 2 || r.out != "a1\na2\nend\n" || !strings.Contains(r.err, "can't open file nope.txt") {
			t.Fatalf("got %+v", r)
		}
	})
	t.Run("ARGC", func(t *testing.T) {
		r := run(t, Config{Program: `BEGIN { print ARGC }`, Operands: []string{"a", "b"}}, "")
		if r.out != "3\n" {
			t.Fatalf("got %q", r.out)
		}
	})
	t.Run("nextfile", func(t *testing.T) {
		r := run(t, Config{Dir: dir, Program: `{ print FILENAME, $0; nextfile }`, Operands: []string{"a.txt", "b.txt"}}, "")
		if want := "a.txt a1\nb.txt b1\n"; r.out != want {
			t.Fatalf("got %q", r.out)
		}
	})
	t.Run("SUBSEP default", func(t *testing.T) {
		r := run(t, Config{Program: `BEGIN { printf "%d", length(SUBSEP) }`}, "")
		if r.out != "1" {
			t.Fatalf("got %q", r.out)
		}
	})
	t.Run("unused assignment operand ignored", func(t *testing.T) {
		r := run(t, Config{Program: `{ print }`, Operands: []string{"unused=1"}}, "x\n")
		if r.out != "x\n" {
			t.Fatalf("got %q", r.out)
		}
	})
}

func TestUnsupported(t *testing.T) {
	cases := []struct{ name, prog, feature string }{
		{"array index", `BEGIN { a[1] = 2 }`, "arrays"},
		{"in operator", `BEGIN { if (1 in a) print }`, "arrays"},
		{"for in", `BEGIN { for (k in a) print k }`, "arrays"},
		{"delete", `BEGIN { delete a }`, "arrays"},
		{"function", `function f(x) { return x } BEGIN { }`, "user-defined functions"},
		{"func keyword", `func f(x) { return x }`, "user-defined functions"},
		{"call undefined", `BEGIN { f(1) }`, "user-defined functions"},
		{"return", `BEGIN { return 1 }`, "user-defined functions"},
		{"getline", `BEGIN { getline }`, "getline"},
		{"getline var file", `BEGIN { getline x < "f" }`, "getline"},
		{"pipe getline", `BEGIN { "date" | getline }`, "pipes"},
		{"print pipe", `BEGIN { print "x" | "cat" }`, "pipes"},
		{"print redirect", `BEGIN { print "x" > "f" }`, "output redirection"},
		{"print append", `BEGIN { print "x" >> "f" }`, "output redirection"},
		{"printf redirect", `BEGIN { printf("x") > "f" }`, "output redirection"},
		{"split", `BEGIN { split("a b", p) }`, "split"},
		{"close", `BEGIN { close("f") }`, "close"},
		{"system", `BEGIN { system("true") }`, "system"},
		{"fflush", `BEGIN { fflush() }`, "fflush"},
		{"group in", `BEGIN { if ((1,2) in a) print }`, "arrays"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := awkRun(t, c.prog, "")
			if r.status != 2 || r.out != "" {
				t.Fatalf("status %d out %q", r.status, r.out)
			}
			if !strings.Contains(r.err, c.feature) || !strings.Contains(r.err, "not supported") && !strings.Contains(r.err, "undefined function") {
				t.Fatalf("stderr %q does not name %q", r.err, c.feature)
			}
		})
	}
}

func TestSyntaxErrors(t *testing.T) {
	for _, prog := range []string{
		`BEGIN {`, `BEGIN { print ( }`, `{ print 1 +`, `BEGIN { x = }`, `BEGIN`, `BEGIN { "abc }`,
		`BEGIN { print /abc }`, `BEGIN { break }`, `BEGIN { continue }`, `BEGIN { print (1,2) 3 }`,
		`BEGIN { substr("a") }`, `BEGIN { sub(/a/, "b", "c") }`, `BEGIN { x = 1 = 2 }`, `BEGIN { @ }`,
		`BEGIN { print 1 2 3 }}`, `BEGIN { if }`, `BEGIN { rand(1) }`,
	} {
		r := awkRun(t, prog, "")
		if r.status != 2 || r.out != "" || !strings.HasPrefix(r.err, "awk: ") {
			t.Errorf("%q: status %d out %q err %q", prog, r.status, r.out, r.err)
		}
	}
}

func TestCancel(t *testing.T) {
	done := make(chan struct{})
	close(done)
	var out, errb bytes.Buffer
	status := Run(Config{
		Program: `BEGIN { while (1) x++ }`, Stdin: strings.NewReader(""),
		Stdout: &out, Stderr: &errb, Done: done,
	})
	if status != 1 || !strings.Contains(errb.String(), "canceled") {
		t.Fatalf("status %d stderr %q", status, errb.String())
	}
}

func TestLexer(t *testing.T) {
	kinds := func(src string) []tokKind {
		toks, err := lex(src)
		if err != nil {
			t.Fatal(err)
		}
		var ks []tokKind
		for _, tk := range toks {
			ks = append(ks, tk.kind)
		}
		return ks
	}
	cases := []struct {
		src  string
		want []tokKind
	}{
		{`a / b / c`, []tokKind{tName, tSlash, tName, tSlash, tName, tEOF}},
		{`/a/ ~ /b/`, []tokKind{tRegex, tTilde, tRegex, tEOF}},
		{`(a) / 2`, []tokKind{tLparen, tName, tRparen, tSlash, tNumber, tEOF}},
		{`x = /=/`, []tokKind{tName, tAssign, tRegex, tEOF}},
		{`x /= 2`, []tokKind{tName, tDivAssign, tNumber, tEOF}},
		{`f(x) g (x)`, []tokKind{tFuncName, tLparen, tName, tRparen, tName, tLparen, tName, tRparen, tEOF}},
		{`length / 2`, []tokKind{tBuiltin, tSlash, tNumber, tEOF}},
		{"a \\\n b", []tokKind{tName, tName, tEOF}},
		{"a # c\nb", []tokKind{tName, tNewline, tName, tEOF}},
		{`a**b **= c`, []tokKind{tName, tCaret, tName, tPowAssign, tName, tEOF}},
		{`a>>b>=c`, []tokKind{tName, tAppend, tName, tGe, tName, tEOF}},
		{`"a\"b"`, []tokKind{tString, tEOF}},
		{`/[/]\//`, []tokKind{tRegex, tEOF}},
		{`a++ + ++b`, []tokKind{tName, tIncr, tPlus, tIncr, tName, tEOF}},
		{`.5 5. 1e3 1e`, []tokKind{tNumber, tNumber, tNumber, tNumber, tName, tEOF}},
	}
	for _, c := range cases {
		got := kinds(c.src)
		if len(got) != len(c.want) {
			t.Errorf("%q: got %v want %v", c.src, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%q: got %v want %v", c.src, got, c.want)
				break
			}
		}
	}
	toks, _ := lex(`/a\/b[/]/`)
	if toks[0].text != `a/b[/]` {
		t.Errorf("regex text %q", toks[0].text)
	}
	toks, _ = lex(`"a\tb\.\101"`)
	if toks[0].text != "a\tb\\.A" {
		t.Errorf("string text %q", toks[0].text)
	}
	for _, bad := range []string{`"abc`, "\"abc\n\"", `/abc`, `a @ b`} {
		if _, err := lex(bad); err == nil {
			t.Errorf("%q should not lex", bad)
		}
	}
}

func TestNumberText(t *testing.T) {
	cases := []struct {
		n    float64
		want string
	}{
		{0, "0"}, {1, "1"}, {-5, "-5"}, {1e15, "1000000000000000"}, {1e16, "10000000000000000"}, {1e30, "1e+30"},
		{0.5, "0.5"}, {1.0 / 3, "0.333333"}, {123456789.5, "1.23457e+08"}, {1e-7, "1e-07"},
	}
	for _, c := range cases {
		if got := numToString(c.n, "%.6g"); got != c.want {
			t.Errorf("numToString(%v) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestLooksNumeric(t *testing.T) {
	yes := []string{"1", " 1 ", "-1", "+1", "1.5", ".5", "5.", "1e3", "1E-3", "\t7\n"}
	no := []string{"", " ", "a", "1a", "1 2", "0x10", "e5", "-", ".", "1e", "inf", "nan"}
	for _, s := range yes {
		if _, ok := looksNumeric(s); !ok {
			t.Errorf("%q should look numeric", s)
		}
	}
	for _, s := range no {
		if _, ok := looksNumeric(s); ok {
			t.Errorf("%q should not look numeric", s)
		}
	}
}

func TestRegexCache(t *testing.T) {
	in := &interp{reCache: map[string]*posixre.Regexp{}}
	for i := 0; i < 600; i++ {
		in.regex(strings.Repeat("a", i%300+1))
	}
	if len(in.reCache) > 256 {
		t.Fatalf("cache grew to %d", len(in.reCache))
	}
}
