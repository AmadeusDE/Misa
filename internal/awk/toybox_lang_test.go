package awk

// Cases adapted from Toybox tests/awk.test (0BSD), checkout
// b7ec52ac35e075caffca5d330995d44e8dbfc8c3; the roadmap pins
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb. These are the U42 cases: arrays,
// split, getline, redirection and ARGV. The programs are Toybox's; where a
// case depended on `ls -l` output or on a user name, the input here is
// original, and where Toybox's expected output reflects its hash order for
// `for (k in a)` the expectation here is this implementation's insertion
// order.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestToyboxArrays(t *testing.T) {
	const lines = "-rw-r--r-- 1 alice staff 10 Jan 1 file1 extra\n-rw-r--r-- 1 bob staff 20 Jan 2 file2 extra\n-rw-r--r-- 1 alice staff 30 Jan 3 file3 extra\nshort line here\n"
	cases := []struct{ name, prog, in, want string }{
		{"SPLIT input", `{ split($0,arr," "); if(arr[3] == "abc") print $2 }`, tbFile1, "vwx\n"},
		{"count per user (test17)", `BEGIN { number_of_users=0; } { if (NF>7) {
  user=0; for (i=1; i<=number_of_users; i++) { if (username[i] == $3) { user=i;
  } } if (user == 0) { username[++number_of_users]=$3; user=number_of_users; }
  count[user]++; } } END { for (i=1; i<=number_of_users; i++) {
  print count[i], username[i] } } `, lines, "2 alice\n1 bob\n"},
		{"for in with empty key (test18)", `{ usrname[$3]++;}END{for(i in usrname){print usrname[i],i;} }`, lines, "2 alice\n1 bob\n1 here\n"},
		{"for in filtered (test19)", `{ if (NF>7) { username[$3]++; } } END { for (i in username) {
  print username[i], i; } }`, lines, "2 alice\n1 bob\n"},
		{"for in skipping empty (test20)", `BEGIN { username[""]=0; } { username[$3]++; } END {
  for (i in username) { if (i != "") { print username[i], i; }}}`, lines, "2 alice\n1 bob\n1 here\n"},
		{"uninitialised index (test48)", `{ l[lines] = $0; ++lines } END { for (i = lines-1; i >= 0; --i) {
  print l[i]} }`, "Pat 100 97 58\nSandy 84 72 93\nChris 72 92 89\n", "Chris 72 92 89\nSandy 84 72 93\n\n"},
		{"split on empty string", `BEGIN { n = split("abc", a, "");print n, length(a)}`, "", "3 3\n"},
		{"split on empty regex", `BEGIN { n = split("abc", a, //);print n, length(a)}`, "", "3 3\n"},
		{"split() utf8", `BEGIN{n = split("aβc", a, ""); printf "%d %d", n, length(a);for (e = 1; e <= n; e++) printf " %s %s", e, "(" a[e] ")";print ""}`, "", "3 3 1 (a) 2 (β) 3 (c)\n"},
		{"getline of continued lines (test27)", `{ line = $0; while (substr(line,length(line),1) == "\\") {
  line = substr(line,1,length(line)-1); i=getline; if (i > 0) {
  line = line $0; } else { printf("%d", NR); } } print line; }`, tbFile1, tbFile1},
		{"getline var numeric string", `BEGIN{getline var; print (var < 10.0)}`, "5.0\n", "1\n"},
	}
	for _, c := range cases {
		check(t, c.name, c.prog, c.in, c.want)
	}
}

func TestToyboxArgv(t *testing.T) {
	r := run(t, Config{
		Program:  `BEGIN { for (i = 1; i < ARGC; i++) { print ARGV[i] } }`,
		Operands: []string{"input1", "input2", "input3", "input4"},
	}, "")
	if r.status != 0 || r.out != "input1\ninput2\ninput3\ninput4\n" {
		t.Fatalf("status %d out %q err %q", r.status, r.out, r.err)
	}
}

func TestToyboxRedirect(t *testing.T) {
	dir := t.TempDir()
	r := run(t, Config{Dir: dir, Program: `END { for (i=1;i<=2;i++) {
  printf("i=%d\n", i) > "ConcatedFile_a" i; } }`}, "")
	if r.status != 0 || r.out != "" {
		t.Fatalf("status %d out %q err %q", r.status, r.out, r.err)
	}
	for i, want := range []string{"i=1\n", "i=2\n"} {
		got, err := os.ReadFile(filepath.Join(dir, "ConcatedFile_a"+string(rune('1'+i))))
		if err != nil || string(got) != want {
			t.Fatalf("file %d = %q (%v), want %q", i+1, got, err, want)
		}
	}
}
