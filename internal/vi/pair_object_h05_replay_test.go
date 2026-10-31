package vi

import (
	"os"
	"path/filepath"
	"testing"
)

// Original public-event fixtures for H05 paired and quoted text objects. The
// expectations were confirmed by black-box replay of the same keystrokes in a
// stock vi-compatible editor and then written down as independent fixtures; no
// editor source was consulted and none is needed to run these tests. Cases
// that depend on keys continuing after a failure are written by hand
// (see docs/plans/vi-compat.md).

func TestPairObjectsH05(t *testing.T) {
	for _, f := range []replayFixture{
		{name: "inner paren", input: "a (b c) d\n", keys: "fbdi(", want: replayState{lines: []string{"a () d"}, col: 3, reg: []string{"b c"}, modified: true}},
		{name: "inner paren on open delimiter", input: "a (b c) d\n", keys: "f(di(", want: replayState{lines: []string{"a () d"}, col: 3, reg: []string{"b c"}, modified: true}},
		{name: "inner paren on close delimiter", input: "a (b c) d\n", keys: "f)di(", want: replayState{lines: []string{"a () d"}, col: 3, reg: []string{"b c"}, modified: true}},
		{name: "around paren", input: "a (b c) d\n", keys: "fbda(", want: replayState{lines: []string{"a  d"}, col: 2, reg: []string{"(b c)"}, modified: true}},
		{name: "around paren on close", input: "a (b c) d\n", keys: "f)da(", want: replayState{lines: []string{"a  d"}, col: 2, reg: []string{"(b c)"}, modified: true}},
		{name: "inner paren b alias", input: "a (b c) d\n", keys: "fbdib", want: replayState{lines: []string{"a () d"}, col: 3, reg: []string{"b c"}, modified: true}},
		{name: "around paren b alias", input: "a (b c) d\n", keys: "fbdab", want: replayState{lines: []string{"a  d"}, col: 2, reg: []string{"(b c)"}, modified: true}},
		{name: "inner paren close key", input: "a (b c) d\n", keys: "fbdi)", want: replayState{lines: []string{"a () d"}, col: 3, reg: []string{"b c"}, modified: true}},
		{name: "around paren close key", input: "a (b c) d\n", keys: "fbda)", want: replayState{lines: []string{"a  d"}, col: 2, reg: []string{"(b c)"}, modified: true}},
		{name: "nested inner picks innermost", input: "f(a, (b), c)\n", keys: "fbdi(", want: replayState{lines: []string{"f(a, (), c)"}, col: 6, reg: []string{"b"}, modified: true}},
		{name: "nested count 2", input: "f(a, (b), c)\n", keys: "fb2di(", want: replayState{lines: []string{"f()"}, col: 2, reg: []string{"a, (b), c"}, modified: true}},
		{name: "nested count too big fails", input: "f(a, (b), c)\n", keys: "fb3di(", want: replayState{lines: []string{"f(a, (b), c)"}, col: 6}},
		{name: "nested around count 2", input: "f(a, (b), c)\n", keys: "fb2da(", want: replayState{lines: []string{"f"}, reg: []string{"(a, (b), c)"}, modified: true}},
		{name: "nested cursor in outer only", input: "f(a, (b), c)\n", keys: "fadi(", want: replayState{lines: []string{"f()"}, col: 2, reg: []string{"a, (b), c"}, modified: true}},
		{name: "nested cursor on outer open", input: "f(a, (b), c)\n", keys: "f(di(", want: replayState{lines: []string{"f()"}, col: 2, reg: []string{"a, (b), c"}, modified: true}},
		{name: "nested cursor on outer close", input: "f(a, (b), c)\n", keys: "$di(", want: replayState{lines: []string{"f()"}, col: 2, reg: []string{"a, (b), c"}, modified: true}},
		{name: "nested around outer open", input: "f(a, (b), c)\n", keys: "f(da(", want: replayState{lines: []string{"f"}, reg: []string{"(a, (b), c)"}, modified: true}},
		{name: "operator count multiplies", input: "f(a, (b), c)\n", keys: "fb1d2i(", want: replayState{lines: []string{"f()"}, col: 2, reg: []string{"a, (b), c"}, modified: true}},
		{name: "count before object after operator count", input: "f(a, (b), c)\n", keys: "fb2d1i(", want: replayState{lines: []string{"f()"}, col: 2, reg: []string{"a, (b), c"}, modified: true}},
		{name: "empty pair d is a no-op", input: "x ()\n", keys: "f(di(", want: replayState{lines: []string{"x ()"}, col: 3}},
		{name: "empty pair d on close", input: "x ()\n", keys: "f)di(", want: replayState{lines: []string{"x ()"}, col: 3}},
		{name: "empty pair y is a no-op", input: "x ()\n", keys: "f(yi(", want: replayState{lines: []string{"x ()"}, col: 3}},
		{name: "empty pair c inserts", input: "x ()\n", keys: "f(ci(XY<Esc>", want: replayState{lines: []string{"x (XY)"}, col: 4, modified: true}},
		{name: "empty pair around", input: "x ()\n", keys: "f(da(", want: replayState{lines: []string{"x "}, col: 1, reg: []string{"()"}, modified: true}},
		{name: "no pair fails d", input: "abc\n", keys: "di(", want: replayState{lines: []string{"abc"}}},
		{name: "no pair fails around", input: "abc\n", keys: "da(", want: replayState{lines: []string{"abc"}}},
		{name: "unmatched open fails", input: "a (b\n", keys: "fbdi(", want: replayState{lines: []string{"a (b"}, col: 3}},
		{name: "unmatched close fails", input: "a b)\n", keys: "fbdi(", want: replayState{lines: []string{"a b)"}, col: 2}},
		{name: "cursor after pair fails", input: "(b) a\n", keys: "$di(", want: replayState{lines: []string{"(b) a"}, col: 4}},
		{name: "yank inner", input: "a (b c) d\n", keys: "fbyi(", want: replayState{lines: []string{"a (b c) d"}, col: 3, reg: []string{"b c"}}},
		{name: "yank around", input: "a (b c) d\n", keys: "fbya(", want: replayState{lines: []string{"a (b c) d"}, col: 2, reg: []string{"(b c)"}}},
		{name: "change inner", input: "a (b c) d\n", keys: "fbci(X<Esc>", want: replayState{lines: []string{"a (X) d"}, col: 3, reg: []string{"b c"}, modified: true}},
		{name: "change around", input: "a (b c) d\n", keys: "fbca(X<Esc>", want: replayState{lines: []string{"a X d"}, col: 2, reg: []string{"(b c)"}, modified: true}},
		{name: "square inner", input: "a [b c] d\n", keys: "fbdi[", want: replayState{lines: []string{"a [] d"}, col: 3, reg: []string{"b c"}, modified: true}},
		{name: "square inner close key", input: "a [b c] d\n", keys: "fbdi]", want: replayState{lines: []string{"a [] d"}, col: 3, reg: []string{"b c"}, modified: true}},
		{name: "square around", input: "a [b c] d\n", keys: "fbda]", want: replayState{lines: []string{"a  d"}, col: 2, reg: []string{"[b c]"}, modified: true}},
		{name: "square nested", input: "[a [b] c]\n", keys: "fbda[", want: replayState{lines: []string{"[a  c]"}, col: 3, reg: []string{"[b]"}, modified: true}},
		{name: "square nested count", input: "[a [b] c]\n", keys: "fb2di[", want: replayState{lines: []string{"[]"}, col: 1, reg: []string{"a [b] c"}, modified: true}},
		{name: "brace inner", input: "a {b c} d\n", keys: "fbdi{", want: replayState{lines: []string{"a {} d"}, col: 3, reg: []string{"b c"}, modified: true}},
		{name: "brace inner B alias", input: "a {b c} d\n", keys: "fbdiB", want: replayState{lines: []string{"a {} d"}, col: 3, reg: []string{"b c"}, modified: true}},
		{name: "brace around B alias", input: "a {b c} d\n", keys: "fbdaB", want: replayState{lines: []string{"a  d"}, col: 2, reg: []string{"{b c}"}, modified: true}},
		{name: "brace around close key", input: "a {b c} d\n", keys: "fbda}", want: replayState{lines: []string{"a  d"}, col: 2, reg: []string{"{b c}"}, modified: true}},
		{name: "brace inner close key", input: "a {b c} d\n", keys: "fbdi}", want: replayState{lines: []string{"a {} d"}, col: 3, reg: []string{"b c"}, modified: true}},
		{name: "brace nested count B", input: "{a {b} c}\n", keys: "fb2daB", want: replayState{lines: []string{""}, reg: []string{"{a {b} c}"}, modified: true}},
		{name: "angle inner", input: "a <b c> d\n", keys: "fbdi<lt>", want: replayState{lines: []string{"a <> d"}, col: 3, reg: []string{"b c"}, modified: true}},
		{name: "angle inner close key", input: "a <b c> d\n", keys: "fbdi>", want: replayState{lines: []string{"a <> d"}, col: 3, reg: []string{"b c"}, modified: true}},
		{name: "angle around", input: "a <b c> d\n", keys: "fbda<lt>", want: replayState{lines: []string{"a  d"}, col: 2, reg: []string{"<b c>"}, modified: true}},
		{name: "angle around close key", input: "a <b c> d\n", keys: "fbda>", want: replayState{lines: []string{"a  d"}, col: 2, reg: []string{"<b c>"}, modified: true}},
		{name: "angle nested", input: "<a <b> c>\n", keys: "fbda<lt>", want: replayState{lines: []string{"<a  c>"}, col: 3, reg: []string{"<b>"}, modified: true}},
		{name: "mixed kinds ignore other brackets", input: "(a [b) c]\n", keys: "fcdi(", want: replayState{lines: []string{"(a [b) c]"}, col: 7}},
		{name: "multiline linewise inner", input: "x (\n  foo\n  bar\n)\ny\n", keys: "jdi(", want: replayState{lines: []string{"x (", ")", "y"}, row: 1, reg: []string{"  foo", "  bar"}, linewise: true, modified: true}},
		{name: "multiline linewise around", input: "x (\n  foo\n  bar\n)\ny\n", keys: "jda(", want: replayState{lines: []string{"x ", "y"}, col: 1, reg: []string{"(\n  foo\n  bar\n)"}, modified: true}},
		{name: "multiline linewise change", input: "x (\n  foo\n  bar\n)\ny\n", keys: "jci(Z<Esc>", want: replayState{lines: []string{"x (", "Z", ")", "y"}, row: 1, reg: []string{"  foo", "  bar"}, linewise: true, modified: true}},
		{name: "multiline linewise yank", input: "x (\n  foo\n  bar\n)\ny\n", keys: "jyi(", want: replayState{lines: []string{"x (", "  foo", "  bar", ")", "y"}, row: 1, reg: []string{"  foo", "  bar"}, linewise: true}},
		{name: "multiline linewise yank last line", input: "x (\n  foo\n  bar\n)\ny\n", keys: "jjyi(", want: replayState{lines: []string{"x (", "  foo", "  bar", ")", "y"}, row: 1, reg: []string{"  foo", "  bar"}, linewise: true}},
		{name: "multiline indented close keeps the close", input: "x (\n  foo\n  bar\n  )\ny\n", keys: "jdi(", want: replayState{lines: []string{"x (", "  )", "y"}, row: 1, col: 2, reg: []string{"  foo", "  bar"}, linewise: true, modified: true}},
		{name: "multiline text after open is charwise", input: "x (foo\n  bar\n)\ny\n", keys: "jdi(", want: replayState{lines: []string{"x (", ")", "y"}, col: 2, reg: []string{"foo\n  bar"}, modified: true}},
		{name: "multiline text before close is charwise", input: "x (\n  foo\n  bar)\ny\n", keys: "jdi(", want: replayState{lines: []string{"x (", ")", "y"}, row: 1, reg: []string{"  foo\n  bar"}, modified: true}},
		{name: "multiline text on both sides is charwise", input: "x (a\n  foo\n  bar)\ny\n", keys: "jdi(", want: replayState{lines: []string{"x ()", "y"}, col: 3, reg: []string{"a\n  foo\n  bar"}, modified: true}},
		{name: "multiline text after open yank", input: "x (a\n  foo\n  bar\n)\ny\n", keys: "jyi(", want: replayState{lines: []string{"x (a", "  foo", "  bar", ")", "y"}, col: 3, reg: []string{"a\n  foo\n  bar"}}},
		{name: "multiline cursor on open delimiter", input: "x (\n  foo\n)\ny\n", keys: "f(di(", want: replayState{lines: []string{"x (", ")", "y"}, row: 1, reg: []string{"  foo"}, linewise: true, modified: true}},
		{name: "multiline cursor on close delimiter", input: "x (\n  foo\n)\ny\n", keys: "jj0di(", want: replayState{lines: []string{"x (", ")", "y"}, row: 1, reg: []string{"  foo"}, linewise: true, modified: true}},
		{name: "multiline blank inner line", input: "x (\n\n)\ny\n", keys: "jdi(", want: replayState{lines: []string{"x (", ")", "y"}, row: 1, reg: []string{""}, linewise: true, modified: true}},
		{name: "multiline blank inner line change", input: "x (\n\n)\ny\n", keys: "jci(Z<Esc>", want: replayState{lines: []string{"x (", "Z", ")", "y"}, row: 1, reg: []string{""}, linewise: true, modified: true}},
		{name: "multiline adjacent delimiters d", input: "x (\n)\ny\n", keys: "jdi(", want: replayState{lines: []string{"x (", ")", "y"}, row: 1}},
		{name: "multiline adjacent delimiters c", input: "x (\n)\ny\n", keys: "jci(Z<Esc>", want: replayState{lines: []string{"x (", "Z)", "y"}, row: 1, modified: true}},
		{name: "multiline around from inside", input: "f {\n  a\n  b\n}\n", keys: "jda{", want: replayState{lines: []string{"f "}, col: 1, reg: []string{"{\n  a\n  b\n}"}, modified: true}},
		{name: "multiline shift right", input: "x (\n  foo\n  bar\n)\ny\n", keys: "jj>i(", want: replayState{lines: []string{"x (", "\t  foo", "\t  bar", ")", "y"}, row: 1, col: 3, modified: true}},
		{name: "multiline shift left", input: "x (\n\tfoo\n\tbar\n)\ny\n", keys: "jj<lt>i(", want: replayState{lines: []string{"x (", "foo", "bar", ")", "y"}, row: 1, modified: true}},
		{name: "multiline shift single line", input: "a (b) c\n", keys: "fb>i(", want: replayState{lines: []string{"\ta (b) c"}, col: 1, modified: true}},
		{name: "multiline shift around", input: "x (\n  foo\n)\ny\n", keys: "jj>a(", want: replayState{lines: []string{"\tx (", "\t  foo", "\t)", "y"}, col: 1, modified: true}},
		{name: "multiline nested", input: "f(\n  g(a),\n  h(b)\n)\n", keys: "jfadi(", want: replayState{lines: []string{"f(", "  g(),", "  h(b)", ")"}, row: 1, col: 4, reg: []string{"a"}, modified: true}},
		{name: "multiline nested count", input: "f(\n  g(a),\n  h(b)\n)\n", keys: "jfa2di(", want: replayState{lines: []string{"f(", ")"}, row: 1, reg: []string{"  g(a),", "  h(b)"}, linewise: true, modified: true}},
		{name: "multiline around nested", input: "f(\n  g(a),\n  h(b)\n)\n", keys: "jfada(", want: replayState{lines: []string{"f(", "  g,", "  h(b)", ")"}, row: 1, col: 3, reg: []string{"(a)"}, modified: true}},
		{name: "multiline unmatched fails", input: "x (\n  foo\ny\n", keys: "jdi(", want: replayState{lines: []string{"x (", "  foo", "y"}, row: 1}},
		{name: "undo restores", input: "a (b c) d\n", keys: "fbdi(u", want: replayState{lines: []string{"a (b c) d"}, col: 3, reg: []string{"b c"}}},
		{name: "undo restores multiline", input: "x (\n  foo\n)\ny\n", keys: "jdi(u", want: replayState{lines: []string{"x (", "  foo", ")", "y"}, row: 1, reg: []string{"  foo"}, linewise: true}},
		{name: "undo change is one unit", input: "a (b c) d\n", keys: "fbci(XYZ<Esc>u", want: replayState{lines: []string{"a (b c) d"}, col: 3, reg: []string{"b c"}}},
		{name: "redo", input: "a (b c) d\n", keys: "fbdi(u<C-r>", want: replayState{lines: []string{"a () d"}, col: 3, reg: []string{"b c"}, modified: true}},
		{name: "dot after ci", input: "(a) (b)\n", keys: "ci(X<Esc>f(.", want: replayState{lines: []string{"(X) (X)"}, col: 5, reg: []string{"b"}, modified: true}},
		{name: "dot after di", input: "(a) (b)\n", keys: "di(f(.", want: replayState{lines: []string{"() ()"}, col: 4, reg: []string{"b"}, modified: true}},
		{name: "dot after da", input: "(a) (b) c\n", keys: "da(w.", want: replayState{lines: []string{"  c"}, col: 1, reg: []string{"(b)"}, modified: true}},
		{name: "dot after ci with empty pair", input: "() (b)\n", keys: "ci(X<Esc>f(.", want: replayState{lines: []string{"(X) (X)"}, col: 5, reg: []string{"b"}, modified: true}},
		{name: "dot fails quietly", input: "(a) b\n", keys: "di($.", want: replayState{lines: []string{"() b"}, col: 3, reg: []string{"a"}, modified: true}},
		{name: "yank does not set dot", input: "(a) (b)\n", keys: "di(f(yi(.", want: replayState{lines: []string{"() ()"}, col: 4, reg: []string{"b"}, modified: true}},
		{name: "visual inner", input: "a (b c) d\n", keys: "fbvi(d", want: replayState{lines: []string{"a () d"}, col: 3, reg: []string{"b c"}, modified: true}},
		{name: "visual inner yank", input: "a (b c) d\n", keys: "fbvi(y", want: replayState{lines: []string{"a (b c) d"}, col: 3, reg: []string{"b c"}}},
		{name: "visual around", input: "a (b c) d\n", keys: "fbva(d", want: replayState{lines: []string{"a  d"}, col: 2, reg: []string{"(b c)"}, modified: true}},
		{name: "visual inner b", input: "a (b c) d\n", keys: "fbvibd", want: replayState{lines: []string{"a () d"}, col: 3, reg: []string{"b c"}, modified: true}},
		{name: "visual around b", input: "a (b c) d\n", keys: "fbvabd", want: replayState{lines: []string{"a  d"}, col: 2, reg: []string{"(b c)"}, modified: true}},
		{name: "visual inner brace", input: "a {b c} d\n", keys: "fbviBd", want: replayState{lines: []string{"a {} d"}, col: 3, reg: []string{"b c"}, modified: true}},
		{name: "visual around close key", input: "a {b c} d\n", keys: "fbva}d", want: replayState{lines: []string{"a  d"}, col: 2, reg: []string{"{b c}"}, modified: true}},
		{name: "visual angle", input: "a <b c> d\n", keys: "fbvi<lt>d", want: replayState{lines: []string{"a <> d"}, col: 3, reg: []string{"b c"}, modified: true}},
		{name: "visual on open delimiter", input: "a (b c) d\n", keys: "f(vi(d", want: replayState{lines: []string{"a () d"}, col: 3, reg: []string{"b c"}, modified: true}},
		{name: "visual on close delimiter", input: "a (b c) d\n", keys: "f)vi(d", want: replayState{lines: []string{"a () d"}, col: 3, reg: []string{"b c"}, modified: true}},
		{name: "visual around on delimiter", input: "a (b c) d\n", keys: "f)va(d", want: replayState{lines: []string{"a  d"}, col: 2, reg: []string{"(b c)"}, modified: true}},
		{name: "visual repeated inner expands", input: "(a (b c) d)\n", keys: "fbvi(i(y", want: replayState{lines: []string{"(a (b c) d)"}, col: 1, reg: []string{"a (b c) d"}}},
		{name: "visual repeated around expands", input: "(a (b c) d)\n", keys: "fbva(a(y", want: replayState{lines: []string{"(a (b c) d)"}, reg: []string{"(a (b c) d)"}}},
		{name: "visual inner then around same pair", input: "(a (b c) d)\n", keys: "fbvi(a(y", want: replayState{lines: []string{"(a (b c) d)"}, col: 3, reg: []string{"(b c)"}}},
		{name: "visual around then inner outer pair", input: "(a (b c) d)\n", keys: "fbva(i(y", want: replayState{lines: []string{"(a (b c) d)"}, col: 1, reg: []string{"a (b c) d"}}},
		{name: "visual count", input: "(a (b c) d)\n", keys: "fbv2i(y", want: replayState{lines: []string{"(a (b c) d)"}, col: 1, reg: []string{"a (b c) d"}}},
		{name: "visual count around", input: "(a (b c) d)\n", keys: "fbv2a(y", want: replayState{lines: []string{"(a (b c) d)"}, reg: []string{"(a (b c) d)"}}},
		{name: "visual explicit selection equal to inner expands", input: "(a (b c) d)\n", keys: "fbvlli(y", want: replayState{lines: []string{"(a (b c) d)"}, col: 1, reg: []string{"a (b c) d"}}},
		{name: "visual selection wider than inner", input: "(a (b c) d)\n", keys: "fbvllla(y", want: replayState{lines: []string{"(a (b c) d)"}, col: 3, reg: []string{"(b c)"}}},
		{name: "visual backwards selection expands", input: "(a (b c) d)\n", keys: "fbvhhi(y", want: replayState{lines: []string{"(a (b c) d)"}, col: 1, reg: []string{"a (b c) d"}}},
		{name: "visual single char inner reselects", input: "((a))\n", keys: "llvi(i(y", want: replayState{lines: []string{"((a))"}, col: 2, reg: []string{"a"}}},
		{name: "visual two char inner expands", input: "((ab))\n", keys: "llvi(i(y", want: replayState{lines: []string{"((ab))"}, col: 1, reg: []string{"(ab)"}}},
		{name: "visual linewise becomes charwise", input: "(a (b c) d)\n", keys: "fbVi(y", want: replayState{lines: []string{"(a (b c) d)"}, col: 4, reg: []string{"b c"}}},
		{name: "visual multiline inner takes the break", input: "x (\n  foo\n  bar\n)\ny\n", keys: "jjvi(d", want: replayState{lines: []string{"x (", ")", "y"}, row: 1, reg: []string{"  foo\n  bar\n"}, modified: true}},
		{name: "visual multiline inner yank", input: "x (\n  foo\n  bar\n)\ny\n", keys: "jjvi(y", want: replayState{lines: []string{"x (", "  foo", "  bar", ")", "y"}, row: 1, reg: []string{"  foo\n  bar\n"}}},
		{name: "visual multiline text after open", input: "x (a\n  foo\n  bar\n)\ny\n", keys: "jjvi(y", want: replayState{lines: []string{"x (a", "  foo", "  bar", ")", "y"}, col: 3, reg: []string{"a\n  foo\n  bar\n"}}},
		{name: "visual multiline text before close", input: "x (\n  foo\n  bar)\ny\n", keys: "jjvi(y", want: replayState{lines: []string{"x (", "  foo", "  bar)", "y"}, row: 1, reg: []string{"  foo\n  bar"}}},
		{name: "visual multiline indented close", input: "x (\n  foo\n  bar\n  )\ny\n", keys: "jjvi(y", want: replayState{lines: []string{"x (", "  foo", "  bar", "  )", "y"}, row: 1, reg: []string{"  foo\n  bar\n"}}},
		{name: "visual multiline around", input: "x (\n  foo\n  bar\n)\ny\n", keys: "jjva(y", want: replayState{lines: []string{"x (", "  foo", "  bar", ")", "y"}, col: 2, reg: []string{"(\n  foo\n  bar\n)"}}},
		{name: "visual multiline change", input: "x (\n  foo\n)\ny\n", keys: "jvi(c1<Esc>", want: replayState{lines: []string{"x (", "1)", "y"}, row: 1, reg: []string{"  foo\n"}, modified: true}},
		{name: "visual multiline blank line", input: "x (\n\n)\ny\n", keys: "jvi(d", want: replayState{lines: []string{"x (", ")", "y"}, row: 1, reg: []string{"\n"}, modified: true}},
		{name: "visual change", input: "a (b c) d\n", keys: "fbvi(cX<Esc>", want: replayState{lines: []string{"a (X) d"}, col: 3, reg: []string{"b c"}, modified: true}},
		{name: "no pair fails c", input: "abc\n", keys: "ci(", want: replayState{lines: []string{"abc"}}},
		{name: "mixed kinds inner square", input: "(a [b) c]\n", keys: "fcdi[", want: replayState{lines: []string{"(a []"}, col: 4, reg: []string{"b) c"}, modified: true}},
		{name: "close indent only", input: "x (\n  )\ny\n", keys: "jdi(", want: replayState{lines: []string{"x (", "  )", "y"}, row: 1}},
		{name: "close indent only c", input: "x (\n  )\ny\n", keys: "jci(Z<Esc>", want: replayState{lines: []string{"x (", "Z  )", "y"}, row: 1, modified: true}},
		{name: "text then indented close", input: "x (foo\n  )\ny\n", keys: "jdi(", want: replayState{lines: []string{"x (", "  )", "y"}, col: 2, reg: []string{"foo"}, modified: true}},
		{name: "open then spaces", input: "x (  \n  foo\n)\ny\n", keys: "jdi(", want: replayState{lines: []string{"x (", ")", "y"}, col: 2, reg: []string{"  \n  foo"}, modified: true}},
		{name: "open then spaces yank", input: "x (  \n  foo\n)\ny\n", keys: "jyi(", want: replayState{lines: []string{"x (  ", "  foo", ")", "y"}, col: 3, reg: []string{"  \n  foo"}}},
		{name: "three line count", input: "f(\n  g(\n    a\n  )\n)\n", keys: "jjdi(", want: replayState{lines: []string{"f(", "  g(", "  )", ")"}, row: 2, col: 2, reg: []string{"    a"}, linewise: true, modified: true}},
		{name: "three line count 2", input: "f(\n  g(\n    a\n  )\n)\n", keys: "jj2di(", want: replayState{lines: []string{"f(", ")"}, row: 1, reg: []string{"  g(", "    a", "  )"}, linewise: true, modified: true}},
		{name: "around count multi", input: "f(\n  g(\n    a\n  )\n)\n", keys: "jj2da(", want: replayState{lines: []string{"f"}, reg: []string{"(\n  g(\n    a\n  )\n)"}, modified: true}},
		{name: "visual count multi", input: "f(\n  g(\n    a\n  )\n)\n", keys: "jjv2i(y", want: replayState{lines: []string{"f(", "  g(", "    a", "  )", ")"}, row: 1, reg: []string{"  g(\n    a\n  )\n"}}},
		{name: "visual repeat multi", input: "f(\n  g(\n    a\n  )\n)\n", keys: "jjvi(i(y", want: replayState{lines: []string{"f(", "  g(", "    a", "  )", ")"}, row: 1, reg: []string{"  g(\n    a\n  )\n"}}},
		{name: "visual around multi repeat", input: "f(\n  g(\n    a\n  )\n)\n", keys: "jjva(a(y", want: replayState{lines: []string{"f(", "  g(", "    a", "  )", ")"}, col: 1, reg: []string{"(\n  g(\n    a\n  )\n)"}}},
		{name: "tab indent close", input: "x {\n\tfoo\n\t}\n", keys: "jdiB", want: replayState{lines: []string{"x {", "\t}"}, row: 1, col: 1, reg: []string{"\tfoo"}, linewise: true, modified: true}},
		{name: "yank cursor moves multiline pair", input: "x (\n  foo\n  bar\n)\n", keys: "jjyi(", want: replayState{lines: []string{"x (", "  foo", "  bar", ")"}, row: 1, reg: []string{"  foo", "  bar"}, linewise: true}},
		{name: "yank cursor moves single pair", input: "a (b c) d\n", keys: "fcyi(", want: replayState{lines: []string{"a (b c) d"}, col: 3, reg: []string{"b c"}}},
		{name: "yank around cursor", input: "a (b c) d\n", keys: "fcya(", want: replayState{lines: []string{"a (b c) d"}, col: 2, reg: []string{"(b c)"}}},
		// --- forward search: outside every pair the next pair is used; failures ---
		{name: "fwd same line", input: "a (b) c\n", keys: "di(", want: replayState{lines: []string{"a () c"}, col: 3, reg: []string{"b"}, modified: true}},
		{name: "fwd skip nested picks first", input: "a (b) (c)\n", keys: "di(", want: replayState{lines: []string{"a () (c)"}, col: 3, reg: []string{"b"}, modified: true}},
		{name: "fwd after pair fails", input: "(b) a\n", keys: "$di(", want: replayState{lines: []string{"(b) a"}, col: 4}},
		{name: "fwd next line", input: "a\n(b)\n", keys: "di(", want: replayState{lines: []string{"a", "()"}, row: 1, col: 1, reg: []string{"b"}, modified: true}},
		{name: "fwd next line far", input: "a\nx\n(b)\n", keys: "di(", want: replayState{lines: []string{"a", "x", "()"}, row: 2, col: 1, reg: []string{"b"}, modified: true}},
		{name: "fwd around", input: "a (b) c\n", keys: "da(", want: replayState{lines: []string{"a  c"}, col: 2, reg: []string{"(b)"}, modified: true}},
		{name: "fwd change", input: "a (b) c\n", keys: "ci(X<Esc>", want: replayState{lines: []string{"a (X) c"}, col: 3, reg: []string{"b"}, modified: true}},
		{name: "fwd count 2", input: "a ((b)) c\n", keys: "d2i(", want: replayState{lines: []string{"a (()) c"}, col: 4, reg: []string{"b"}, modified: true}},
		{name: "fwd count with sibling", input: "a (b) (c)\n", keys: "d2i(", want: replayState{lines: []string{"a (b) (c)"}}},
		{name: "fwd on square", input: "a [b] c\n", keys: "di[", want: replayState{lines: []string{"a [] c"}, col: 3, reg: []string{"b"}, modified: true}},
		{name: "fwd other kind only", input: "a [b] (c)\n", keys: "di(", want: replayState{lines: []string{"a [b] ()"}, col: 7, reg: []string{"c"}, modified: true}},
		{name: "fwd visual", input: "a (b) c\n", keys: "vi(y", want: replayState{lines: []string{"a (b) c"}, col: 3, reg: []string{"b"}}},
		{name: "fwd visual around", input: "a (b) c\n", keys: "va(y", want: replayState{lines: []string{"a (b) c"}, col: 2, reg: []string{"(b)"}}},
		{name: "fwd cursor between pairs", input: "(a) x (b)\n", keys: "fxdi(", want: replayState{lines: []string{"(a) x ()"}, col: 7, reg: []string{"b"}, modified: true}},
		{name: "fwd cursor between pairs nested", input: "((a) x (b))\n", keys: "fxdi(", want: replayState{lines: []string{"()"}, col: 1, reg: []string{"(a) x (b)"}, modified: true}},
		{name: "fwd yank", input: "a (b) c\n", keys: "yi(", want: replayState{lines: []string{"a (b) c"}, col: 3, reg: []string{"b"}}},
		{name: "fwd unmatched open fwd", input: "a (b c\n", keys: "di(", want: replayState{lines: []string{"a (b c"}}},
		{name: "fwd multiline open", input: "a (\n  b\n)\n", keys: "di(", want: replayState{lines: []string{"a (", ")"}, row: 1, reg: []string{"  b"}, linewise: true, modified: true}},
		{name: "fwd later line only after 2", input: "a\nx (b)\n", keys: "di(", want: replayState{lines: []string{"a", "x ()"}, row: 1, col: 3, reg: []string{"b"}, modified: true}},
		{name: "fwd unmatched first then pair", input: "a (b (c)\n", keys: "di(", want: replayState{lines: []string{"a (b (c)"}}},
		{name: "fwd count 3 inward", input: "a (((b))) c\n", keys: "d3i(", want: replayState{lines: []string{"a ((())) c"}, col: 5, reg: []string{"b"}, modified: true}},
		{name: "fwd count 2 inward not first child", input: "a ((x)(b)) c\n", keys: "d2i(", want: replayState{lines: []string{"a (()(b)) c"}, col: 4, reg: []string{"x"}, modified: true}},
		{name: "fwd count 2 deeper", input: "a (x (y)) c\n", keys: "d2i(", want: replayState{lines: []string{"a (x ()) c"}, col: 6, reg: []string{"y"}, modified: true}},
		{name: "fwd close only before", input: "a ) (b)\n", keys: "di(", want: replayState{lines: []string{"a ) (b)"}}},
		{name: "fwd cursor on close of earlier line", input: "(a)\nx\n", keys: "jdi(", want: replayState{lines: []string{"(a)", "x"}, row: 1}},
		{name: "fwd cursor mid line later pair same line", input: "x a (b)\n", keys: "lldi(", want: replayState{lines: []string{"x a ()"}, col: 5, reg: []string{"b"}, modified: true}},
		{name: "no pair ahead fails", input: "abc\n", keys: "di(", want: replayState{lines: []string{"abc"}}},
		{name: "failure keeps the register", input: "abc\n", keys: "yiwdi(", want: replayState{lines: []string{"abc"}, reg: []string{"abc"}}},
		{name: "failure does not swallow the next key", input: "abc\n", keys: "di(x", want: replayState{lines: []string{"bc"}, reg: []string{"a"}, modified: true}},
		{name: "failed dot keeps the previous change", input: "(a) b\n", keys: "di(fb.", want: replayState{lines: []string{"() b"}, col: 3, reg: []string{"a"}, modified: true}},
		{name: "visual with no pair ahead keeps selection", input: "a b\n", keys: "vi(y", want: replayState{lines: []string{"a b"}, reg: []string{"a"}}},
		{name: "unknown object key cancels the operator", input: "a (b)\n", keys: "fbdizx", want: replayState{lines: []string{"a ()"}, col: 3, reg: []string{"b"}, modified: true}},

		// --- hand-written: Visual failures leave the selection alone and later keys still run ---
		{name: "visual repeated beyond outermost keeps selection", input: "(a (b c) d)\n", keys: "fbvi(i(i(y", want: replayState{lines: []string{"(a (b c) d)"}, col: 1, reg: []string{"a (b c) d"}}},
		{name: "visual empty pair keeps selection", input: "x ()\n", keys: "f(vi(y", want: replayState{lines: []string{"x ()"}, col: 2, reg: []string{"("}}},
		{name: "visual motion extends from the object start", input: "a (b c) d\n", keys: "fbvi(ly", want: replayState{lines: []string{"a (b c) d"}, col: 3, reg: []string{"b c)"}}},

		{name: "pair with multibyte text", input: "é (λ μ) ü\n", keys: "fλdi(", want: replayState{lines: []string{"é () ü"}, col: 3, reg: []string{"λ μ"}, modified: true}},

		// --- named registers ---
		{name: "named register round trip", input: "a (b c) d\n", keys: "fb\"adi(\"ap", want: replayState{lines: []string{"a ()b c d"}, col: 6, reg: []string{"b c"}, modified: true}},
		{name: "named register yank", input: "a (b c) d\n", keys: "fb\"ayi(0\"ap", want: replayState{lines: []string{"ab c (b c) d"}, col: 3, reg: []string{"b c"}, modified: true}},
	} {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}

func TestQuoteObjectsH05(t *testing.T) {
	for _, f := range []replayFixture{
		{name: "double quotes inner", input: "say \"hello world\" now\n", keys: "fwdi\"", want: replayState{lines: []string{"say \"\" now"}, col: 5, reg: []string{"hello world"}, modified: true}},
		{name: "double quotes around", input: "say \"hello world\" now\n", keys: "fwda\"", want: replayState{lines: []string{"say now"}, col: 4, reg: []string{"\"hello world\" "}, modified: true}},
		{name: "quotes count 2 inner takes the quotes", input: "say \"hello world\" now\n", keys: "fwd2i\"", want: replayState{lines: []string{"say  now"}, col: 4, reg: []string{"\"hello world\""}, modified: true}},
		{name: "quotes count 2 around", input: "say \"hello world\" now\n", keys: "fwd2a\"", want: replayState{lines: []string{"say now"}, col: 4, reg: []string{"\"hello world\" "}, modified: true}},
		{name: "quotes count 3 inner", input: "say \"hello world\" now\n", keys: "fwd3i\"", want: replayState{lines: []string{"say  now"}, col: 4, reg: []string{"\"hello world\""}, modified: true}},
		{name: "quotes before first quote use next string", input: "say \"hello world\" now\n", keys: "di\"", want: replayState{lines: []string{"say \"\" now"}, col: 5, reg: []string{"hello world"}, modified: true}},
		{name: "quotes before first quote around", input: "say \"hello world\" now\n", keys: "da\"", want: replayState{lines: []string{"say now"}, col: 4, reg: []string{"\"hello world\" "}, modified: true}},
		{name: "quotes after last quote fails", input: "say \"hello world\" now\n", keys: "$di\"", want: replayState{lines: []string{"say \"hello world\" now"}, col: 20}},
		{name: "quotes after last quote around fails", input: "say \"hello world\" now\n", keys: "$da\"", want: replayState{lines: []string{"say \"hello world\" now"}, col: 20}},
		{name: "quotes cursor on opening quote", input: "say \"hello world\" now\n", keys: "f\"di\"", want: replayState{lines: []string{"say \"\" now"}, col: 5, reg: []string{"hello world"}, modified: true}},
		{name: "quotes cursor on opening quote around", input: "say \"hello world\" now\n", keys: "f\"da\"", want: replayState{lines: []string{"say now"}, col: 4, reg: []string{"\"hello world\" "}, modified: true}},
		{name: "quotes cursor on closing quote", input: "say \"hello world\" now\n", keys: "f\";da\"", want: replayState{lines: []string{"say now"}, col: 4, reg: []string{"\"hello world\" "}, modified: true}},
		{name: "quotes cursor on closing quote inner", input: "say \"hello world\" now\n", keys: "f\";di\"", want: replayState{lines: []string{"say \"\" now"}, col: 5, reg: []string{"hello world"}, modified: true}},
		{name: "quotes around at end uses leading space", input: "say \"hello world\"\n", keys: "f\";da\"", want: replayState{lines: []string{"say"}, col: 2, reg: []string{" \"hello world\""}, modified: true}},
		{name: "quotes around at start uses trailing space", input: "\"hello world\" now\n", keys: "fwda\"", want: replayState{lines: []string{"now"}, reg: []string{"\"hello world\" "}, modified: true}},
		{name: "quotes around prefers trailing space", input: "x  \"a\"  y\n", keys: "fada\"", want: replayState{lines: []string{"x  y"}, col: 3, reg: []string{"\"a\"  "}, modified: true}},
		{name: "quotes around leading spaces only", input: "x  \"a\"y\n", keys: "fada\"", want: replayState{lines: []string{"xy"}, col: 1, reg: []string{"  \"a\""}, modified: true}},
		{name: "quotes around no spaces", input: "x\"a\"y\n", keys: "fada\"", want: replayState{lines: []string{"xy"}, col: 1, reg: []string{"\"a\""}, modified: true}},
		{name: "quotes around whole line", input: "\"a\"\n", keys: "lda\"", want: replayState{lines: []string{""}, reg: []string{"\"a\""}, modified: true}},
		{name: "quotes second string", input: "x \"a\" y \"b\" z\n", keys: "fbdi\"", want: replayState{lines: []string{"x \"a\" y \"\" z"}, col: 9, reg: []string{"b"}, modified: true}},
		{name: "quotes second string around", input: "x \"a\" y \"b\" z\n", keys: "fbda\"", want: replayState{lines: []string{"x \"a\" y z"}, col: 8, reg: []string{"\"b\" "}, modified: true}},
		{name: "quotes between strings selects the gap", input: "x \"a\" y \"b\" z\n", keys: "fydi\"", want: replayState{lines: []string{"x \"a\"\"b\" z"}, col: 5, reg: []string{" y "}, modified: true}},
		{name: "quotes between strings around", input: "x \"a\" y \"b\" z\n", keys: "fyda\"", want: replayState{lines: []string{"x \"ab\" z"}, col: 4, reg: []string{"\" y \""}, modified: true}},
		{name: "quotes pairing from line start", input: "x \"a\" y \"b\" z\n", keys: "f\";;di\"", want: replayState{lines: []string{"x \"a\" y \"\" z"}, col: 9, reg: []string{"b"}, modified: true}},
		{name: "quotes pairing closing second", input: "x \"a\" y \"b\" z\n", keys: "f\";;;da\"", want: replayState{lines: []string{"x \"a\" y z"}, col: 8, reg: []string{"\"b\" "}, modified: true}},
		{name: "quotes empty d is a no-op", input: "x \"\" y\n", keys: "f\"di\"", want: replayState{lines: []string{"x \"\" y"}, col: 3}},
		{name: "quotes empty c inserts", input: "x \"\" y\n", keys: "f\"ci\"X<Esc>", want: replayState{lines: []string{"x \"X\" y"}, col: 3, modified: true}},
		{name: "quotes empty c on closing quote", input: "x \"\" y\n", keys: "f\"lci\"X<Esc>", want: replayState{lines: []string{"x \"X\" y"}, col: 3, modified: true}},
		{name: "quotes empty around", input: "x \"\" y\n", keys: "f\"da\"", want: replayState{lines: []string{"x y"}, col: 2, reg: []string{"\"\" "}, modified: true}},
		{name: "quotes escaped quote stays inside", input: "x \"a\\\"b\" y\n", keys: "fbdi\"", want: replayState{lines: []string{"x \"\" y"}, col: 3, reg: []string{"a\\\"b"}, modified: true}},
		{name: "quotes escaped quote around", input: "x \"a\\\"b\" y\n", keys: "fbda\"", want: replayState{lines: []string{"x y"}, col: 2, reg: []string{"\"a\\\"b\" "}, modified: true}},
		{name: "quotes cursor on backslash", input: "x \"a\\\"b\" y\n", keys: "f\\\\di\"", want: replayState{lines: []string{"x \"\" y"}, col: 3, reg: []string{"a\\\"b"}, modified: true}},
		{name: "quotes cursor on escaped quote", input: "x \"a\\\"b\" y\n", keys: "f\";di\"", want: replayState{lines: []string{"x \"\" y"}, col: 3, reg: []string{"a\\\"b"}, modified: true}},
		{name: "quotes cursor on opening before escape", input: "x \"a\\\"b\" y\n", keys: "f\"di\"", want: replayState{lines: []string{"x \"\" y"}, col: 3, reg: []string{"a\\\"b"}, modified: true}},
		{name: "quotes cursor on closing after escape", input: "x \"a\\\"b\" y\n", keys: "f\";;di\"", want: replayState{lines: []string{"x \"\" y"}, col: 3, reg: []string{"a\\\"b"}, modified: true}},
		{name: "quotes escaped backslash closes", input: "x \"a\\\\\" y\n", keys: "fadi\"", want: replayState{lines: []string{"x \"\" y"}, col: 3, reg: []string{"a\\\\"}, modified: true}},
		{name: "quotes unmatched fails", input: "x \"a b\n", keys: "fadi\"", want: replayState{lines: []string{"x \"a b"}, col: 3}},
		{name: "quotes unmatched around fails", input: "x \"a b\n", keys: "fada\"", want: replayState{lines: []string{"x \"a b"}, col: 3}},
		{name: "quotes single quote line only", input: "\"a\n\"b\"\n", keys: "jfbdi\"", want: replayState{lines: []string{"\"a", "\"\""}, row: 1, col: 1, reg: []string{"b"}, modified: true}},
		{name: "quotes are line local", input: "x \"a\nb\" y\n", keys: "di\"", want: replayState{lines: []string{"x \"a", "b\" y"}}},
		{name: "quotes inner spaces kept", input: "x \"a  \" y\n", keys: "fadi\"", want: replayState{lines: []string{"x \"\" y"}, col: 3, reg: []string{"a  "}, modified: true}},
		{name: "quotes around trailing spaces", input: "x \"a  \" y\n", keys: "fada\"", want: replayState{lines: []string{"x y"}, col: 2, reg: []string{"\"a  \" "}, modified: true}},
		{name: "single quotes inner", input: "x 'a b' y\n", keys: "fadi'", want: replayState{lines: []string{"x '' y"}, col: 3, reg: []string{"a b"}, modified: true}},
		{name: "single quotes around", input: "x 'a b' y\n", keys: "fada'", want: replayState{lines: []string{"x y"}, col: 2, reg: []string{"'a b' "}, modified: true}},
		{name: "single quotes count 2", input: "x 'a b' y\n", keys: "fad2i'", want: replayState{lines: []string{"x  y"}, col: 2, reg: []string{"'a b'"}, modified: true}},
		{name: "backtick inner", input: "x `a b` y\n", keys: "fadi`", want: replayState{lines: []string{"x `` y"}, col: 3, reg: []string{"a b"}, modified: true}},
		{name: "backtick around", input: "x `a b` y\n", keys: "fada`", want: replayState{lines: []string{"x y"}, col: 2, reg: []string{"`a b` "}, modified: true}},
		{name: "quotes of another kind are text", input: "x \"a 'b' c\" y\n", keys: "fbdi\"", want: replayState{lines: []string{"x \"\" y"}, col: 3, reg: []string{"a 'b' c"}, modified: true}},
		{name: "quotes of another kind single", input: "x \"a 'b' c\" y\n", keys: "fbdi'", want: replayState{lines: []string{"x \"a '' c\" y"}, col: 6, reg: []string{"b"}, modified: true}},
		{name: "quotes yank inner", input: "say \"hi\" now\n", keys: "fhyi\"", want: replayState{lines: []string{"say \"hi\" now"}, col: 5, reg: []string{"hi"}}},
		{name: "quotes yank around", input: "say \"hi\" now\n", keys: "fhya\"", want: replayState{lines: []string{"say \"hi\" now"}, col: 4, reg: []string{"\"hi\" "}}},
		{name: "quotes change inner", input: "say \"hi\" now\n", keys: "fhci\"X<Esc>", want: replayState{lines: []string{"say \"X\" now"}, col: 5, reg: []string{"hi"}, modified: true}},
		{name: "quotes change around", input: "say \"hi\" now\n", keys: "fhca\"X<Esc>", want: replayState{lines: []string{"say Xnow"}, col: 4, reg: []string{"\"hi\" "}, modified: true}},
		{name: "quotes shift", input: "say \"hi\" now\n", keys: "fh>i\"", want: replayState{lines: []string{"\tsay \"hi\" now"}, col: 1, modified: true}},
		{name: "quotes undo", input: "say \"hi\" now\n", keys: "fhdi\"u", want: replayState{lines: []string{"say \"hi\" now"}, col: 5, reg: []string{"hi"}}},
		{name: "quotes redo", input: "say \"hi\" now\n", keys: "fhdi\"u<C-r>", want: replayState{lines: []string{"say \"\" now"}, col: 5, reg: []string{"hi"}, modified: true}},
		{name: "quotes dot after ci", input: "\"a\" \"b\"\n", keys: "ci\"X<Esc>fb.", want: replayState{lines: []string{"\"X\" \"X\""}, col: 5, reg: []string{"b"}, modified: true}},
		{name: "quotes dot after da", input: "\"a\" \"b\" \"c\"\n", keys: "da\".", want: replayState{lines: []string{"\"c\""}, reg: []string{"\"b\" "}, modified: true}},
		{name: "quotes visual inner", input: "x \"a b\" y\n", keys: "favi\"d", want: replayState{lines: []string{"x \"\" y"}, col: 3, reg: []string{"a b"}, modified: true}},
		{name: "quotes visual inner yank", input: "x \"a b\" y\n", keys: "favi\"y", want: replayState{lines: []string{"x \"a b\" y"}, col: 3, reg: []string{"a b"}}},
		{name: "quotes visual repeated includes quotes", input: "x \"a b\" y\n", keys: "favi\"i\"y", want: replayState{lines: []string{"x \"a b\" y"}, col: 2, reg: []string{"\"a b\""}}},
		{name: "quotes visual around", input: "x \"a b\" y\n", keys: "fava\"y", want: replayState{lines: []string{"x \"a b\" y"}, col: 2, reg: []string{"\"a b\" "}}},
		{name: "quotes visual inner then around", input: "x \"a b\" y\n", keys: "favi\"a\"y", want: replayState{lines: []string{"x \"a b\" y"}, col: 2, reg: []string{"\"a b\" "}}},
		{name: "quotes visual count 2 inner", input: "x \"a b\" y\n", keys: "fav2i\"y", want: replayState{lines: []string{"x \"a b\" y"}, col: 2, reg: []string{"\"a b\""}}},
		{name: "quotes visual count 2 around", input: "x \"a b\" y\n", keys: "fav2a\"y", want: replayState{lines: []string{"x \"a b\" y"}, col: 2, reg: []string{"\"a b\" "}}},
		{name: "quotes visual on opening quote", input: "x \"a b\" y\n", keys: "f\"vi\"y", want: replayState{lines: []string{"x \"a b\" y"}, col: 3, reg: []string{"a b"}}},
		{name: "quotes visual leading space", input: "x \"a b\" \"c\"\n", keys: "fcva\"y", want: replayState{lines: []string{"x \"a b\" \"c\""}, col: 7, reg: []string{" \"c\""}}},
		{name: "quotes visual between strings", input: "x \"a b\" \"c\"\n", keys: "favi\"i\"y", want: replayState{lines: []string{"x \"a b\" \"c\""}, col: 2, reg: []string{"\"a b\""}}},
		{name: "quotes visual single quotes", input: "x 'a b' y\n", keys: "fav2i'y", want: replayState{lines: []string{"x 'a b' y"}, col: 2, reg: []string{"'a b'"}}},
		{name: "quotes visual backtick around", input: "x `a b` y\n", keys: "fava`y", want: replayState{lines: []string{"x `a b` y"}, col: 2, reg: []string{"`a b` "}}},
		{name: "quotes count 2 yank", input: "a \"b c\" d\n", keys: "fcy2i\"", want: replayState{lines: []string{"a \"b c\" d"}, col: 2, reg: []string{"\"b c\""}}},
		{name: "quotes change around at end", input: "a \"b\"\n", keys: "fbca\"X<Esc>", want: replayState{lines: []string{"aX"}, col: 1, reg: []string{" \"b\""}, modified: true}},
		// --- hand-written ---
		{name: "quotes with multibyte text", input: "é \"λ μ\" ü\n", keys: "fμda\"", want: replayState{lines: []string{"é ü"}, col: 2, reg: []string{"\"λ μ\" "}, modified: true}},
		{name: "quotes outside any string fail", input: "say hi\n", keys: "di\"", want: replayState{lines: []string{"say hi"}}},
		{name: "quotes failure keeps the register", input: "abc \"d\" e\n", keys: "yiw$di\"", want: replayState{lines: []string{"abc \"d\" e"}, col: 8, reg: []string{"abc"}}},
		{name: "quotes named register", input: "say \"hi\" now\n", keys: "fh\"adi\"\"ap", want: replayState{lines: []string{"say \"\"hi now"}, col: 7, reg: []string{"hi"}, modified: true}},
		{name: "quotes visual outside string keeps selection", input: "say hi\n", keys: "vi\"y", want: replayState{lines: []string{"say hi"}, reg: []string{"s"}}},
	} {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}

// TestPairObjectFilterH05 checks that !i( routes the object's lines to the
// filter prompt without creating an edit.
func TestPairObjectFilterH05(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "filter.txt")
	if err := os.WriteFile(path, []byte("x (\n  foo\n  bar\n)\ny\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e := New(path, dir)
	e.Resize(80, 24)
	for _, key := range replayKeys(t, "j!i(") {
		e.Update(key)
	}
	if e.mode != ModeCommand || e.cmdBuf != "2,3!" {
		t.Fatalf("mode=%v cmdBuf=%q", e.mode, e.cmdBuf)
	}
	if e.modified || len(e.undoStack) != 0 {
		t.Fatalf("filter created an edit: modified=%v undo=%d", e.modified, len(e.undoStack))
	}
}

// TestDelimiterObjectUndoUnitH05 checks that a change takes one undo step
// however much was typed.
func TestDelimiterObjectUndoUnitH05(t *testing.T) {
	for _, keys := range []string{"fbci(XYZ<Esc>", "fbca(XYZ<Esc>", "fhci\"XYZ<Esc>"} {
		e := New("", t.TempDir())
		e.Resize(80, 24)
		e.lines = []string{"a (b c) d \"hi\""}
		for _, key := range replayKeys(t, keys+"u") {
			e.Update(key)
		}
		if got := e.lines[0]; got != "a (b c) d \"hi\"" {
			t.Fatalf("%s then u left %q", keys, got)
		}
	}
}
