package builtins_test

import (
	"fmt"
	"go/build"
	"go/build/constraint"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/builtins"
	"github.com/AmadeusDE/Misa/internal/inventory"
	_ "github.com/AmadeusDE/Misa/internal/vi"
)

// projectTags is populated by files with the same constraints as the shipped binary.
var projectTags = map[string]bool{"nough": false, "nogui": false, "noweb": false}

// matchesBuild rejects unknown tags instead of treating a misspelled constraint as
// false. The standard toolchain's platform, feature and release tags remain valid.
func matchesBuild(expr string) (bool, error) {
	if expr == "all" {
		return true, nil
	}
	x, err := constraint.Parse("//go:build " + expr)
	if err != nil {
		return false, err
	}
	ctx := build.Default
	known := map[string]bool{}
	active := map[string]bool{}
	for _, s := range strings.Fields("aix android darwin dragonfly freebsd hurd illumos ios js linux netbsd openbsd plan9 solaris wasip1 windows zos 386 amd64 amd64p32 arm armbe arm64 arm64be loong64 mips mipsle mips64 mips64le mips64p32 mips64p32le ppc ppc64 ppc64le riscv riscv64 s390 s390x sparc sparc64 wasm unix gc gccgo cgo") {
		known[s] = true
	}
	for _, s := range append(append([]string{}, ctx.ReleaseTags...), ctx.ToolTags...) {
		known[s] = true
		active[s] = true
	}
	for s, enabled := range projectTags {
		known[s] = true
		active[s] = enabled
	}
	active[runtime.GOOS], active[runtime.GOARCH], active[ctx.Compiler] = true, true, true
	active["cgo"] = ctx.CgoEnabled
	switch runtime.GOOS {
	case "android":
		active["linux"] = true
	case "ios":
		active["darwin"] = true
	case "illumos":
		active["solaris"] = true
	}
	switch runtime.GOOS {
	case "aix", "android", "darwin", "dragonfly", "freebsd", "hurd", "illumos", "ios", "linux", "netbsd", "openbsd", "solaris":
		active["unix"] = true
	}
	var unknown []string
	var check func(constraint.Expr)
	check = func(e constraint.Expr) {
		switch e := e.(type) {
		case *constraint.TagExpr:
			if !known[e.Tag] {
				unknown = append(unknown, e.Tag)
			}
		case *constraint.NotExpr:
			check(e.X)
		case *constraint.AndExpr:
			check(e.X)
			check(e.Y)
		case *constraint.OrExpr:
			check(e.X)
			check(e.Y)
		}
	}
	check(x)
	result := x.Eval(func(tag string) bool { return active[tag] })
	if len(unknown) > 0 {
		return false, fmt.Errorf("unknown build tags %v (add test helpers for supported project tags)", unknown)
	}
	return result, nil
}

func TestRegistryInventory(t *testing.T) {
	t.Cleanup(func() {
		if t.Failed() {
			t.Log("Update internal/inventory/commands.json to match the shipped registry and its aliases.")
		}
	})
	inv, err := inventory.Load()
	if err != nil {
		t.Fatal(err)
	}
	entries := make(map[string]inventory.Command, len(inv.Commands))
	enabled := make(map[string]bool, len(inv.Commands))
	for _, c := range inv.Commands {
		if c.Name == "" {
			t.Error("inventory contains empty command name")
			continue
		}
		if _, exists := entries[c.Name]; exists {
			t.Errorf("duplicate inventory command %q", c.Name)
			continue
		}
		entries[c.Name] = c
		match, err := matchesBuild(c.Build)
		if err != nil {
			t.Errorf("%s: invalid build constraint %q: %v", c.Name, c.Build, err)
		}
		enabled[c.Name] = match
		switch c.Kind {
		case "builtin":
			if c.AliasOf != "" {
				t.Errorf("%s: builtin has alias_of %q", c.Name, c.AliasOf)
			}
		case "alias":
			if c.AliasOf == "" || c.AliasOf == c.Name {
				t.Errorf("%s: alias must name a different canonical command", c.Name)
			}
		default:
			t.Errorf("%s: unknown kind %q", c.Name, c.Kind)
		}
	}
	for _, c := range inv.Commands {
		if c.Kind != "alias" {
			continue
		}
		target, ok := entries[c.AliasOf]
		if !ok || target.Kind != "builtin" {
			t.Errorf("%s: alias target %q is not a canonical builtin", c.Name, c.AliasOf)
			continue
		}
		if enabled[c.Name] && !enabled[c.AliasOf] {
			t.Errorf("%s: alias is enabled but target %s is not", c.Name, c.AliasOf)
		}
		// Function pointers catch differing handler code, but cannot distinguish
		// closures sharing code with different captured state.
		if enabled[c.Name] && enabled[c.AliasOf] {
			alias, aok := builtins.Lookup(c.Name)
			canonical, cok := builtins.Lookup(c.AliasOf)
			if aok && cok && reflect.ValueOf(alias.Run).Pointer() != reflect.ValueOf(canonical.Run).Pointer() {
				t.Errorf("%s: alias handler differs from %s", c.Name, c.AliasOf)
			}
		}
	}
	for _, name := range builtins.Names() {
		if _, ok := entries[name]; !ok {
			t.Errorf("registered command %q missing from inventory", name)
		} else if !enabled[name] {
			t.Errorf("registered command %q excluded by inventory build constraint", name)
		}
		c, ok := builtins.Lookup(name)
		if !ok || c.Name != name || c.Run == nil {
			t.Errorf("invalid registry entry %q", name)
		}
	}
	for name, match := range enabled {
		if match {
			if _, ok := builtins.Lookup(name); !ok {
				t.Errorf("inventory command %q is not registered for this build", name)
			}
		}
	}
}

func TestMatchesBuild(t *testing.T) {
	for _, tc := range []struct {
		expr string
		want bool
		bad  bool
	}{
		{"all", true, false},
		{runtime.GOOS + " && " + runtime.GOARCH, true, false},
		{"!" + runtime.GOOS, false, false},
		{"noweb", projectTags["noweb"], false},
		{"nough || nogui", projectTags["nough"] || projectTags["nogui"], false},
		{"", false, true},
		{"all || misspelledtag", false, true},
		{runtime.GOOS + " || misspelledtag", false, true},
	} {
		got, err := matchesBuild(tc.expr)
		if (err != nil) != tc.bad || (!tc.bad && got != tc.want) {
			t.Errorf("matchesBuild(%q) = %v, %v; want %v, bad=%v", tc.expr, got, err, tc.want, tc.bad)
		}
	}
}
