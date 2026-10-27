package native

import "testing"

func TestTee_Basic(t *testing.T) {
	dir := t.TempDir()
	r := runStdinIn(t, dir, "tee", "hello\n", dir+"/out.txt")
	wantSuccess(t, r)
	wantOut(t, r, "hello\n")
	got := readFile(t, dir+"/out.txt")
	if got != "hello\n" {
		t.Errorf("file content = %q, want %q", got, "hello\n")
	}
}

func TestTee_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	r := runStdinIn(t, dir, "tee", "data\n", dir+"/a.txt", dir+"/b.txt")
	wantSuccess(t, r)
	wantOut(t, r, "data\n")
	for _, name := range []string{"a.txt", "b.txt"} {
		got := readFile(t, dir+"/"+name)
		if got != "data\n" {
			t.Errorf("%s content = %q, want %q", name, got, "data\n")
		}
	}
}

func TestTee_Append(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "out.txt", "old\n")
	r := runStdinIn(t, dir, "tee", "new\n", "-a", dir+"/out.txt")
	wantSuccess(t, r)
	got := readFile(t, dir+"/out.txt")
	if got != "old\nnew\n" {
		t.Errorf("appended content = %q, want %q", got, "old\nnew\n")
	}
}

func TestTee_NoFiles(t *testing.T) {
	r := runStdin(t, "tee", "passthrough\n")
	wantSuccess(t, r)
	wantOut(t, r, "passthrough\n")
}
