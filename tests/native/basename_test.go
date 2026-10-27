package native

import "testing"

func TestBasename_Simple(t *testing.T) {
	r := run(t, "basename", "/usr/bin/sort")
	wantSuccess(t, r)
	wantOut(t, r, "sort\n")
}

func TestBasename_NoDir(t *testing.T) {
	r := run(t, "basename", "hello")
	wantSuccess(t, r)
	wantOut(t, r, "hello\n")
}

func TestBasename_Suffix(t *testing.T) {
	r := run(t, "basename", "/usr/local/bin/file.txt", ".txt")
	wantSuccess(t, r)
	wantOut(t, r, "file\n")
}

func TestBasename_SuffixS(t *testing.T) {
	r := run(t, "basename", "-s", ".txt", "/path/file.txt")
	wantSuccess(t, r)
	wantOut(t, r, "file\n")
}

func TestBasename_TrailingSlash(t *testing.T) {
	r := run(t, "basename", "/usr/bin/")
	wantSuccess(t, r)
	wantOut(t, r, "bin\n")
}

func TestBasename_Root(t *testing.T) {
	r := run(t, "basename", "/")
	wantSuccess(t, r)
	wantOut(t, r, "/\n")
}

func TestBasename_Multiple(t *testing.T) {
	r := run(t, "basename", "-a", "/usr/bin/sort", "/usr/bin/cat")
	wantSuccess(t, r)
	wantOut(t, r, "sort\ncat\n")
}

func TestBasename_MultipleSuffix(t *testing.T) {
	r := run(t, "basename", "-s", ".go", "-a", "main.go", "test.go")
	wantSuccess(t, r)
	wantOut(t, r, "main\ntest\n")
}

func TestBasename_DotFile(t *testing.T) {
	r := run(t, "basename", "/home/user/.bashrc")
	wantSuccess(t, r)
	wantOut(t, r, ".bashrc\n")
}

func TestBasename_Empty(t *testing.T) {
	r := run(t, "basename")
	wantFail(t, r)
}
