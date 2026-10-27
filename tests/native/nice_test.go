package native

import "testing"

func TestNice_GetNiceness(t *testing.T) {
	r := run(t, "nice")
	wantSuccess(t, r)
}

func TestNice_RunCommand(t *testing.T) {
	r := run(t, "nice", "echo", "hello")
	wantSuccess(t, r)
	wantOut(t, r, "hello\n")
}

func TestNice_Adjustment(t *testing.T) {
	r := run(t, "nice", "-n", "5", "echo", "hello")
	wantSuccess(t, r)
	wantOut(t, r, "hello\n")
}

func TestNice_BareAdjustment(t *testing.T) {
	r := run(t, "nice", "-5", "echo", "hello")
	wantSuccess(t, r)
	wantOut(t, r, "hello\n")
}

func TestNice_InvalidArgument(t *testing.T) {
	r := run(t, "nice", "--invalid")
	wantCode(t, r, 125)
}

func TestNice_AdjustmentWithoutCommand(t *testing.T) {
	r := run(t, "nice", "-n", "10")
	wantCode(t, r, 125)
	wantErrContains(t, r, "nice:")
}
