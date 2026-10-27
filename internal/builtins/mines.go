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
		key, long string
		dst       *int
		min, max  int
	}{{"width", "width", &level.W, 2, 99}, {"height", "height", &level.H, 2, 99}, {"m", "mines", &level.Mines, 1, 9999}} {
		if n, ok := flags.Num(opt.key); ok {
			if int(n) < opt.min || int(n) > opt.max {
				return mines.Level{}, fmt.Errorf("--%s must be %d-%d", opt.long, opt.min, opt.max)
			}
			*opt.dst = int(n)
			custom = true
		}
	}
	if custom {
		level.Name = "Custom"
	}
	return level, nil
}

func runMines(ctx *Context) int {
	if flags, err := minesSpec.ParseArgs(ctx.Args); err == nil && HandleHelpVersion(ctx, flags, minesSpec) {
		return 0
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
