package awk

// Cases adapted from Toybox tests/awk.test (0BSD), checkout
// b7ec52ac35e075caffca5d330995d44e8dbfc8c3; the roadmap pins
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb. Only cases inside the U41 scope
// are included; bit functions and the locale fixtures are not. The array,
// split, getline, redirection and ARGV cases (U42) are in toybox_lang_test.go.
// Expected output is Toybox's, except where noted.

import "testing"

const (
	tbFile1 = "abc def ghi 5\nghi jkl mno 10\nmno pqr stu 15\nstu vwx abc 20\n"
	tbFile3 = "abc:def:ghi:5\nghi:jkl:mno:10\nmno:pqr:stu:15\nstu:vwx:abc:20\n"
	tbMulti = "abc defxy ghi\njkl mno\n\n\npqr stu\nvwxy abc\n"
)

func TestToyboxFiles(t *testing.T) {
	cases := []struct{ name, prog, in, want string }{
		{"FS switch by #START", `{ if ($1 == "#START") { FS=":"; } else if ($1 == "#STOP") { FS=" "; } else { print $3 } }`, tbFile1, "ghi\nmno\nstu\nabc\n"},
		{"FS in begin 2", `BEGIN{FS=":";}{if($2=="pqr"){print "first one:", $1;}}`, tbFile3, "first one: mno\n"},
		{"FS change mid-record (test07)", `{ print $2; FS=":"; print $2 }`, tbFile3, "\n\njkl\njkl\npqr\npqr\nvwx\nvwx\n"},
		{"FS by content (test04)", `{ if ( $0 ~ /:/ ) {FS=":";} else {FS=" ";} print $3 }`, tbFile1 + tbFile3, "ghi\nmno\nstu\nabc\n\nmno\nstu\nabc\n"},
		{"assign empty field (test09)", `BEGIN { FS=":"; OFS=":"; } { $2=""; print }`, tbFile3, "abc::ghi:5\nghi::mno:10\nmno::stu:15\nstu::abc:20\n"},
		{"RS paragraph newline FS (test12)", "BEGIN { RS=\"\"; FS=\"\\n\" } { print $2,$3; }", tbFile1, "ghi jkl mno 10 mno pqr stu 15\n"},
		{"RS space (test13)", `BEGIN { RS=" "; } { print; }`, tbFile1, "abc\ndef\nghi\n5\nghi\njkl\nmno\n10\nmno\npqr\nstu\n15\nstu\nvwx\nabc\n20\n\n"},
		{"FILENAME empty on stdin", `{ print "[" FILENAME "]" }`, "a\n", "[]\n"},
		{"NF test (test16)", `{ if (NF == 6) { } else { print "ERROR", NR; } }`, tbFile1, "ERROR 1\nERROR 2\nERROR 3\nERROR 4\n"},
		{"while and for printf (test02)", `BEGIN { i=1; while (i <= 5) { printf i "-" i*i " "; i=i+1; } for (i=1; i <= 5; i++) { printf i "-" i*i " "; } }`, "", "1-1 2-4 3-9 4-16 5-25 1-1 2-4 3-9 4-16 5-25 "},
		{"begin main end (test03)", `BEGIN { print "Start." } { print $1,"-",$1*$1; } END { print "End." }`, "5\n10\n", "Start.\n5 - 25\n10 - 100\nEnd.\n"},
		{"for loop with continue (test28)", `BEGIN { for (x = 0; x <= 20; x++) { if (x == 5) { continue; } printf "%d ",x } print "" }`, "", "0 1 2 3 4 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 \n"},
		{"center with length (test26)", `{ if (length($0) < 80) { prefix = ""; for (i = 1;i<(40-length($0))/2;i++) { prefix = prefix " " }; print prefix $0; } else { print; } }`, "abc def ghi 5\n", "             abc def ghi 5\n"},
		{"length filter", `length($0) > 12`, "Pat 100 97 58 77 89 11 45\nSandy 84 729\nChris 92 89\nsagar 22 2213\n", "Pat 100 97 58 77 89 11 45\nsagar 22 2213\n"},
		{"price fields", `{ print $5 }`, "1, Justin Timberlake, Title 545, Price $7.30\n", "545,\n"},
		{"FS=k;lksa;lkf;l", `BEGIN {FS=k;lksa;lkf;l} {print $2}`, "abcd efgh ijkl mnop 4\nqrst uvwx yzab cdef 6\n", "b\nr\n"},
		{"split fields utf8", `BEGIN{FS=""}; {printf "%d", NF; for (e = 1; e <= NF; e++) printf " %s %s", e, "(" $e ")"; print ""}`, "aβc", "3 1 (a) 2 (β) 3 (c)\n"},
		{"srand default seed", `BEGIN{ print srand()}`, "", "1\n"},
		{"srand returns previous seed", `BEGIN { a = srand(10); b = srand(20); print a, b }`, "", "1 10\n"},
		{"numeric string bug fixed 20240514", `{ var = $0; print (var < 10.0) }`, "5.0\n", "1\n"},
		{"print ARGC without args", `BEGIN { print ARGC }`, "", "1\n"},
		{"printf %c 0", `BEGIN{a=0; printf "(%c)", a}`, "", "(\x00)"},
		{"assign NF=0", `BEGIN { $0 = "a b"; print NF, "x" $0 "y"; NF = 0; print NF, "x" $0 "y" }`, "", "2 xa by\n0 xy\n"},
	}
	for _, c := range cases {
		check(t, c.name, c.prog, c.in, c.want)
	}
}

func TestToyboxMultiline(t *testing.T) {
	const program = `{print NR, NF, $0; for (i=1;i<=NF;i++)printf " %s %s", i, $i; print ""}`
	cases := []struct{ name, begin, want string }{
		{"multiline 1", `RS=""; FS=""`, "1 21 abc defxy ghi\njkl mno\n 1 a 2 b 3 c 4   5 d 6 e 7 f 8 x 9 y 10   11 g 12 h 13 i 14 \n 15 j 16 k 17 l 18   19 m 20 n 21 o\n2 16 pqr stu\nvwxy abc\n 1 p 2 q 3 r 4   5 s 6 t 7 u 8 \n 9 v 10 w 11 x 12 y 13   14 a 15 b 16 c\n"},
		{"multiline 2", `RS=""; FS=" "`, "1 5 abc defxy ghi\njkl mno\n 1 abc 2 defxy 3 ghi 4 jkl 5 mno\n2 4 pqr stu\nvwxy abc\n 1 pqr 2 stu 3 vwxy 4 abc\n"},
		{"multiline 3", `RS=""; FS="x"`, "1 3 abc defxy ghi\njkl mno\n 1 abc def 2 y ghi 3 jkl mno\n2 3 pqr stu\nvwxy abc\n 1 pqr stu 2 vw 3 y abc\n"},
		{"multiline 4", `RS=""; FS="[ ]"`, "1 4 abc defxy ghi\njkl mno\n 1 abc 2 defxy 3 ghi\njkl 4 mno\n2 3 pqr stu\nvwxy abc\n 1 pqr 2 stu\nvwxy 3 abc\n"},
		{"multiline 5", `RS=""; FS="xy"`, "1 2 abc defxy ghi\njkl mno\n 1 abc def 2  ghi\njkl mno\n2 2 pqr stu\nvwxy abc\n 1 pqr stu\nvw 2  abc\n"},
		{"null RS", `RS="()"; FS=" "`, "1 9 abc defxy ghi\njkl mno\n\n\npqr stu\nvwxy abc\n\n 1 abc 2 defxy 3 ghi 4 jkl 5 mno 6 pqr 7 stu 8 vwxy 9 abc\n"},
	}
	for _, c := range cases {
		check(t, c.name, "BEGIN { "+c.begin+" } "+program, tbMulti, c.want)
	}
}
