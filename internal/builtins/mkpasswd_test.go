package builtins

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/pwhash"
)

// The toybox tests/mkpasswd.test vectors, for the password "password".
const (
	mkpasswdMD5Vector       = "$1$abcdefgh$G//4keteveJp0qb8z2DxG/"
	mkpasswdSHA256Vector    = "$5$abcdefgh$ZLdkj8mkc2XVSrPVjskDAgZPGjtj1VGVaa1aUkrMTU/"
	mkpasswdSHA256LongSalt  = "$5$./Aa0Bb1Cc2Dd3Ee$5iXcesTggTRGvAAa3cWlpxmUqNGOeQh/iO3Furo4y/D"
	mkpasswdSHA512Vector    = "$6$abcdefgh$yVfUwsw5T.JApa8POvClA1pQ5peiq97DUNyXCZN5IrF.BMSkiaLQ5kvpuEm/VQ1Tvh/KV2TcaWh8qinoW5dhA1"
	mkpasswdSHA512LongSalt  = "$6$./Aa0Bb1Cc2Dd3Ee$PvmedaPf329sM25Jn2jv3MsfK9DaDh6tyVtJucp35A/Lmrtp9g1Ab35Mr59pkuMU3QJlbXYoWJFaxyD4OwIZ60"
	mkpasswdEmptyPassSHA512 = "$6$abcdefgh$v7sYNA18/BerGOYQLppYLyjH4yJilp8kqe/ef3KYMK9hOIdzH1yzcmP74Ay.m51y1jP3QqxM7Jl75S4CxDhBq."
)

// mkpasswdRun runs the builtin from a hand-built Context, so a test can pass a
// nil Stdin, a custom reader or a CmdName.
func mkpasswdRun(t *testing.T, stdin io.Reader, args ...string) runResult {
	t.Helper()
	cmd, ok := Lookup("mkpasswd")
	if !ok {
		t.Fatalf("builtin %q not registered", "mkpasswd")
	}
	var stdout, stderr bytes.Buffer
	ctx := &Context{
		Dir:     t.TempDir(),
		Args:    args,
		CmdName: "mkpasswd",
		Width:   80,
		Height:  24,
		Stdout:  &stdout,
		Stderr:  &stderr,
		Stdin:   stdin,
	}
	code := cmd.Run(ctx)
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// mkpasswdWantHash asserts that r printed exactly want and nothing else.
func mkpasswdWantHash(t *testing.T, r runResult, want string) {
	t.Helper()
	if r.code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", r.code, r.stderr)
	}
	if r.stdout != want+"\n" {
		t.Errorf("stdout = %q, want %q", r.stdout, want+"\n")
	}
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
}

func TestMkpasswd_ToyboxVectors(t *testing.T) {
	tests := []struct {
		name   string
		method string
		salt   string
		want   string
	}{
		{"md5", "md5", "abcdefgh", mkpasswdMD5Vector},
		{"sha256", "sha-256", "abcdefgh", mkpasswdSHA256Vector},
		{"sha256-long-salt", "sha-256", "./Aa0Bb1Cc2Dd3Ee", mkpasswdSHA256LongSalt},
		{"sha512", "sha-512", "abcdefgh", mkpasswdSHA512Vector},
		{"sha512-long-salt", "sha-512", "./Aa0Bb1Cc2Dd3Ee", mkpasswdSHA512LongSalt},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := runBuiltinStdin(t, "mkpasswd", "password",
				"-P0", "-m", tc.method, "-S", tc.salt)
			mkpasswdWantHash(t, r, tc.want)
		})
	}
}

func TestMkpasswd_DefaultMethodIsSHA512(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"flag-salt", []string{"-S", "abcdefgh", "password"}},
		{"operand-salt", []string{"password", "abcdefgh"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mkpasswdWantHash(t, runBuiltin(t, "mkpasswd", tc.args...), mkpasswdSHA512Vector)
		})
	}
}

func TestMkpasswd_MethodAliases(t *testing.T) {
	tests := []struct {
		method string
		want   string
	}{
		{"sha-256", mkpasswdSHA256Vector},
		{"SHA-256", mkpasswdSHA256Vector},
		{"sha_256", mkpasswdSHA256Vector},
		{"sha256", mkpasswdSHA256Vector},
		{"md5", mkpasswdMD5Vector},
		{"MD5", mkpasswdMD5Vector},
	}
	for _, tc := range tests {
		t.Run(tc.method, func(t *testing.T) {
			r := runBuiltin(t, "mkpasswd", "-m", tc.method, "-S", "abcdefgh", "password")
			mkpasswdWantHash(t, r, tc.want)
		})
	}
}

func TestMkpasswd_RoundsSalt(t *testing.T) {
	tests := []struct {
		name   string
		method string
		salt   string
		want   string
	}{
		{
			"sha256",
			"sha256",
			"rounds=1000$abcdefgh",
			"$5$rounds=1000$abcdefgh$GEazsRB0Tipk2UJZtYJM/eHHRMtwea4Vnto3YxMbrG0",
		},
		{
			"sha512",
			"sha512",
			"rounds=10000$abcdefgh",
			"$6$rounds=10000$abcdefgh$kF57o18Xuu8qWSW2h0YBILpIeW/KEpcSse9BHaTTStgl9j7nCGzty1xRBjmP7wDA01Idc.ivBCoIRvlUbkXmQ/",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := runBuiltin(t, "mkpasswd", "-m", tc.method, "-S", tc.salt, "password")
			mkpasswdWantHash(t, r, tc.want)
		})
	}
}

func TestMkpasswd_DESUnsupported(t *testing.T) {
	for _, method := range []string{"des", "DES", "d-es"} {
		t.Run(method, func(t *testing.T) {
			r := runBuiltin(t, "mkpasswd", "-m", method, "password")
			assertExitCode(t, r, 1)
			assertStdout(t, r, "")
			assertStderrContains(t, r, "unsupported")
			assertStderrContains(t, r, "DES crypt is not implemented")
		})
	}
	r := runBuiltin(t, "mkpasswd", "-m", "des", "password")
	assertStderrContains(t, r, "mkpasswd: unsupported hash method 'des'")
}

func TestMkpasswd_UnknownMethod(t *testing.T) {
	r := runBuiltin(t, "mkpasswd", "-m", "bcrypt", "password")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "unsupported hash method 'bcrypt'")
	assertStderrContains(t, r, "md5, sha256, sha512")
}

func TestMkpasswd_MethodHelp(t *testing.T) {
	r := runBuiltin(t, "mkpasswd", "-m", "help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "md5")
	assertStdoutContains(t, r, "sha256")
	assertStdoutContains(t, r, "sha512")
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
}

func TestMkpasswd_Help(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		t.Run(flag, func(t *testing.T) {
			r := runBuiltin(t, "mkpasswd", flag)
			assertExitCode(t, r, 0)
			assertStdoutContains(t, r, "usage: mkpasswd")
			assertStdoutContains(t, r, "md5")
			assertStdoutContains(t, r, "sha256")
			assertStdoutContains(t, r, "sha512")
			assertStdoutContains(t, r, "default")
			assertStdoutContains(t, r, "DES crypt")
		})
	}
}

func TestMkpasswd_BadSalt(t *testing.T) {
	tests := []struct {
		name   string
		method string
		salt   string
	}{
		{"dollar", "sha512", "ab$cd"},
		{"empty", "sha512", ""},
		{"too-long", "sha256", "abcdefghijklmnopq"},
		{"bad-rounds", "sha256", "rounds=abc$abcdefgh"},
		{"space", "sha512", "abc defgh"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := runBuiltin(t, "mkpasswd", "-m", tc.method, "-S", tc.salt, "password")
			assertExitCode(t, r, 1)
			assertStdout(t, r, "")
			assertStderrContains(t, r, "mkpasswd: bad SALT: ")
			if n := strings.Count(r.stderr, "\n"); n != 1 {
				t.Errorf("stderr = %q, want a single line of output", r.stderr)
			}
		})
	}
}

func TestMkpasswd_SaltLengthLimits(t *testing.T) {
	// 8 characters is the md5 maximum: one more must fail rather than be
	// truncated.
	r := runBuiltin(t, "mkpasswd", "-m", "md5", "-S", "abcdefghi", "password")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "mkpasswd: bad SALT: ")

	r = runBuiltin(t, "mkpasswd", "-m", "md5", "-S", "abcdefgh", "password")
	mkpasswdWantHash(t, r, mkpasswdMD5Vector)

	// The SHA methods accept 16 and reject 17.
	r = runBuiltin(t, "mkpasswd", "-m", "sha512", "-S", "./Aa0Bb1Cc2Dd3Ee", "password")
	mkpasswdWantHash(t, r, mkpasswdSHA512LongSalt)
}

func TestMkpasswd_DuplicateSalt(t *testing.T) {
	r := runBuiltin(t, "mkpasswd", "-S", "abc", "password", "def")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "mkpasswd: duplicate salt")
}

func TestMkpasswd_TooManyArguments(t *testing.T) {
	r := runBuiltin(t, "mkpasswd", "a", "b", "c")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "mkpasswd: too many arguments")
}

func TestMkpasswd_ParseError(t *testing.T) {
	r := runBuiltin(t, "mkpasswd", "-z")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "mkpasswd: unknown option -z")
}

func TestMkpasswd_StdinLineEndings(t *testing.T) {
	tests := []struct {
		name  string
		stdin string
	}{
		{"no-newline", "password"},
		{"newline", "password\n"},
		{"crlf", "password\r\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := runBuiltinStdin(t, "mkpasswd", tc.stdin, "-S", "abcdefgh")
			mkpasswdWantHash(t, r, mkpasswdSHA512Vector)
		})
	}
}

func TestMkpasswd_EmptyStdin(t *testing.T) {
	r := runBuiltinStdin(t, "mkpasswd", "", "-P0", "-S", "abcdefgh")
	mkpasswdWantHash(t, r, mkpasswdEmptyPassSHA512)
}

func TestMkpasswd_NilStdin(t *testing.T) {
	r := mkpasswdRun(t, nil, "-S", "abcdefgh")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "mkpasswd: password read failed")
}

func TestMkpasswd_PasswordFromFD(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pw")
	if err := os.WriteFile(path, []byte("password"), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	// The builtin closes the descriptor it reads from, so a second close here
	// may fail with EBADF; that is expected.
	t.Cleanup(func() { _ = f.Close() })

	r := runBuiltin(t, "mkpasswd", "-P", strconv.FormatUint(uint64(f.Fd()), 10), "-S", "abcdefgh")
	mkpasswdWantHash(t, r, mkpasswdSHA512Vector)
}

func TestMkpasswd_ExplicitStdinFD(t *testing.T) {
	r := runBuiltinStdin(t, "mkpasswd", "password\n", "-P0", "-S", "abcdefgh")
	mkpasswdWantHash(t, r, mkpasswdSHA512Vector)
}

func TestMkpasswd_ClosedFD(t *testing.T) {
	// Descriptor 99 is not open in this process: reading from it must fail
	// rather than hash an empty password.
	r := runBuiltin(t, "mkpasswd", "-P", "99", "-S", "abcdefgh")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "mkpasswd: password read failed")
}

func TestMkpasswd_RandomSalt(t *testing.T) {
	old := mkpasswdRand
	mkpasswdRand = bytes.NewReader(bytes.Repeat([]byte{0}, 64))
	t.Cleanup(func() { mkpasswdRand = old })

	r := runBuiltin(t, "mkpasswd", "password")
	if r.code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", r.code, r.stderr)
	}
	line := strings.TrimSuffix(r.stdout, "\n")
	if !strings.HasPrefix(line, "$6$") {
		t.Fatalf("stdout = %q, want a $6$ (sha512) hash", r.stdout)
	}
	salt, hash, ok := strings.Cut(strings.TrimPrefix(line, "$6$"), "$")
	if !ok {
		t.Fatalf("stdout = %q, want a $6$SALT$HASH setting", r.stdout)
	}
	if len(salt) != pwhash.MaxSaltLen(pwhash.MethodSHA512) {
		t.Errorf("salt %q has length %d, want %d", salt, len(salt), pwhash.MaxSaltLen(pwhash.MethodSHA512))
	}
	for i := 0; i < len(salt); i++ {
		if !strings.ContainsRune("./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz", rune(salt[i])) {
			t.Errorf("salt %q contains %q, which is not in the crypt alphabet", salt, salt[i])
		}
	}
	if hash == "" {
		t.Fatalf("stdout = %q, want a hash after the salt", r.stdout)
	}
	// The printed setting must verify: hashing again with the same salt gives
	// the same hash.
	again, err := pwhash.Crypt("password", "$6$"+salt)
	if err != nil {
		t.Fatalf("Crypt: %v", err)
	}
	if again != line {
		t.Errorf("re-verified hash = %q, want %q", again, line)
	}
	if strings.Contains(r.stderr, "password") || strings.Contains(r.stdout, "password") {
		t.Errorf("the password leaked: stdout=%q stderr=%q", r.stdout, r.stderr)
	}
}

func TestMkpasswd_RandomSaltMD5Length(t *testing.T) {
	old := mkpasswdRand
	mkpasswdRand = bytes.NewReader(bytes.Repeat([]byte{0}, 64))
	t.Cleanup(func() { mkpasswdRand = old })

	r := runBuiltin(t, "mkpasswd", "-m", "md5", "password")
	if r.code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", r.code, r.stderr)
	}
	salt, _, ok := strings.Cut(strings.TrimPrefix(strings.TrimSuffix(r.stdout, "\n"), "$1$"), "$")
	if !ok || len(salt) != pwhash.MaxSaltLen(pwhash.MethodMD5) {
		t.Fatalf("stdout = %q, want 8 salt characters", r.stdout)
	}
}

type mkpasswdFailingReader struct{}

func (mkpasswdFailingReader) Read([]byte) (int, error) {
	return 0, errors.New("no entropy")
}

func TestMkpasswd_RandomSaltFailure(t *testing.T) {
	old := mkpasswdRand
	mkpasswdRand = mkpasswdFailingReader{}
	t.Cleanup(func() { mkpasswdRand = old })

	r := runBuiltin(t, "mkpasswd", "password")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "mkpasswd: random salt failed")
}

func TestMkpasswd_PasswordNotEchoed(t *testing.T) {
	want := "$6$abcdefgh$iwzjgpU8ZvASrbLs/ZtGQoDPmkuwZdiNWCr6r7lIAYsLy/7t96o.BHyaUC/xFOpsUJ2oJ7Nw3sr3MZylXfiVQ1"
	r := runBuiltin(t, "mkpasswd", "-S", "abcdefgh", "s3cretpw")
	mkpasswdWantHash(t, r, want)
	if strings.Contains(r.stdout, "s3cretpw") || strings.Contains(r.stderr, "s3cretpw") {
		t.Errorf("the password leaked: stdout=%q stderr=%q", r.stdout, r.stderr)
	}
}

func TestMkpasswd_EmptyPasswordOperand(t *testing.T) {
	// An empty PASSWORD operand is used verbatim instead of falling back to
	// reading standard input.
	r := runBuiltinStdin(t, "mkpasswd", "different\n", "-S", "abcdefgh", "")
	mkpasswdWantHash(t, r, mkpasswdEmptyPassSHA512)
}
