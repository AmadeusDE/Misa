package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
)

const approvalsUsage = "usage: ughbot approvals [list [-a]] | show <id> | approve <id> | deny <id>"

func approvalsCmd(args []string, out io.Writer) error {
	verb := "list"
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		verb, args = args[0], args[1:]
	}
	var all *bool
	c, rest, err := connect("approvals", args, func(fs *flag.FlagSet) {
		all = fs.Bool("a", false, "list every request, not just pending ones")
	})
	if err != nil {
		return err
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	if verb == "list" {
		state := "pending"
		if *all {
			state = ""
		}
		list, err := c.Approvals(ctx, state)
		if err != nil {
			return err
		}
		printApprovals(out, list)
		return nil
	}
	if len(rest) != 1 {
		return fmt.Errorf("%s", approvalsUsage)
	}
	id, err := strconv.ParseInt(rest[0], 10, 64)
	if err != nil {
		return fmt.Errorf("approval id must be a number: %q", rest[0])
	}
	switch verb {
	case "show":
		list, err := c.Approvals(ctx, "")
		if err != nil {
			return err
		}
		for _, a := range list {
			if a.ID == id {
				printApproval(out, a)
				return nil
			}
		}
		return fmt.Errorf("no approval #%d", id)
	case "approve", "deny":
		decide := c.Deny
		if verb == "approve" {
			decide = c.Approve
		}
		a, err := decide(ctx, id, "cli")
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "#%d %s: %s\n", a.ID, a.State, a.Summary)
		if a.Result != "" {
			fmt.Fprintln(out, a.Result)
		}
		return nil
	}
	return fmt.Errorf("%s", approvalsUsage)
}

func printApprovals(out io.Writer, list []api.ApprovalInfo) {
	if len(list) == 0 {
		fmt.Fprintln(out, "no approvals")
		return
	}
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tBOT\tSTATE\tAGE\tSUMMARY")
	for _, a := range list {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", a.ID, a.Bot, a.State, age(a.Created), a.Summary)
	}
	w.Flush()
}

func printApproval(out io.Writer, a api.ApprovalInfo) {
	fmt.Fprintf(out, "#%d %s, asked by %s %s ago\n%s\n", a.ID, a.State, a.Bot, age(a.Created), a.Summary)
	if a.State == "pending" {
		fmt.Fprintf(out, "expires in %s\n", time.Until(a.Expires).Round(time.Minute))
	} else if a.DecidedBy != "" {
		fmt.Fprintf(out, "decided by %s\n", a.DecidedBy)
	}
	if a.Result != "" {
		fmt.Fprintf(out, "result: %s\n", a.Result)
	}
	var pretty bytes.Buffer
	if json.Indent(&pretty, a.Args, "", "  ") == nil {
		fmt.Fprintf(out, "\n%s %s\n", a.Action, pretty.String())
	}
}

func age(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
