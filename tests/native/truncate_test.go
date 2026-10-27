package native

import (
	"os"
	"testing"
)

func TestTruncate_CreateSize(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "truncate", "-s", "100", "f.txt")
	wantSuccess(t, r)
	info, err := os.Stat(dir + "/f.txt")
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 100 {
		t.Errorf("size = %d, want 100", info.Size())
	}
}

func TestTruncate_Shrink(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "hello world")
	r := runIn(t, dir, "truncate", "-s", "5", "f.txt")
	wantSuccess(t, r)
	got := readFile(t, dir+"/f.txt")
	if got != "hello" {
		t.Errorf("truncated = %q, want %q", got, "hello")
	}
}

func TestTruncate_Grow(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "hi")
	r := runIn(t, dir, "truncate", "-s", "10", "f.txt")
	wantSuccess(t, r)
	info, err := os.Stat(dir + "/f.txt")
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 10 {
		t.Errorf("size = %d, want 10", info.Size())
	}
}

func TestTruncate_Zero(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "data")
	r := runIn(t, dir, "truncate", "-s", "0", "f.txt")
	wantSuccess(t, r)
	info, _ := os.Stat(dir + "/f.txt")
	if info.Size() != 0 {
		t.Errorf("size = %d, want 0", info.Size())
	}
}

func TestTruncate_NoSize(t *testing.T) {
	r := run(t, "truncate", "f.txt")
	wantFail(t, r)
}

func TestTruncate_Plus(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "hello")
	r := runIn(t, dir, "truncate", "-s", "+5", "f.txt")
	wantSuccess(t, r)
	info, _ := os.Stat(dir + "/f.txt")
	if info.Size() != 10 {
		t.Errorf("size = %d, want 10", info.Size())
	}
}

func TestTruncate_Minus(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "truncate", "-s", "100", "f.txt")
	wantSuccess(t, r)
	r = runIn(t, dir, "truncate", "-s", "-30", "f.txt")
	wantSuccess(t, r)
	info, _ := os.Stat(dir + "/f.txt")
	if info.Size() != 70 {
		t.Errorf("size = %d, want 70", info.Size())
	}
}

func TestTruncate_AtMostHit(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "truncate", "-s", "5000", "f.txt")
	wantSuccess(t, r)
	r = runIn(t, dir, "truncate", "-s", "<4000", "f.txt")
	wantSuccess(t, r)
	info, _ := os.Stat(dir + "/f.txt")
	if info.Size() != 4000 {
		t.Errorf("size = %d, want 4000", info.Size())
	}
}

func TestTruncate_AtMostMiss(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "truncate", "-s", "4000", "f.txt")
	wantSuccess(t, r)
	r = runIn(t, dir, "truncate", "-s", "<6000", "f.txt")
	wantSuccess(t, r)
	info, _ := os.Stat(dir + "/f.txt")
	if info.Size() != 4000 {
		t.Errorf("size = %d, want 4000", info.Size())
	}
}

func TestTruncate_AtLeastHit(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "truncate", "-s", "3000", "f.txt")
	wantSuccess(t, r)
	r = runIn(t, dir, "truncate", "-s", ">4000", "f.txt")
	wantSuccess(t, r)
	info, _ := os.Stat(dir + "/f.txt")
	if info.Size() != 4000 {
		t.Errorf("size = %d, want 4000", info.Size())
	}
}

func TestTruncate_AtLeastMiss(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "truncate", "-s", "4000", "f.txt")
	wantSuccess(t, r)
	r = runIn(t, dir, "truncate", "-s", ">2000", "f.txt")
	wantSuccess(t, r)
	info, _ := os.Stat(dir + "/f.txt")
	if info.Size() != 4000 {
		t.Errorf("size = %d, want 4000", info.Size())
	}
}

func TestTruncate_RoundDown(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "truncate", "-s", "7168", "f.txt")
	wantSuccess(t, r)
	r = runIn(t, dir, "truncate", "-s", "/3072", "f.txt")
	wantSuccess(t, r)
	info, _ := os.Stat(dir + "/f.txt")
	if info.Size() != 6144 {
		t.Errorf("size = %d, want 6144", info.Size())
	}
}

func TestTruncate_RoundUp(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "truncate", "-s", "7168", "f.txt")
	wantSuccess(t, r)
	r = runIn(t, dir, "truncate", "-s", "%3072", "f.txt")
	wantSuccess(t, r)
	info, _ := os.Stat(dir + "/f.txt")
	if info.Size() != 9216 {
		t.Errorf("size = %d, want 9216", info.Size())
	}
}

func TestTruncate_SuffixK(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "truncate", "-s", "1k", "f.txt")
	wantSuccess(t, r)
	info, _ := os.Stat(dir + "/f.txt")
	if info.Size() != 1024 {
		t.Errorf("size = %d, want 1024", info.Size())
	}
}

func TestTruncate_SuffixM(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "truncate", "-s", "1m", "f.txt")
	wantSuccess(t, r)
	info, _ := os.Stat(dir + "/f.txt")
	if info.Size() != 1048576 {
		t.Errorf("size = %d, want 1048576", info.Size())
	}
}

func TestTruncate_ReferenceAndIOBlocks(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "ref.txt", "1234567890")
	r := runIn(t, dir, "truncate", "-r", "ref.txt", "-s", "+5", "out.txt")
	wantSuccess(t, r)
	info, err := os.Stat(dir + "/out.txt")
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 15 {
		t.Errorf("size = %d, want 15", info.Size())
	}

	r2 := runIn(t, dir, "truncate", "-o", "-s", "2", "blk.txt")
	wantSuccess(t, r2)
}
