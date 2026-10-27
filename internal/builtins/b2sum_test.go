// Behaviour adapted from uutils coreutils b2sum (MIT).
package builtins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestB2sum(t *testing.T) {
	const empty = "786a02f742015903c6c6fd852552d272912f4740e15847618a86e217f71f5419d25e1031afee585313896444934eb04b903a685b1448b755d56f701afe9be2ce"
	for _, tc := range []struct {
		args []string
		want string
	}{
		{nil, empty + "  -\n"},
		{[]string{"-l0", "--tag"}, "BLAKE2b (-) = " + empty + "\n"},
		{[]string{"-l512", "--tag"}, "BLAKE2b (-) = " + empty + "\n"},
		{[]string{"-l128", "--tag"}, "BLAKE2b-128 (-) = cae66941d9efbd404e4d88758ea67670\n"},
		{[]string{"-l123", "-l128", "-b", "-z"}, "cae66941d9efbd404e4d88758ea67670 *-\x00"},
	} {
		r := runBuiltinStdin(t, "b2sum", "", tc.args...)
		assertExitCode(t, r, 0)
		assertStdout(t, r, tc.want)
	}
	for _, length := range []string{"-1", "7", "513", "18446744073709552000", "abc"} {
		r := runBuiltin(t, "b2sum", "--length", length)
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, "b2sum: invalid length: '"+length+"'")
	}
	dir := t.TempDir()
	writeFile(t, dir, "f", "")
	for _, line := range []string{empty + "  f\n", "cae66941d9efbd404e4d88758ea67670  f\n", "BLAKE2b-128 (f) = cae66941d9efbd404e4d88758ea67670\n", "2e  f\ne4a6a0577479b2b4  f\n"} {
		writeFile(t, dir, "check", line)
		r := runBuiltinIn(t, dir, "b2sum", "-c", "check")
		assertExitCode(t, r, 0)
		assertStdoutContains(t, r, "f: OK\n")
		r = runBuiltinIn(t, dir, "b2sum", "--status", "-c", "check")
		assertExitCode(t, r, 0)
		assertStdout(t, r, "")
	}
	writeFile(t, dir, "check", "00  f\n")
	r := runBuiltinIn(t, dir, "b2sum", "-c", "check")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "f: FAILED\n")
	r = runBuiltinStdin(t, "b2sum", "garbage\n", "-c", "--status")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "'standard input': no properly formatted checksum lines found")
}

func TestB2sumFixtures(t *testing.T) {
	dir := "../../tests/uutils/testdata/tests/fixtures/b2sum"
	expected, err := os.ReadFile(filepath.Join(dir, "b2sum.expected"))
	if err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "b2sum", "input.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, strings.TrimSpace(string(expected))+"  input.txt\n")
	r = runBuiltinIn(t, dir, "b2sum", "-c", "b2sum.checkfile")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "input.txt: OK\n")
}

func TestB2sumStrictCheck(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f", "")
	checksums := []string{
		"2e  f\n",
		"e4a6a0577479b2b4  f\n",
		"cae66941d9efbd404e4d88758ea67670  f\n",
		"246c0442cd564aced8145b8b60f1370aa7  f\n",
		"0e5751c026e543b2e8ab2eb06099daa1d1e5df47778f7787faab45cdf12fe3a8  f\n",
		"4ded8c5fc8b12f3273f877ca585a44ad6503249a2b345d6d9c0e67d85bcb700db4178c0303e93b8f4ad758b8e2c9fd8b3d0c28e585f1928334bb77d36782e8  f\n",
		"786a02f742015903c6c6fd852552d272912f4740e15847618a86e217f71f5419d25e1031afee585313896444934eb04b903a685b1448b755d56f701afe9be2ce  f\n",
	}
	writeFile(t, dir, "check", strings.Join(checksums, ""))
	r := runBuiltinIn(t, dir, "b2sum", "--strict", "-c", "check")
	assertExitCode(t, r, 0)
	assertStdout(t, r, strings.Repeat("f: OK\n", len(checksums)))
	writeFile(t, dir, "check", "bad line\n"+checksums[0])
	r = runBuiltinIn(t, dir, "b2sum", "--strict", "-c", "check")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "f: OK\n")
	r = runBuiltinStdin(t, "b2sum", "foobar\n", "--length=123", "--length=128")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "d6d45901dec53e65d2b55fb6e2ab67b0  -\n")
	r = runBuiltinIn(t, dir, "b2sum", "missing", "f")
	assertExitCode(t, r, 1)
	assertStdoutContains(t, r, "  f\n")
	assertStderrContains(t, r, "b2sum: missing: No such file or directory\n")
}
