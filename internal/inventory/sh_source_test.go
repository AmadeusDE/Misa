package inventory

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const shOld = "mvdan.cc/sh/v3/"
const shNew = "github.com/AmadeusDE/Misa/internal/sh/"

type shPatch struct {
	Kind   string   `json:"kind"`
	From   string   `json:"from"`
	To     string   `json:"to"`
	Files  []string `json:"files"`
	Reason string   `json:"reason"`
}
type shFile struct {
	Original       string `json:"original"`
	Local          string `json:"local"`
	UpstreamSHA256 string `json:"upstream_sha256"`
	LocalSHA256    string `json:"local_sha256"`
	Generated      bool   `json:"generated,omitempty"`
}
type shSource struct {
	Source       string    `json:"source"`
	URL          string    `json:"url"`
	Revision     string    `json:"revision"`
	License      string    `json:"license"`
	Packages     []string  `json:"packages"`
	Omissions    []string  `json:"omissions"`
	LocalPatches []shPatch `json:"local_patches"`
	Files        []shFile  `json:"files"`
}

var shSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

func shDigest(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func validShPath(p string) bool {
	return p != "" && p != "." && !strings.HasPrefix(p, "/") && path.Clean(p) == p && p != ".." && !strings.HasPrefix(p, "../") && !strings.Contains(p, `\`)
}

// checkShFiles verifies the imported bytes and the declared reversible patches.
func checkShFiles(dir string, s shSource) error {
	if len(s.LocalPatches) < 1 || len(s.LocalPatches) > 3 {
		return fmt.Errorf("invalid patches")
	}
	p := s.LocalPatches[0]
	if p.Kind != "import-path-rewrite" || p.From != shOld || p.To != shNew || len(p.Files) != 0 || p.Reason != "" {
		return fmt.Errorf("patch must be exactly the approved import-path rewrite")
	}
	gofmtPatch := len(s.LocalPatches) >= 2
	if gofmtPatch {
		p := s.LocalPatches[1]
		if p.Kind != "gofmt" || p.From != "" || p.To != "" || len(p.Files) != 1 || p.Files[0] != "interp/os_unix.go" || p.Reason == "" {
			return fmt.Errorf("invalid gofmt patch")
		}
	}
	sessionPatch := len(s.LocalPatches) == 3
	if sessionPatch {
		p := s.LocalPatches[2]
		if p.Kind != "session-support" || p.From != "" || p.To != "" || len(p.Files) != 2 || p.Files[0] != "syntax/parser.go" || p.Files[1] != "interp/session_status.go" || p.Reason == "" {
			return fmt.Errorf("invalid session patch")
		}
	}
	if len(s.Packages) == 0 || len(s.Omissions) == 0 {
		return fmt.Errorf("packages and omissions must be declared")
	}
	packages := map[string]bool{}
	for _, p := range s.Packages {
		if !validShPath(p) || packages[p] {
			return fmt.Errorf("invalid or duplicate package %q", p)
		}
		packages[p] = true
	}
	locals, originals, usedPackages := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, f := range s.Files {
		if !validShPath(f.Local) || !validShPath(f.Original) || f.Local != f.Original || locals[f.Local] || originals[f.Original] {
			return fmt.Errorf("invalid or duplicate mapping %q -> %q", f.Original, f.Local)
		}
		if f.Local != "LICENSE" {
			if !strings.HasSuffix(f.Local, ".go") || strings.HasSuffix(f.Local, "_test.go") || !packages[path.Dir(f.Local)] {
				return fmt.Errorf("invalid imported file %q", f.Local)
			}
			usedPackages[path.Dir(f.Local)] = true
		}
		locals[f.Local], originals[f.Original] = true, true
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f.Local)))
		if err != nil {
			return fmt.Errorf("read %s: %w", f.Local, err)
		}
		if shDigest(b) != f.LocalSHA256 {
			return fmt.Errorf("local hash mismatch: %s", f.Local)
		}
		upstream := bytes.ReplaceAll(b, []byte(shNew), []byte(shOld))
		if gofmtPatch && f.Local == "interp/os_unix.go" {
			canonical, err := format.Source(b)
			if err != nil || !bytes.Equal(canonical, b) {
				return fmt.Errorf("noncanonical gofmt-patched file: %s", f.Local)
			}
			upstream, err = format.Source(upstream)
			if err != nil {
				return fmt.Errorf("reverse gofmt %s: %w", f.Local, err)
			}
		}
		if sessionPatch && f.Local == "syntax/parser.go" {
			patched := `case "export", "readonly":
			if p.lang.in(LangPOSIX | langBashLike | LangMirBSDKorn | LangZsh) {
				p.declClause(s)
			}
		case "local", "typeset", "nameref":`
			original := `case "local", "export", "readonly", "typeset", "nameref":`
			if bytes.Count(upstream, []byte(patched)) != 1 {
				return fmt.Errorf("invalid POSIX declaration patch")
			}
			upstream = bytes.Replace(upstream, []byte(patched), []byte(original), 1)
		}
		if shDigest(upstream) != f.UpstreamSHA256 {
			return fmt.Errorf("upstream hash mismatch: %s", f.Local)
		}
	}
	if sessionPatch {
		b, err := os.ReadFile(filepath.Join(dir, "interp", "session_status.go"))
		if err != nil || shDigest(b) != "ebef9f14b6f9c008267d81aac77e8dc2c16e0585376b773f65a031235404deb9" {
			return fmt.Errorf("session status file missing or modified")
		}
	}
	if !locals["LICENSE"] {
		return fmt.Errorf("LICENSE missing")
	}
	if gofmtPatch && !locals["interp/os_unix.go"] {
		return fmt.Errorf("gofmt patch file missing")
	}
	for p := range packages {
		if !usedPackages[p] {
			return fmt.Errorf("package %s has no imported files", p)
		}
	}
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink not allowed: %s", p)
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		if sessionPatch && filepath.ToSlash(rel) == "interp/session_status.go" {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if shDigest(b) != "ebef9f14b6f9c008267d81aac77e8dc2c16e0585376b773f65a031235404deb9" {
				return fmt.Errorf("session status file mismatch")
			}
			return nil
		}
		if !locals[filepath.ToSlash(rel)] {
			return fmt.Errorf("unlisted Go file: %s", rel)
		}
		return nil
	})
}

func TestShSource(t *testing.T) {
	inv, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	var entry *Source
	for i := range inv.Sources {
		if inv.Sources[i].Name == "mvdan/sh" {
			if entry != nil {
				t.Fatal("duplicate mvdan/sh source")
			}
			entry = &inv.Sources[i]
		}
	}
	if entry == nil || entry.Status != "approved" || entry.URL != "https://github.com/mvdan/sh" || entry.License != "BSD-3-Clause" || !shSHA.MatchString(entry.ImportRevision) || entry.SourceMap != "internal/sh/SOURCE.json" {
		t.Fatalf("invalid approved mvdan/sh inventory entry: %+v", entry)
	}
	data, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(entry.SourceMap)))
	if err != nil {
		t.Fatal(err)
	}
	var source shSource
	if err := decode(data, &source); err != nil {
		t.Fatal(err)
	}
	if source.Source != entry.Name || source.URL != entry.URL || source.License != entry.License || source.Revision != entry.ImportRevision {
		t.Fatal("SOURCE.json does not match inventory")
	}
	if err := checkShFiles(filepath.Join("..", "sh"), source); err != nil {
		t.Fatal(err)
	}
}

func TestShFileChecker(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "syntax"), 0755); err != nil {
		t.Fatal(err)
	}
	original := []byte("package syntax\nimport \"" + shOld + "foo\"\n")
	local := bytes.ReplaceAll(original, []byte(shOld), []byte(shNew))
	write := func(p string, b []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, p), b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("LICENSE", []byte("license"))
	write("syntax/a.go", local)
	s := shSource{Packages: []string{"syntax"}, Omissions: []string{"tests"}, LocalPatches: []shPatch{{Kind: "import-path-rewrite", From: shOld, To: shNew}}, Files: []shFile{{Original: "LICENSE", Local: "LICENSE", UpstreamSHA256: shDigest([]byte("license")), LocalSHA256: shDigest([]byte("license"))}, {Original: "syntax/a.go", Local: "syntax/a.go", UpstreamSHA256: shDigest(original), LocalSHA256: shDigest(local), Generated: true}}}
	if err := checkShFiles(dir, s); err != nil {
		t.Fatal(err)
	}
	tampered := append(bytes.Clone(local), []byte("// changed\n")...)
	write("syntax/a.go", tampered)
	s.Files[1].LocalSHA256 = shDigest(tampered)
	if err := checkShFiles(dir, s); err == nil {
		t.Fatal("tampering with updated local hash accepted")
	}
	write("syntax/a.go", local)
	s.Files[1].LocalSHA256 = shDigest(local)
	write("syntax/extra.go", []byte("package syntax\n"))
	if err := checkShFiles(dir, s); err == nil {
		t.Fatal("unlisted file accepted")
	}
}

func TestShGofmtPatchChecker(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "interp"), 0755); err != nil {
		t.Fatal(err)
	}
	write := func(p string, b []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, p), b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	original, err := format.Source([]byte("package interp\nimport (\n\"golang.org/x/sys/unix\"\n\"" + shOld + "syntax\"\n)\n"))
	if err != nil {
		t.Fatal(err)
	}
	rewritten := bytes.ReplaceAll(original, []byte(shOld), []byte(shNew))
	local, err := format.Source(rewritten)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(local, rewritten) {
		t.Fatal("fixture must reorder imports")
	}
	write("LICENSE", []byte("license"))
	write("interp/os_unix.go", local)
	s := shSource{Packages: []string{"interp"}, Omissions: []string{"tests"}, LocalPatches: []shPatch{{Kind: "import-path-rewrite", From: shOld, To: shNew}, {Kind: "gofmt", Files: []string{"interp/os_unix.go"}, Reason: "import-path rewrite changes sorted order"}}, Files: []shFile{{Original: "LICENSE", Local: "LICENSE", UpstreamSHA256: shDigest([]byte("license")), LocalSHA256: shDigest([]byte("license"))}, {Original: "interp/os_unix.go", Local: "interp/os_unix.go", UpstreamSHA256: shDigest(original), LocalSHA256: shDigest(local)}}}
	if err := checkShFiles(dir, s); err != nil {
		t.Fatal(err)
	}
	for _, b := range [][]byte{append(bytes.Clone(local), []byte("// tampered\n")...), append(bytes.Clone(local), '\n')} {
		write("interp/os_unix.go", b)
		s.Files[1].LocalSHA256 = shDigest(b)
		if err := checkShFiles(dir, s); err == nil {
			t.Fatal("tampering with updated local hash accepted")
		}
	}
}

func TestShSessionPatchChecker(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "sh", "SOURCE.json"))
	if err != nil {
		t.Fatal(err)
	}
	var source shSource
	if err := decode(data, &source); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, f := range source.Files {
		b, err := os.ReadFile(filepath.Join("..", "sh", filepath.FromSlash(f.Local)))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(dir, filepath.FromSlash(f.Local))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	status := filepath.Join(dir, "interp", "session_status.go")
	b, err := os.ReadFile(filepath.Join("..", "sh", "interp", "session_status.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(status, b, 0644); err != nil {
		t.Fatal(err)
	}
	if err := checkShFiles(dir, source); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(status, append(bytes.Clone(b), '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	if checkShFiles(dir, source) == nil {
		t.Fatal("modified session extension accepted")
	}
	_ = os.WriteFile(status, b, 0644)
	parser := filepath.Join(dir, "syntax", "parser.go")
	b, err = os.ReadFile(parser)
	if err != nil {
		t.Fatal(err)
	}
	b = append(b, []byte("// tampered\n")...)
	if err := os.WriteFile(parser, b, 0644); err != nil {
		t.Fatal(err)
	}
	for i := range source.Files {
		if source.Files[i].Local == "syntax/parser.go" {
			source.Files[i].LocalSHA256 = shDigest(b)
		}
	}
	if checkShFiles(dir, source) == nil {
		t.Fatal("modified parser with updated local hash accepted")
	}
}
