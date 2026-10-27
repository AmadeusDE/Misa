package native

import (
	"testing"
)

func TestChecksum_Md5sum(t *testing.T) {
	r := runStdin(t, "md5sum", "hello world\n")
	wantSuccess(t, r)
	wantOut(t, r, "6f5902ac237024bdd0c176cb93063dc4  -\n")
}

func TestChecksum_Sha1sum(t *testing.T) {
	r := runStdin(t, "sha1sum", "hello world\n")
	wantSuccess(t, r)
	wantOut(t, r, "22596363b3de40b06f981fb85d82312e8c0ed511  -\n")
}

func TestChecksum_CheckMode(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "foo.txt", "hello world\n")
	sumFile := mkFile(t, dir, "sums.txt", "22596363b3de40b06f981fb85d82312e8c0ed511  foo.txt\n")
	r := runIn(t, dir, "sha1sum", "-c", sumFile)
	wantSuccess(t, r)
	wantOut(t, r, "foo.txt: OK\n")
}

func TestChecksum_CheckModeBinaryMarker(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "foo.txt", "hello world\n")
	sumFile := mkFile(t, dir, "sums.txt", "22596363b3de40b06f981fb85d82312e8c0ed511 *foo.txt\n")
	r := runIn(t, dir, "sha1sum", "-c", sumFile)
	wantSuccess(t, r)
	wantOut(t, r, "foo.txt: OK\n")
}

func TestChecksum_CheckModeFailed(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "foo.txt", "wrong content\n")
	sumFile := mkFile(t, dir, "sums.txt", "22596363b3de40b06f981fb85d82312e8c0ed511  foo.txt\n")
	r := runIn(t, dir, "sha1sum", "-c", sumFile)
	wantCode(t, r, 1)
	wantOut(t, r, "foo.txt: FAILED\n")
}
