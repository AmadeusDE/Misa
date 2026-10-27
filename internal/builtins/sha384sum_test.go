// Behaviour adapted from toybox md5sum.c (0BSD) and uutils coreutils sha384sum (MIT).
package builtins

import "testing"

func TestSha384sum(t *testing.T) {
	const digest = "cb00753f45a35e8bb5a03d699ac65007272c32ab0eded1631a8b605a43ff5bed8086072ba1e7cc2358baeca134c825a7"
	r := runBuiltinStdin(t, "sha384sum", "abc")
	assertExitCode(t, r, 0)
	assertStdout(t, r, digest+"  -\n")
	dir := t.TempDir()
	writeFile(t, dir, "input", "abc")
	writeFile(t, dir, "check", digest+"  input\n")
	r = runBuiltinIn(t, dir, "sha384sum", "--check", "check")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "input: OK\n")
}
