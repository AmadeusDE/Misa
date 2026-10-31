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

// processEffectViolations also catches references assigned to a function value.
// Dot imports are rejected because their identifiers cannot be resolved locally.
func processEffectViolations(f *ast.File, path string) []token.Pos {
	packages := map[string]string{}
	var violations []token.Pos
	for _, imp := range f.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		if p != "os" && p != "os/signal" && p != "syscall" && p != "golang.org/x/sys/unix" {
			continue
		}
		name := filepath.Base(p)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name == "." {
			violations = append(violations, imp.Pos())
		} else if name != "_" {
			packages[name] = p
		}
	}
	for _, decl := range f.Decls {
		fn, _ := decl.(*ast.FuncDecl)
		var allowed *ast.SelectorExpr
		if path == "procmon/htop.go" && fn != nil && fn.Name.Name == "adjustNice" {
			ast.Inspect(decl, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok || len(c.Args) != 3 {
					return true
				}
				s, ok := c.Fun.(*ast.SelectorExpr)
				if !ok || s.Sel.Name != "Setpriority" {
					return true
				}
				id, ok := s.X.(*ast.Ident)
				if !ok || packages[id.Name] != "syscall" {
					return true
				}
				a, ok := c.Args[0].(*ast.SelectorExpr)
				if !ok || a.Sel.Name != "PRIO_PROCESS" {
					return true
				}
				aid, ok := a.X.(*ast.Ident)
				if !ok || packages[aid.Name] != "syscall" {
					return true
				}
				p, ok := c.Args[1].(*ast.SelectorExpr)
				if !ok || p.Sel.Name != "PID" {
					return true
				}
				pid, ok := p.X.(*ast.Ident)
				if !ok || pid.Name != "p" {
					return true
				}
				allowed = s
				return true
			})
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			s, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			id, ok := s.X.(*ast.Ident)
			if !ok {
				return true
			}
			pkg := packages[id.Name]
			name := s.Sel.Name
			forbidden := pkg == "os" && name == "Chdir" || pkg == "os/signal" && name == "Ignore"
			if pkg == "syscall" || pkg == "golang.org/x/sys/unix" {
				switch name {
				case "Setpriority", "Umask", "Setrlimit", "Setuid", "Setgid", "Setgroups", "Chroot", "Setsid", "Setpgid", "Chdir":
					forbidden = true
				}
			}
			// Existing htop renice targets the selected PID, not pid 0.
			// It can nevertheless select the host itself; this exception is
			// deliberately limited to this one existing selector and is not
			// evidence of isolation from the host process.
			if forbidden && s == allowed {
				return true
			}
			if forbidden {
				violations = append(violations, s.Pos())
			}
			return true
		})
	}
	return violations
}

func TestBuiltinProcessEffectGuard(t *testing.T) {
	for _, dir := range []string{".", "../vi", "../mweb", "../procmon"} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range files {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			abs, err := filepath.Abs(path)
			if err != nil {
				t.Fatal(err)
			}
			key := filepath.Base(filepath.Dir(abs)) + "/" + filepath.Base(path)
			for _, pos := range processEffectViolations(f, key) {
				t.Errorf("%s: process-global effect must be isolated", fset.Position(pos))
			}
		}
	}
}

func TestProcessEffectGuardAliases(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want int
	}{
		{`package p; import s "syscall"; func f(){ s.Setpriority(0,0,0); s.Umask(0) }`, 2},
		{`package p; import u "golang.org/x/sys/unix"; var f = u.Setrlimit`, 1},
		{`package p; import o "os"; import sig "os/signal"; func f(){ o.Chdir("/"); sig.Ignore() }`, 2},
		{`package p; import . "syscall"; func f(){ Umask(0) }`, 1},
		{`package p; import "os"; func f(){ os.Getpid() }`, 0},
	} {
		f, err := parser.ParseFile(token.NewFileSet(), "fixture.go", tc.src, 0)
		if err != nil {
			t.Fatal(err)
		}
		if got := len(processEffectViolations(f, "fixture.go")); got != tc.want {
			t.Errorf("%s: got %d violations, want %d", tc.src, got, tc.want)
		}
	}
}
