package builtins

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

// prRun runs pr with a context that can also set the environment.
func prRunCtx(t *testing.T, dir, stdin string, env []string, args ...string) runResult {
	t.Helper()
	cmd, ok := Lookup("pr")
	if !ok {
		t.Fatal("pr not registered")
	}
	var stdout, stderr bytes.Buffer
	ctx := &Context{
		Dir:     dir,
		Args:    args,
		Width:   80,
		Height:  24,
		CmdName: "pr",
		Stdout:  &stdout,
		Stderr:  &stderr,
		Stdin:   strings.NewReader(stdin),
		Env:     env,
	}
	code := cmd.Run(ctx)
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// prHeader builds the page header line the way pr lays it out: the date on the
// left, the file name centered, "Page N" on the right, 72 columns wide.
func prHeader(date, name string, page int) string {
	pagePart := "Page " + string(rune('0'+page))
	spaceForName := 72 - len(date) - len(pagePart)
	before := (spaceForName - len(name)) / 2
	after := spaceForName - len(name) - before
	return date + strings.Repeat(" ", before) + name + strings.Repeat(" ", after) + pagePart
}

// prPage wraps body in the five-line header, the empty rows of a full page and
// the five-line trailer. rows is how many content rows body already holds.
func prPage(date, name string, page, contentRows int, body string, rows int) string {
	return "\n\n" + prHeader(date, name, page) + "\n\n\n" +
		body + strings.Repeat("\n", contentRows-rows) + strings.Repeat("\n", 5)
}

func TestPrStdinOmitHeader(t *testing.T) {
	r := prRunCtx(t, t.TempDir(), "a\nb\n", nil, "-t")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\nb\n")
}

func TestPrDashOperandReadsStdin(t *testing.T) {
	r := prRunCtx(t, t.TempDir(), "a\nb\n", nil, "-t", "-")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\nb\n")
}

func TestPrDefaultPage(t *testing.T) {
	r := prRunCtx(t, t.TempDir(), "a\n", nil, "-D", "DATE")
	assertExitCode(t, r, 0)
	assertStdout(t, r, prPage("DATE", "", 1, 56, "a\n", 1))
}

func TestPrHeaderOption(t *testing.T) {
	r := prRunCtx(t, t.TempDir(), "a\n", nil, "-D", "DATE", "-h", "hdr")
	assertExitCode(t, r, 0)
	assertStdout(t, r, prPage("DATE", "hdr", 1, 56, "a\n", 1))
}

func TestPrHeaderFromFileName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "a\n")
	r := prRunCtx(t, dir, "", nil, "-D", "DATE", "f.txt")
	assertExitCode(t, r, 0)
	if !strings.Contains(r.stdout, "f.txt") {
		t.Errorf("header should name the file: %q", r.stdout)
	}
}

func TestPrPageLength(t *testing.T) {
	// -l 11 leaves one content line per page.
	r := prRunCtx(t, t.TempDir(), "a\nb\n", nil, "-l", "11", "-D", "DATE")
	assertExitCode(t, r, 0)
	assertStdout(t, r, prPage("DATE", "", 1, 1, "a\n", 1)+prPage("DATE", "", 2, 1, "b\n", 1))
}

func TestPrPageLengthImpliesOmitHeader(t *testing.T) {
	r := prRunCtx(t, t.TempDir(), "a\nb\nc\n", nil, "-l", "10", "-h", "hdr")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\nb\nc\n")
}

func TestPrOmitPagination(t *testing.T) {
	r := prRunCtx(t, t.TempDir(), "a\nb\n", nil, "-T")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\nb\n")
}

func TestPrPageRange(t *testing.T) {
	in := "a\nb\nc\n"
	// One-line pages: +2 starts at the second and prints to the end.
	r := prRunCtx(t, t.TempDir(), in, nil, "-l", "11", "-t", "+2")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "b\nc\n")

	r = prRunCtx(t, t.TempDir(), in, nil, "-l", "11", "-t", "--pages=2:3")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "b\nc\n")

	r = prRunCtx(t, t.TempDir(), in, nil, "-l", "11", "-t", "+2:2")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "b\n")
}

func TestPrPageRangeInvalid(t *testing.T) {
	for _, arg := range []string{"--pages=0", "--pages=-1:5", "--pages=1:-5", "--pages=5:1", "--pages=20:5"} {
		r := prRunCtx(t, t.TempDir(), "", nil, arg)
		assertExitCode(t, r, 1)
		if !strings.Contains(r.stderr, "invalid --pages argument") {
			t.Errorf("%s: stderr = %q", arg, r.stderr)
		}
		assertStdout(t, r, "")
	}
}

func TestPrStartPageExceedsPageCount(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f", "a\nb\n")
	r := prRunCtx(t, dir, "", nil, "--pages=2", "f")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
	if want := "pr: starting page number 2 exceeds page count 1\n"; r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}
}

func TestPrNoFileWarnings(t *testing.T) {
	r := prRunCtx(t, t.TempDir(), "", nil, "-r", "--pages=20:5")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty with -r", r.stderr)
	}
}

func TestPrColumns(t *testing.T) {
	// Three lines over two columns, filled down: the first column takes the
	// ceiling of 3/2 and "c" ends up next to "a".
	r := prRunCtx(t, t.TempDir(), "a\nb\nc\n", nil, "-t", "-2", "-w", "20")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a        \tc        \nb        \n")
}

func TestPrColumnsAcross(t *testing.T) {
	r := prRunCtx(t, t.TempDir(), "a\nb\nc\n", nil, "-t", "-a", "-2", "-w", "20")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a        \tb        \nc        \n")
}

func TestPrColumnsLongOption(t *testing.T) {
	r := prRunCtx(t, t.TempDir(), "a\nb\n", nil, "-t", "--columns=2", "-w", "20")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a        \tb        \n")
}

func TestPrColumnsFewerLinesThanColumns(t *testing.T) {
	r := prRunCtx(t, t.TempDir(), "a\n", nil, "-t", "-3", "-w", "20")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a     \n")

	r = prRunCtx(t, t.TempDir(), "a\nb\n", nil, "-t", "-3", "-w", "20")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a     \tb     \n")
}

func TestPrColumnSeparators(t *testing.T) {
	r := prRunCtx(t, t.TempDir(), "1\n2\n3\n4\n", nil, "-t", "-2", "-w", "20", "-s|")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1        |3        \n2        |4        \n")

	r = prRunCtx(t, t.TempDir(), "p\nq\nr\ns\n", nil, "-t", "-2", "-w", "20", "-S:::")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "p        :::r        \nq        :::s        \n")
}

func TestPrJoinLines(t *testing.T) {
	r := prRunCtx(t, t.TempDir(), "1\n2\n3\n4\n", nil, "-t", "-2", "-J")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1\t3\n2\t4\n")
}

func TestPrWidthTruncates(t *testing.T) {
	r := prRunCtx(t, t.TempDir(), "abcdefghijkl\n", nil, "-t", "-W", "10")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "abcdefghij\n")
}

func TestPrOffset(t *testing.T) {
	r := prRunCtx(t, t.TempDir(), "a\n", nil, "-t", "-o", "3")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "   a\n")

	// The indent is written before every cell.
	r = prRunCtx(t, t.TempDir(), "a\nb\n", nil, "-t", "-2", "-w", "20", "-o", "2")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "  a        \t  b        \n")
}

func TestPrNumbering(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"-t", "-n"}, "    1\ta\n    2\tb\n"},
		{[]string{"-t", "-n2"}, " 1\ta\n 2\tb\n"},
		{[]string{"-t", "-nc"}, "    1ca\n    2cb\n"},
		{[]string{"-t", "-nc2"}, " 1ca\n 2cb\n"},
		{[]string{"-t", "-N", "10", "-n"}, "   10\ta\n   11\tb\n"},
		{[]string{"-t", "--number-lines"}, "    1\ta\n    2\tb\n"},
		{[]string{"-t", "--first-line-number=9", "-n"}, "    9\ta\n   10\tb\n"},
	}
	for _, tc := range tests {
		r := prRunCtx(t, t.TempDir(), "a\nb\n", nil, tc.args...)
		assertExitCode(t, r, 0)
		assertStdout(t, r, tc.want)
	}
}

func TestPrDoubleSpace(t *testing.T) {
	r := prRunCtx(t, t.TempDir(), "a\nb\n", nil, "-t", "-d")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\n\nb\n\n")
}

func TestPrExpandTabs(t *testing.T) {
	tests := []struct {
		args []string
		in   string
		want string
	}{
		{[]string{"-t", "-e"}, "a\tb\n", "a       b\n"},
		{[]string{"-t", "-e4"}, "a\tb\n", "a   b\n"},
		{[]string{"-t", "--expand-tabs=2"}, "a\tb\n", "a b\n"},
		// A CHAR other than TAB is expanded with the given width; a plain TAB
		// still uses 8.
		{[]string{"-t", "-ea2"}, "aaa\tb\n", "        b\n"},
	}
	for _, tc := range tests {
		r := prRunCtx(t, t.TempDir(), tc.in, nil, tc.args...)
		assertExitCode(t, r, 0)
		assertStdout(t, r, tc.want)
	}
}

func TestPrFormFeed(t *testing.T) {
	r := prRunCtx(t, t.TempDir(), "a\n", nil, "-f", "-D", "DATE")
	assertExitCode(t, r, 0)
	// 63-line pages, no trailer, and a form feed instead of a page delimiter.
	want := "\n\n" + prHeader("DATE", "", 1) + "\n\n\na\n\f"
	assertStdout(t, r, want)
}

func TestPrMerge(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f", "a\n")
	writeFile(t, dir, "g", "b\n")
	r := prRunCtx(t, dir, "", nil, "-t", "-m", "f", "g")
	assertExitCode(t, r, 0)
	// Merge mode prints every row of the page: cells are 35 columns wide
	// ((72-1)/2), and an empty row is a padded cell on either side of the
	// separator.
	cell := strings.Repeat(" ", 35)
	want := "a" + strings.Repeat(" ", 34) + "\t" + "b" + strings.Repeat(" ", 34) + "\n" +
		strings.Repeat(cell+"\t"+cell+"\n", 55)
	assertStdout(t, r, want)
}

func TestPrMergeOneFileLonger(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f", "a\na\n")
	writeFile(t, dir, "g", "b\n")
	r := prRunCtx(t, dir, "", nil, "-t", "-l", "11", "-m", "f", "g")
	assertExitCode(t, r, 0)
	// -l 11 leaves room for one content row per page.
	cell := strings.Repeat(" ", 35)
	want := "a" + strings.Repeat(" ", 34) + "\t" + "b" + strings.Repeat(" ", 34) + "\n" +
		"a" + strings.Repeat(" ", 34) + "\t" + cell + "\n"
	assertStdout(t, r, want)
}

func TestPrMergeRejectsColumns(t *testing.T) {
	r := prRunCtx(t, t.TempDir(), "", nil, "--columns=2", "-m", "f")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "cannot specify number of columns when printing in parallel")

	r = prRunCtx(t, t.TempDir(), "", nil, "-a", "-m", "f")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "cannot specify both printing across and printing in parallel")
}

func TestPrMultipleOperands(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f", "a\n")
	writeFile(t, dir, "g", "b\n")
	r := prRunCtx(t, dir, "", nil, "-t", "f", "g")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\nb\n")
}

func TestPrEmptyFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f", "")
	r := prRunCtx(t, dir, "", nil, "f")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
}

func TestPrMissingFile(t *testing.T) {
	r := prRunCtx(t, t.TempDir(), "", nil, "nonexistent")
	assertExitCode(t, r, 1)
	if !strings.HasPrefix(r.stderr, "pr: nonexistent: ") {
		t.Errorf("stderr = %q", r.stderr)
	}
	if strings.Contains(r.stderr, "(os error") {
		t.Errorf("stderr should not leak the Go error: %q", r.stderr)
	}
}

func TestPrUnknownOption(t *testing.T) {
	r := prRunCtx(t, t.TempDir(), "", nil, "--invalid-argument")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	if !strings.Contains(r.stderr, "unknown option") {
		t.Errorf("stderr = %q", r.stderr)
	}
}

func TestPrInvalidOptionValues(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"-w", "0"}, "pr: invalid --width argument '0'\n"},
		{[]string{"-W", "0"}, "pr: invalid --page-width argument '0'\n"},
		{[]string{"-l", "0"}, "pr: invalid --length argument '0'\n"},
		{[]string{"-o", "abc"}, "pr: '-o MARGIN' invalid line offset: 'abc'\n"},
		{[]string{"--indent=-5"}, "pr: '-o MARGIN' invalid line offset: '-5'\n"},
		{[]string{"-o", "2147483648"}, "pr: '-o MARGIN' invalid line offset: '2147483648': Value too large for defined data type\n"},
		{[]string{"-l", "9999999999999999999"}, "pr: '-l PAGE_LENGTH' invalid number of lines: '9999999999999999999': Value too large for defined data type\n"},
		{[]string{"-N", "18446744073709551615"}, "pr: '-N NUMBER' invalid starting line number: '18446744073709551615': Value too large for defined data type\n"},
		{[]string{"--columns", "9999999999999999999"}, "pr: invalid number of columns: '9999999999999999999': Value too large for defined data type\n"},
		{[]string{"--columns=0"}, "pr: invalid --columns argument '0'\n"},
		{[]string{"-0"}, "pr: invalid --columns argument '0'\n"},
		{[]string{"-e0"}, "pr: '-e' extra characters or invalid number in the argument: ‘0’\nTry 'pr --help' for more information.\n"},
		{[]string{"-e€"}, "pr: '-e' extra characters or invalid number in the argument: ‘€’\nTry 'pr --help' for more information.\n"},
		{[]string{"-n", "€"}, "pr: '-n' extra characters or invalid number in the argument: ‘€’\nTry 'pr --help' for more information.\n"},
		{[]string{"--number-lines="}, "pr: '-n' extra characters or invalid number in the argument: ‘’\nTry 'pr --help' for more information.\n"},
		{[]string{"-n", "18446744073709551615"}, "pr: '-n' extra characters or invalid number in the argument: '18446744073709551615': Value too large for defined data type\nTry 'pr --help' for more information.\n"},
	}
	for _, tc := range tests {
		r := prRunCtx(t, t.TempDir(), "x\n", nil, tc.args...)
		assertExitCode(t, r, 1)
		assertStdout(t, r, "")
		if r.stderr != tc.want {
			t.Errorf("%q: stderr = %q, want %q", tc.args, r.stderr, tc.want)
		}
	}
}

func TestPrNumberValueFallsBackToFileName(t *testing.T) {
	// `-n FILE` numbers lines with the defaults and keeps FILE an operand; a
	// token that cannot be [SEP][DIGITS] is not a value.
	r := prRunCtx(t, t.TempDir(), "", nil, "-t", "-n", "f.txt")
	assertExitCode(t, r, 1)
	if !strings.HasPrefix(r.stderr, "pr: f.txt: ") {
		t.Errorf("stderr = %q", r.stderr)
	}
	// A [SEP][DIGITS] token is a numbering value instead.
	r = prRunCtx(t, t.TempDir(), "a\n", nil, "-t", "-n", "*5")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "    1*a\n")
}

func TestPrDoubleDashEndsOptions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "-n", "RUST-pr\n")
	r := prRunCtx(t, dir, "", nil, "-t", "--", "-n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "RUST-pr\n")
}

func TestPrFileNameEndingInDashNumber(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a-0", "a-b-0", "a-3"} {
		writeFile(t, dir, name, "RUST-pr\n")
		r := prRunCtx(t, dir, "", nil, "-t", name)
		assertExitCode(t, r, 0)
		assertStdout(t, r, "RUST-pr\n")
	}
}

func TestPrHeaderDateFromMtime(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "f", "a\n")
	when := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Skipf("cannot set mtime: %v", err)
	}
	r := prRunCtx(t, dir, "", nil, "-t", "f") // -t keeps the date out of the way
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\n")

	r = prRunCtx(t, dir, "", nil, "f")
	want := when.Local().Format("2006-01-02 15:04")
	if !strings.Contains(r.stdout, want) {
		t.Errorf("header should show the file mtime %q: %q", want, strings.SplitN(r.stdout, "\n", 4)[2])
	}

	r = prRunCtx(t, dir, "", nil, "-D", "%Y/%m/%d", "f")
	want = when.Local().Format("2006/01/02")
	if !strings.Contains(r.stdout, want) {
		t.Errorf("-D output should show %q: %q", want, r.stdout[:72])
	}
}

func TestPrPosixDateEnvironment(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "f", "a\n")
	when := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Skipf("cannot set mtime: %v", err)
	}
	env := []string{"POSIXLY_CORRECT=1", "LC_TIME=POSIX"}
	r := prRunCtx(t, dir, "", env, "f")
	want := when.Local().Format("Jan _2 15:04 2006")
	if !strings.Contains(r.stdout, want) {
		t.Errorf("POSIX header should show %q: %q", want, r.stdout[:72])
	}
}

func TestPrDateFromLiteralFormat(t *testing.T) {
	// A format without conversions is printed as-is; the header is still 72
	// columns wide, so the block of spaces is 60 columns for "Page 1".
	r := prRunCtx(t, t.TempDir(), "a\n", nil, "-D", "Hello!")
	assertExitCode(t, r, 0)
	want := "\n\nHello!" + strings.Repeat(" ", 60) + "Page 1\n\n\na" + strings.Repeat("\n", 61)
	assertStdout(t, r, want)

	r = prRunCtx(t, t.TempDir(), "a\n", nil, "--date-format=Hello!")
	assertExitCode(t, r, 0)
	assertStdout(t, r, want)
}

func TestPrHelp(t *testing.T) {
	r := prRunCtx(t, t.TempDir(), "", nil, "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "--pages")

	r = prRunCtx(t, t.TempDir(), "", nil, "--version")
	assertExitCode(t, r, 0)
	if !strings.Contains(r.stdout, "0.1.0") {
		t.Errorf("--version printed %q", r.stdout)
	}
}

func TestPrFormFeedSplitsPages(t *testing.T) {
	// A form feed ends the page; "\n\f" does not add an empty line.
	r := prRunCtx(t, t.TempDir(), "abc\n\f", nil, "-t", "-l", "20")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "abc\n")

	// "\f\n" ends the page and the newline does not open a new line.
	r = prRunCtx(t, t.TempDir(), "\f\nabc", nil, "-t", "-l", "20")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "\nabc\n")
}

func TestPrLargeNumberWidth(t *testing.T) {
	if testing.Short() {
		t.Skip("allocates 70k columns")
	}
	r := prRunCtx(t, t.TempDir(), "x\n", nil, "-t", "-n", "70000")
	assertExitCode(t, r, 0)
	assertStdout(t, r, strings.Repeat(" ", 69999)+"1\tx\n")
}

func TestPrFormFeedLinesPerPage(t *testing.T) {
	// -f makes pages 63 lines long (53 content rows with a header): 60 lines
	// then fill two pages, and each page ends with a form feed.
	in := strings.Repeat("x\n", 60)
	r := prRunCtx(t, t.TempDir(), in, nil, "-f", "-t")
	assertExitCode(t, r, 0)
	if got := strings.Count(r.stdout, "\f"); got != 2 {
		t.Errorf("form feeds = %d, want 2", got)
	}
	if got := strings.Count(r.stdout, "x\n"); got != 60 {
		t.Errorf("content lines = %d, want 60", got)
	}
	r = prRunCtx(t, t.TempDir(), in, nil, "-t")
	assertExitCode(t, r, 0)
	if got := strings.Count(r.stdout, "\f"); got != 0 {
		t.Errorf("form feeds without -f = %d, want 0", got)
	}
}

func TestPrRelativePathUsesContextDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "sub", "a\n")
	r := prRunCtx(t, dir, "", nil, "-t", "sub")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\n")
}
