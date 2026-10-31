package inventory

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Parse files directly rather than using go list: build constraints must not
// hide an import, and tests are consumers too.
func TestShImportBoundary(t *testing.T) {
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "vendor" || d.Name() == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		inSh := strings.HasPrefix(rel, "internal/sh/")
		for _, imp := range f.Imports {
			s, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if s == "mvdan.cc/sh/v3" || strings.HasPrefix(s, "mvdan.cc/sh/v3/") {
				t.Errorf("%s imports external shell package %q", rel, s)
			}
			if inSh && (s == "C" || s == "mvdan.cc/sh" || strings.HasPrefix(s, "mvdan.cc/sh/") || (strings.HasPrefix(s, "github.com/AmadeusDE/Misa/") && !strings.HasPrefix(s, shNew))) {
				t.Errorf("%s imports forbidden %q", rel, s)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
