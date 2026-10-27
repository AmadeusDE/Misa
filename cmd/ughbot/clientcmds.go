package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/ughbot/client"
	"github.com/AmadeusDE/Misa/internal/ughbot/config"
)

const callTimeout = 10 * time.Second

// connect parses the shared -c/-s flags plus any extra the command registers, then dials the daemon.
func connect(name string, args []string, extra func(*flag.FlagSet)) (*client.Client, []string, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	cfgPath := fs.String("c", config.DefaultPath(), "config file (to find the socket)")
	socket := fs.String("s", "", "daemon socket")
	if extra != nil {
		extra(fs)
	}
	if err := fs.Parse(args); err != nil {
		return nil, nil, err
	}
	c, err := client.Dial(socketPath(*socket, *cfgPath))
	return c, fs.Args(), err
}

func socketPath(socket, cfgPath string) string {
	if socket != "" {
		return socket
	}
	if cfg, err := config.Load(cfgPath); err == nil {
		return cfg.Socket
	}
	return config.DefaultSocket()
}

func botsCmd(args []string, out io.Writer) error {
	c, _, err := connect("bots", args, nil)
	if err != nil {
		return err
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	bots, err := c.ListBots(ctx)
	if err != nil {
		return err
	}
	printBots(out, bots)
	return nil
}

func printBots(out io.Writer, bots []api.BotInfo) {
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "BOT\tSTATE\tMODEL\tCAPABILITIES")
	for _, b := range bots {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", b.Name, b.State, b.Model, b.Capabilities)
	}
	w.Flush()
}

func statusCmd(args []string, out io.Writer) error {
	c, _, err := connect("status", args, nil)
	if err != nil {
		return err
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	st, err := c.Status(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "ughbot %s, up %s\n", st.Version, (time.Duration(st.UptimeSeconds) * time.Second).String())
	if st.VM != "" {
		fmt.Fprintln(out, "vm:", st.VM)
	}
	printBots(out, st.Bots)
	return nil
}

func costCmd(args []string, out io.Writer) error {
	var by *string
	c, _, err := connect("cost", args, func(fs *flag.FlagSet) {
		by = fs.String("by", "", "group by bot or day")
	})
	if err != nil {
		return err
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	rows, err := c.Cost(ctx, api.CostQuery{})
	if err != nil {
		return err
	}
	rows, err = groupCost(rows, *by)
	if err != nil {
		return err
	}
	printCost(out, rows)
	return nil
}

// groupCost sums rows by bot or by day; "" keeps one row per bot and day.
func groupCost(rows []api.CostRow, by string) ([]api.CostRow, error) {
	if by == "" {
		return rows, nil
	}
	if by != "bot" && by != "day" {
		return nil, fmt.Errorf("--by must be bot or day, not %q", by)
	}
	sums := map[string]float64{}
	for _, r := range rows {
		key := r.Bot
		if by == "day" {
			key = r.Day
		}
		sums[key] += r.USD
	}
	keys := make([]string, 0, len(sums))
	for k := range sums {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]api.CostRow, len(keys))
	for i, k := range keys {
		out[i] = api.CostRow{USD: sums[k]}
		if by == "bot" {
			out[i].Bot = k
		} else {
			out[i].Day = k
		}
	}
	return out, nil
}

func printCost(out io.Writer, rows []api.CostRow) {
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "BOT\tDAY\tUSD")
	var total float64
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%s\t%.4f\n", dash(r.Bot), dash(r.Day), r.USD)
		total += r.USD
	}
	fmt.Fprintf(w, "total\t\t%.4f\n", total)
	w.Flush()
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func sendCmd(args []string, out io.Writer) error {
	c, rest, err := connect("send", args, nil)
	if err != nil {
		return err
	}
	defer c.Close()
	if len(rest) < 2 {
		return fmt.Errorf("usage: ughbot send <bot> <text>")
	}
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	ticket, err := c.Send(ctx, rest[0], strings.Join(rest[1:], " "))
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "queued %s for %s\n", ticket, rest[0])
	return nil
}
