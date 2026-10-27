package native

import "testing"

func TestDirname_Simple(t *testing.T) {
	r := run(t, "dirname", "/usr/bin/sort")
	wantSuccess(t, r)
	wantOut(t, r, "/usr/bin\n")
}

func TestDirname_NoDir(t *testing.T) {
	r := run(t, "dirname", "hello")
	wantSuccess(t, r)
	wantOut(t, r, ".\n")
}

func TestDirname_Root(t *testing.T) {
	r := run(t, "dirname", "/")
	wantSuccess(t, r)
	wantOut(t, r, "/\n")
}

func TestDirname_TrailingSlash(t *testing.T) {
	r := run(t, "dirname", "/usr/bin/")
	wantSuccess(t, r)
	wantOut(t, r, "/usr\n")
}

func TestDirname_Multiple(t *testing.T) {
	r := run(t, "dirname", "/a/b", "/c/d")
	wantSuccess(t, r)
	wantOut(t, r, "/a\n/c\n")
}

func TestDirname_DotSlash(t *testing.T) {
	r := run(t, "dirname", "./file")
	wantSuccess(t, r)
	wantOut(t, r, ".\n")
}

func TestDirname_Nested(t *testing.T) {
	r := run(t, "dirname", "/a/b/c/d")
	wantSuccess(t, r)
	wantOut(t, r, "/a/b/c\n")
}

func TestDirname_Empty(t *testing.T) {
	r := run(t, "dirname")
	wantFail(t, r)
}
