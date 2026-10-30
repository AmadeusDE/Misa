package builtins

import (
	"errors"
	"fmt"

	"github.com/AmadeusDE/Misa/internal/mines"
)

var minesSpec = mustSpec("b(beginner)i(intermediate)e(expert)(width)#(height)#m(mines)#")

func init() {
	Register(Command{Name: "mines", Run: runMines})
}

var errMinesInfo = errors.New("help or version requested")

// MinesLevel reads a board from mines' options: a named level, adjusted by
// --width, --height and --mines.
func MinesLevel(args []string) (mines.Level, error) {
	flags, err := minesSpec.ParseArgs(args)
	if err != nil {
		return mines.Level{}, err
	}
	if flags.HelpRequested || flags.VersionRequested {
		return mines.Level{}, errMinesInfo
	}
	level := mines.Levels[0]
	switch {
	case flags.Bool("e"):
		level = mines.Levels[2]
	case flags.Bool("i"):
		level = mines.Levels[1]
	}
	custom := false
	for _, opt := range []struct {
		key string
		dst *int
	}{{"width", &level.W}, {"height", &level.H}, {"m", &level.Mines}} {
		if n, ok := flags.Num(opt.key); ok {
			*opt.dst = int(n)
			if int64(*opt.dst) != n {
				*opt.dst = -1 // Ensure out-of-range int64 values fail validation on 32-bit systems.
			}
			custom = true
		}
	}
	if err := mines.ValidateLevel(level); err != nil {
		return mines.Level{}, err
	}
	if custom {
		level.Name = "Custom"
	}
	return level, nil
}

func runMines(ctx *Context) int {
	if flags, err := minesSpec.ParseArgs(ctx.Args); err == nil {
		if flags.HelpRequested {
			fmt.Fprint(ctx.Stdout, minesSpec.GenerateUsage(ctx.CmdName))
			fmt.Fprintln(ctx.Stdout, "Controls: space open, f flag, n new, 1-3 level, c custom, q quit")
			return 0
		}
		if HandleHelpVersion(ctx, flags, minesSpec) {
			return 0
		}
	}
	level, err := MinesLevel(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "mines:", err)
		return 1
	}
	if err := mines.Run(level); err != nil {
		fmt.Fprintln(ctx.Stderr, "mines:", err)
		return 1
	}
	return 0
}
