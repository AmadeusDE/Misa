package native

import (
	"os"
	"testing"
)

func TestEnv_Basic(t *testing.T) {
	r := run(t, "env")
	wantSuccess(t, r)
}

func TestEnv_IgnoreEnv(t *testing.T) {
	r := run(t, "env", "-i")
	wantSuccess(t, r)
	wantOut(t, r, "")
}

func TestEnv_SetVars(t *testing.T) {
	r := run(t, "env", "-i", "FOO=bar", "BAZ=qux")
	wantSuccess(t, r)
	wantOut(t, r, "BAZ=qux\nFOO=bar\n")
}

func TestEnv_UnsetVars(t *testing.T) {
	r2 := run(t, "env", "-i", "A=1", "B=2", "C=3", "-u", "B")
	wantSuccess(t, r2)
	wantOut(t, r2, "A=1\nC=3\n")
}

func TestEnv_Subcommand(t *testing.T) {
	r := run(t, "env", "-i", "echo", "hello", "world")
	wantSuccess(t, r)
	wantOut(t, r, "hello world\n")
}

func TestEnv_NullDelimiter(t *testing.T) {
	r := run(t, "env", "-i", "-0", "FOO=bar", "BAZ=qux")
	wantSuccess(t, r)
	wantOut(t, r, "BAZ=qux\x00FOO=bar\x00")
}

func TestEnv_EmptyNameWarnsAndIsDropped(t *testing.T) {
	r := run(t, "env", "-i", "=value")
	wantSuccess(t, r)
	wantOut(t, r, "")
	wantErrContains(t, r, "no name specified for value 'value'")
}

func TestEnv_NonExecutableCommandIsPermissionDenied(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "empty", "")
	r := runIn(t, dir, "env", "./empty")
	wantCode(t, r, 126)
	wantErrContains(t, r, "Permission denied")
}

func TestEnv_Help(t *testing.T) {
	r := run(t, "env", "--help")
	wantSuccess(t, r)
	wantOutContains(t, r, "Options:")
}

func TestEnv_BlockSignalOption(t *testing.T) {
	r := run(t, "env", "--block-signal", "TERM", "true")
	wantSuccess(t, r)
}

func TestEnv_ChangeDirectory(t *testing.T) {
	dir := t.TempDir()
	child := dir + "/child"
	if err := os.Mkdir(child, 0755); err != nil {
		t.Fatal(err)
	}
	r := runIn(t, dir, "env", "-C", "child", "pwd")
	wantSuccess(t, r)
	wantOut(t, r, child+"\n")
}

func TestEnv_LeadingEqualsVariable(t *testing.T) {
	r := run(t, "env", "-i", "=BLAH", "env")
	wantSuccess(t, r)
	wantOut(t, r, "")
	wantErrContains(t, r, "no name specified for value 'BLAH'")
}

func TestEnv_SplitString(t *testing.T) {
	r := run(t, "env", "-i", "VAR=hello", "-S", "echo ${VAR}")
	wantSuccess(t, r)
	wantOut(t, r, "hello\n")
}

func TestEnv_SplitStringUnterminatedQuote(t *testing.T) {
	r := run(t, "env", "-S", "echo 'unterminated")
	wantCode(t, r, 125)
	wantErrContains(t, r, "no terminating quote in -S string at position 18 for quote '''")
}
