package native

import "testing"

func TestPathchk_Valid(t *testing.T) {
	r := run(t, "pathchk", "/usr/bin/sort")
	wantSuccess(t, r)
}

func TestPathchk_Empty(t *testing.T) {
	r := run(t, "pathchk", "")
	wantFail(t, r)
}

func TestPathchk_Portable(t *testing.T) {
	r := run(t, "pathchk", "-p", "valid-name")
	wantSuccess(t, r)
}

func TestPathchk_PortableInvalid(t *testing.T) {
	r := run(t, "pathchk", "-p", "has space")
	wantFail(t, r)
}

func TestPathchk_Multiple(t *testing.T) {
	r := run(t, "pathchk", "/usr", "/tmp")
	wantSuccess(t, r)
}
