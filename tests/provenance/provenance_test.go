// Package provenance tracks legacy test imports. A differing pin hash means
// only that the bytes differ, not that the local file was derived from that
// revision. Source license metadata does not establish individual fixture
// provenance or license. Unknown origins are unresolved reported locations.
package provenance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/inventory"
)

type entry struct {
	Path                 string `json:"path"`
	SHA256               string `json:"sha256"`
	Source               string `json:"source"`
	UpstreamPath         string `json:"upstream_path"`
	UpstreamResolvedPath string `json:"upstream_resolved_path,omitempty"`
	Kind                 string `json:"kind"`
	Verification         string `json:"verification"`
	UpstreamSHA256       string `json:"upstream_sha256,omitempty"`
	VerifiedRevision     string `json:"verified_revision,omitempty"`
}

// secureFile refuses symlinks in any path component, including links that
// happen to resolve inside the checkout. Never read through host escapes.
func secureFile(root, rel string) ([]byte, error) {
	if !fs.ValidPath(rel) || rel == "." {
		return nil, fmt.Errorf("invalid path %q", rel)
	}
	p := root
	for _, part := range strings.Split(rel, "/") {
		p = filepath.Join(p, part)
		st, err := os.Lstat(p)
		if err != nil {
			return nil, err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symlink in %s", rel)
		}
	}
	st, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file: %s", rel)
	}
	return os.ReadFile(p)
}

func validate(root string, entries []entry, expected []string, sources []inventory.Source) error {
	srcs := map[string]inventory.Source{}
	for _, s := range sources {
		srcs[s.Name] = s
	}
	remaining := map[string]bool{}
	for _, p := range expected {
		remaining[p] = true
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.Path] {
			return fmt.Errorf("duplicate entry: %s", e.Path)
		}
		seen[e.Path] = true
		if !remaining[e.Path] {
			return fmt.Errorf("stale or unexpected entry: %s", e.Path)
		}
		delete(remaining, e.Path)
		s, ok := srcs[e.Source]
		if !ok || s.URL == "" || s.License == "" {
			return fmt.Errorf("unknown or incomplete source: %s", e.Source)
		}
		// The excluded source is recorded solely to expose pre-existing derivations,
		// not to authorize further copying or resolve its disputed license.
		if s.Status != "approved" && !(e.Source == "uutils/diffutils" && s.Status == "excluded") {
			return fmt.Errorf("unapproved source: %s", e.Source)
		}
		if !fs.ValidPath(e.UpstreamPath) || e.UpstreamPath == "." {
			return fmt.Errorf("invalid upstream path: %s", e.UpstreamPath)
		}
		// When the pinned upstream path is a symlink, the stored upstream
		// digest is of this resolved file's contents, not the link bytes.
		if e.UpstreamResolvedPath != "" && (!fs.ValidPath(e.UpstreamResolvedPath) || e.UpstreamResolvedPath == "." || e.Verification == "unknown-origin") {
			return fmt.Errorf("invalid upstream resolved path: %s", e.Path)
		}
		if e.Kind != "legacy-copy" && e.Kind != "legacy-derivation" {
			return fmt.Errorf("invalid kind: %s", e.Kind)
		}
		switch e.Verification {
		case "unknown-origin":
			if e.UpstreamSHA256 != "" || e.VerifiedRevision != "" {
				return fmt.Errorf("unknown origin has pin evidence: %s", e.Path)
			}
		case "matches-pin", "modified-from-pin":
			digest, err := hex.DecodeString(e.UpstreamSHA256)
			if s.Status != "approved" || s.Revision == "" || e.VerifiedRevision != s.Revision || err != nil || len(digest) != sha256.Size || hex.EncodeToString(digest) != e.UpstreamSHA256 {
				return fmt.Errorf("invalid or stale pin evidence: %s", e.Path)
			}
			if (e.Verification == "matches-pin") != (e.UpstreamSHA256 == e.SHA256) {
				return fmt.Errorf("pin comparison inconsistent: %s", e.Path)
			}
		default:
			return fmt.Errorf("unsupported verification: %s", e.Verification)
		}
		data, err := secureFile(root, e.Path)
		if err != nil {
			return fmt.Errorf("%s: %w", e.Path, err)
		}
		hash := sha256.Sum256(data)
		if hex.EncodeToString(hash[:]) != e.SHA256 {
			return fmt.Errorf("hash changed: %s", e.Path)
		}
	}
	if len(remaining) != 0 {
		keys := make([]string, 0, len(remaining))
		for k := range remaining {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return fmt.Errorf("missing entry: %s", keys[0])
	}
	return nil
}

func fixturePaths(root string) ([]string, error) {
	var paths []string
	for _, base := range []string{"tests/toybox", "tests/uutils/testdata"} {
		err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(base)), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("symlink in fixture tree: %s", rel)
			}
			if d.IsDir() {
				return nil
			}
			if base == "tests/uutils/testdata" || path.Ext(rel) == ".test" || rel == "tests/toybox/LICENSE" {
				paths = append(paths, rel)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	// These two ports explicitly identify tests/integration.rs as their source;
	// other diffutils Go tests are locally authored and are not imports.
	paths = append(paths, "tests/diffutils/cmp/cmp_test.go", "tests/diffutils/diff/diff_test.go", "tests/diffutils/LICENSE")
	return paths, nil
}

func TestFixtureProvenance(t *testing.T) {
	data, err := os.ReadFile("fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var entries []entry
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&entries); err != nil {
		t.Fatal(err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		t.Fatalf("trailing manifest data: %v", err)
	}
	inv, err := inventory.Load()
	if err != nil {
		t.Fatal(err)
	}
	root := "../.."
	paths, err := fixturePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(root, entries, paths, inv.Sources); err != nil {
		t.Fatal(err)
	}
}

func TestValidationRejectsDrift(t *testing.T) {
	inv, err := inventory.Load()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "fixture"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("original"))
	e := entry{Path: "fixture", SHA256: hex.EncodeToString(sum[:]), Source: "landley/toybox", UpstreamPath: "tests/a.test", Kind: "legacy-copy", Verification: "unknown-origin"}
	check := func(entries []entry, paths []string, fail bool) {
		t.Helper()
		err := validate(root, entries, paths, inv.Sources)
		if (err != nil) != fail {
			t.Fatalf("validate error = %v, want failure %v", err, fail)
		}
	}
	check([]entry{e}, []string{"fixture"}, false)
	var revision string
	for _, s := range inv.Sources {
		if s.Name == e.Source {
			revision = s.Revision
		}
	}
	verified := e
	verified.Verification = "matches-pin"
	verified.UpstreamSHA256 = e.SHA256
	verified.VerifiedRevision = revision
	check([]entry{verified}, []string{"fixture"}, false)
	badPin := verified
	badPin.VerifiedRevision = "old-revision"
	check([]entry{badPin}, []string{"fixture"}, true)
	badPin = verified
	badPin.Verification = "modified-from-pin"
	check([]entry{badPin}, []string{"fixture"}, true)
	other := sha256.Sum256([]byte("upstream"))
	badPin.UpstreamSHA256 = hex.EncodeToString(other[:])
	check([]entry{badPin}, []string{"fixture"}, false)
	badPin.Verification = "matches-pin"
	check([]entry{badPin}, []string{"fixture"}, true)
	check(nil, []string{"fixture"}, true)
	check([]entry{e}, nil, true)
	check([]entry{e, e}, []string{"fixture"}, true)
	bad := e
	bad.Source = "unknown"
	check([]entry{bad}, []string{"fixture"}, true)
	bad = e
	bad.UpstreamPath = "../escape"
	check([]entry{bad}, []string{"fixture"}, true)
	if err := os.WriteFile(filepath.Join(root, "fixture"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	check([]entry{e}, []string{"fixture"}, true)
	if err := os.Remove(filepath.Join(root, "fixture")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "fixture")); err != nil {
		t.Fatal(err)
	}
	check([]entry{e}, []string{"fixture"}, true)
}
