// Tests for dd. The cases marked "toybox" are ported from toybox
// tests/dd.test (0BSD) at commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb and
// the ones marked "uutils" from uutils coreutils tests/by-util/test_dd.rs
// (MIT) at commit fee61ff5d4f87e0f912a52e639942b5f7ad58663.
package builtins

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// chunkReader returns at most chunk bytes per Read, the way a pipe hands over
// whatever has arrived.
type chunkReader struct {
	data  []byte
	chunk int
}

func (r *chunkReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := min(len(p), r.chunk, len(r.data))
	copy(p, r.data[:n])
	r.data = r.data[n:]
	return n, nil
}

// ddRun runs dd with a reader as its standard input and buffers for its output.
func ddRun(t *testing.T, dir string, stdin io.Reader, args ...string) runResult {
	t.Helper()
	cmd, ok := Lookup("dd")
	if !ok {
		t.Fatal("dd not registered")
	}
	var stdout, stderr strings.Builder
	ctx := &Context{Dir: dir, Args: args, Width: 80, Height: 24, Stdout: &stdout, Stderr: &stderr, Stdin: stdin}
	code := cmd.Run(ctx)
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func ddRunString(t *testing.T, dir, stdin string, args ...string) runResult {
	t.Helper()
	return ddRun(t, dir, strings.NewReader(stdin), args...)
}

// stubDDClock makes the transfer-rate line deterministic.
func stubDDClock(t *testing.T, d time.Duration) {
	t.Helper()
	old := ddElapsed
	ddElapsed = func(time.Time) time.Duration { return d }
	t.Cleanup(func() { ddElapsed = old })
}

func readFileT(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestDDParseSize(t *testing.T) {
	tests := []struct {
		in   string
		want uint64
	}{
		{"1", 1},
		{"512", 512},
		{"1c", 1},
		{"2c", 2},
		{"2w", 4},
		{"2b", 1024},
		{"1B", 1},
		{"3B", 3},
		{"1k", 1024},
		{"1K", 1024},
		{"2kB", 2000},
		{"2KiB", 2048},
		{"1M", 1 << 20},
		{"1MB", 1000 * 1000},
		{"1MiB", 1 << 20},
		{"1G", 1 << 30},
		{"1GiB", 1 << 30},
		{"1T", 1 << 40},
		{"1TB", 1000 * 1000 * 1000 * 1000},
		{"2x512", 1024},
		{"1x2x3", 6},
		{"2x3", 6},
		{"0x10", 16},
		{"0x2", 2},
		{"2x0x10", 32},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ddParseNumber(tc.in)
			if err != nil {
				t.Fatalf("ddParseNumber(%q) = error %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("ddParseNumber(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestDDParseSizeInvalid(t *testing.T) {
	for _, in := range []string{
		"", "abc", "29d", "1x", "x2", "1x2x", "-2", "1.5", "0x", "1 Q", "2kk",
	} {
		t.Run(in, func(t *testing.T) {
			if got, err := ddParseNumber(in); err == nil {
				t.Fatalf("ddParseNumber(%q) = %d, want error", in, got)
			}
			want := "invalid number: '" + in + "'"
			_, err := ddParseNumber(in)
			if err == nil || err.Error() != want {
				t.Errorf("error = %v, want %q", err, want)
			}
		})
	}
}

func TestDDParseSizeTooLarge(t *testing.T) {
	for _, in := range []string{
		"99999999999999999999999999999999",
		"9223372036854775808",
	} {
		if _, err := ddParseNumber(in); !errors.Is(err, errDDTooLarge) {
			t.Errorf("ddParseNumber(%q) error = %v, want too large", in, err)
		}
	}
}

// A product that overflows is a malformed number, as uutils reports it.
func TestDDParseSizeProductOverflow(t *testing.T) {
	const in = "1024x1024x1024x1024x1024x1024x1024x1024x1024"
	if _, err := ddParseNumber(in); err == nil || err.Error() != "invalid number: '"+in+"'" {
		t.Errorf("ddParseNumber(%q) error = %v", in, err)
	}
}

func TestDDParseOperandErrors(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"foo"}, "unrecognized operand 'foo'"},
		{[]string{"if"}, "unrecognized operand 'if'"},
		{[]string{"bogus=1"}, "unrecognized operand 'bogus=1'"},
		{[]string{"count=29d"}, "invalid number: '29d'"},
		{[]string{"skip="}, "invalid number: ''"},
		{[]string{"seek=abc"}, "invalid number: 'abc'"},
		{[]string{"bs=0"}, "invalid number: '0'"},
		{[]string{"ibs=0"}, "invalid number: '0'"},
		{[]string{"obs=0"}, "invalid number: '0'"},
		{[]string{"ibs=5GB"}, "ibs=N cannot fit into memory"},
		{[]string{"bs=1PB"}, "bs=N cannot fit into memory"},
		{[]string{"obs=18446744073709551616"}, "Value too large for defined data type"},
		{[]string{"cbs=9223372036854775808"}, "Value too large for defined data type"},
		{[]string{"count=99999999999999999999"}, "Value too large for defined data type"},
		{[]string{"skip=18446744073709551616"}, "Value too large for defined data type"},
		{[]string{"cbs=x"}, "invalid number: 'x'"},
		{[]string{"cbs=0"}, "invalid number: '0'"},
		{[]string{"conv=bogus"}, "invalid conversion: 'bogus'"},
		{[]string{"conv=excl,nocreat"}, "cannot combine excl and nocreat"},
		{[]string{"conv=ucase,lcase"}, "cannot combine ucase and lcase"},
		{[]string{"conv=block,unblock", "cbs=4"}, "cannot combine block and unblock"},
		// The blocking conflict is reported before the case one.
		{[]string{"conv=block,unblock,lcase,ucase", "cbs=4"}, "cannot combine block and unblock"},
		{[]string{"conv=ascii,ebcdic"}, "cannot combine ascii, ebcdic and ibm"},
		{[]string{"conv=ascii,ibm"}, "cannot combine ascii, ebcdic and ibm"},
		{[]string{"conv=ascii,block", "cbs=4"}, "cannot combine block and unblock"},
		{[]string{"conv=nocache"}, "invalid conversion: 'nocache'"},
		{[]string{"iflag=nocache"}, "iflag=nocache: unsupported in this version"},
		{[]string{"oflag=nocache"}, "oflag=nocache: unsupported in this version"},
		{[]string{"iflag=cio"}, "iflag=cio: unsupported in this version"},
		{[]string{"iflag=nonsense"}, "invalid input flag: 'nonsense'"},
		{[]string{"oflag=nonsense"}, "invalid output flag: 'nonsense'"},
		{[]string{"status=loud"}, "invalid status level: 'loud'"},
	}
	for _, tc := range tests {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			_, err := parseDDArgs(tc.args)
			if err == nil {
				t.Fatalf("parseDDArgs(%q) succeeded, want %q", tc.args, tc.want)
			}
			if err.Error() != tc.want {
				t.Errorf("error = %q, want %q", err.Error(), tc.want)
			}
		})
	}
}

func TestDDParseOperandValues(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		check func(*testing.T, *ddConfig)
	}{
		{"block sizes", []string{"ibs=1", "obs=2"}, func(t *testing.T, c *ddConfig) {
			if c.ibs != 1 || c.obs != 2 {
				t.Errorf("ibs=%d obs=%d, want 1 2", c.ibs, c.obs)
			}
		}},
		{"bs wins", []string{"ibs=1", "bs=7", "obs=2"}, func(t *testing.T, c *ddConfig) {
			if c.ibs != 7 || c.obs != 7 {
				t.Errorf("ibs=%d obs=%d, want 7 7", c.ibs, c.obs)
			}
		}},
		{"count blocks", []string{"count=2", "ibs=4"}, func(t *testing.T, c *ddConfig) {
			if !c.countSet || c.count != 8 {
				t.Errorf("count=%d set=%v, want 8 true", c.count, c.countSet)
			}
		}},
		{"count bytes suffix", []string{"count=3B", "ibs=4"}, func(t *testing.T, c *ddConfig) {
			if c.count != 3 {
				t.Errorf("count=%d, want 3", c.count)
			}
		}},
		{"count_bytes flag", []string{"iflag=count_bytes", "count=2", "ibs=4"}, func(t *testing.T, c *ddConfig) {
			if c.count != 2 {
				t.Errorf("count=%d, want 2", c.count)
			}
		}},
		{"skip blocks", []string{"skip=2", "ibs=4"}, func(t *testing.T, c *ddConfig) {
			if c.skip != 8 {
				t.Errorf("skip=%d, want 8", c.skip)
			}
		}},
		{"skip_bytes flag", []string{"skip=2", "iflag=skip_bytes", "ibs=4"}, func(t *testing.T, c *ddConfig) {
			if c.skip != 2 {
				t.Errorf("skip=%d, want 2", c.skip)
			}
		}},
		{"iseek is skip", []string{"iseek=2", "ibs=4"}, func(t *testing.T, c *ddConfig) {
			if c.skip != 8 {
				t.Errorf("skip=%d, want 8", c.skip)
			}
		}},
		{"seek blocks", []string{"seek=2", "obs=8"}, func(t *testing.T, c *ddConfig) {
			if c.seek != 16 {
				t.Errorf("seek=%d, want 16", c.seek)
			}
		}},
		{"seek_bytes flag", []string{"seek=2", "oflag=seek_bytes", "obs=8"}, func(t *testing.T, c *ddConfig) {
			if c.seek != 2 {
				t.Errorf("seek=%d, want 2", c.seek)
			}
		}},
		{"oseek is seek", []string{"oseek=2", "obs=8"}, func(t *testing.T, c *ddConfig) {
			if c.seek != 16 {
				t.Errorf("seek=%d, want 16", c.seek)
			}
		}},
		{"status", []string{"status=noxfer"}, func(t *testing.T, c *ddConfig) {
			if c.status != ddStatusNoxfer {
				t.Errorf("status=%v, want noxfer", c.status)
			}
		}},
		{"conv flags", []string{"conv=notrunc,sync,noerror,fsync"}, func(t *testing.T, c *ddConfig) {
			if !c.conv.notrunc || !c.conv.sync || !c.conv.noerror || !c.conv.fsync {
				t.Errorf("conv = %+v", c.conv)
			}
		}},
		{"iflag fullblock", []string{"iflag=fullblock"}, func(t *testing.T, c *ddConfig) {
			if !c.iflag.fullblock {
				t.Error("fullblock not set")
			}
		}},
		{"oflag append", []string{"oflag=append"}, func(t *testing.T, c *ddConfig) {
			if !c.oflag.append {
				t.Error("append not set")
			}
		}},
		{"unbuffered with bs", []string{"bs=4"}, func(t *testing.T, c *ddConfig) {
			if !c.ddUnbuffered() {
				t.Error("bs= alone should pass short reads straight out")
			}
		}},
		{"buffered with conv", []string{"bs=4", "conv=sync"}, func(t *testing.T, c *ddConfig) {
			if c.ddUnbuffered() {
				t.Error("conv= should buffer output blocks")
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := parseDDArgs(tc.args)
			if err != nil {
				t.Fatalf("parseDDArgs(%q): %v", tc.args, err)
			}
			tc.check(t, cfg)
		})
	}
}

func TestDDStdinStdout(t *testing.T) {
	r := ddRunString(t, t.TempDir(), "hello\n", "status=none")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hello\n")
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
}

func TestDDDefaultSummary(t *testing.T) {
	stubDDClock(t, time.Second)
	r := ddRunString(t, t.TempDir(), "abc")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "abc")
	// 3 bytes are a short read, so the record is partial.
	if want := "0+1 records in\n0+1 records out\n3 bytes copied, 1 s, 3.0 B/s\n"; r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}
}

func TestDDNoxfer(t *testing.T) {
	r := ddRunString(t, t.TempDir(), "abc", "status=noxfer")
	assertExitCode(t, r, 0)
	if want := "0+1 records in\n0+1 records out\n"; r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}
}

func TestDDEmptyInputSummary(t *testing.T) {
	stubDDClock(t, time.Second)
	r := ddRunString(t, t.TempDir(), "")
	assertExitCode(t, r, 0)
	if want := "0+0 records in\n0+0 records out\n0 bytes copied, 1 s, 0.0 B/s\n"; r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}
}

// toybox: "count=2" "if=input count=2 ibs=1" -> "hi" from "high\n".
func TestDDCountBlocks(t *testing.T) {
	r := ddRunString(t, t.TempDir(), "high\n", "count=2", "ibs=1", "status=none")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hi")
}

// uutils: test_stdin_stdout_count_bytes.
func TestDDCountBytes(t *testing.T) {
	r := ddRunString(t, t.TempDir(), "high\n", "count=2", "ibs=4096", "iflag=count_bytes", "status=none")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hi")
}

// toybox: 'posix 2x3x4' "if=input count=2x3x4 ibs=1".
func TestDDCountProduct(t *testing.T) {
	input := "abcdefghijklmnopqrstuvwxyz"
	r := ddRunString(t, t.TempDir(), input, "count=2x3x4", "ibs=1", "status=none")
	assertExitCode(t, r, 0)
	assertStdout(t, r, input[:24])
}

// toybox: "count=-2" ... || echo errored.
func TestDDCountNegative(t *testing.T) {
	r := ddRunString(t, t.TempDir(), "x", "count=-2")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "dd: invalid number: '-2'")
}

// toybox: "with ibs obs" (bs overrides) and "with ibs obs count".
func TestDDIbsObs(t *testing.T) {
	dir := t.TempDir()
	r := ddRunString(t, dir, "I WANT\n", "ibs=2", "obs=5", "bs=9", "status=none")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "I WANT\n")

	r = ddRunString(t, dir, "I WANT\n", "ibs=2", "obs=5", "status=noxfer")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "I WANT\n")
	// 7 bytes read as 2+2+2+1: three full records and one partial, written as
	// one 5-byte block and a 2-byte remainder.
	if want := "3+1 records in\n1+1 records out\n"; r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}
}

// uutils: test_partial_records_out.
func TestDDPartialRecordsOut(t *testing.T) {
	r := ddRunString(t, t.TempDir(), "abc", "bs=2", "status=noxfer")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "abc")
	if want := "1+1 records in\n1+1 records out\n"; r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}
}

func TestDDShortReadsFromPipe(t *testing.T) {
	dir := t.TempDir()
	in := &chunkReader{data: []byte("abcdefghij"), chunk: 3}
	r := ddRun(t, dir, in, "ibs=4", "obs=4", "status=noxfer")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "abcdefghij")
	// Reads of 3,3,3,1 bytes; output blocks of 4: 4,4,2.
	if want := "0+4 records in\n2+1 records out\n"; r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}
}

func TestDDFullblock(t *testing.T) {
	in := &chunkReader{data: []byte("abcdefghij"), chunk: 3}
	r := ddRun(t, t.TempDir(), in, "ibs=4", "obs=4", "iflag=fullblock", "status=noxfer")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "abcdefghij")
	// Full blocks are assembled: 4,4 then a final 2.
	if want := "2+1 records in\n2+1 records out\n"; r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}
}

// uutils: test_sync_delayed_reader in miniature: every short read is padded to
// a whole input block.
func TestDDSyncPadding(t *testing.T) {
	in := &chunkReader{data: []byte("abcdef"), chunk: 3}
	r := ddRun(t, t.TempDir(), in, "ibs=4", "obs=4", "conv=sync", "status=noxfer")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "abc\x00def\x00")
	if want := "0+2 records in\n2+0 records out\n"; r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}
}

// uutils: test_sync_delayed_reader with a zero block size mismatch: sync pads
// to ibs and the output is written in obs blocks.
func TestDDSyncPadsToIbs(t *testing.T) {
	in := &chunkReader{data: []byte("12345678"), chunk: 3}
	r := ddRun(t, t.TempDir(), in, "ibs=4", "obs=8", "conv=sync", "status=noxfer")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "123\x00"+"456\x00"+"78\x00\x00")
	if want := "0+3 records in\n1+1 records out\n"; r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}
}

// toybox: "with skip if" skip=0, and skip=1 ibs=2 discards the first block.
func TestDDSkipBlocks(t *testing.T) {
	dir := t.TempDir()
	r := ddRunString(t, dir, "I WANT\n", "skip=0", "status=none")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "I WANT\n")

	r = ddRunString(t, dir, "abcdef", "skip=1", "ibs=2", "status=none")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "cdef")
}

// uutils: test_stdin_stdout_skip_bytes.
func TestDDSkipBytes(t *testing.T) {
	in := &chunkReader{data: []byte("abcdef"), chunk: 2}
	r := ddRun(t, t.TempDir(), in, "skip=2", "ibs=4", "iflag=skip_bytes", "status=none")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "cdef")
}

// uutils: test_skip_beyond_file warns and copies nothing, without failing.
func TestDDSkipBeyondInput(t *testing.T) {
	in := &chunkReader{data: []byte("abcd"), chunk: 2}
	r := ddRun(t, t.TempDir(), in, "bs=1", "skip=5", "count=0", "status=noxfer")
	assertExitCode(t, r, 0)
	assertStderrContains(t, r, "'standard input': cannot skip to specified offset")
	assertStderrContains(t, r, "0+0 records in\n0+0 records out\n")
}

// toybox: "seek truncate" - the output is truncated at the seek offset.
func TestDDSeekTruncates(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "input", "1234567890abcdefghijklmnopqrstuvwxyz")
	r := ddRunString(t, dir, "ABCDEFGHIJK", "of=input", "bs=3", "count=2", "seek=7", "oflag=seek_bytes", "status=none")
	assertExitCode(t, r, 0)
	if got := readFileT(t, filepath.Join(dir, "input")); got != "1234567ABCDEF" {
		t.Errorf("output = %q, want %q", got, "1234567ABCDEF")
	}
}

// toybox: "seek notrunc" - the tail of the output survives.
func TestDDSeekNotrunc(t *testing.T) {
	dir := t.TempDir()
	const content = "1234567890abcdefghijklmnopqrstuvwxyz"
	writeFile(t, dir, "input", content)
	r := ddRunString(t, dir, "ABCDEFGHIJK", "of=input", "bs=3", "count=2", "seek=7",
		"conv=notrunc", "oflag=seek_bytes", "status=none")
	assertExitCode(t, r, 0)
	if got, want := readFileT(t, filepath.Join(dir, "input")), "1234567ABCDEF"+content[13:]; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// uutils: test_seek_do_not_overwrite.
func TestDDSeekDoesNotOverwriteBeforeOffset(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "outfile", "abc")
	r := ddRunString(t, dir, "123", "bs=1", "skip=1", "seek=1", "count=1", "of=outfile", "status=noxfer")
	assertExitCode(t, r, 0)
	if got := readFileT(t, filepath.Join(dir, "outfile")); got != "a2" {
		t.Errorf("output = %q, want %q", got, "a2")
	}
	assertStderrContains(t, r, "1+0 records in\n1+0 records out\n")
}

func TestDDSeekOnPipeFails(t *testing.T) {
	r := ddRunString(t, t.TempDir(), "abc", "seek=1")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "'standard output': cannot seek: Invalid argument")
}

func TestDDFileToFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "in.txt", "0123456789")
	r := ddRun(t, dir, nil, "if=in.txt", "of=out.txt", "status=none")
	assertExitCode(t, r, 0)
	if got := readFileT(t, filepath.Join(dir, "out.txt")); got != "0123456789" {
		t.Errorf("output = %q", got)
	}
}

func TestDDOutputTruncatesByDefault(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "out.txt", "0123456789")
	r := ddRunString(t, dir, "abc", "of=out.txt", "status=none")
	assertExitCode(t, r, 0)
	if got := readFileT(t, filepath.Join(dir, "out.txt")); got != "abc" {
		t.Errorf("output = %q, want %q", got, "abc")
	}
}

// toybox: "same file notrunc" leaves the bytes past the copied data alone.
func TestDDNotruncKeepsTail(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "out.txt", "I WANT\n")
	r := ddRunString(t, dir, "AB", "of=out.txt", "conv=notrunc", "ibs=2", "status=none")
	assertExitCode(t, r, 0)
	if got := readFileT(t, filepath.Join(dir, "out.txt")); got != "ABWANT\n" {
		t.Errorf("output = %q, want %q", got, "ABWANT\n")
	}
}

func TestDDMissingInput(t *testing.T) {
	r := ddRun(t, t.TempDir(), nil, "if=nope", "status=none")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "dd: failed to open 'nope': No such file or directory")
}

func TestDDEmptyOperandFilenames(t *testing.T) {
	dir := t.TempDir()
	r := ddRun(t, dir, nil, "if=", "status=none")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "dd: failed to open ''")
	r = ddRunString(t, dir, "", "of=", "status=none")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "dd: failed to open ''")
}

// toybox: "conv=nocreat" fails when the output does not exist.
func TestDDNocreat(t *testing.T) {
	dir := t.TempDir()
	r := ddRunString(t, dir, "x", "conv=nocreat", "of=noexist", "status=none")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "dd: failed to open 'noexist'")
	if _, err := os.Stat(filepath.Join(dir, "noexist")); err == nil {
		t.Error("conv=nocreat created the output")
	}
}

// uutils: test_excl_causes_failure_when_present.
func TestDDExclExistingFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "exists", "keep me")
	r := ddRunString(t, dir, "new", "conv=excl", "of=exists", "status=none")
	assertExitCode(t, r, 1)
	if got := readFileT(t, filepath.Join(dir, "exists")); got != "keep me" {
		t.Errorf("output = %q, want %q", got, "keep me")
	}
}

// toybox: "oflag=append".
func TestDDOflagAppend(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file", "hello ")
	r := ddRunString(t, dir, "world\n", "of=file", "oflag=append", "conv=notrunc", "status=none")
	assertExitCode(t, r, 0)
	if got := readFileT(t, filepath.Join(dir, "file")); got != "hello world\n" {
		t.Errorf("output = %q, want %q", got, "hello world\n")
	}
}

func TestDDConvFsync(t *testing.T) {
	dir := t.TempDir()
	r := ddRunString(t, dir, "abc", "of=out", "conv=fsync", "status=none")
	assertExitCode(t, r, 0)
	if got := readFileT(t, filepath.Join(dir, "out")); got != "abc" {
		t.Errorf("output = %q", got)
	}
}

func TestDDUnsupportedConvFails(t *testing.T) {
	r := ddRunString(t, t.TempDir(), "abc", "iflag=nocache")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "dd: iflag=nocache: unsupported in this version")
	assertStdout(t, r, "")
}

func TestDDUnknownOperandFails(t *testing.T) {
	r := ddRunString(t, t.TempDir(), "abc", "iflag=cio")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "unsupported in this version")
}

func TestDDWriteErrorReported(t *testing.T) {
	cmd, _ := Lookup("dd")
	var stderr strings.Builder
	ctx := &Context{
		Dir: t.TempDir(), Args: []string{"status=none"},
		Stdout: brokenWriter{}, Stderr: &stderr,
		Stdin: strings.NewReader("abcdef"),
	}
	if code := cmd.Run(ctx); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "dd: 'standard output': write error") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

// brokenWriter fails every write, like a full device.
type brokenWriter struct{}

func (brokenWriter) Write(p []byte) (int, error) {
	return 0, io.ErrClosedPipe
}

// errorReader yields its data and then fails once, like a device with a bad
// sector.
type errorReader struct {
	data []byte
	fail bool
}

func (r *errorReader) Read(p []byte) (int, error) {
	if len(r.data) > 0 {
		n := copy(p, r.data)
		r.data = r.data[n:]
		return n, nil
	}
	if !r.fail {
		r.fail = true
		return 0, errors.New("input/output error")
	}
	return 0, io.EOF
}

func TestDDNoerrorContinues(t *testing.T) {
	r := ddRun(t, t.TempDir(), &errorReader{data: []byte("abc")}, "ibs=3", "obs=3", "conv=noerror", "status=noxfer")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "abc")
	assertStderrContains(t, r, "dd: 'standard input': read error")
	assertStderrContains(t, r, "1+0 records in\n1+0 records out\n")
}

func TestDDReadErrorWithoutNoerrorFails(t *testing.T) {
	r := ddRun(t, t.TempDir(), &errorReader{data: []byte("abc")}, "ibs=3", "obs=3", "status=noxfer")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "dd: 'standard input': read error")
}

func TestDDNoerrorSyncWritesZeroBlock(t *testing.T) {
	r := ddRun(t, t.TempDir(), &errorReader{data: []byte("abc")}, "ibs=4", "obs=4", "conv=noerror,sync")
	assertExitCode(t, r, 0)
	// "abc" padded to four bytes, then a zero block for the failed read.
	assertStdout(t, r, "abc\x00\x00\x00\x00\x00")
}

func TestDDHelpAndVersion(t *testing.T) {
	dir := t.TempDir()
	r := ddRun(t, dir, strings.NewReader(""))
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "records in")

	r = ddRun(t, dir, strings.NewReader(""), "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "usage: dd")

	r = ddRun(t, dir, strings.NewReader(""), "--version")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "(Misa)")
}

func TestDDBytesCopiedLine(t *testing.T) {
	tests := []struct {
		bytes uint64
		want  string
	}{
		{0, "0 bytes copied"},
		{1, "1 byte copied"},
		{999, "999 bytes copied"},
		{1000, "1000 bytes (1.0 kB) copied"},
		{1023, "1023 bytes (1.0 kB) copied"},
		{1024, "1024 bytes (1.0 kB, 1.0 KiB) copied"},
		{34 << 30, "36507222016 bytes (37 GB, 34 GiB) copied"},
	}
	for _, tc := range tests {
		if got := ddBytesCopied(tc.bytes); got != tc.want {
			t.Errorf("ddBytesCopied(%d) = %q, want %q", tc.bytes, got, tc.want)
		}
	}
}

// The magnitudes follow uutils numbers.rs.
func TestDDMagnitude(t *testing.T) {
	tests := []struct {
		bytes uint64
		si    bool
		want  string
	}{
		{0, true, "0.0 B"},
		{1, true, "1.0 B"},
		{999, true, "999 B"},
		{1000, true, "1.0 kB"},
		{1023, true, "1.0 kB"},
		{1025, true, "1.0 kB"},
		{10001, true, "10 kB"},
		{999000, true, "999 kB"},
		{999001, true, "1.0 MB"},
		{1100000, true, "1.1 MB"},
		{9900000, true, "9.9 MB"},
		{999000000, true, "999 MB"},
		{1024, false, "1.0 KiB"},
		{2048, false, "2.0 KiB"},
		{34 << 30, false, "34 GiB"},
		{34 << 30, true, "37 GB"},
	}
	for _, tc := range tests {
		if got := ddMagnitude(tc.bytes, tc.si); got != tc.want {
			t.Errorf("ddMagnitude(%d, si=%v) = %q, want %q", tc.bytes, tc.si, got, tc.want)
		}
	}
}

func TestDDRate(t *testing.T) {
	if got := ddRate(1000, 2*time.Second); got != 500 {
		t.Errorf("ddRate = %d, want 500", got)
	}
	if got := ddRate(10, time.Nanosecond); got != 10000 {
		t.Errorf("ddRate under a millisecond = %d, want 10000", got)
	}
}
