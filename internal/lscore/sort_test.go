package lscore

import (
	"testing"
	"time"
)

func names(files []*FileInfo) []string {
	out := make([]string, len(files))
	for i, fi := range files {
		out[i] = fi.Name
	}
	return out
}

func assertOrder(t *testing.T, got []*FileInfo, want []string) {
	t.Helper()
	gotNames := names(got)
	if len(gotNames) != len(want) {
		t.Fatalf("got %v, want %v", gotNames, want)
	}
	for i := range want {
		if gotNames[i] != want[i] {
			t.Fatalf("got %v, want %v", gotNames, want)
		}
	}
}

// TestSortTimeReverseAppliesToWholeOrderIncludingTieBreak guards against a
// bug where combining -t (sort by time, newest first by default) with -r
// canceled out via a shared Reverse flag, silently reverting to plain
// ascending name order whenever all entries tied on modification time
// (coreutils ls/rt-1.sh).
func TestSortTimeReverseAppliesToWholeOrderIncludingTieBreak(t *testing.T) {
	same := time.Date(1998, 1, 15, 0, 0, 0, 0, time.UTC)
	files := []*FileInfo{
		{Name: "c", ModTime: same},
		{Name: "a", ModTime: same},
		{Name: "b", ModTime: same},
	}

	timeSorted := make([]*FileInfo, len(files))
	copy(timeSorted, files)
	Sort(timeSorted, SortOpts{Field: SortModTime})
	assertOrder(t, timeSorted, []string{"a", "b", "c"})

	reversed := make([]*FileInfo, len(files))
	copy(reversed, files)
	Sort(reversed, SortOpts{Field: SortModTime, Reverse: true})
	assertOrder(t, reversed, []string{"c", "b", "a"})
}

func TestSortSizeDefaultsLargestFirst(t *testing.T) {
	files := []*FileInfo{
		{Name: "small", Size: 1},
		{Name: "big", Size: 100},
		{Name: "mid", Size: 10},
	}
	Sort(files, SortOpts{Field: SortSize})
	assertOrder(t, files, []string{"big", "mid", "small"})
}

// TestSortDirsFirstGroupsSymlinksToDirectories guards against a bug where
// --group-directories-first failed to group a symlink pointing at a
// directory alongside real directories (coreutils ls/group-dirs.sh),
// because grouping used the raw (Lstat-based) IsDir flag instead of
// treating directory-resolving symlinks the same as real directories.
func TestSortDirsFirstGroupsSymlinksToDirectories(t *testing.T) {
	files := []*FileInfo{
		{Name: "a"},
		{Name: "bl", IsSymlink: true, LinksToDir: true},
		{Name: "b", IsDir: true},
	}
	Sort(files, SortOpts{DirsFirst: true})
	assertOrder(t, files, []string{"b", "bl", "a"})
}

func TestSortModTimeDefaultsNewestFirst(t *testing.T) {
	older := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	files := []*FileInfo{
		{Name: "old", ModTime: older},
		{Name: "new", ModTime: newer},
	}
	Sort(files, SortOpts{Field: SortModTime})
	assertOrder(t, files, []string{"new", "old"})
}

func TestSortByWidth(t *testing.T) {
	files := []*FileInfo{
		{Name: "aaaaa"},
		{Name: "zz"},
		{Name: "cccc"},
		{Name: "d"},
		{Name: "bbb"},
	}
	Sort(files, SortOpts{Field: SortWidth})

	var got []string
	for _, fi := range files {
		got = append(got, fi.Name)
	}
	want := []string{"d", "zz", "bbb", "cccc", "aaaaa"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
