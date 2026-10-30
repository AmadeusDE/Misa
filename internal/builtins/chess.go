package builtins

import (
	"errors"
	"fmt"
	"time"

	"github.com/AmadeusDE/Misa/internal/chess"
)

var chessSpec = mustSpec("l(list)f(file):(fen):c(computer):(think)#(level):")

// chessSides maps --computer to the sides the computer plays (White, Black).
var chessSides = map[string][2]bool{
	"black": {false, true}, "white": {true, false}, "both": {true, true}, "none": {false, false},
}

func init() {
	Register(Command{Name: "chess", Run: runChess})
}

var errChessNoGame = errors.New("no game requested")

// ChessGame sets up a game from chess' arguments: a variant by name (or a
// variant file with --file), optionally from a --fen position, against the
// computer (--computer white|black|both|none, --level, --think seconds).
func ChessGame(args []string) (*chess.Game, error) {
	flags, err := chessSpec.ParseArgs(args)
	if err != nil {
		return nil, err
	}
	if flags.HelpRequested || flags.VersionRequested || flags.Bool("l") {
		return nil, errChessNoGame
	}
	if len(flags.Operands) > 1 {
		return nil, fmt.Errorf("one variant at a time, got %d", len(flags.Operands))
	}
	vs, _ := chess.Load()
	var v *chess.Variant
	switch file, ok := flags.String("f"); {
	case ok && len(flags.Operands) > 0:
		return nil, errors.New("give a variant name or --file, not both")
	case ok:
		v, vs, err = chess.LoadFile(file)
	case len(flags.Operands) == 1:
		v, err = chess.Find(vs, flags.Operands[0])
	default:
		v, err = chess.Find(vs, "chess")
	}
	if err != nil {
		return nil, err
	}
	g := chess.New(vs, v)
	if name, ok := flags.String("level"); ok {
		level, err := chess.ParseLevel(name)
		if err != nil {
			return nil, err
		}
		g.SetLevel(level)
	}
	if side, ok := flags.String("c"); ok {
		sides, known := chessSides[side]
		if !known {
			return nil, fmt.Errorf("--computer must be white, black, both or none, not %q", side)
		}
		g.SetComputer(sides[0], sides[1])
	}
	if secs, ok := flags.Num("think"); ok {
		if secs < 1 || secs > 3600 {
			return nil, errors.New("--think must be 1-3600 seconds")
		}
		g.SetThinkTime(time.Duration(secs) * time.Second)
	}
	if fen, ok := flags.String("fen"); ok {
		p, err := chess.ParseFEN(v, fen)
		if err != nil {
			return nil, fmt.Errorf("--fen: %v", err)
		}
		g.Load(p)
	}
	return g, nil
}

func runChess(ctx *Context) int {
	flags, err := chessSpec.ParseArgs(ctx.Args)
	if err == nil && HandleHelpVersion(ctx, flags, chessSpec) {
		if flags.HelpRequested {
			fmt.Fprintln(ctx.Stdout, "\nLevels: "+chess.LevelNames+" (default: normal).")
			fmt.Fprintln(ctx.Stdout, "--think overrides only the time ceiling (1-3600 seconds); d cycles level during play.")
		}
		return 0
	}
	if err == nil && flags.Bool("l") {
		return listChessVariants(ctx)
	}
	g, err := ChessGame(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "chess:", err)
		return 1
	}
	if err := chess.Run(g); err != nil {
		fmt.Fprintln(ctx.Stderr, "chess:", err)
		return 1
	}
	return 0
}

func listChessVariants(ctx *Context) int {
	vs, errs := chess.Load()
	for _, v := range vs {
		fmt.Fprintf(ctx.Stdout, "%-10s %-28s %2d×%-2d  %s\n", v.ID, v.Name, v.Files, v.Ranks, v.Source)
	}
	for _, err := range errs {
		fmt.Fprintln(ctx.Stderr, "chess:", err)
	}
	if len(errs) > 0 {
		return 1
	}
	return 0
}
