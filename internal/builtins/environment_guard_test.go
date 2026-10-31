package builtins

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Environment access belongs to the invocation, except the explicit inherited
// fallback and web server entrypoints that have no builtin invocation.
func TestBuiltinEnvironmentAccessGuard(t *testing.T) {
	allowed := map[string]bool{
		"builtins/invocation.go:Environ":   true,
		"builtins/invocation.go:LookupEnv": true,
		"mweb/run.go:Run":                  true,
		"mweb/paths.go:StateDir":           true,
	}
	for _, dir := range []string{".", "../vi", "../mweb"} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range files {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			osName := "os"
			for _, imp := range file.Imports {
				if name, _ := strconv.Unquote(imp.Path.Value); name == "os" && imp.Name != nil {
					osName = imp.Name.Name
					if osName == "." {
						t.Errorf("%s: dot import of os bypasses environment guard", path)
					}
				}
			}
			abs, err := filepath.Abs(path)
			if err != nil {
				t.Fatal(err)
			}
			key := filepath.Base(filepath.Dir(abs)) + "/" + filepath.Base(path)
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if ok && allowed[key+":"+fn.Name.Name] {
					continue
				}
				ast.Inspect(decl, func(n ast.Node) bool {
					sel, ok := n.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					pkg, ok := sel.X.(*ast.Ident)
					if !ok || pkg.Name != osName {
						return true
					}
					switch sel.Sel.Name {
					case "Getenv", "LookupEnv", "Environ", "Setenv", "Unsetenv", "Clearenv":
						t.Errorf("%s: use invocation environment, not os.%s", fset.Position(sel.Pos()), sel.Sel.Name)
					}
					return true
				})
			}
		}
	}
}
