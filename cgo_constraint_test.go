package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// import "C" is allowed only in files tagged android && cgo. GOOS=android
// also matches the linux build tag, so a bare import "C" would leak cgo
// onto every other target.
func TestImportCIsAndroidCgo(t *testing.T) {
	var bad []string
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "vendor" || name == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".c") || strings.HasSuffix(path, ".h") {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("read %s: %v", path, err)
				return nil
			}
			text := string(data)
			if strings.Contains(text, "jni.h") || strings.Contains(text, "native_window.h") {
				if !androidCgoConstraint(path) {
					bad = append(bad, path)
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Errorf("parse %s: %v", path, err)
			return nil
		}
		for _, imp := range f.Imports {
			if imp.Path.Value != `"C"` {
				continue
			}
			if !androidCgoConstraint(path) {
				bad = append(bad, path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) > 0 {
		t.Fatalf("import \"C\" without an android && cgo build constraint: %s", strings.Join(bad, ", "))
	}
}

func androidCgoConstraint(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "//") {
			if strings.Contains(trim, "go:build") || strings.Contains(trim, "+build") {
				if strings.Contains(trim, "android") && strings.Contains(trim, "cgo") {
					return true
				}
			}
			continue
		}
		break
	}
	return false
}
