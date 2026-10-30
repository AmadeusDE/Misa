package docgen

import (
	"bytes"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/inventory"
)

func TestRender(t *testing.T) {
	inv := &inventory.Inventory{
		Commands: []inventory.Command{
			{Name: "z", Build: "linux || darwin", Sources: []string{"one.go", "two.go"}},
			{Name: "a", Kind: "alias", AliasOf: "z", Provenance: "line one\nline two"},
		},
		UIOnly:  []inventory.UIOnly{{Name: "z"}, {Name: "a"}},
		Sources: []inventory.Source{{Name: "z"}, {Name: "a"}},
	}
	first := Render(inv)
	if !bytes.Equal(first, Render(inv)) {
		t.Fatal("rendering is not deterministic")
	}
	text := string(first)
	for _, want := range []string{"linux \\|\\| darwin", "one.go<br>two.go", "line one<br>line two", "| a | alias | z |"} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered document missing %q", want)
		}
	}
	if strings.Index(text, "| a | alias") > strings.Index(text, "| z |  |") {
		t.Error("commands are not sorted by name")
	}
	if inv.Commands[0].Name != "z" || inv.UIOnly[0].Name != "z" || inv.Sources[0].Name != "z" {
		t.Error("Render mutated its input")
	}
}
