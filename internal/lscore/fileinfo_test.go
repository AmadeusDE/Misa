package lscore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCollectSymlinkToDirListsContentsByDefault(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "inside"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link-to-sub")
	if err := os.Symlink(sub, link); err != nil {
		t.Fatal(err)
	}

	files, err := Collect(link, CollectOpts{})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(files) != 1 || files[0].Name != "inside" {
		t.Fatalf("expected to list the symlinked directory's contents, got %+v", files)
	}
}

func TestCollectDanglingSymlinkWithoutDerefListsSelf(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "dangle")
	if err := os.Symlink("no-such-target", link); err != nil {
		t.Fatal(err)
	}

	files, err := Collect(link, CollectOpts{})
	if err != nil {
		t.Fatalf("Collect without DerefRoot should not error on a dangling symlink: %v", err)
	}
	if len(files) != 1 || !files[0].IsSymlink || !files[0].LinkBroken {
		t.Fatalf("expected a single broken-symlink entry, got %+v", files)
	}
}

func TestCollectDanglingSymlinkWithDerefRootFails(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "dangle")
	if err := os.Symlink("no-such-target", link); err != nil {
		t.Fatal(err)
	}

	_, err := Collect(link, CollectOpts{DerefRoot: true})
	if !errors.Is(err, ErrDerefFailed) {
		t.Fatalf("Collect with DerefRoot on a dangling symlink: got err=%v, want ErrDerefFailed", err)
	}
}

func TestCollectAllSynthesizesDotEntries(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "visible"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := Collect(dir, CollectOpts{All: true})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	var names []string
	for _, fi := range files {
		names = append(names, fi.Name)
	}
	if len(files) != 3 {
		t.Fatalf("expected \".\", \"..\" and \"visible\", got %v", names)
	}
	hasDot, hasDotDot, hasVisible := false, false, false
	for _, n := range names {
		switch n {
		case ".":
			hasDot = true
		case "..":
			hasDotDot = true
		case "visible":
			hasVisible = true
		}
	}
	if !hasDot || !hasDotDot || !hasVisible {
		t.Fatalf("missing expected entries, got %v", names)
	}
}

func TestCollectAlmostAllOmitsDotEntries(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "visible"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := Collect(dir, CollectOpts{AlmostAll: true})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(files) != 1 || files[0].Name != "visible" {
		t.Fatalf("expected only \"visible\" with AlmostAll, got %+v", files)
	}
}

func TestCollectEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	files, err := Collect(dir, CollectOpts{})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected no entries in an empty directory, got %+v", files)
	}
}

func TestIdentifyMatchesForSameDirectory(t *testing.T) {
	dir := t.TempDir()
	id1, ok1 := Identify(dir)
	id2, ok2 := Identify(dir)
	if !ok1 || !ok2 {
		t.Fatalf("Identify failed: ok1=%v ok2=%v", ok1, ok2)
	}
	if id1 != id2 {
		t.Fatalf("Identify should be stable for the same path: %+v != %+v", id1, id2)
	}
}
