package inventory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShExpansionPatchValidation(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "sh", "SOURCE.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*shPatch)
		valid  bool
	}{
		{"overlap with dirs-vars", func(*shPatch) {}, true},
		{"duplicate file", func(p *shPatch) { p.Files[2] = p.Files[0] }, false},
		{"unknown file", func(p *shPatch) { p.Files[2] = "expand/braces.go" }, false},
		{"missing file", func(p *shPatch) { p.Files = p.Files[:3] }, false},
		{"unknown kind", func(p *shPatch) { p.Kind = "other" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var s shSource
			if err := decode(data, &s); err != nil {
				t.Fatal(err)
			}
			// Replace any expansion entry already added to the manifest, so this
			// test also works while the implementation and manifest evolve.
			var patches []shPatch
			for _, p := range s.LocalPatches {
				if p.Kind != "posix-expansion" {
					patches = append(patches, p)
				}
			}
			p := shPatch{Kind: "posix-expansion", Files: []string{"expand/expand.go", "expand/param.go", "expand/arith.go", "interp/runner.go"}, Reason: "expansion semantics"}
			for i := range s.Files {
				switch s.Files[i].Local {
				case "expand/expand.go", "expand/param.go", "expand/arith.go", "interp/runner.go":
					s.Files[i].Patched = true
				}
			}
			tc.mutate(&p)
			s.LocalPatches = append(patches, p)
			err := checkShFiles(filepath.Join("..", "sh"), s)
			if (err == nil) != tc.valid {
				t.Fatalf("checkShFiles: %v", err)
			}
		})
	}
}
