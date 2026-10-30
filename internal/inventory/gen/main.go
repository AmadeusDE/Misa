// Command gen regenerates the human-readable command inventory from the JSON.
// Run it from the repository root: go run ./internal/inventory/gen.
package main

import (
	"fmt"
	"os"

	"github.com/AmadeusDE/Misa/internal/inventory"
	"github.com/AmadeusDE/Misa/internal/inventory/docgen"
)

func main() {
	inv, err := inventory.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "load inventory:", err)
		os.Exit(1)
	}
	if err := os.WriteFile("docs/plans/command-inventory.md", docgen.Render(inv), 0644); err != nil {
		fmt.Fprintln(os.Stderr, "write command inventory:", err)
		os.Exit(1)
	}
}
