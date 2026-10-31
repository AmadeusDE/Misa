// Tests for dd's conv= transformations, conv=sparse and the status=progress
// line. Cases marked "toybox" come from toybox tests/dd.test (0BSD) and ones
// marked "uutils" from uutils coreutils tests/by-util/test_dd.rs (MIT), both at
// the commits pinned in docs/CREDITS.md.
package builtins

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDDConvCase(t *testing.T) {
	tests := []struct {
		name string
		args []string
		in   string
		want string
	}{
		{"ucase", []string{"conv=ucase"}, "aBc dEf\n", "ABC DEF\n"},
		{"lcase", []string{"conv=lcase"}, "AbC DeF\n", "abc def\n"},
		{"ucase keeps non letters", []string{"conv=ucase"}, "a1-_z", "A1-_Z"},
		// uutils: the case fold applies to each input block.
		{"ucase per block", []string{"conv=ucase", "ibs=2"}, "aBcDe", "ABCDE"},
		{"ucase with sync", []string{"conv=ucase,sync", "ibs=4"}, "ab", "AB\x00\x00"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := ddRunString(t, t.TempDir(), tc.in, append([]string{"status=none"}, tc.args...)...)
			assertExitCode(t, r, 0)
			assertStdout(t, r, tc.want)
		})
	}
}

func TestDDConvSwab(t *testing.T) {
	tests := []struct {
		name string
		args []string
		in   string
		want string
	}{
		// toybox "swab": every pair of bytes swaps, an odd final byte stays.
		{"even", []string{"conv=swab"}, "abcdef", "badcfe"},
		{"odd final byte", []string{"conv=swab"}, "abcde", "badce"},
		{"single byte", []string{"conv=swab"}, "x", "x"},
		// uutils: swab works per input block, so an odd block keeps its byte.
		{"per block", []string{"conv=swab", "ibs=4"}, "abcdefg", "badcfeg"},
		{"with lcase", []string{"conv=swab,lcase", "ibs=4"}, "AbCd", "badc"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := ddRunString(t, t.TempDir(), tc.in, append([]string{"status=none"}, tc.args...)...)
			assertExitCode(t, r, 0)
			assertStdout(t, r, tc.want)
		})
	}
}

func TestDDConvBlock(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		in        string
		want      string
		wantErr   string
		truncated int
	}{
		{
			// uutils test_truncated_record.
			name: "truncated records", args: []string{"conv=block", "cbs=1"},
			in: "ab\ncd\n", want: "ac", truncated: 2,
		},
		{
			// uutils test_block_record_across_reads.
			name: "record across reads", args: []string{"conv=block", "cbs=6", "ibs=2", "obs=2"},
			in: "1234\n56\n123456789\n78", want: "1234  56    12345678    ", truncated: 1,
		},
		{
			// uutils test_block_keeps_trailing_record_of_spaces.
			name: "trailing space record", args: []string{"conv=block", "cbs=4"},
			in: "ab\n  ", want: "ab      ",
		},
		{
			// A record with no trailing newline is padded at the end.
			name: "unterminated last record", args: []string{"conv=block", "cbs=4"},
			in: "ab", want: "ab  ",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := ddRunString(t, t.TempDir(), tc.in, append([]string{"status=noxfer"}, tc.args...)...)
			assertExitCode(t, r, 0)
			assertStdout(t, r, tc.want)
			assertStderrContains(t, r, "records in")
			if tc.truncated == 1 {
				assertStderrContains(t, r, "1 truncated record\n")
			}
			if tc.truncated > 1 {
				assertStderrContains(t, r, "2 truncated records\n")
			}
			if tc.truncated == 0 {
				assertStderrContains(t, r, "records out\n")
				if strings.Contains(r.stderr, "truncated") {
					t.Errorf("stderr = %q, want no truncated record line", r.stderr)
				}
			}
		})
	}
}

func TestDDConvUnblock(t *testing.T) {
	tests := []struct {
		name string
		args []string
		in   string
		want string
	}{
		{"pads to lines", []string{"conv=unblock", "cbs=4"}, "ab  cd  \n", "ab\ncd\n\n\n"},
		{"trailing spaces dropped", []string{"conv=unblock", "cbs=5"}, "abc     de", "abc\n   de\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := ddRunString(t, t.TempDir(), tc.in, append([]string{"status=none"}, tc.args...)...)
			assertExitCode(t, r, 0)
			assertStdout(t, r, tc.want)
		})
	}
}

func TestDDConvBlockWithoutCbsIsIgnored(t *testing.T) {
	// uutils test_block_unblock_without_cbs_is_ignored: with no cbs= there is
	// no record length, so block/unblock pass the data through.
	for _, conv := range []string{"block", "unblock", "block,unblock"} {
		t.Run(conv, func(t *testing.T) {
			r := ddRunString(t, t.TempDir(), "ab\ncd\n", "conv="+conv, "status=none")
			assertExitCode(t, r, 0)
			assertStdout(t, r, "ab\ncd\n")
		})
	}
}

func TestDDConvCharset(t *testing.T) {
	tests := []struct {
		name string
		args []string
		in   []byte
		want []byte
	}{
		{
			// uutils test_conv_ascii_implies_unblock: EBCDIC 0x40 is a space,
			// EBCDIC 0xc1 is 'A', and conv=ascii trims the trailing spaces.
			name: "ascii implies unblock", args: []string{"conv=ascii", "cbs=4"},
			in: []byte{0x40, 0xc1, 0x40, 0xc1, 0x40, 0xc1, 0x40, 0x40}, want: []byte(" A A\n A\n"),
		},
		{
			// uutils test_conv_ebcdic_implies_block: conv=ebcdic pads every
			// cbs= record with the EBCDIC space it converts from.
			name: "ebcdic implies block", args: []string{"conv=ebcdic", "cbs=4"},
			in: []byte(" A A\n A\n"), want: []byte{0x40, 0xc1, 0x40, 0xc1, 0x40, 0xc1, 0x40, 0x40},
		},
		{
			// uutils test_ascii_ebcdic_without_cbs_still_convert.
			name: "ascii without cbs", args: []string{"conv=ascii"},
			in: []byte{0xc1, 0xc2, 0x40}, want: []byte("AB "),
		},
		{
			name: "ebcdic without cbs", args: []string{"conv=ebcdic"},
			in: []byte("AB "), want: []byte{0xc1, 0xc2, 0x40},
		},
		{
			name: "ebcdic with ucase", args: []string{"conv=ebcdic,ucase"},
			in: []byte("ab"), want: []byte{ddASCIIToEBCDICLowerToUpper['a'], ddASCIIToEBCDICLowerToUpper['b']},
		},
		{
			name: "ibm converts", args: []string{"conv=ibm"},
			in: []byte("A"), want: []byte{ddASCIIToIBM['A']},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := ddRunString(t, t.TempDir(), string(tc.in), append([]string{"status=none"}, tc.args...)...)
			assertExitCode(t, r, 0)
			if r.stdout != string(tc.want) {
				t.Errorf("stdout = %q, want %q", r.stdout, string(tc.want))
			}
		})
	}
}

// uutils test_block_sync: conv=sync pads short reads with spaces when
// conv=block is active, so the padding becomes part of the blocked record.
func TestDDConvBlockSync(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		in      string // standard input, unless file says otherwise
		file    string // written to "in" and read with if=in
		want    string
		wantErr string
	}{
		{
			name: "sizes align", args: []string{"ibs=5", "cbs=5", "conv=block,sync"},
			in: "012\nabcde\n", want: "012  abcde",
			wantErr: "2+0 records in\n0+1 records out\n",
		},
		{
			name: "short read padded", args: []string{"ibs=5", "cbs=5", "conv=block,sync"},
			in: "012\nabcdefg\n", want: "012  abcde     ",
			wantErr: "2+1 records in\n0+1 records out\n1 truncated record\n",
		},
		{
			name: "record spans reads", args: []string{"if=in", "ibs=3", "cbs=4", "conv=block,sync"},
			file: "ab\ncdefg\nh", want: "ab  cdefh   ",
			wantErr: "3+1 records in\n0+1 records out\n1 truncated record\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			args := append([]string{"status=noxfer"}, tc.args...)
			var r runResult
			if tc.file != "" {
				if err := os.WriteFile(filepath.Join(dir, "in"), []byte(tc.file), 0o600); err != nil {
					t.Fatal(err)
				}
				r = ddRun(t, dir, nil, args...)
			} else {
				r = ddRunString(t, dir, tc.in, args...)
			}
			assertExitCode(t, r, 0)
			assertStdout(t, r, tc.want)
			assertStderrContains(t, r, tc.wantErr)
		})
	}
}

func TestDDConvNoerrorSyncWithBlock(t *testing.T) {
	// A failed read is padded like a short one: with conv=block,sync the pad
	// byte is the space the blocking step pads records with.
	r := ddRun(t, t.TempDir(), &errorReader{data: []byte("ab")},
		"ibs=3", "cbs=3", "conv=noerror,sync,block", "status=noxfer")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "ab    ")
	assertStderrContains(t, r, "read error")
}

func TestDDConvSparse(t *testing.T) {
	dir := t.TempDir()
	input := append(bytes.Repeat([]byte{0}, 4096), []byte("data")...)
	input = append(input, bytes.Repeat([]byte{0}, 512)...)
	if err := os.WriteFile(filepath.Join(dir, "in"), input, 0o600); err != nil {
		t.Fatal(err)
	}
	r := ddRun(t, dir, nil, "if=in", "of=out", "bs=512", "conv=sparse", "status=none")
	assertExitCode(t, r, 0)
	got, err := os.ReadFile(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, input) {
		t.Errorf("conv=sparse changed the data: %d bytes, want %d", len(got), len(input))
	}
}

func TestDDConvSparseNotruncExtendsFile(t *testing.T) {
	// uutils test_sparse_notrunc_extends_file: trailing zero blocks are
	// seeked over, so the file still has the length of the copy.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "in"), make([]byte, 1024), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "out"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	r := ddRun(t, dir, nil, "bs=512", "if=in", "of=out", "conv=notrunc,sparse", "status=none")
	assertExitCode(t, r, 0)
	info, err := os.Stat(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 1024 {
		t.Errorf("size = %d, want 1024", info.Size())
	}
}

func TestDDConvSparseDoesNotShrinkFile(t *testing.T) {
	// uutils test_sparse_notrunc_does_not_shrink.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "in"), make([]byte, 1024), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "out"), bytes.Repeat([]byte{'x'}, 4096), 0o600); err != nil {
		t.Fatal(err)
	}
	r := ddRun(t, dir, nil, "bs=512", "if=in", "of=out", "conv=notrunc,sparse", "status=none")
	assertExitCode(t, r, 0)
	info, err := os.Stat(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 4096 {
		t.Errorf("size = %d, want 4096", info.Size())
	}
}

func TestDDConvSparseToPipeWrites(t *testing.T) {
	// A pipe cannot hold a hole, so conv=sparse writes the zeroes out.
	r := ddRunString(t, t.TempDir(), "a\x00\x00b", "bs=4", "conv=sparse", "status=none")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\x00\x00b")
}

func TestDDStatusProgress(t *testing.T) {
	stubDDClock(t, 2*time.Second)
	ticks := make(chan time.Time, 1)
	printed := make(chan struct{})
	oldTicks, oldPrinted := ddProgressTicks, ddProgressPrinted
	ddProgressTicks = func() (<-chan time.Time, func()) { return ticks, func() {} }
	ddProgressPrinted = func() { close(printed) }
	t.Cleanup(func() { ddProgressTicks, ddProgressPrinted = oldTicks, oldPrinted })

	// One tick waiting in the channel: the line is printed while the second
	// block is still to be read, which keeps the output deterministic.
	ticks <- time.Now()
	in := &blockingReader{data: []byte("abcdef"), block: 3, wait: printed}
	r := ddRun(t, t.TempDir(), in, "ibs=3", "obs=3", "status=progress")

	assertExitCode(t, r, 0)
	assertStdout(t, r, "abcdef")
	// The progress line has no newline of its own; the summary starts on a
	// fresh line after it.
	if got, want := r.stderr, "\r3 bytes copied, 2 s, 1.0 B/s\n2+0 records in"; !strings.Contains(got, want) {
		t.Errorf("stderr = %q, want it to contain %q", got, want)
	}
	if strings.Count(r.stderr, "\r") != 1 {
		t.Errorf("stderr = %q, want exactly one progress line", r.stderr)
	}
}

func TestDDNoProgressWithoutStatus(t *testing.T) {
	r := ddRunString(t, t.TempDir(), "abc", "ibs=3")
	assertExitCode(t, r, 0)
	if strings.Contains(r.stderr, "\r") {
		t.Errorf("stderr = %q, want no progress line", r.stderr)
	}
}

// blockingReader hands out its data in blocks and, before the second read,
// waits for wait to be closed so that a test can be sure a progress line has
// already been printed.
type blockingReader struct {
	data  []byte
	block int
	wait  <-chan struct{}
	reads int
}

func (r *blockingReader) Read(p []byte) (int, error) {
	if r.reads == 1 {
		<-r.wait
	}
	r.reads++
	if len(r.data) == 0 {
		return 0, nil
	}
	n := copy(p, r.data[:min(len(p), r.block)])
	r.data = r.data[n:]
	return n, nil
}

func TestDDCancellationBetweenBlocks(t *testing.T) {
	// The copy loop asks stop between blocks, which is where cooperative
	// cancellation stops the work: the blocks already copied stay written.
	var stderr strings.Builder
	var out bytes.Buffer
	in := strings.NewReader("abcdefghij")
	blocks := 0
	copier := ddCopier{
		in: in, out: &out, ibs: 2, obs: 2, remaining: -1,
		inName: "standard input", outName: "standard output", stderr: &stderr,
		stop: func() bool { blocks++; return blocks > 2 },
	}
	if err := copier.run(); err != errDDInterrupted {
		t.Fatalf("run() = %v, want %v", err, errDDInterrupted)
	}
	if out.String() != "abcd" {
		t.Errorf("output = %q, want %q", out.String(), "abcd")
	}
	if got := copier.finalStats().inFull; got != 2 {
		t.Errorf("records in = %d, want 2", got)
	}
}

func TestDDPlatformFlagNames(t *testing.T) {
	// Every name the parser knows either yields flag bits or is refused as
	// unsupported on this platform; it is never accepted and then ignored.
	for name := range ddPlatformFlagNames {
		bits, err := ddPlatformOpenFlag(name)
		switch {
		case err != nil:
			if err != errPlatformFlag {
				t.Errorf("ddPlatformOpenFlag(%q) error = %v", name, err)
			}
		case bits == 0 && !ddPlatformFlagNames[name]:
			t.Errorf("ddPlatformOpenFlag(%q) accepted %q without flag bits", name, name)
		}
	}
	if bits, err := ddPlatformOpenFlag("nonsense"); err != nil || bits != 0 {
		t.Errorf("ddPlatformOpenFlag(nonsense) = %d, %v; want 0, nil", bits, err)
	}
}

func TestDDPlatformFlagAppliesToFiles(t *testing.T) {
	// The flags the platform supports have to reach open(2) and still copy
	// the data unchanged.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "in"), []byte("hello dd"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name := range ddPlatformFlagNames {
		if _, err := ddPlatformOpenFlag(name); err != nil {
			continue // not supported here: the parser reports that.
		}
		t.Run(name, func(t *testing.T) {
			r := ddRun(t, dir, nil, "if=in", "of=out", "conv=notrunc",
				"iflag="+name, "oflag="+name, "status=none")
			if name == "directory" {
				// A regular file is not a directory, so open(2) refuses it.
				assertExitCode(t, r, 1)
				assertStderrContains(t, r, "failed to open 'in'")
				return
			}
			assertExitCode(t, r, 0)
			if got := readFileT(t, filepath.Join(dir, "out")); !strings.HasPrefix(got, "hello dd") {
				t.Errorf("output = %q, want the input copied", got)
			}
		})
	}
}

func TestDDPlatformFlagOnRedirectedStream(t *testing.T) {
	// iflag= applies to an inherited standard stream through fcntl; the flags
	// that only exist at open(2) time are checked or refused, never ignored.
	dir := t.TempDir()
	rfd, wfd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer rfd.Close()
	defer wfd.Close()
	if _, err := wfd.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}
	wfd.Close()

	r := ddRun(t, dir, rfd, "iflag=directory", "status=none")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "dd: setting flags for 'standard input': Not a directory")

	r = ddRun(t, dir, rfd, "iflag=nonblock", "status=none")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "abc")

	// A directory on standard input is what iflag=directory asks for.
	rdir, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer rdir.Close()
	r = ddRun(t, dir, rdir, "iflag=directory", "count=0", "status=none")
	assertExitCode(t, r, 0)
}
