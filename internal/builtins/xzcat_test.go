package builtins

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixtures are .xz streams made with XZ Utils 5.8.1, so the tests need no
// external xz binary and no user configuration. Regenerate them with:
//
//	printf 'hello\n' | xz -c | base64
//	printf 'hello\n' | xz -c --check=none | base64
//	printf 'hello\n' | xz -c --delta=dist=1 --lzma2 | base64
//	printf 'hello\n' | xz -c --lzma2=dict=128MiB | base64
//	python3 -c 'import sys;sys.stdout.buffer.write(b"the quick brown fox jumps over the lazy dog\n"*50)' \
//	    | xz -c --block-size=256 | base64
//	( printf 'hello\n' | xz -c; printf 'one\n' | xz -c; printf 'two\n' | xz -c; \
//	  printf 'three\n' | xz -c ) | base64
//	python3 -c 'import sys;sys.stdout.buffer.write(bytes(range(256))*8)' | xz -c --x86 --lzma2 | base64

const (
	// "hello\n", CRC64.
	xzHelloFixture = "/Td6WFoAAATm1rRGBMAKBiEBFgAAAAAAAAAAAKowjqYBAAVoZWxsbwoAAAClYJfxlPb94AABJgY6" +
		"kzsKH7bzfQEAAAAABFla"

	// "hello\n", "one\n", "two\n" and "three\n" as four concatenated streams.
	xzMultiFixture = "/Td6WFoAAATm1rRGBMAKBiEBFgAAAAAAAAAAAKowjqYBAAVoZWxsbwoAAAClYJfxlPb94AABJgY6" +
		"kzsKH7bzfQEAAAAABFla/Td6WFoAAATm1rRGBMAIBCEBFgAAAAAAAAAAAExBvCcBAANvbmUKAPVc" +
		"u1Yfxv2GAAEkBJSQA9YftvN9AQAAAAAEWVr9N3pYWgAABObWtEYEwAgEIQEWAAAAAAAAAAAATEG8" +
		"JwEAA3R3bwoAZQbW/8TA+ToAASQElJAD1h+2830BAAAAAARZWv03elhaAAAE5ta0RgTACgYhARYA" +
		"AAAAAAAAAACqMI6mAQAFdGhyZWUKAAAALOfhsoKxk2UAASYGOpM7Ch+2830BAAAAAARZWg=="

	// "the quick brown fox jumps over the lazy dog\n" repeated 50 times,
	// in 256-byte blocks, so the stream has many blocks and an Index.
	xzTextFixture = "/Td6WFoAAATm1rRGA8A5gAIhARYAAAAAgBVCUOAA/wAxXQA6GgjOdsfl6dYHNMPRDr/OVeGqveDk" +
		"j5gB3Y3lB1SeZSVfJzpqfrTTSP5QrNTk9gAAAAAAAH546u7suUDMA8A5gAIhARYAAAAAgBVCUOAA" +
		"/wAxXQAwnot9CkvOGrJtm6MaMJZnYI1kMSgh7GOXncQAO/69o4Nj/lOs6A5fNBB3gpep+gAAAAAA" +
		"AGwuSCra/Si0A8A5gAIhARYAAAAAgBVCUOAA/wAxXQAynIAGQbcovLbIVLoYoKtxu9jsR9OzqIgD" +
		"JncM67iS2xP+0tyJm5t8FPtYPFfKvAAAAAAAABETYFhufJ82A8A5gAIhARYAAAAAgBVCUOAA/wAx" +
		"XQA1HUneswTpDqXpBhR+Os4sfcBPVU+uTlW4SSos8Y8SLSz2MjPqyL3ls18+Sg1e2AAAAAAAAC7u" +
		"9v94CwETA8A5gAIhARYAAAAAgBVCUOAA/wAxXQA3ncn7D2jpRRQ02SMdfeltAsPf6lcS7DpYxum+" +
		"US1vEygHR5cfpkGB9URkK+uUiAAAAAAAAHrXbAIm7/MhA8A4gAIhARYAAAAAvn6Av+AA/wAwXQA4" +
		"nUlVqZl4LbriMS2g66zKIIgKbc8UVZbxhUrrk2dsWfVIIGqS8uTVoplxrgChwAAA/rdhMH1m6WED" +
		"wDmAAiEBFgAAAACAFUJQ4AD/ADFdADIbyRMWpgekPWbWasDLuqxqX/ZW/TaqGOykvsOu2l+EjSt7" +
		"sbRtjFsb+idxVmFkAAAAAAAAO4yV12OS8F0DwDqAAiEBFgAAAACDrnW74AD/ADJdADQZQAXBmzGZ" +
		"Sk/JFsDTUjGEG/1MeeFM4sv92z+PQcgsTAARBhB0sc9q1WlCegaSUgAAAAAAcmvJ05sId2kDwDmY" +
		"ASEBFgAAAAAK8tXJ4ACXADFdADmICeejlMPCYiSn1C78WKeGiRaihy7C3YGR1PzHzxvll5iECusY" +
		"T0hH9X3lBmXYAAAAAAAAF7rNcbwcBLwACVGAAlGAAlGAAlGAAlGAAlCAAlGAAlKAAlGYAQAAAMQ1" +
		"lrHX5/xaCAAAAAAEWVo="

	// "hello\n" with the integrity check disabled.
	xzNoneFixture = "/Td6WFoAAAD/EtlBBMAKBiEBFgAAAAAAAAAAAKowjqYBAAVoZWxsbwoAAAAAAR4GwS+kHQZynnoB" +
		"AAAAAABZWg=="

	// "hello\n" through the delta filter, which the decoder does not support.
	xzDeltaFixture = "/Td6WFoAAATm1rRGBMEKBgMBACEBFgAAAAAAAABg4ZwBAAVo/QcAA5sAAAClYJfxlPb94AABJgY6" +
		"kzsKH7bzfQEAAAAABFla"

	// "hello\n" in a stream asking for a 128 MiB dictionary.
	xzBigDictFixture = "/Td6WFoAAATm1rRGBMAKBiEBHgAAAAAAAAAAAF5wWbUBAAVoZWxsbwoAAAClYJfxlPb94AABJgY6" +
		"kzsKH7bzfQEAAAAABFla"

	// bytes 0-255 repeated 8 times through the x86 BCJ filter.
	xzX86Fixture = "/Td6WFoAAATm1rRGBMH/AYAQBAAhARYAAAAAAIPl7WbgB/8A910AAABSUAqE+ZuygCGpadYn4D4G" +
		"Wl8EjVPUBLo5VwUJwVUk3p24cVkxYKGf+W9Jc/LI6oy6GospaSGA/jODZq9GbeyeiYoLg/A8DomO" +
		"P+1f556Q2Rz/MvSy4DlRstIUFbTFcbrbBuN5mp+7OMGwAKyTC6oGGQMSCBVbm8hI8DIu/i2gh8jw" +
		"pODSUeuNZ1aSsk2ExfGGMd9qYlvCeS3Z9zxzunR0B9g8qVYiJKFm+FqEXzBn0vZLSS5/IOvb+BAO" +
		"lHh3xz9r77TNleJv9kRuBs8LghrL23rwV42Y/5DAPubBEkF17gMolusT+6cozK8yu6QOJfJWOsEa" +
		"AAAAFZAJApNZ2hkAAZsCgBAAAHZ0Sz+xxGf7AgAAAAAEWVo="
)

const xzTextPayload = "the quick brown fox jumps over the lazy dog\n"

func xzFixture(t *testing.T, encoded string) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("bad fixture %q: %v", encoded, err)
	}
	return data
}

// TestXzcatStreams covers bytes in and bytes out, the exit status and the
// diagnostic for every failure.
func TestXzcatStreams(t *testing.T) {
	hello := xzFixture(t, xzHelloFixture)
	text := xzFixture(t, xzTextFixture)
	wantText := strings.Repeat(xzTextPayload, 50)
	garbage := append(append([]byte(nil), hello...), 'j', 'u', 'n', 'k')

	tests := []struct {
		name   string
		stdin  []byte
		args   []string
		stdout string
		code   int
		stderr string
	}{
		{"single stream", hello, []string{"-"}, "hello\n", 0, ""},
		{"no operands reads stdin", hello, nil, "hello\n", 0, ""},
		{"check none", xzFixture(t, xzNoneFixture), nil, "hello\n", 0, ""},
		{"concatenated streams", xzFixture(t, xzMultiFixture), nil, "hello\none\ntwo\nthree\n", 0, ""},
		{"multiple blocks", text, nil, wantText, 0, ""},
		{"bcj x86", xzFixture(t, xzX86Fixture), nil, strings.Repeat(string(bytes0To255()), 8), 0, ""},
		{"empty input", nil, nil, "", 1, "not in xz format"},
		{"garbage", []byte("hello there\n"), nil, "", 1, "not in xz format"},
		// The bytes decoded before the damage has been found reach stdout,
		// exactly like gzip -c; the exit status is what matters.
		{"truncated", hello[:len(hello)-5], nil, "hello\n", 1, "unexpected end of file"},
		{"trailing garbage", garbage, nil, "hello\n", 1, "file is corrupt"},
		{"delta filter", xzFixture(t, xzDeltaFixture), nil, "", 1, "unsupported options in the .xz headers"},
		{"dictionary too big", xzFixture(t, xzBigDictFixture), nil, "", 1, "memory limit exceeded"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := runBuiltinStdin(t, "xzcat", string(tc.stdin), tc.args...)
			assertExitCode(t, r, tc.code)
			assertStdout(t, r, tc.stdout)
			if tc.stderr != "" {
				assertStderrContains(t, r, tc.stderr)
			} else if strings.TrimSpace(r.stderr) != "" {
				t.Errorf("unexpected stderr: %q", r.stderr)
			}
		})
	}
}

func bytes0To255() []byte {
	b := make([]byte, 256)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

// TestXzcatFiles covers operands: several files in one run, a missing file,
// a directory, and that the inputs are never touched.
func TestXzcatFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.xz", string(xzFixture(t, xzHelloFixture)))
	writeFile(t, dir, "b.bin", string(xzFixture(t, xzMultiFixture)))
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	// xzcat needs no known suffix and leaves its inputs alone.
	assertStdout(t, runBuiltinIn(t, dir, "xzcat", "a.xz"), "hello\n")
	assertStdout(t, runBuiltinIn(t, dir, "xzcat", "a.xz", "b.bin"), "hello\nhello\none\ntwo\nthree\n")
	for _, name := range []string{"a.xz", "b.bin"} {
		if _, err := os.Lstat(filepath.Join(dir, name)); err != nil {
			t.Errorf("xzcat removed or changed its input %s: %v", name, err)
		}
	}

	r := runBuiltinIn(t, dir, "xzcat", "missing.xz")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "No such file or directory")

	r = runBuiltinIn(t, dir, "xzcat", "sub")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "Is a directory")

	// One bad operand does not stop the others, and the status reports it.
	writeFile(t, dir, "bad.xz", "not xz at all\n")
	r = runBuiltinIn(t, dir, "xzcat", "bad.xz", "a.xz")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "hello\n")
	assertStderrContains(t, r, "not in xz format")
}

// TestXzcatOptions checks the option surface: xzcat has none of its own.
func TestXzcatOptions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.xz", string(xzFixture(t, xzHelloFixture)))

	for _, opt := range []string{"-c", "-f", "-k", "-t", "-d", "--stdout", "--bogus"} {
		r := runBuiltinIn(t, dir, "xzcat", opt, "a.xz")
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, "unknown option")
		assertStdout(t, r, "")
	}

	r := runBuiltinIn(t, dir, "xzcat", "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Usage: xzcat")
	assertStdoutContains(t, r, "--help")

	r = runBuiltinIn(t, dir, "xzcat", "--version")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "xzcat (Misa)")
}
