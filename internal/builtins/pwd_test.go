package builtins

import "testing"

func TestPwd(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "pwd")
	assertExitCode(t, r, 0)
	assertStdout(t, r, dir+"\n")
}
