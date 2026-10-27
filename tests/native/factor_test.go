package native

import "testing"

func TestFactor_Small(t *testing.T) {
	r := run(t, "factor", "12")
	wantSuccess(t, r)
	wantOut(t, r, "12: 2 2 3\n")
}

func TestFactor_Prime(t *testing.T) {
	r := run(t, "factor", "7")
	wantSuccess(t, r)
	wantOut(t, r, "7: 7\n")
}

func TestFactor_One(t *testing.T) {
	r := run(t, "factor", "1")
	wantSuccess(t, r)
	wantOut(t, r, "1:\n")
}

func TestFactor_Two(t *testing.T) {
	r := run(t, "factor", "2")
	wantSuccess(t, r)
	wantOut(t, r, "2: 2\n")
}

func TestFactor_Multiple(t *testing.T) {
	r := run(t, "factor", "6", "8", "15")
	wantSuccess(t, r)
	wantOut(t, r, "6: 2 3\n8: 2 2 2\n15: 3 5\n")
}

func TestFactor_Large(t *testing.T) {
	r := run(t, "factor", "100")
	wantSuccess(t, r)
	wantOut(t, r, "100: 2 2 5 5\n")
}

func TestFactor_Stdin(t *testing.T) {
	r := runStdin(t, "factor", "42\n")
	wantSuccess(t, r)
	wantOut(t, r, "42: 2 3 7\n")
}

func TestFactor_StdinMultiple(t *testing.T) {
	r := runStdin(t, "factor", "6\n15\n")
	wantSuccess(t, r)
	wantOut(t, r, "6: 2 3\n15: 3 5\n")
}

func TestFactor_StdinWhitespace(t *testing.T) {
	r := runStdin(t, "factor", "6 15\n")
	wantSuccess(t, r)
	wantOut(t, r, "6: 2 3\n15: 3 5\n")
}

func TestFactor_Zero(t *testing.T) {
	r := run(t, "factor", "0")
	wantSuccess(t, r)
	wantOut(t, r, "0:\n")
}
