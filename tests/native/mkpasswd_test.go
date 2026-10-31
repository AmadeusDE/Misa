package native

import "testing"

const (
	mkpasswdMD5Vector    = "$1$abcdefgh$G//4keteveJp0qb8z2DxG/"
	mkpasswdSHA512Vector = "$6$abcdefgh$yVfUwsw5T.JApa8POvClA1pQ5peiq97DUNyXCZN5IrF.BMSkiaLQ5kvpuEm/VQ1Tvh/KV2TcaWh8qinoW5dhA1"
)

func TestMkpasswd_DefaultSHA512(t *testing.T) {
	r := run(t, "mkpasswd", "-S", "abcdefgh", "password")
	wantSuccess(t, r)
	wantOut(t, r, mkpasswdSHA512Vector+"\n")
}

func TestMkpasswd_MD5(t *testing.T) {
	r := run(t, "mkpasswd", "-m", "md5", "-S", "abcdefgh", "password")
	wantSuccess(t, r)
	wantOut(t, r, mkpasswdMD5Vector+"\n")
}

func TestMkpasswd_Stdin(t *testing.T) {
	r := runStdin(t, "mkpasswd", "password\n", "-m", "sha-512", "-S", "./Aa0Bb1Cc2Dd3Ee")
	wantSuccess(t, r)
	wantOut(t, r, "$6$./Aa0Bb1Cc2Dd3Ee$PvmedaPf329sM25Jn2jv3MsfK9DaDh6tyVtJucp35A/Lmrtp9g1Ab35Mr59pkuMU3QJlbXYoWJFaxyD4OwIZ60\n")
}

func TestMkpasswd_DESUnsupported(t *testing.T) {
	r := run(t, "mkpasswd", "-m", "des", "password")
	wantCode(t, r, 1)
	wantOut(t, r, "")
	wantErrContains(t, r, "DES crypt is not implemented")
}

func TestMkpasswd_Help(t *testing.T) {
	r := run(t, "mkpasswd", "--help")
	wantSuccess(t, r)
	wantOutContains(t, r, "usage: mkpasswd")
	wantOutContains(t, r, "sha512")
}

func TestMkpasswd_TooManyArguments(t *testing.T) {
	r := run(t, "mkpasswd", "a", "b", "c")
	wantCode(t, r, 1)
	wantErr(t, r, "mkpasswd: too many arguments\n")
}
