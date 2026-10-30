package inventory_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/AmadeusDE/Misa/internal/inventory"
	"github.com/AmadeusDE/Misa/internal/inventory/docgen"
)

func TestCommandInventoryDocFresh(t *testing.T) {
	inv, err := inventory.Load()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("../../docs/plans/command-inventory.md")
	if err != nil {
		t.Fatalf("read command inventory: %v; run go run ./internal/inventory/gen from the repository root", err)
	}
	if !bytes.Equal(got, docgen.Render(inv)) {
		t.Fatal("docs/plans/command-inventory.md is stale; run go run ./internal/inventory/gen from the repository root")
	}
}
