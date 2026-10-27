package native

import (
	"strings"
	"testing"
)

func TestBase64_EncodeStdin(t *testing.T) {
	r := runStdin(t, "base64", "hello, world!")
	wantSuccess(t, r)
	wantOut(t, r, "aGVsbG8sIHdvcmxkIQ==\n")
}

func TestBase64_EncodeDash(t *testing.T) {
	r := runStdin(t, "base64", "hello, world!", "-")
	wantSuccess(t, r)
	wantOut(t, r, "aGVsbG8sIHdvcmxkIQ==\n")
}

func TestBase64_DecodeShort(t *testing.T) {
	r := runStdin(t, "base64", "aQ==", "--decode")
	wantSuccess(t, r)
	wantOut(t, r, "i")
}

func TestBase64_DecodeUnpadded(t *testing.T) {
	r := runStdin(t, "base64", "QUJD", "-d")
	wantSuccess(t, r)
	wantOut(t, r, "ABC")
}

func TestBase64_RoundTrip(t *testing.T) {
	input := "The quick brown fox jumps over the lazy dog"
	enc := runStdin(t, "base64", input)
	wantSuccess(t, enc)
	dec := runStdin(t, "base64", enc.stdout, "-d")
	wantSuccess(t, dec)
	wantOut(t, dec, input)
}

func TestBase64_Wrap10(t *testing.T) {
	r := runStdin(t, "base64", "hello, world!", "-w", "10")
	wantSuccess(t, r)
	lines := strings.Split(strings.TrimRight(r.stdout, "\n"), "\n")
	for _, line := range lines[:len(lines)-1] {
		if len(line) != 10 {
			t.Errorf("expected line length 10, got %d: %q", len(line), line)
		}
	}
}

func TestBase64_Wrap0(t *testing.T) {
	long := strings.Repeat("a", 200)
	r := runStdin(t, "base64", long, "-w", "0")
	wantSuccess(t, r)
	if strings.Contains(r.stdout, "\n") {
		t.Error("expected no newlines with -w 0")
	}
}

func TestBase64_DecodeInvalid(t *testing.T) {
	r := runStdin(t, "base64", "not valid!!!", "-d")
	wantFail(t, r)
}

func TestBase64_DecodeIgnoreGarbage(t *testing.T) {
	r := runStdin(t, "base64", "aGVs\n!!!bG8K\n", "-d", "-i")
	wantSuccess(t, r)
	wantOut(t, r, "hello\n")
}

func TestBase64_EncodeFile(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "in.txt", "hello world")
	r := runIn(t, dir, "base64", "in.txt")
	wantSuccess(t, r)
	wantOut(t, r, "aGVsbG8gd29ybGQ=\n")
}

func TestBase64_EncodeEmpty(t *testing.T) {
	r := runStdin(t, "base64", "")
	wantSuccess(t, r)
}

func TestBase64_DecodePaddedFollowedByUnpadded(t *testing.T) {
	r := runStdin(t, "base64", "MTIzNA==MTIzNA", "-d")
	wantSuccess(t, r)
	wantOut(t, r, "12341234")
}

func TestBase64_DecodePaddedFollowedByAlignedTail(t *testing.T) {
	r := runStdin(t, "base64", "MTIzNA==QUJD", "-d")
	wantSuccess(t, r)
	wantOut(t, r, "1234ABC")
}

func TestBase64_FileNotFoundErrMsg(t *testing.T) {
	r := run(t, "base64", "nosuchfile.txt")
	wantFail(t, r)
	wantErrContains(t, r, "nosuchfile.txt: No such file or directory")
}
