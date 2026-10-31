package awk

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeShell is an in-memory Runner. Its commands:
//
//	echo words...   print the words
//	lines N         print 1..N
//	upper           copy stdin to stdout in upper case
//	sortlines       print stdin's lines sorted
//	cat             copy stdin to stdout
//	fail N          exit with status N
//	err words...    write the words to stderr
//	block           wait for cancellation
type fakeShell struct {
	mu   sync.Mutex
	log  []string
	live int
}

func (f *fakeShell) Run(ctx context.Context, cmd string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	f.mu.Lock()
	f.log = append(f.log, cmd)
	f.live++
	f.mu.Unlock()
	defer func() { f.mu.Lock(); f.live--; f.mu.Unlock() }()
	words := strings.Fields(cmd)
	switch words[0] {
	case "echo":
		fmt.Fprintln(stdout, strings.Join(words[1:], " "))
	case "lines":
		n, _ := strconv.Atoi(words[1])
		for i := 1; i <= n; i++ {
			fmt.Fprintln(stdout, i)
		}
	case "upper":
		data, _ := io.ReadAll(stdin)
		io.WriteString(stdout, strings.ToUpper(string(data)))
	case "cat":
		io.Copy(stdout, stdin)
	case "sortlines":
		var lines []string
		sc := bufio.NewScanner(stdin)
		for sc.Scan() {
			lines = append(lines, sc.Text())
		}
		sort.Strings(lines)
		for _, l := range lines {
			fmt.Fprintln(stdout, l)
		}
	case "fail":
		n, _ := strconv.Atoi(words[1])
		return n, nil
	case "err":
		fmt.Fprintln(stderr, strings.Join(words[1:], " "))
	case "block":
		<-ctx.Done()
		return 137, nil
	case "missing":
		return 127, fmt.Errorf("command not found")
	}
	return 0, nil
}

func shellRun(t *testing.T, prog, stdin string, operands ...string) (result, *fakeShell) {
	t.Helper()
	sh := &fakeShell{}
	return run(t, Config{Program: prog, Operands: operands, Run: sh.Run}, stdin), sh
}

func dirRun(t *testing.T, dir, prog, stdin string, operands ...string) result {
	t.Helper()
	sh := &fakeShell{}
	return run(t, Config{Program: prog, Operands: operands, Dir: dir, Run: sh.Run}, stdin)
}

func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func table(t *testing.T, cases []struct{ name, prog, in, want string }) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, _ := shellRun(t, c.prog, c.in)
			if r.status != 0 || r.out != c.want {
				t.Fatalf("awk %q <%q>\nstatus %d stderr %q\ngot  %q\nwant %q", c.prog, c.in, r.status, r.err, r.out, c.want)
			}
		})
	}
}

func failsWith(t *testing.T, prog, in, msg string) {
	t.Helper()
	r, _ := shellRun(t, prog, in)
	if r.status != 2 || !strings.Contains(r.err, msg) {
		t.Fatalf("awk %q: status %d stderr %q, want status 2 containing %q", prog, r.status, r.err, msg)
	}
}

func TestArrays(t *testing.T) {
	table(t, []struct{ name, prog, in, want string }{
		{"store and length", `BEGIN { a["x"] = 1; a["y"] = 2; print a["x"] + a["y"], length(a) }`, "", "3 2\n"},
		{"insertion order", `BEGIN { a["z"]; a["a"]; a["m"]; for (k in a) printf "%s ", k; print "" }`, "", "z a m \n"},
		{"numeric keys", `BEGIN { a[1] = 1; a["1"] = 2; a[1.0] = 3; print length(a), a[1] }`, "", "1 3\n"},
		{"fractional key uses CONVFMT", `BEGIN { CONVFMT = "%.2g"; a[0.1 + 0.2] = 1; for (k in a) print k }`, "", "0.3\n"},
		{"SUBSEP keys", `BEGIN { a[1, 2] = 3; for (k in a) { split(k, p, SUBSEP); print p[1], p[2] }
			if ((1, 2) in a) print "yes"; if (!((2, 1) in a)) print "no" }`, "", "1 2\nyes\nno\n"},
		{"SUBSEP changed", `BEGIN { SUBSEP = ":"; a[1, 2]; for (k in a) print k }`, "", "1:2\n"},
		{"in does not create", `BEGIN { if ("x" in a) print "bad"; print length(a); a["x"]; print length(a); y = a["q"]; print length(a) }`, "", "0\n1\n2\n"},
		{"in precedence", `BEGIN { a["x"]; print ("x" in a) ? "y" : "n"; print "x" in a, !("y" in a) }`, "", "y\n1 1\n"},
		{"in with &&", `BEGIN { a[1]; if (1 in a && !(2 in a)) print "ok" }`, "", "ok\n"},
		{"delete element", `BEGIN { a[1]; a[2]; delete a[1]; print length(a), (1 in a), (2 in a) }`, "", "1 0 1\n"},
		{"delete multi subscript", `BEGIN { a[1, 2]; a[3, 4]; delete a[1, 2]; print length(a) }`, "", "1\n"},
		{"delete whole array", `BEGIN { a[1]; a[2]; delete a; print length(a); a[5] = 1; print length(a) }`, "", "0\n1\n"},
		{"delete missing is fine", `BEGIN { delete a["nope"]; print length(a) }`, "", "0\n"},
		{"delete during iteration skips deleted", `BEGIN { for (i = 1; i <= 5; i++) a[i] = i
			for (k in a) { delete a[k + 1]; n++; s = s k }; print n, s, length(a) }`, "", "3 135 3\n"},
		{"insert during iteration not visited", `BEGIN { a[1]; for (k in a) { a[k + 1]; n++; if (n > 10) break }; print n, length(a) }`, "", "1 2\n"},
		{"order survives compaction", `BEGIN { for (i = 1; i <= 100; i++) a[i]; for (i = 1; i <= 90; i++) delete a[i]
			a["new"]; for (k in a) printf "%s ", k; print length(a) }`, "", "91 92 93 94 95 96 97 98 99 100 new 11\n"},
		{"uninitialised element", `BEGIN { print (a[1] == ""), (a[1] == 0), length(a) }`, "", "1 1 1\n"},
		{"element arithmetic", `BEGIN { a["k"] += 5; a["k"]++; ++a["k"]; a["k"] *= 2; print a["k"] }`, "", "14\n"},
		{"increment subscript once", `BEGIN { i = 1; a[i++] = "x"; print i, length(a), a[1] }`, "", "2 1 x\n"},
		{"count words", `{ for (i = 1; i <= NF; i++) c[$i]++ } END { for (w in c) print w, c[w] }`, "b a b c a b\n", "b 3\na 2\nc 1\n"},
		{"loop var is a string", `BEGIN { a[10]; for (k in a) print (k == 10), (k == "10") }`, "", "1 1\n"},
		{"field from element", `BEGIN { a[1] = 2; $0 = "x y z"; print $a[1] }`, "", "y\n"},
		{"sub on element", `BEGIN { a[1] = "foo"; gsub(/o/, "0", a[1]); print a[1] }`, "", "f00\n"},
	})
}

func TestArrayTypeErrors(t *testing.T) {
	failsWith(t, `BEGIN { x = 1; x[1] = 2 }`, "", "scalar x as an array")
	failsWith(t, `BEGIN { a[1] = 1; print a }`, "", "array a in a scalar context")
	failsWith(t, `BEGIN { a[1]; a = 2 }`, "", "can't assign to a")
	failsWith(t, `BEGIN { NF[1] = 2 }`, "", "scalar NF as an array")
	failsWith(t, `BEGIN { a[1]; b = a + 1 }`, "", "array a")
	failsWith(t, `function f(x) { x[1] = 1 } BEGIN { y = 1; f(y) }`, "", "scalar x as an array")
	failsWith(t, `function f(x) { return x + 1 } BEGIN { a[1]; f(a) }`, "", "array x")
	failsWith(t, `BEGIN { a[1]; for (a in a) ; }`, "", "assign to a")
}

func TestArgvAndEnviron(t *testing.T) {
	r := run(t, Config{Program: `BEGIN { for (i = 0; i < ARGC; i++) print i, ARGV[i] }`, Operands: []string{"a", "b=1"}}, "")
	if r.out != "0 awk\n1 a\n2 b=1\n" {
		t.Fatalf("got %q %q", r.out, r.err)
	}
	dir := writeFiles(t, map[string]string{"f1": "one\n", "f2": "two\n", "f3": "three\n"})
	r = dirRun(t, dir, `BEGIN { ARGV[1] = ""; ARGV[ARGC++] = "f3" } { print FILENAME, $0 }`, "", "f1", "f2")
	if r.out != "f2 two\nf3 three\n" {
		t.Fatalf("modified ARGV: %q %q", r.out, r.err)
	}
	r = dirRun(t, dir, `BEGIN { delete ARGV[1] } { print FILENAME, $0 }`, "", "f1", "f2")
	if r.out != "f2 two\n" {
		t.Fatalf("deleted ARGV: %q %q", r.out, r.err)
	}
	r = dirRun(t, dir, `BEGIN { ARGV[1] = ""; ARGV[2] = "" } { print "stdin", $0 }`, "in\n", "f1", "f2")
	if r.out != "stdin in\n" {
		t.Fatalf("ARGV all empty reads stdin: %q %q", r.out, r.err)
	}
	r = dirRun(t, dir, `BEGIN { ARGC = 2 } { print FILENAME }`, "", "f1", "f2")
	if r.out != "f1\n" {
		t.Fatalf("smaller ARGC: %q %q", r.out, r.err)
	}
	r = run(t, Config{Program: `BEGIN { print ENVIRON["FOO"], ENVIRON["N"] + 1, ("NOPE" in ENVIRON) }`, Env: []string{"FOO=bar", "N=5", "junk"}}, "")
	if r.out != "bar 6 0\n" {
		t.Fatalf("ENVIRON: %q %q", r.out, r.err)
	}
}

func TestFunctions(t *testing.T) {
	table(t, []struct{ name, prog, in, want string }{
		{"simple", `function f(x) { return x * 2 } BEGIN { print f(4) }`, "", "8\n"},
		{"func keyword", `func f(x) { return x + 1 } BEGIN { print f(1) }`, "", "2\n"},
		{"call before definition", `BEGIN { print f(3) } function f(x) { return x * x }`, "", "9\n"},
		{"newline before brace", "function f(x)\n{\n  return x + 1\n}\nBEGIN { print f(1) }", "", "2\n"},
		{"space before paren in definition", `function f (x) { return x } BEGIN { print f(7) }`, "", "7\n"},
		{"recursion", `function fib(n) { return n < 2 ? n : fib(n - 1) + fib(n - 2) } BEGIN { print fib(15) }`, "", "610\n"},
		{"deep recursion", `function d(n) { return n == 0 ? 0 : 1 + d(n - 1) } BEGIN { print d(5000) }`, "", "5000\n"},
		{"adjacent calls concatenate", `function f(x) { return x * 2 } BEGIN { print f(3) f(4) }`, "", "68\n"},
		{"scalar by value", `function inc(x) { x++; return x } BEGIN { y = 1; z = inc(y); print y, z }`, "", "1 2\n"},
		{"missing args are uninitialised", `function f(a, b) { return (b == "") + (b == 0) } BEGIN { print f(1) }`, "", "2\n"},
		{"return without value", `function f() { return } BEGIN { print f() + 0, length(f()) }`, "", "0 0\n"},
		{"no return", `function f(x) { x = 1 } BEGIN { print f(0) + 5 }`, "", "5\n"},
		{"locals shadow globals", `function f(x,   i) { i = 99; return x } BEGIN { i = 1; f(0); print i }`, "", "1\n"},
		{"globals are shared", `function bump() { n++ } BEGIN { bump(); bump(); print n }`, "", "2\n"},
		{"array by reference", `function fill(a, n,   i) { for (i = 1; i <= n; i++) a[i] = i * i } BEGIN { fill(sq, 3); print sq[3], length(sq) }`, "", "9 3\n"},
		{"array delete in callee", `function clr(a) { delete a } BEGIN { x[1]; x[2]; clr(x); print length(x) }`, "", "0\n"},
		{"untyped param becomes array", `function mk(arr) { arr["k"] = 1 }
			function outer(x) { mk(x); return x["k"] }
			BEGIN { print outer(q); print q["k"], length(q) }`, "", "1\n1 1\n"},
		{"local array is fresh per call", `function f(n,   loc) { loc[1] = n; return loc[1] + length(loc) } BEGIN { print f(5), f(6) }`, "", "6 7\n"},
		{"array passed through", `function a2(z) { return z[1] } function a1(y) { return a2(y) } BEGIN { w[1] = "deep"; print a1(w) }`, "", "deep\n"},
		{"split into param", `function parts(s, out) { return split(s, out, ",") } BEGIN { n = parts("a,b,c", p); print n, p[3] }`, "", "3 c\n"},
		{"recursion with array", `function walk(a, n) { if (n > 0) { a[n] = n; walk(a, n - 1) } } BEGIN { walk(t, 4); print length(t) }`, "", "4\n"},
		{"getline into local", `function rd(   line) { "echo fromcmd" | getline line; return line } BEGIN { print rd() }`, "", "fromcmd\n"},
		{"next in function", `function skip() { next } NR == 2 { skip() } { print }`, "a\nb\nc\n", "a\nc\n"},
		{"exit in function", `function bye() { exit 0 } { bye() } END { print "end" }`, "a\n", "end\n"},
		{"user function named like a variable elsewhere", `function g(x) { return x } BEGIN { v = g(1) + g(2); print v }`, "", "3\n"},
		{"function with regex and fields", `function first(s) { match(s, /[a-z]+/); return substr(s, RSTART, RLENGTH) } { print first($2) }`, "1 ab12\n", "ab\n"},
	})
}

func TestFunctionErrors(t *testing.T) {
	failsWith(t, `function f(a) { } BEGIN { f(1, 2) }`, "", "accepts only 1")
	failsWith(t, `BEGIN { nope(1) }`, "", "undefined function nope")
	failsWith(t, `function f(a, a) { }`, "", "duplicate parameter")
	failsWith(t, `function f() { } function f() { }`, "", "defined twice")
	failsWith(t, `BEGIN { return 1 }`, "", "return outside")
	failsWith(t, `function length(x) { }`, "", "built-in")
	failsWith(t, `function f(x) { return f(x) } BEGIN { f(1) }`, "", "nesting is too deep")
	failsWith(t, `function f() { break } BEGIN { f() }`, "", "break outside")
}

func TestSplit(t *testing.T) {
	table(t, []struct{ name, prog, in, want string }{
		{"default", `BEGIN { n = split("a b  c", p); print n, p[1], p[2], p[3] }`, "", "3 a b c\n"},
		{"string separator", `BEGIN { n = split("a:b:c", p, ":"); print n, p[3] }`, "", "3 c\n"},
		{"literal dot", `BEGIN { n = split("a.b.c", p, "."); print n }`, "", "3\n"},
		{"regex literal", `BEGIN { n = split("a1b22c", p, /[0-9]+/); print n, p[2], p[3] }`, "", "3 b c\n"},
		{"regex string", `BEGIN { n = split("a1b22c", p, "[0-9]+"); print n, p[3] }`, "", "3 c\n"},
		{"blank separator is whitespace", `BEGIN { n = split("  a   b ", p, " "); print n, p[1], p[2] }`, "", "2 a b\n"},
		{"empty separator splits characters", `BEGIN { n = split("abc", p, ""); print n, p[3] }`, "", "3 c\n"},
		{"empty string clears", `BEGIN { p[9] = 1; n = split("", p); print n, length(p) }`, "", "0 0\n"},
		{"replaces old elements", `BEGIN { split("a b c", p); split("x", p); print length(p), p[1] }`, "", "1 x\n"},
		{"source evaluated first", `BEGIN { a[1] = "x y z"; n = split(a[1], a); print n, a[1], a[3] }`, "", "3 x z\n"},
		{"elements are numeric strings", `BEGIN { split("10 9", p); print (p[1] > p[2]) }`, "", "1\n"},
		{"default separator is FS", `BEGIN { FS = ","; split("a,b c", p); print length(p), p[2] }`, "", "2 b c\n"},
		{"tab separator", "BEGIN { n = split(\"a\\tb c\", p, \"\\t\"); print n, p[2] }", "", "2 b c\n"},
	})
}

func TestGetlineFiles(t *testing.T) {
	dir := writeFiles(t, map[string]string{"f": "a b c\nd\ne\n", "num": "5.0\n", "semi": "x;y;z", "empty": ""})
	cases := []struct{ name, prog, in, want string }{
		{"read all", `BEGIN { while ((getline line < "f") > 0) n++; print n, line, NR }`, "", "3 e 0\n"},
		{"sets $0 and NF only", `BEGIN { getline < "f"; print $0, NF, NR, FNR }`, "", "a b c 3 0 0\n"},
		{"var form leaves $0", `BEGIN { $0 = "keep"; getline v < "f"; print v, $0, NF }`, "", "a b c keep 1\n"},
		{"stream is shared", `BEGIN { getline a < "f"; getline b < "f"; print a "|" b }`, "", "a b c|d\n"},
		{"close restarts", `BEGIN { getline a < "f"; r = close("f"); getline b < "f"; print r, a == b }`, "", "0 1\n"},
		{"missing file", `BEGIN { r = (getline x < "nope"); print r }`, "", "-1\n"},
		{"empty file", `BEGIN { r = (getline x < "empty"); print r }`, "", "0\n"},
		{"empty name", `BEGIN { r = (getline x < ""); print r }`, "", "-1\n"},
		{"end then zero", `BEGIN { getline x < "num"; print (getline x < "num") }`, "", "0\n"},
		{"unparenthesised comparison", `BEGIN { while (getline line < "f" > 0) n++; print n }`, "", "3\n"},
		{"numeric string", `BEGIN { getline v < "num"; print (v < 10) }`, "", "1\n"},
		{"honours RS", `BEGIN { RS = ";"; while ((getline v < "semi") > 0) printf "[%s]", v; print "" }`, "", "[x][y][z]\n"},
		{"into field", `BEGIN { $0 = "p q r"; getline $2 < "num"; print }`, "", "p 5.0 r\n"},
		{"into element", `BEGIN { getline a["k"] < "num"; print a["k"] }`, "", "5.0\n"},
		{"stdin dash", `BEGIN { getline x < "-"; print "got " x }`, "line\n", "got line\n"},
		{"stdin shared with main input", `BEGIN { getline x < "-" } { print "main " $0 } END { print x }`, "1\n2\n", "main 2\nmain 1\n"[:0] + "main 2\n1\n"},
		{"file named by concatenation", `BEGIN { n = "f"; getline v < n; print v }`, "", "a b c\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := dirRun(t, dir, c.prog, c.in)
			if r.status != 0 || r.out != c.want {
				t.Fatalf("awk %q\nstatus %d stderr %q\ngot  %q\nwant %q", c.prog, r.status, r.err, r.out, c.want)
			}
		})
	}
}

func TestGetlineMain(t *testing.T) {
	table(t, []struct{ name, prog, in, want string }{
		{"plain getline advances", `{ r = getline; print r, $0, NR, FNR }`, "a\nb\nc\n", "1 b 2 2\n0 c 3 3\n"},
		{"getline var", `NR == 1 { getline x; print x, $0, NR }`, "a\nb\nc\n", "b a 2\n"},
		{"getline resplits", `NR == 1 { getline; print NF, $2 }`, "a\nb c\n", "2 c\n"},
		{"in BEGIN", `BEGIN { getline; print "first:" $0, NR } { print "rule", $0 }`, "x\ny\n", "first:x 1\nrule y\n"},
		{"in END returns zero", `END { r = getline; print r, $0 }`, "a\nb\n", "0 b\n"},
		{"drain", `BEGIN { while ((getline line) > 0) n++; print n, NR }`, "a\nb\nc\n", "3 3\n"},
		{"numeric string", `BEGIN { getline var; print (var < 10.0) }`, "5.0\n", "1\n"},
	})
}

func TestGetlineFilesAndArgv(t *testing.T) {
	dir := writeFiles(t, map[string]string{"one": "1\n2\n", "two": "3\n"})
	r := dirRun(t, dir, `FNR == 1 && FILENAME == "one" { getline; print FILENAME, $0, NR, FNR } END { print NR }`, "", "one", "two")
	if r.out != "one 2 2 2\n3\n" {
		t.Fatalf("got %q %q", r.out, r.err)
	}
}

func TestGetlineCommand(t *testing.T) {
	table(t, []struct{ name, prog, in, want string }{
		{"sets $0 NF NR", `BEGIN { "echo hi there" | getline; print $0, NF, NR, FNR }`, "", "hi there 2 1 0\n"},
		{"var form sets var and NR", `BEGIN { "echo hi there" | getline v; print v, NR, NF }`, "", "hi there 1 0\n"},
		{"loop to end", `BEGIN { while (("lines 3" | getline n) > 0) s = s n; print s, NR }`, "", "123 3\n"},
		{"unparenthesised comparison", `BEGIN { while ("lines 2" | getline n > 0) s = s n; print s }`, "", "12\n"},
		{"exhausted stays exhausted", `BEGIN { c = "lines 1"; c | getline a; r = (c | getline b); print a, r }`, "", "1 0\n"},
		{"close reruns", `BEGIN { c = "lines 1"; c | getline a; close(c); c | getline b; print a, b, NR }`, "", "1 1 2\n"},
		{"command built by concatenation", `BEGIN { x = "hello"; "echo " x | getline y; print y }`, "", "hello\n"},
		{"exit status from close", `BEGIN { "fail 4" | getline; print close("fail 4") }`, "", "4\n"},
		{"close of unopened", `BEGIN { print close("nothing") }`, "", "-1\n"},
		{"stderr passes through", `BEGIN { "err oops" | getline; print "done" }`, "", "done\n"},
		{"flushes stdout first", `BEGIN { printf "a"; "echo b" | getline x; print x }`, "", "ab\n"},
	})
}

func TestRedirection(t *testing.T) {
	dir := writeFiles(t, map[string]string{"old": "keep\n"})
	read := func(name string) string {
		b, _ := os.ReadFile(filepath.Join(dir, name))
		return string(b)
	}
	cases := []struct{ name, prog, file, want, out string }{
		{"print to file", `BEGIN { print "a" > "o"; print "b" > "o" }`, "o", "a\nb\n", ""},
		{"printf to file", `BEGIN { printf "%d-%s\n", 5, "x" > "o" }`, "o", "5-x\n", ""},
		{"append", `BEGIN { print "new" >> "old" }`, "old", "keep\nnew\n", ""},
		{"truncate once", `BEGIN { print "x" > "old"; print "y" > "old" }`, "old", "x\ny\n", ""},
		{"reopen after close truncates", `BEGIN { print "x" > "o"; close("o"); print "y" > "o" }`, "o", "y\n", ""},
		{"expression name", `BEGIN { for (i = 1; i <= 2; i++) print i > "f" i }`, "f2", "2\n", ""},
		{"concatenated name", `END { for (i = 1; i <= 2; i++) { printf("i=%d\n", i) > "Cat_a" i; } }`, "Cat_a1", "i=1\n", ""},
		{"print without args", `{ print > "o" }`, "o", "line\n", ""},
		{"stdout name keeps order", `BEGIN { print "1"; print "2" > "/dev/stdout"; print "3"; print "4" > "-" }`, "", "", "1\n2\n3\n4\n"},
		{"redirect then compare", `BEGIN { x = (3 > 2); print x > "o" }`, "o", "1\n", ""},
		{"parenthesised comparison", `BEGIN { print (1 > 2) > "o" }`, "o", "0\n", ""},
		{"comma list", `BEGIN { print "a", "b" > "o" }`, "o", "a b\n", ""},
		{"parenthesised list", `BEGIN { print("a", "b") > "o" }`, "o", "a b\n", ""},
		{"flush makes data readable", `BEGIN { print "x" > "o"; fflush("o"); getline l < "o"; print l }`, "o", "x\n", "x\n"},
		{"write then read after close", `BEGIN { print "a" > "o"; print "b" > "o"; close("o"); while ((getline l < "o") > 0) print "got", l }`, "o", "a\nb\n", "got a\ngot b\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := dirRun(t, dir, c.prog, "line\n")
			if r.status != 0 || r.out != c.out {
				t.Fatalf("status %d stderr %q out %q, want %q", r.status, r.err, r.out, c.out)
			}
			if c.file != "" {
				if got := read(c.file); got != c.want {
					t.Fatalf("file %s = %q, want %q", c.file, got, c.want)
				}
			}
		})
	}
	t.Run("stderr", func(t *testing.T) {
		r := dirRun(t, dir, `BEGIN { print "to out"; print "to err" > "/dev/stderr"; print "again" }`, "")
		if r.out != "to out\nagain\n" || r.err != "to err\n" {
			t.Fatalf("out %q err %q", r.out, r.err)
		}
	})
	t.Run("close status", func(t *testing.T) {
		r := dirRun(t, dir, `BEGIN { print "x" > "o"; print close("o"), close("o") }`, "")
		if r.out != "0 -1\n" {
			t.Fatalf("got %q %q", r.out, r.err)
		}
	})
	t.Run("cannot open", func(t *testing.T) {
		r := dirRun(t, dir, `BEGIN { print "x" > "no/such/dir/f" }`, "")
		if r.status != 2 || !strings.Contains(r.err, "can't redirect to no/such/dir/f") {
			t.Fatalf("status %d err %q", r.status, r.err)
		}
	})
	t.Run("streams closed at exit", func(t *testing.T) {
		dirRun(t, dir, `BEGIN { for (i = 0; i < 50; i++) print i > "many" i % 5 }`, "")
		if got := read("many0"); got != "0\n5\n10\n15\n20\n25\n30\n35\n40\n45\n" {
			t.Fatalf("many0 = %q", got)
		}
	})
}

func TestPipesAndSystem(t *testing.T) {
	table(t, []struct{ name, prog, in, want string }{
		{"output pipe", `BEGIN { print "b" | "sortlines"; print "a" | "sortlines"; close("sortlines"); print "done" }`, "", "a\nb\ndone\n"},
		{"output pipe closed at exit", `BEGIN { print "z" | "upper"; print "hello" }`, "", "hello\nZ\n"},
		{"stdout flushed before pipe starts", `BEGIN { print "before"; print "x" | "upper"; close("upper"); print "after" }`, "", "before\nX\nafter\n"},
		{"printf to pipe", `BEGIN { printf "%s\n", "abc" | "upper" }`, "", "ABC\n"},
		{"pipe reused by name", `BEGIN { print "b" | "cat"; print "a" | "cat" }`, "", "b\na\n"},
		{"pipe status", `BEGIN { print "x" | "fail 3"; r = close("fail 3"); print r }`, "", "3\n"},
		{"each record to the pipe", `{ print $1 | "sortlines" }`, "c\nb\na\n", "a\nb\nc\n"},
		{"fflush pipe", `BEGIN { print "x" | "cat"; r = fflush("cat"); close("cat"); print r, "end" }`, "", "x\n0 end\n"},
		{"system status", `BEGIN { a = system("fail 5"); b = system("echo hi"); print a, b }`, "", "hi\n5 0\n"},
		{"system flushes first", `BEGIN { printf "a"; system("echo b"); print "c" }`, "", "ab\nc\n"},
		{"system flushes files", `BEGIN { print "x" > "sf"; system("echo cmd"); getline l < "sf"; print l }`, "", "cmd\nx\n"},
		{"system stderr", `BEGIN { system("err boom") }`, "", ""},
		{"fflush all", `BEGIN { printf "a"; print fflush(); print fflush("zzz") }`, "", "a0\n-1\n"},
		{"fflush stdout name", `BEGIN { print fflush("/dev/stdout") }`, "", "0\n"},
		{"close unopened", `BEGIN { print close("zzz") }`, "", "-1\n"},
		{"command failing to run", `BEGIN { print system("missing") }`, "", "127\n"},
	})
	r, _ := shellRun(t, `BEGIN { system("err boom") }`, "")
	if r.err != "boom\n" {
		t.Fatalf("stderr %q", r.err)
	}
	r, _ = shellRun(t, `BEGIN { print "x" | "missing" }`, "")
	if !strings.Contains(r.err, "missing: command not found") {
		t.Fatalf("stderr %q", r.err)
	}
}

func TestNoExecutor(t *testing.T) {
	for _, prog := range []string{`BEGIN { system("true") }`, `BEGIN { print 1 | "cat" }`, `BEGIN { "date" | getline }`} {
		r := awkRun(t, prog, "")
		if r.status != 2 || !strings.Contains(r.err, "not available") {
			t.Errorf("%q: status %d err %q", prog, r.status, r.err)
		}
	}
	r := awkRun(t, `BEGIN { print "a" > "/dev/stdout"; print length(ENVIRON) }`, "")
	if r.status != 0 || r.out != "a\n0\n" {
		t.Fatalf("no executor needed for plain features: %q %q", r.out, r.err)
	}
}

func TestCommandCancellation(t *testing.T) {
	cases := map[string]string{
		"input pipe":  `BEGIN { "block" | getline; print "unreachable" }`,
		"output pipe": `BEGIN { print "x" | "block"; close("block"); print "unreachable" }`,
		"system":      `BEGIN { system("block"); print "unreachable" }`,
		"at exit":     `BEGIN { print "x" | "block" }`,
	}
	for name, prog := range cases {
		t.Run(name, func(t *testing.T) {
			sh := &fakeShell{}
			done := make(chan struct{})
			finished := make(chan result, 1)
			go func() {
				finished <- run(t, Config{Program: prog, Run: sh.Run, Done: done}, "")
			}()
			time.Sleep(50 * time.Millisecond)
			close(done)
			select {
			case r := <-finished:
				if r.status == 0 || strings.Contains(r.out, "unreachable") {
					t.Fatalf("status %d out %q err %q", r.status, r.out, r.err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("program did not stop after cancellation")
			}
			sh.mu.Lock()
			defer sh.mu.Unlock()
			if sh.live != 0 {
				t.Fatalf("%d commands still running", sh.live)
			}
		})
	}
}

func TestNoLeakedCommands(t *testing.T) {
	_, sh := shellRun(t, `BEGIN { print "a" | "cat"; "lines 100000" | getline x; print "y" | "upper" }`, "")
	sh.mu.Lock()
	defer sh.mu.Unlock()
	if sh.live != 0 {
		t.Fatalf("%d commands still running after exit", sh.live)
	}
}
