package main

import (
	"errors"
	"go/build"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const modulePath = "github.com/AmadeusDE/Misa"

// windowsTags is the reduced feature set the Windows port builds first
// (docs/plans/windows.md). Default-feature Windows builds come with the agent and web gates.
var windowsTags = []string{"nough", "noweb"}

// windowsProduct is the explicit set of in-module packages the reduced Windows
// application imports, so it is also the set that must load and, in time, compile.
// It is derived from the import graph of the root package; a change here is a
// deliberate decision to grow or shrink the Windows product.
var windowsProduct = []string{
	".",
	"internal/ascii",
	"internal/battleship",
	"internal/builtins",
	"internal/chess",
	"internal/clipboard",
	"internal/color",
	"internal/companion",
	"internal/config",
	"internal/fileops",
	"internal/gitinfo",
	"internal/gradle",
	"internal/gui",
	"internal/help",
	"internal/jobleader",
	"internal/justfile",
	"internal/launcher",
	"internal/lscore",
	"internal/makefile",
	"internal/mines",
	"internal/pkgmanager",
	"internal/procmon",
	"internal/sshclient",
	"internal/sysinfo",
	"internal/term",
	"internal/termkeys",
	"internal/thefuck",
	"internal/ui",
	"internal/vi",
}

// windowsExcluded lists packages the Windows port leaves out, with the reason.
// None may appear in windowsProduct's import closure.
var windowsExcluded = map[string]string{
	"internal/wayland":     "Wayland wire protocol: SCM_RIGHTS and shm are Unix; no Windows files at all",
	"internal/gui/android": "Android ANativeWindow backend (cgo file plus session code for the APK)",
	"cmd/misagui":          "Android JNI library; a stub main elsewhere",
	"cmd/ughbot":           "ughbot daemon: outside the Windows port",
	"internal/ughbot":      "ughbot packages: outside the Windows port (prefix)",
}

type target struct{ goos, goarch string }

func (tg target) context(tags ...string) *build.Context {
	ctx := build.Default
	ctx.GOOS, ctx.GOARCH, ctx.CgoEnabled = tg.goos, tg.goarch, false
	ctx.BuildTags = tags
	return &ctx
}

// closure returns the in-module packages reachable from root and the first
// resolution failure, if any. withTests also follows test imports.
func closure(ctx *build.Context, root string, withTests bool) (pkgs []string, err error) {
	seen := map[string]bool{}
	var visit func(dir string) error
	visit = func(dir string) error {
		if seen[dir] {
			return nil
		}
		seen[dir] = true
		p, err := ctx.ImportDir(filepath.FromSlash(dir), 0)
		if err != nil {
			return errors.New(dir + ": " + err.Error())
		}
		imports := slices.Clone(p.Imports)
		if withTests {
			imports = append(append(imports, p.TestImports...), p.XTestImports...)
		}
		for _, imp := range imports {
			rel, ok := strings.CutPrefix(imp, modulePath+"/")
			if ok {
				if err := visit(rel); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visit(root); err != nil {
		return nil, err
	}
	for dir := range seen {
		pkgs = append(pkgs, dir)
	}
	slices.Sort(pkgs)
	return pkgs, nil
}

func hasGoFiles(ctx *build.Context, dir string) bool {
	_, err := ctx.ImportDir(filepath.FromSlash(dir), 0)
	var none *build.NoGoError
	return !errors.As(err, &none)
}

func excluded(pkg string) (string, bool) {
	for ex, why := range windowsExcluded {
		if pkg == ex || strings.HasPrefix(pkg, ex+"/") {
			return why, true
		}
	}
	return "", false
}

func TestWindowsProductPackagesLoad(t *testing.T) {
	for _, tg := range []target{{"windows", "amd64"}, {"windows", "arm64"}} {
		t.Run(tg.goos+"_"+tg.goarch, func(t *testing.T) {
			ctx := tg.context(windowsTags...)
			got, err := closure(ctx, ".", false)
			if err != nil {
				t.Fatalf("package loading: %v", err)
			}
			if !slices.Equal(got, windowsProduct) {
				t.Errorf("Windows product set drifted.\n got: %q\nwant: %q", got, windowsProduct)
			}
			if _, err := closure(ctx, ".", true); err != nil {
				t.Errorf("test package loading: %v", err)
			}
			for _, pkg := range got {
				if why, ok := excluded(pkg); ok {
					t.Errorf("%s is excluded (%s) but the product imports it", pkg, why)
				}
			}
		})
	}
}

func TestWindowsExcludesUnixOnlyPackages(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		ctx := target{"windows", arch}.context(windowsTags...)
		if hasGoFiles(ctx, "internal/wayland") {
			t.Errorf("windows/%s selects files in internal/wayland", arch)
		}
		for _, file := range []string{"wl_backend.go", "wl_window.go", "wl_input.go", "wl_keys.go", "wl_clipboard.go"} {
			ok, err := ctx.MatchFile("internal/gui", file)
			if err != nil || ok {
				t.Errorf("windows/%s selects internal/gui/%s (err %v)", arch, file, err)
			}
		}
		if ok, _ := ctx.MatchFile("internal/gui", "backend_other.go"); !ok {
			t.Errorf("windows/%s lacks the backend_other.go fallback", arch)
		}
		if ok, _ := ctx.MatchFile("cmd/misagui", "jni.go"); ok {
			t.Errorf("windows/%s selects the Android JNI file", arch)
		}
	}
}

func TestUnixSelectionsKeepTheirPackages(t *testing.T) {
	unixTargets := []target{
		{"linux", "amd64"}, {"android", "arm64"}, {"openbsd", "amd64"},
		{"freebsd", "amd64"}, {"darwin", "arm64"},
	}
	for _, tg := range unixTargets {
		ctx := tg.context()
		for _, dir := range []string{"internal/wayland", "internal/jobleader", "internal/term", "internal/gui"} {
			if !hasGoFiles(ctx, dir) {
				t.Errorf("%s/%s lost %s", tg.goos, tg.goarch, dir)
			}
		}
		if ok, _ := ctx.MatchFile("internal/jobleader", "jobleader.go"); !ok {
			t.Errorf("%s: jobleader.go not selected", tg.goos)
		}
		if ok, _ := ctx.MatchFile("internal/jobleader", "jobleader_other.go"); ok {
			t.Errorf("%s: the non-Unix jobleader stub is selected", tg.goos)
		}
		if _, err := closure(tg.context(), ".", true); err != nil {
			t.Errorf("%s/%s package loading: %v", tg.goos, tg.goarch, err)
		}
	}
}
