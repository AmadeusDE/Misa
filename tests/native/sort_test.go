package native

import "testing"

func TestSort_Basic(t *testing.T) {
	r := runStdin(t, "sort", "c\na\nb\n")
	wantSuccess(t, r)
	wantOut(t, r, "a\nb\nc\n")
}

func TestSort_NumericOrder(t *testing.T) {
	r := runStdin(t, "sort", "3\n1\n010\n", "-n")
	wantSuccess(t, r)
	wantOut(t, r, "1\n3\n010\n")
}

func TestSort_Reverse(t *testing.T) {
	r := runStdin(t, "sort", "point\nwook\npabst\naargh\nwalrus\n", "-r")
	wantSuccess(t, r)
	wantOut(t, r, "wook\nwalrus\npoint\npabst\naargh\n")
}

func TestSort_LeadingNumbers(t *testing.T) {
	r := runStdin(t, "sort", "3\n1\n010\n")
	wantSuccess(t, r)
	wantOut(t, r, "010\n1\n3\n")
}

func TestSort_Unique(t *testing.T) {
	r := runStdin(t, "sort", "b\na\nb\nc\na\n", "-u")
	wantSuccess(t, r)
	wantOut(t, r, "a\nb\nc\n")
}

func TestSort_OutputFile(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "in.txt", "c\na\nb\n")
	r := runIn(t, dir, "sort", "in.txt", "-o", "out.txt")
	wantSuccess(t, r)
	got := readFile(t, dir+"/out.txt")
	if got != "a\nb\nc\n" {
		t.Errorf("output file = %q, want %q", got, "a\nb\nc\n")
	}
}

func TestSort_OutputSameFile(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "in.txt", "c\na\nb\n")
	r := runIn(t, dir, "sort", "in.txt", "-o", "in.txt")
	wantSuccess(t, r)
	got := readFile(t, dir+"/in.txt")
	if got != "a\nb\nc\n" {
		t.Errorf("in-place output = %q, want %q", got, "a\nb\nc\n")
	}
}

func TestSort_Key(t *testing.T) {
	data := "999\t3\t0\talgebra\negg\t1\t2\tpapyrus\n7\t3\t42\tsoup\n42\t1\t3\twoot\n42\t1\t010\tzoology\n"
	r := runStdin(t, "sort", data, "-k4,4")
	wantSuccess(t, r)
	wantOut(t, r, "999\t3\t0\talgebra\negg\t1\t2\tpapyrus\n7\t3\t42\tsoup\n42\t1\t3\twoot\n42\t1\t010\tzoology\n")
}

func TestSort_CheckSorted(t *testing.T) {
	r := runStdin(t, "sort", "a\nb\nc\n", "-c")
	wantSuccess(t, r)
}

func TestSort_CheckUnsorted(t *testing.T) {
	r := runStdin(t, "sort", "a\nb\na\nc", "-c")
	wantCode(t, r, 1)
	wantErrContains(t, r, "3")
	wantErrContains(t, r, "disorder")
}

func TestSort_CheckUniqueSorted(t *testing.T) {
	r := runStdin(t, "sort", "a\nb\nc\n", "-uc")
	wantSuccess(t, r)
}

func TestSort_CheckUniqueDuplicate(t *testing.T) {
	r := runStdin(t, "sort", "a\nb\nb\nc", "-uc")
	wantCode(t, r, 1)
	wantErrContains(t, r, "3")
}

func TestSort_CheckSilentUnsorted(t *testing.T) {
	r := runStdin(t, "sort", "one\ntwo\nthree", "-C")
	wantCode(t, r, 1)
	wantOut(t, r, "")
}

func TestSort_CheckSilentSorted(t *testing.T) {
	r := runStdin(t, "sort", "a\nb\nc\n", "-C")
	wantSuccess(t, r)
}

func TestSort_Stable(t *testing.T) {
	r := runStdin(t, "sort", "b 2\na 1\nb 1\na 2\n", "-s", "-k1,1")
	wantSuccess(t, r)
	wantOut(t, r, "a 1\na 2\nb 2\nb 1\n")
}

func TestSort_CaseInsensitive(t *testing.T) {
	r := runStdin(t, "sort", "B\na\nC\n", "-f")
	wantSuccess(t, r)
	wantOut(t, r, "a\nB\nC\n")
}

func TestSort_NumericWithField(t *testing.T) {
	r := runStdin(t, "sort", "1,2,3,4\n2,3,4,1\n4,1,2,3\n3,4,1,2\n", "-t,", "-k3n")
	wantSuccess(t, r)
	wantOut(t, r, "3,4,1,2\n4,1,2,3\n1,2,3,4\n2,3,4,1\n")
}

func TestSort_Empty(t *testing.T) {
	r := runStdin(t, "sort", "")
	wantSuccess(t, r)
	wantOut(t, r, "")
}

func TestSort_MonthSort(t *testing.T) {
	r := runStdin(t, "sort", "MAR\nJAN\nDEC\nFEB\n", "-M")
	wantSuccess(t, r)
	wantOut(t, r, "JAN\nFEB\nMAR\nDEC\n")
}

func TestSort_CharOffset(t *testing.T) {
	r := runStdin(t, "sort", "a3b\na1b\na2b\n", "-k1.2,1.2")
	wantSuccess(t, r)
	wantOut(t, r, "a1b\na2b\na3b\n")
}

func TestSort_HumanNumeric(t *testing.T) {
	r := runStdin(t, "sort", "1M\n1K\n1G\n200\n", "-h")
	wantSuccess(t, r)
	wantOut(t, r, "200\n1K\n1M\n1G\n")
}

func TestSort_VersionSort(t *testing.T) {
	r := runStdin(t, "sort", "v1.10\nv1.2\nv1.1\n", "-V")
	wantSuccess(t, r)
	wantOut(t, r, "v1.1\nv1.2\nv1.10\n")
}

func TestSort_GeneralNumeric(t *testing.T) {
	r := runStdin(t, "sort", "1e3\n-2.5\n0\nNaN\ninf\n", "-g")
	wantSuccess(t, r)
	wantOut(t, r, "NaN\n-2.5\n0\n1e3\ninf\n")
}

func TestSort_DictOrder(t *testing.T) {
	r := runStdin(t, "sort", "b@c\na!b\na#c\n", "-d")
	wantSuccess(t, r)
	wantOut(t, r, "a!b\na#c\nb@c\n")
}

// Tests for --buffer-size error message fixes.

func TestSort_BufferSizeInvalidSuffix(t *testing.T) {
	// '8zz': last char 'z' but number portion '8z' is invalid → "invalid suffix".
	r := run(t, "sort", "-S", "8zz", "/dev/null")
	wantFail(t, r)
	wantErr(t, r, "sort: invalid suffix in --buffer-size argument '8zz'\n")
}

func TestSort_BufferSizeZTooLarge(t *testing.T) {
	// '8z': valid number + 'z' suffix → "too large".
	r := run(t, "sort", "-S", "8z", "/dev/null")
	wantFail(t, r)
	wantErr(t, r, "sort: --buffer-size argument '8z' too large\n")
}

func TestSort_BufferSizeInvalidArg(t *testing.T) {
	// 'asd': non-numeric without recognisable suffix → "invalid --buffer-size argument".
	r := run(t, "sort", "-S", "asd", "/dev/null")
	wantFail(t, r)
	wantErr(t, r, "sort: invalid --buffer-size argument 'asd'\n")
}

func TestSort_BufferSizeValidK(t *testing.T) {
	// '1K': valid size, sort should succeed on /dev/null.
	r := run(t, "sort", "-S", "1K", "/dev/null")
	wantSuccess(t, r)
}
