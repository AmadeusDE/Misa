//go:build !nough

// Package register adds the aihelp command to the builtin registry. It is its own package, imported only by
// main, because internal/builtins must not depend on the AI stack.
package register

import (
	"github.com/AmadeusDE/Misa/internal/aihelp"
	"github.com/AmadeusDE/Misa/internal/builtins"
)

func init() {
	builtins.Register(builtins.Command{Name: "aihelp", Run: aihelp.Run})
}
