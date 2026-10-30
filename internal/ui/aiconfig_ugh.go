//go:build !nough

package ui

import (
	"fmt"

	"github.com/AmadeusDE/Misa/internal/aiconfig"
	"github.com/AmadeusDE/Misa/internal/builtins"
	"github.com/AmadeusDE/Misa/internal/procmon"
)

func init() {
	newAIConfig = func() procmon.Viewer { return aiconfig.New() }
	builtins.Register(builtins.Command{Name: "aiconfig", Run: runAIConfig})
}

// runAIConfig is `misa aiconfig` as a multicall: the configurator on the whole terminal.
func runAIConfig(ctx *builtins.Context) int {
	if len(ctx.Args) > 0 {
		fmt.Fprintln(ctx.Stderr, "usage: aiconfig (takes no arguments)")
		return 2
	}
	if err := aiconfig.Run(); err != nil {
		fmt.Fprintln(ctx.Stderr, "aiconfig:", err)
		return 1
	}
	return 0
}
