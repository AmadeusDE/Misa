package native

import "testing"

func TestExpand_SingleTab(t *testing.T) {
	r := runStdin(t, "expand", "\thello\n")
	wantSuccess(t, r)
	wantOut(t, r, "        hello\n")
}

func TestExpand_MidLineTab(t *testing.T) {
	r := runStdin(t, "expand", "ab\tcd\n")
	wantSuccess(t, r)
	wantOut(t, r, "ab      cd\n")
}

func TestExpand_CustomTab4(t *testing.T) {
	r := runStdin(t, "expand", "\thello\n", "-t", "4")
	wantSuccess(t, r)
	wantOut(t, r, "    hello\n")
}

func TestExpand_InitialOnly(t *testing.T) {
	r := runStdin(t, "expand", "\ta\tb\n", "-i")
	wantSuccess(t, r)
	wantOut(t, r, "        a\tb\n")
}

func TestExpand_MultipleTabStops(t *testing.T) {
	r := runStdin(t, "expand", "\t\thello\n", "-t", "4,8")
	wantSuccess(t, r)
	wantOutContains(t, r, "hello")
}

func TestExpand_NoTabs(t *testing.T) {
	r := runStdin(t, "expand", "hello world\n")
	wantSuccess(t, r)
	wantOut(t, r, "hello world\n")
}

func TestExpand_Empty(t *testing.T) {
	r := runStdin(t, "expand", "")
	wantSuccess(t, r)
	wantOut(t, r, "")
}

func TestExpand_File(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "\ttab\n")
	r := runIn(t, dir, "expand", "f.txt")
	wantSuccess(t, r)
	wantOut(t, r, "        tab\n")
}

func TestUnexpand_Basic(t *testing.T) {
	r := runStdin(t, "unexpand", "        hello\n", "-a")
	wantSuccess(t, r)
	wantOut(t, r, "\thello\n")
}

func TestUnexpand_TabStopsShortcut(t *testing.T) {
	r := runStdin(t, "unexpand", "a  b     c\n", "-a", "-3,9")
	wantSuccess(t, r)
	wantOut(t, r, "a\tb\tc\n")
}

func TestUnexpand_TabStopsRepeatPlus(t *testing.T) {
	r := runStdin(t, "unexpand", "a  b     c\n", "-a", "-t", "3,+6")
	wantSuccess(t, r)
	wantOut(t, r, "a\tb\tc\n")
}

func TestExpand_MultibyteWidth(t *testing.T) {
	// '語' has width 2, so tab at column 2 should expand to (8 - 2) = 6 spaces.
	r := runStdin(t, "expand", "語\t\n")
	wantSuccess(t, r)
	wantOut(t, r, "語      \n")
}
