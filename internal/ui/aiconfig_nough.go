//go:build nough

package ui

import (
	"fmt"

	"github.com/AmadeusDE/Misa/internal/builtins"
)

func init() {
	builtins.Register(builtins.Command{Name: "aiconfig", Run: func(ctx *builtins.Context) int {
		fmt.Fprintln(ctx.Stderr, "aiconfig: "+noAIMessage)
		return 1
	}})
}
