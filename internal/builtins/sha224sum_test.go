// Behaviour adapted from toybox md5sum.c (0BSD) and uutils coreutils sha224sum (MIT).
package builtins

import (
	"fmt"
	"strings"
	"testing"
)

func TestSha224sum(t *testing.T) {
	const digest = "23097d223405d8228642a477bda255b32aadbce4bda0b3f7e36c9da7"
	r := runBuiltinStdin(t, "sha224sum", "abc")
	assertExitCode(t, r, 0)
	assertStdout(t, r, digest+"  -\n")
	dir := t.TempDir()
	writeFile(t, dir, "input", "abc")
	for _, tc := range []struct {
		line   string
		code   int
		output string
	}{{digest + "  input\n", 0, "input: OK\n"}, {digest + " *input\n", 0, "input: OK\n"}, {"0000  input\n", 1, "input: FAILED\n"}} {
		writeFile(t, dir, "check", tc.line)
		r = runBuiltinIn(t, dir, "sha224sum", "-c", "check")
		assertExitCode(t, r, tc.code)
		assertStdout(t, r, tc.output)
	}
}

func TestSha224sumToyboxShellCases(t *testing.T) {
	var sequence strings.Builder
	for i := 1; i <= 10000; i++ {
		fmt.Fprintln(&sequence, i)
	}
	for _, tc := range []struct {
		util    string
		digests [3]string
	}{
		{"sha224sum", [3]string{"20794655980c91d8bbb4c1ea97618a4bf03f42581948b2ee4ee7ad67", "567f69f168cd7844e65259ce658fe7aadfa25216e68eca0eb7ab8262", "d8b406f661bc690a1f58241f37c94e04d0a1af74af867b877778f18e"}},
		{"sha384sum", [3]string{"9d0e1809716474cb086e834e310a4a1ced149e9c00f248527972cec5704c2a5b07b8b3dc38ecc4ebae97ddd87f3d8985", "2fc64a4f500ddb6828f6a3430b8dd72a368eb7f3a8322a70bc84275b9c0b3ab00d27a5cc3c2d224aa6b61a0d79fb4596", "7d2a49098f0df0f3c152ca9916a3864542258b2bd487e00ea33cb68e7d27c5c0f25b540d29f62fb33720846073c51b66"}},
		{"sha3sum", [3]string{"d69335b93325192e516a912e6d19a15cb51c6ed5c15243e7a7fd653c", "c97e9b948bcd69bbc0e76b39fc82df963ae61665ee12099b6631ffe6", "7049c446c0fb287872536a7737f61f9ac7100f8e6b421d11011505fb"}},
	} {
		for i, input := range []string{strings.Repeat("a", 1000000), strings.Repeat("01234567", 80), sequence.String()} {
			t.Run(tc.util+"/"+[]string{"a_million_nulls", "digits", "seq_10000"}[i], func(t *testing.T) {
				r := runBuiltinStdin(t, tc.util, input)
				assertExitCode(t, r, 0)
				assertStdout(t, r, tc.digests[i]+"  -\n")
			})
		}
	}
}
