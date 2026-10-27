package native

import (
	"os"
	"testing"
)

// TestMvJustCreated tests that mv refuses to overwrite just-created files
func TestMvJustCreated(t *testing.T) {
	dir := t.TempDir()

	// Create test structure: mkdir a b c; echo a > a/f; echo b > b/f
	os.Mkdir(dir+"/a", 0755)
	os.Mkdir(dir+"/b", 0755)
	os.Mkdir(dir+"/c", 0755)
	mkFile(t, dir+"/a", "f", "a")
	mkFile(t, dir+"/b", "f", "b")

	// Test: mv a/f b/f c
	// This should fail because b/f would overwrite c/f (just created from a/f)
	r := runIn(t, dir, "mv", "a/f", "b/f", "c")

	// Should fail
	wantFail(t, r)

	// Should have error message about just-created file
	wantErrContains(t, r, "will not overwrite just-created")

	// a/f should be gone (moved to c/f)
	if _, err := os.Stat(dir + "/a/f"); err == nil {
		t.Fatal("a/f should have been moved")
	}

	// b/f should still exist
	if _, err := os.Stat(dir + "/b/f"); err != nil {
		t.Fatal("b/f should still exist")
	}

	// c/f should exist with content from a/f
	content := readFile(t, dir+"/c/f")
	if content != "a" {
		t.Fatalf("c/f should contain 'a', got: %s", content)
	}
}

// TestMvHardLink tests that mv detects hard links and warns about duplicates
func TestMvHardLink(t *testing.T) {
	dir := t.TempDir()

	// Create test structure: touch a; ln a b
	mkFile(t, dir, "a", "")
	os.Link(dir+"/a", dir+"/b")
	os.Mkdir(dir+"/d", 0755)

	r := runIn(t, dir, "mv", "a", "b", "d")

	// Should succeed
	wantSuccess(t, r)

	// Should have warning about duplicate source
	wantErrContains(t, r, "warning")
	wantErrContains(t, r, "specified more than once")

	// Only one file should be in d (the other source was skipped as duplicate)
	entries, err := os.ReadDir(dir + "/d")
	if err != nil {
		t.Fatalf("Failed to read d: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Expected 1 file in d, got %d", len(entries))
	}
}

// TestMvDuplicateSourcePath tests behavior with same file via different paths
func TestMvDuplicateSourcePath(t *testing.T) {
	dir := t.TempDir()

	// Create test structure: touch a; mkdir b d
	mkFile(t, dir, "a", "")
	os.Mkdir(dir+"/d", 0755)

	// Test: mv ./a a d - first source ./a should move to d/a, second a should fail
	r := runIn(t, dir, "mv", "./a", "a", "d")

	// Should fail (second 'a' not found - already moved)
	wantFail(t, r)

	// Should have error about cannot stat 'a'
	wantErrContains(t, r, "cannot stat")
	wantErrContains(t, r, "a")

	// a should be gone (moved to d/a)
	if _, err := os.Stat(dir + "/a"); err == nil {
		t.Fatal("a should have been moved")
	}

	// d/a should exist
	if _, err := os.Stat(dir + "/d/a"); err != nil {
		t.Fatal("d/a should exist")
	}
}

// TestMvInteractiveYes tests interactive mode with 'y' response
func TestMvInteractiveYes(t *testing.T) {
	dir := t.TempDir()

	// Create test files
	mkFile(t, dir, "a", "a")
	mkFile(t, dir, "b", "b")

	// Test: mv -i a b with 'y' response from stdin
	r := runStdinIn(t, dir, "mv", "y\n", "-i", "a", "b")

	wantSuccess(t, r)

	// a should be gone, b should have content from a
	if _, err := os.Stat(dir + "/a"); err == nil {
		t.Fatal("a should be gone")
	}

	content := readFile(t, dir+"/b")
	if content != "a" {
		t.Fatalf("b should have content 'a', got: %s", content)
	}
}

// TestMvInteractiveNo tests interactive mode with 'n' response
func TestMvInteractiveNo(t *testing.T) {
	dir := t.TempDir()

	// Create test files
	mkFile(t, dir, "a", "a")
	mkFile(t, dir, "b", "b")

	// Test: mv -i a b with 'n' response from stdin
	r := runStdinIn(t, dir, "mv", "n\n", "-i", "a", "b")

	// Should fail
	if r.code == 0 {
		t.Fatal("mv -i should have failed with 'n' response")
	}

	// a should still exist
	if _, err := os.Stat(dir + "/a"); err != nil {
		t.Fatal("a should still exist")
	}

	// b should have original content
	content := readFile(t, dir+"/b")
	if content != "b" {
		t.Fatalf("b should still have content 'b', got: %s", content)
	}
}

// TestMvTrailingSlash tests trailing slash handling in destination
func TestMvTrailingSlash(t *testing.T) {
	dir := t.TempDir()

	// Create test structure: touch a; mkdir d
	mkFile(t, dir, "a", "")
	os.Mkdir(dir+"/d", 0755)

	// Test: mv a d/ - should work same as mv a d
	r := runIn(t, dir, "mv", "a", "d/")

	wantSuccess(t, r)

	// a should be gone
	if _, err := os.Stat(dir + "/a"); err == nil {
		t.Fatal("a should be gone")
	}

	// d/a should exist
	if _, err := os.Stat(dir + "/d/a"); err != nil {
		t.Fatal("d/a should exist")
	}
}
