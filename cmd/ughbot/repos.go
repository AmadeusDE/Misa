package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
)

const reposUsage = `usage: ughbot repos [list [-bot name]]
       ughbot repos revoke <bot> <dir>`

// reposCmd lists and takes back the code directories bots were granted through approvals.
func reposCmd(args []string, out io.Writer) error {
	verb := "list"
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		verb, args = args[0], args[1:]
	}
	var bot string
	c, rest, err := connect("repos", args, func(fs *flag.FlagSet) {
		fs.StringVar(&bot, "bot", "", "only this bot's grants")
	})
	if err != nil {
		return err
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	switch verb {
	case "list":
		list, err := c.RepoGrants(ctx, bot)
		if err != nil {
			return err
		}
		printRepos(out, list)
		return nil
	case "revoke":
		if len(rest) != 2 {
			return fmt.Errorf("%s", reposUsage)
		}
		if err := c.RevokeRepo(ctx, rest[0], rest[1]); err != nil {
			return err
		}
		fmt.Fprintf(out, "%s may no longer run code tasks in %s (files stay on disk)\n", rest[0], rest[1])
		return nil
	}
	return fmt.Errorf("%s", reposUsage)
}

func printRepos(out io.Writer, list []api.RepoGrant) {
	if len(list) == 0 {
		fmt.Fprintln(out, "no repos granted through approvals (code dirs in bot definitions are not listed)")
		return
	}
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "BOT\tDIR\tAPPROVAL\tGRANTED\tCLONED FROM")
	for _, g := range list {
		src := g.Source
		if src == "" {
			src = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t#%d\t%s\t%s\n", g.Bot, g.Dir, g.Approval, g.Created.Local().Format(time.DateTime[:16]), src)
	}
	w.Flush()
}
