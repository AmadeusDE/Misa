// Behaviour adapted from toybox sha3sum.c (0BSD).
package builtins

import "testing"

func TestSha3sum(t *testing.T) {
	for _, tc := range []struct{ bits, digest string }{
		{"224", "e642824c3f8cf24ad09234ee7d3c766fc9a3a5168d0c94ad73b46fdf"},
		{"256", "3a985da74fe225b2045c172d6bd390bd855f086e3e9d525b46bfe24511431532"},
		{"384", "ec01498288516fc926459f58e2c6ad8df9b473cb0fc08c2596da7cf0e49be4b298d88cea927ac7f539f1edf228376d25"},
		{"512", "b751850b1a57168a5693cd924b6b096e08f621827444f70d884f5d0240d2712e10e116e9192af3c91a7ec57647e3934057340b4cf408d5a56592f8274eec53f0"},
	} {
		t.Run(tc.bits, func(t *testing.T) {
			r := runBuiltinStdin(t, "sha3sum", "abc", "-b", "-a", tc.bits)
			assertExitCode(t, r, 0)
			assertStdout(t, r, tc.digest+"\n")
		})
	}
	for _, tc := range []struct{ bits, digest string }{{"128", "5881092dd818bf5cf8a3ddb793fbcba7"}, {"256", "483366601360a8771c6863080cc4114d8db44530f8f1e1ee4f94ea37e78b5739"}} {
		r := runBuiltinStdin(t, "sha3sum", "abc", "-bS", "-a", tc.bits)
		assertExitCode(t, r, 0)
		assertStdout(t, r, tc.digest+"\n")
	}
	r := runBuiltinStdin(t, "sha3sum", "abc", "-a", "123")
	assertExitCode(t, r, 1)
	dir := t.TempDir()
	writeFile(t, dir, "f", "abc")
	r = runBuiltinIn(t, dir, "sha3sum", "missing", "f")
	assertExitCode(t, r, 1)
	assertStdoutContains(t, r, "  f\n")
}
