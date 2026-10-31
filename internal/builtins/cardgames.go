package builtins

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/AmadeusDE/Misa/internal/cards"
	"github.com/AmadeusDE/Misa/internal/cardui"
	"github.com/AmadeusDE/Misa/internal/freecell"
	"github.com/AmadeusDE/Misa/internal/hearts"
	"github.com/AmadeusDE/Misa/internal/klondike"
	"github.com/AmadeusDE/Misa/internal/spider"
)

var errCardInfo = errors.New("help or version requested")

func cardSpec(name string) *Spec {
	extra := ""
	switch name {
	case "solitaire":
		extra = "(draw):"
	case "spider":
		extra = "(suits):"
	case "hearts":
		extra = "(target):"
	}
	return mustSpec("(seed):(theme):(ascii)" + extra)
}
func init() {
	for _, name := range []string{"solitaire", "freecell", "spider", "hearts"} {
		Register(Command{Name: name, Run: func(ctx *Context) int { return runCardGame(name, ctx) }})
	}
}
func CardGame(name string, args []string) (*cardui.Game, error) {
	if name != "solitaire" && name != "freecell" && name != "spider" && name != "hearts" {
		return nil, fmt.Errorf("unknown card game %q", name)
	}
	f, err := cardSpec(name).ParseArgs(args)
	if err != nil {
		return nil, err
	}
	if f.HelpRequested || f.VersionRequested {
		return nil, errCardInfo
	}
	if len(f.Operands) > 0 {
		return nil, errors.New("no operands expected")
	}
	opts := cardui.Options{Theme: "classic", ASCII: f.Bool("ascii")}
	if v, ok := f.String("theme"); ok {
		opts.Theme = v
	}
	if opts.Theme != "classic" && opts.Theme != "default" {
		return nil, errors.New("theme must be classic or default")
	}
	if v, ok := f.String("seed"); ok {
		opts.Seed, err = strconv.ParseUint(v, 10, 64)
	} else {
		opts.Seed, err = cards.FreshSeed()
	}
	if err != nil {
		return nil, fmt.Errorf("seed: %w", err)
	}
	option, def := "", 0
	switch name {
	case "solitaire":
		option, def = "draw", 1
	case "spider":
		option, def = "suits", 1
	case "hearts":
		option, def = "target", 100
	}
	opts.Rule = def
	if v, ok := f.String(option); option != "" && ok {
		opts.Rule, err = strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid %s", option)
		}
	}
	switch name {
	case "solitaire":
		if opts.Rule != 1 && opts.Rule != 3 {
			return nil, errors.New("draw must be 1 or 3")
		}
		return klondike.New(opts), nil
	case "freecell":
		return freecell.New(opts), nil
	case "spider":
		if opts.Rule != 1 && opts.Rule != 2 && opts.Rule != 4 {
			return nil, errors.New("suits must be 1, 2 or 4")
		}
		return spider.New(opts), nil
	default:
		if opts.Rule <= 0 || opts.Rule > int(^uint(0)>>1)-26 {
			return nil, errors.New("target must be positive and leave room for a 26-point round")
		}
		return hearts.New(opts), nil
	}
}
func runCardGame(name string, ctx *Context) int {
	f, err := cardSpec(name).ParseArgs(ctx.Args)
	if err == nil && f.HelpRequested {
		fmt.Fprintf(ctx.Stdout, "Usage: %s [--seed uint64] [--theme classic|default] [--ascii]", name)
		switch name {
		case "solitaire":
			fmt.Fprint(ctx.Stdout, " [--draw 1|3] (default 1)")
		case "spider":
			fmt.Fprint(ctx.Stdout, " [--suits 1|2|4] (default 1)")
		case "hearts":
			fmt.Fprint(ctx.Stdout, " [--target score] (default 100)")
		}
		fmt.Fprintln(ctx.Stdout, "\nOffline Misa card game. Arrows/Tab, Enter/Space, ? help; q quits with confirmation.\n--seed replays a Misa v1 deal, not a Microsoft deal number.")
		return 0
	}
	if err == nil && HandleHelpVersion(ctx, f, cardSpec(name)) {
		return 0
	}
	g, err := CardGame(name, ctx.Args)
	if err == nil {
		err = cardui.Run(g)
	}
	if err != nil {
		fmt.Fprintln(ctx.Stderr, name+":", err)
		return 1
	}
	return 0
}
