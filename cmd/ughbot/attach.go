package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/ughbot/client"
	"github.com/AmadeusDE/Misa/internal/ughbot/tui"
)

// tuiCmd runs the full UI; attachCmd opens it on one bot.
func tuiCmd(args []string) error { return runTUI("tui", args, 0) }

func attachCmd(args []string) error { return runTUI("attach", args, 1) }

func runTUI(name string, args []string, bots int) error {
	c, rest, err := connect(name, args, nil)
	if err != nil {
		return err
	}
	defer c.Close()
	if len(rest) != bots {
		if bots == 1 {
			return fmt.Errorf("usage: ughbot attach <bot>")
		}
		return fmt.Errorf("usage: ughbot tui")
	}
	bot := ""
	if bots == 1 {
		bot = rest[0]
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	_, err = tea.NewProgram(tui.New(ctx, c, bot)).Run()
	return err
}

var _ tui.DeviceBackend = (*client.Client)(nil)
