package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
)

// browserReserved lists chords a browser keeps for itself: page JavaScript can
// never capture them over plain HTTP (tab, window and history management,
// reload, fullscreen, devtools). The TUI also runs in the web terminal, so it
// must bind none of them. Keys are written the way tea.KeyPressMsg.String()
// names them.
var browserReserved = map[string]bool{
	"ctrl+n": true, "ctrl+t": true, "ctrl+w": true,
	"ctrl+shift+n": true, "ctrl+shift+t": true, "ctrl+shift+w": true,
	"ctrl+tab": true, "ctrl+shift+tab": true, "ctrl+pgup": true, "ctrl+pgdown": true,
	"ctrl+1": true, "ctrl+2": true, "ctrl+3": true, "ctrl+4": true, "ctrl+5": true,
	"ctrl+6": true, "ctrl+7": true, "ctrl+8": true, "ctrl+9": true,
	"f5": true, "ctrl+f5": true, "f11": true, "f12": true,
	"ctrl+shift+q": true, "ctrl+shift+i": true, "ctrl+shift+j": true, "ctrl+shift+c": true,
	"ctrl+shift+delete": true, "ctrl+shift+r": true, "ctrl+shift+p": true,
	"alt+left": true, "alt+right": true, "alt+home": true,
}

// parseNonTestFiles parses the Go sources of every UI package that shares the TUI's keys.
func parseNonTestFiles(fset *token.FileSet) ([]*ast.File, error) {
	var files []*ast.File
	for _, dir := range []string{".", "../../webdevtui"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
			if err != nil {
				return nil, err
			}
			files = append(files, f)
		}
	}
	return files, nil
}

func TestNoBindingIsBrowserReserved(t *testing.T) {
	found := map[string]string{}
	fset := token.NewFileSet()
	files, err := parseNonTestFiles(fset)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CaseClause:
				for _, e := range n.List {
					if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						addKey(found, fset, lit)
					}
				}
			case *ast.CallExpr:
				if sel, ok := n.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "WithKeys" {
					for _, a := range n.Args {
						if lit, ok := a.(*ast.BasicLit); ok && lit.Kind == token.STRING {
							addKey(found, fset, lit)
						}
					}
				}
			}
			return true
		})
	}
	if len(found) == 0 {
		t.Fatal("scanner found no key bindings")
	}
	for k, where := range found {
		if browserReserved[k] {
			t.Errorf("%s binds browser-reserved chord %q", where, k)
		}
	}
}

func TestChatInputKeyMapAvoidsBrowserReserved(t *testing.T) {
	c := newChat(t.Context(), newFake(), 1, "a", 60, 20)
	km := reflect.ValueOf(c.input.KeyMap)
	for i := 0; i < km.NumField(); i++ {
		b, ok := km.Field(i).Interface().(key.Binding)
		if !ok {
			continue
		}
		for _, k := range b.Keys() {
			if browserReserved[k] {
				t.Errorf("textarea %s binds browser-reserved chord %q", km.Type().Field(i).Name, k)
			}
		}
	}
	vp := reflect.ValueOf(c.vp.KeyMap)
	for i := 0; i < vp.NumField(); i++ {
		if b, ok := vp.Field(i).Interface().(key.Binding); ok {
			for _, k := range b.Keys() {
				if browserReserved[k] {
					t.Errorf("viewport %s binds browser-reserved chord %q", vp.Type().Field(i).Name, k)
				}
			}
		}
	}
}

func addKey(found map[string]string, fset *token.FileSet, lit *ast.BasicLit) {
	if s, err := strconv.Unquote(lit.Value); err == nil {
		found[s] = fset.Position(lit.Pos()).String()
	}
}

func TestScannerSeesTheKnownBindings(t *testing.T) {
	// guards the scan itself: if these vanish the reserved check proves nothing
	fset := token.NewFileSet()
	files, err := parseNonTestFiles(fset)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			if cc, ok := n.(*ast.CaseClause); ok {
				for _, e := range cc.List {
					if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						addKey(found, fset, lit)
					}
				}
			}
			return true
		})
	}
	for _, want := range []string{"ctrl+o", "ctrl+c", "tab", "shift+tab", "[", "]", "pgup"} {
		if _, ok := found[want]; !ok {
			t.Errorf("scanner did not find %q", want)
		}
	}
}
