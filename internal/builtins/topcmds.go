package builtins

import (
	"fmt"

	"github.com/AmadeusDE/Misa/internal/procmon"
)

func init() {
	Register(Command{Name: "top", Run: runTop})
	Register(Command{Name: "htop", Run: runHtop})
	Register(Command{Name: "btop", Run: runBtop})
	Register(Command{Name: "bytop", Run: runBtop})
	Register(Command{Name: "bashtop", Run: runBtop})
	Register(Command{Name: "tm", Run: runTM})
}

func runTop(ctx *Context) int {
	if err := procmon.RunTop(); err != nil {
		fmt.Fprintln(ctx.Stderr, "top:", err)
		return 1
	}
	return 0
}

func runHtop(ctx *Context) int {
	if err := procmon.RunHtop(); err != nil {
		fmt.Fprintln(ctx.Stderr, "htop:", err)
		return 1
	}
	return 0
}

func runBtop(ctx *Context) int {
	if err := procmon.RunBtop(); err != nil {
		fmt.Fprintln(ctx.Stderr, "btop:", err)
		return 1
	}
	return 0
}

func runTM(ctx *Context) int {
	if err := procmon.RunTM(); err != nil {
		fmt.Fprintln(ctx.Stderr, "tm:", err)
		return 1
	}
	return 0
}
