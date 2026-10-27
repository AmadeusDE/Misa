package native

import (
	"os"
	"strconv"
	"testing"
)

func TestId_DefaultInfo(t *testing.T) {
	r := run(t, "id")
	wantSuccess(t, r)
	wantOutContains(t, r, "uid=")
}

func TestId_User(t *testing.T) {
	r := run(t, "id", "-u")
	wantSuccess(t, r)
	wantOutContains(t, r, "\n")
}

func TestId_Group(t *testing.T) {
	r := run(t, "id", "-g")
	wantSuccess(t, r)
	wantOutContains(t, r, "\n")
}

func TestId_Groups(t *testing.T) {
	r := run(t, "id", "-G")
	wantSuccess(t, r)
	wantOutContains(t, r, "\n")
}

func TestId_NameOrRealWithoutSpec(t *testing.T) {
	r := run(t, "id", "-n")
	wantCode(t, r, 1)
	wantErrContains(t, r, "requires -u, -g, or -G")
}

func TestId_ZeroWithoutSpec(t *testing.T) {
	r := run(t, "id", "-z")
	wantCode(t, r, 1)
	wantErrContains(t, r, "not permitted in default format")
}

func TestId_ZeroUser(t *testing.T) {
	r := run(t, "id", "-u", "-z")
	wantSuccess(t, r)
	wantOutContains(t, r, "\x00")
}

func TestId_PrettyConflict(t *testing.T) {
	r := run(t, "id", "-p", "-P")
	wantCode(t, r, 1)
	wantErrContains(t, r, "cannot be used with")
}

func TestId_NumericUid(t *testing.T) {
	uid := strconv.Itoa(os.Getuid())
	r := run(t, "id", uid)
	wantSuccess(t, r)
	wantOutContains(t, r, "uid="+uid)
}

func TestId_UnknownNumericUser(t *testing.T) {
	r := run(t, "id", "2147483647")
	wantCode(t, r, 1)
	wantErrContains(t, r, "no such user")
}
