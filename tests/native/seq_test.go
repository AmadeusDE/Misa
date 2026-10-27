package native

import "testing"

func TestSeq_Basic(t *testing.T) {
	r := run(t, "seq", "5")
	wantSuccess(t, r)
	wantOut(t, r, "1\n2\n3\n4\n5\n")
}

func TestSeq_StartEnd(t *testing.T) {
	r := run(t, "seq", "3", "6")
	wantSuccess(t, r)
	wantOut(t, r, "3\n4\n5\n6\n")
}

func TestSeq_StartStepEnd(t *testing.T) {
	r := run(t, "seq", "1", "2", "9")
	wantSuccess(t, r)
	wantOut(t, r, "1\n3\n5\n7\n9\n")
}

func TestSeq_Descending(t *testing.T) {
	r := run(t, "seq", "5", "-1", "1")
	wantSuccess(t, r)
	wantOut(t, r, "5\n4\n3\n2\n1\n")
}

func TestSeq_Separator(t *testing.T) {
	r := run(t, "seq", "-s", ", ", "3")
	wantSuccess(t, r)
	wantOut(t, r, "1, 2, 3\n")
}

func TestSeq_EqualWidth(t *testing.T) {
	r := run(t, "seq", "-w", "8", "10")
	wantSuccess(t, r)
	wantOut(t, r, "08\n09\n10\n")
}

func TestSeq_SingleNumber(t *testing.T) {
	r := run(t, "seq", "1")
	wantSuccess(t, r)
	wantOut(t, r, "1\n")
}

func TestSeq_ZeroStep(t *testing.T) {
	r := run(t, "seq", "1", "0", "5")
	wantFail(t, r)
}

func TestSeq_Negative(t *testing.T) {
	r := run(t, "seq", "-3", "-1")
	wantSuccess(t, r)
	wantOut(t, r, "-3\n-2\n-1\n")
}

func TestSeq_Float(t *testing.T) {
	r := run(t, "seq", "0.5", "0.5", "2.0")
	wantSuccess(t, r)
	wantOutContains(t, r, "0.5")
	wantOutContains(t, r, "1")
	wantOutContains(t, r, "1.5")
	wantOutContains(t, r, "2")
}

func TestSeq_EmptyRange(t *testing.T) {
	r := run(t, "seq", "5", "1")
	wantSuccess(t, r)
	wantOut(t, r, "")
}

func TestSeq_FormatWidth(t *testing.T) {
	r := run(t, "seq", "-w", "1", "3")
	wantSuccess(t, r)
	wantOut(t, r, "1\n2\n3\n")
}

func TestSeq_HexFloatStep(t *testing.T) {
	r := run(t, "seq", "1", "0x1p-1", "2")
	wantSuccess(t, r)
	wantOut(t, r, "1\n1.5\n2\n")
}

func TestSeq_HexFloatSmall(t *testing.T) {
	r := run(t, "seq", "0xa.a9p-30", "0xa.a9p-30")
	wantSuccess(t, r)
	wantOutContains(t, r, "e-09")
}

func TestSeq_ScientificPrecision(t *testing.T) {
	r := run(t, "seq", "1e-2", "1e-2", "3e-2")
	wantSuccess(t, r)
	wantOut(t, r, "0.01\n0.02\n0.03\n")
}

func TestSeq_LargeIntPrecision(t *testing.T) {
	r := run(t, "seq", "-s,", "9007199254740991", "1", "9007199254740991")
	wantSuccess(t, r)
	wantOut(t, r, "9007199254740991\n")
}

func TestSeq_NegativeZeroStart(t *testing.T) {
	r := run(t, "seq", "-0", "1")
	wantSuccess(t, r)
	wantOut(t, r, "-0\n1\n")
}

func TestSeq_BigNumbers(t *testing.T) {
	r := run(t, "seq", "1000000000000000000000000000", "1000000000000000000000000001")
	wantSuccess(t, r)
	wantOut(t, r, "1000000000000000000000000000\n1000000000000000000000000001\n")
}

func TestSeq_FormatOption(t *testing.T) {
	r := run(t, "seq", "-f", "%g", "0", "0.987654321", "2")
	wantSuccess(t, r)
	wantOut(t, r, "0\n0.987654\n1.97531\n")
}
