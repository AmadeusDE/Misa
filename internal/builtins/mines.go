package builtins

import (
	"errors"
	"fmt"

	"github.com/AmadeusDE/Misa/internal/mines"
)

var minesSpec = mustSpec("b(beginner)i(intermediate)e(expert)(width)#(height)#m(mines)#p(percent)#")

func init() {
	Register(Command{Name: "mines", Run: runMines})
}

var errMinesInfo = errors.New("help or version requested")

// MinesLevel reads a board from mines' options: a named level, adjusted by
// --width, --height and --mines, or by --percent of the final board's cells.
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
	// --percent counts mines against the final board, so it is resolved after
	// --width and --height however the arguments were ordered.
	percent, hasPercent := flags.Num("p")
	if hasPercent {
		if _, hasMines := flags.Num("m"); hasMines {
			return mines.Level{}, fmt.Errorf("--mines and --percent are mutually exclusive")
		}
		custom = true
	}
	if err := mines.ValidateLevel(level); err != nil {
		return mines.Level{}, err
	}
	if hasPercent {
		pct := int(percent)
		if int64(pct) != percent {
			pct = -1 // Ensure out-of-range int64 values fail validation on 32-bit systems.
		}
		if err := mines.ValidatePercent(pct); err != nil {
			return mines.Level{}, err
		}
		level.Mines = mines.PercentMines(level.W, level.H, pct)
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
			fmt.Fprintln(ctx.Stdout, "  --percent counts mines as a percentage of the final board's cells (1-90); not with --mines")
			fmt.Fprintln(ctx.Stdout, "Controls: space open, f flag, n new, 1-3 level, c custom, q quit;")
			fmt.Fprintln(ctx.Stdout, "custom sets mines as a percent ([/] ±10, {/} or pgup/pgdown ±100, or click the buttons);")
			fmt.Fprintln(ctx.Stdout, "a board larger than the terminal scrolls with the cursor")
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
