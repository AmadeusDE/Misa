package builtins

import (
	"errors"
	"fmt"

	"github.com/AmadeusDE/Misa/internal/battleship"
)

var battleshipSpec = mustSpec("r(random)")

func init() {
	Register(Command{Name: "battleship", Run: runBattleship})
}

var errBattleshipInfo = errors.New("help or version requested")

// BattleshipGame builds a game from battleship's arguments: -r/--random
// places the fleet randomly instead of interactively.
func BattleshipGame(args []string) (*battleship.Game, error) {
	flags, err := battleshipSpec.ParseArgs(args)
	if err != nil {
		return nil, err
	}
	if flags.HelpRequested || flags.VersionRequested {
		return nil, errBattleshipInfo
	}
	if len(flags.Operands) > 0 {
		return nil, fmt.Errorf("no operands expected, got %d", len(flags.Operands))
	}
	return battleship.New(battleship.Options{Random: flags.Bool("r")}), nil
}

func runBattleship(ctx *Context) int {
	if flags, err := battleshipSpec.ParseArgs(ctx.Args); err == nil && HandleHelpVersion(ctx, flags, battleshipSpec) {
		return 0
	}
	g, err := BattleshipGame(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "battleship:", err)
		return 1
	}
	if err := battleship.Run(g); err != nil {
		fmt.Fprintln(ctx.Stderr, "battleship:", err)
		return 1
	}
	return 0
}
