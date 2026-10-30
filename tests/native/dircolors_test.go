package native

import "testing"

func TestDircolors_BourneDefault(t *testing.T) {
	r := run(t, "dircolors", "-b")
	wantSuccess(t, r)
	wantOutContains(t, r, "LS_COLORS='rs=0:di=01;34:")
	wantOutContains(t, r, "';\nexport LS_COLORS\n")
}

func TestDircolors_CshDefault(t *testing.T) {
	r := run(t, "dircolors", "-c")
	wantSuccess(t, r)
	wantOutContains(t, r, "setenv LS_COLORS 'rs=0:di=01;34:")
}

func TestDircolors_PrintDatabase(t *testing.T) {
	r := run(t, "dircolors", "-p")
	wantSuccess(t, r)
	wantOutContains(t, r, "# Configuration file for dircolors")
	wantOutContains(t, r, "TERM screen*")
}

func TestDircolors_Stdin(t *testing.T) {
	r := runStdin(t, "dircolors", "owt 40;33\n", "-b", "-")
	wantSuccess(t, r)
	wantOut(t, r, "LS_COLORS='tw=40;33:';\nexport LS_COLORS\n")
}

func TestDircolors_MissingFile(t *testing.T) {
	r := run(t, "dircolors", "-b", "no-such-file")
	wantCode(t, r, 1)
	wantErrContains(t, r, "dircolors: no-such-file:")
}

func TestDircolors_ExtraOperand(t *testing.T) {
	r := run(t, "dircolors", "-c", "a", "b")
	wantCode(t, r, 1)
	wantErrContains(t, r, "extra operand 'b'")
}

func TestDircolors_ExclusiveOptions(t *testing.T) {
	r := run(t, "dircolors", "-bp")
	wantCode(t, r, 1)
	wantErrContains(t, r, "mutually exclusive")
}
