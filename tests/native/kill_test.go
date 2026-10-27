package native

import "testing"

func TestKill_List(t *testing.T) {
	r := run(t, "kill", "-l")
	wantSuccess(t, r)
	wantOutContains(t, r, "KILL")
	wantOutContains(t, r, "EXIT")
}

func TestKill_ListSpecificNumber(t *testing.T) {
	r := run(t, "kill", "-l", "9")
	wantSuccess(t, r)
	wantOut(t, r, "KILL\n")

	r2 := run(t, "kill", "-l", "128")
	wantSuccess(t, r2)
	wantOut(t, r2, "EXIT\n")
}

func TestKill_Table(t *testing.T) {
	r := run(t, "kill", "-t")
	wantSuccess(t, r)
	wantOutContains(t, r, "9 KILL")
	wantOutContains(t, r, "RTMAX")
}

func TestKill_NoPid(t *testing.T) {
	r := run(t, "kill")
	wantCode(t, r, 1)
	wantErrContains(t, r, "no process ID specified")
}

func TestKill_InvalidSignal(t *testing.T) {
	r := run(t, "kill", "-s", "IAMNOTASIGNAL", "1234")
	wantCode(t, r, 1)
	wantErrContains(t, r, "invalid signal")
}

func TestKill_NonExistentPid(t *testing.T) {
	r := run(t, "kill", "999999999")
	wantCode(t, r, 1)
	wantErrContains(t, r, "No such process")
}
