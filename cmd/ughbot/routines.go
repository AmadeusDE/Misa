package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
)

const routinesUsage = `usage: ughbot routines [list [-bot name]]
       ughbot routines add -bot name (-cron spec | -at time) [-tz zone] [-missed once|skip] prompt...
       ughbot routines pause|resume|delete <id>`

func routinesCmd(args []string, out io.Writer) error {
	verb := "list"
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		verb, args = args[0], args[1:]
	}
	var req api.RoutineRequest
	c, rest, err := connect("routines", args, func(fs *flag.FlagSet) {
		fs.StringVar(&req.Bot, "bot", "", "the bot the routine mails")
		fs.StringVar(&req.Cron, "cron", "", `cron spec, e.g. "0 9 * * 1" or @daily`)
		fs.StringVar(&req.At, "at", "", `one-shot time: "in 2h", "2006-01-02 15:04" or RFC 3339`)
		fs.StringVar(&req.TZ, "tz", "", "IANA timezone (default: the daemon's)")
		fs.StringVar(&req.Missed, "missed", "", "once (default) or skip, for fires missed while the daemon was down")
	})
	if err != nil {
		return err
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	switch verb {
	case "list":
		list, err := c.Routines(ctx, req.Bot)
		if err != nil {
			return err
		}
		printRoutines(out, list)
		return nil
	case "add":
		req.Prompt, req.Owner = strings.Join(rest, " "), "cli"
		info, err := c.AddRoutine(ctx, req)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "routine #%d added; next fire %s\n", info.ID, info.Next.Local().Format("2006-01-02 15:04 MST"))
		return nil
	case "pause", "resume", "delete":
		if len(rest) != 1 {
			return fmt.Errorf("%s", routinesUsage)
		}
		id, err := strconv.ParseInt(rest[0], 10, 64)
		if err != nil {
			return fmt.Errorf("routine id must be a number: %q", rest[0])
		}
		if verb == "delete" {
			if err := c.DeleteRoutine(ctx, id); err != nil {
				return err
			}
			fmt.Fprintf(out, "routine #%d deleted\n", id)
			return nil
		}
		info, err := c.PauseRoutine(ctx, id, verb == "pause")
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "routine #%d %s\n", info.ID, info.State)
		return nil
	}
	return fmt.Errorf("%s", routinesUsage)
}

// routineWhen is how a routine's schedule reads in lists: the cron spec or the one-shot time.
func routineWhen(r api.RoutineInfo) string {
	when := r.Cron
	if when == "" {
		when = "at " + r.At.Local().Format("2006-01-02 15:04")
	}
	if r.TZ != "" {
		when += " " + r.TZ
	}
	return when
}

func printRoutines(out io.Writer, list []api.RoutineInfo) {
	if len(list) == 0 {
		fmt.Fprintln(out, "no routines")
		return
	}
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tBOT\tSTATE\tWHEN\tNEXT\tBY\tPROMPT")
	for _, r := range list {
		next := "-"
		if r.State == "active" && !r.Next.IsZero() {
			next = r.Next.Local().Format(time.DateTime[:16])
		}
		prompt := r.Prompt
		if len([]rune(prompt)) > 50 {
			prompt = string([]rune(prompt)[:49]) + "…"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n", r.ID, r.Bot, r.State, routineWhen(r), next, r.Owner, prompt)
	}
	w.Flush()
}
