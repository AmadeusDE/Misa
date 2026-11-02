package imgview

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBrowserImageSignatures(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name, data string
		want       bool
	}{
		{"a.PNG", "\x89PNG\r\n\x1a\n", true}, {"a.jpg", "\xff\xd8\xff", true}, {"a.jpeg", "\xff\xd8\xff", true},
		{"a.gif", "GIF87a", true}, {"a.GIF", "GIF89a", true}, {"a.webp", "RIFF\x04\x00\x00\x00WEBP", true},
		{"a.bmp", "BM", true}, {"a.tif", "II\x2a\x00", true}, {"a.tiff", "MM\x00\x2a", true},
		{"a.png", "GIF89a", false}, {"a.txt", "GIF89a", false}, {"a.jpg", "not an image", false}, {"a.png", "\x89PNG", false},
	}
	for i, tc := range cases {
		p := filepath.Join(dir, string(rune('a'+i))+tc.name)
		if err := os.WriteFile(p, []byte(tc.data), 0600); err != nil {
			t.Fatal(err)
		}
		if got := IsImageFile(p, Limits{}); got != tc.want {
			t.Errorf("%s %q: got %v", tc.name, tc.data, got)
		}
	}
	p := filepath.Join(dir, "large.png")
	os.WriteFile(p, []byte("\x89PNG\r\n\x1a\n"), 0600)
	if IsImageFile(p, Limits{MaxEncodedBytes: 7}) {
		t.Error("oversize accepted")
	}
	if IsImageFile(filepath.Join(dir, "missing.png"), Limits{}) || IsImageFile(dir, Limits{}) {
		t.Error("non-file accepted")
	}
	if err := os.Chmod(p, 0); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() != 0 && IsImageFile(p, Limits{}) {
		t.Error("unreadable accepted")
	}
}

func TestCollectBrowserPaths(t *testing.T) {
	dir := t.TempDir()
	makeFile := func(name, contents string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	png := "\x89PNG\r\n\x1a\n"
	hidden := makeFile(".hidden.png", png)
	a := makeFile("img10.png", png)
	b := makeFile("img2.png", png)
	c := makeFile("img3.png", png)
	bad := makeFile("other.png", "bad")
	paths := []string{hidden, a, b, c, bad}
	tests := []struct {
		name     string
		selected string
		opts     CollectOptions
		want     []string
		index    int
		warning  bool
	}{
		{"lexical", b, CollectOptions{}, []string{a, b, c}, 1, false},
		{"file limit selected outside", c, CollectOptions{MaxFiles: 2}, []string{b, c}, 1, true},
		{"file limit selected inside", b, CollectOptions{MaxFiles: 2}, []string{a, b}, 1, true},
		{"scan limit", c, CollectOptions{MaxEntriesScanned: 2}, []string{b, c}, 1, true},
		{"hidden selected", hidden, CollectOptions{}, []string{hidden, a, b, c}, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			col, idx, err := CollectBrowserPaths(paths, tc.selected, tc.opts, Limits{})
			if err != nil || idx != tc.index || !reflect.DeepEqual(col.Paths, tc.want) || (col.Warning != "") != tc.warning {
				t.Fatalf("got %#v idx %d err %v", col, idx, err)
			}
		})
	}
	col, idx, err := CollectBrowserPaths([]string{a, b}, c, CollectOptions{MaxFiles: 2}, Limits{})
	if err != nil || idx != 1 || !reflect.DeepEqual(col.Paths, []string{b, c}) || col.Warning == "" {
		t.Fatalf("absent selected at capacity: %#v idx %d err %v", col, idx, err)
	}
	if _, idx, err := CollectBrowserPaths(paths, bad, CollectOptions{}, Limits{}); err != ErrNoImages || idx != -1 {
		t.Fatalf("invalid selected: %d %v", idx, err)
	}
}

func TestBrowserArgumentByteBudget(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	for i := 0; i < 400; i++ {
		p := filepath.Join(dir, fmt.Sprintf("%04d", i)+strings.Repeat("x", 190)+".png")
		if err := os.WriteFile(p, []byte("\x89PNG\r\n\x1a\n"), 0600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	selected := paths[200]
	col, index, err := CollectBrowserPaths(paths, selected, CollectOptions{}, Limits{})
	if err != nil || col.Warning == "" || col.Paths[index] != selected || index == 0 {
		t.Fatalf("selection/truncation: idx %d err %v warning %q", index, err, col.Warning)
	}
	bytes := len(selected) + 1 // --start repeats the selected path
	for i, p := range col.Paths {
		bytes += len(p) + 1
		if i > 0 && p <= col.Paths[i-1] {
			t.Fatal("not lexical")
		}
	}
	if bytes > 64*1024 {
		t.Fatalf("argv path bytes = %d", bytes)
	}
}

func TestCheckImageFileErrors(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.png")
	if ok, err := CheckImageFile(missing, Limits{}); ok || err == nil {
		t.Fatalf("missing: %v %v", ok, err)
	}
	if ok, err := CheckImageFile(filepath.Join(dir, "missing.txt"), Limits{}); ok || err != nil {
		t.Fatalf("unsupported: %v %v", ok, err)
	}
	p := filepath.Join(dir, "oversize.png")
	if err := os.WriteFile(p, []byte("\x89PNG\r\n\x1a\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if ok, err := CheckImageFile(p, Limits{MaxEncodedBytes: 7}); ok || !errors.Is(err, ErrLimit) {
		t.Fatalf("oversize: %v %v", ok, err)
	}
	if err := os.WriteFile(p, []byte("not png"), 0600); err != nil {
		t.Fatal(err)
	}
	if ok, err := CheckImageFile(p, Limits{}); ok || err != nil {
		t.Fatalf("ordinary nonimage: %v %v", ok, err)
	}
	if err := os.Chmod(p, 0); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() != 0 {
		if ok, err := CheckImageFile(p, Limits{}); ok || err == nil {
			t.Fatalf("unreadable: %v %v", ok, err)
		}
	}
}

func TestBrowserAbsentSelectionLexical(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	for _, n := range []string{"a.png", "b.png", "c.png", "d.png", "e.png"} {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte("\x89PNG\r\n\x1a\n"), 0600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	for _, idx := range []int{0, 2, 4} {
		ordered := append([]string{}, paths[:idx]...)
		ordered = append(ordered, paths[idx+1:]...)
		col, got, err := CollectBrowserPaths(ordered, paths[idx], CollectOptions{}, Limits{})
		if err != nil || got != idx || !reflect.DeepEqual(col.Paths, paths) {
			t.Fatalf("absent %d: %#v %d %v", idx, col, got, err)
		}
	}
}
