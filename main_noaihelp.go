//go:build nough

package main

import (
	"fmt"

	"github.com/AmadeusDE/Misa/internal/builtins"
)

func init() {
	builtins.Register(builtins.Command{Name: "aihelp", Run: func(ctx *builtins.Context) int {
		fmt.Fprintln(ctx.Stderr, "aihelp: built without AI support (-tags nough)")
		return 1
	}})
}
