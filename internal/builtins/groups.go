package builtins

import (
	"fmt"
	"os/user"
	"strings"
)

func init() {
	Register(Command{Name: "groups", Run: runGroups})
}

func runGroups(ctx *Context) int {
	users := ctx.Args
	if len(users) == 1 && (users[0] == "--help" || users[0] == "--version") {
		if users[0] == "--help" {
			fmt.Fprintf(ctx.Stdout, "Usage: groups [USERNAME]...\nPrint the groups a user is in.\n")
			return 0
		}
		fmt.Fprintf(ctx.Stdout, "groups (Misa) 0.1.0\n")
		return 0
	}

	var filteredUsers []string
	for _, u := range users {
		if u == "--" {
			break
		}
		filteredUsers = append(filteredUsers, u)
	}
	if len(filteredUsers) == 0 {
		filteredUsers = []string{""}
	}
	users = filteredUsers

	rc := 0
	for _, name := range users {
		var u *user.User
		var err error
		if name == "" {
			u, err = user.Current()
		} else {
			u, err = user.Lookup(name)
		}
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "groups: '%s': no such user\n", name)
			rc = 1
			continue
		}
		gids, err := u.GroupIds()
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "groups: %s\n", err)
			rc = 1
			continue
		}
		names := make([]string, 0, len(gids))
		for _, gid := range gids {
			g, err := user.LookupGroupId(gid)
			if err != nil {
				names = append(names, gid)
			} else {
				names = append(names, g.Name)
			}
		}
		if name != "" {
			fmt.Fprintf(ctx.Stdout, "%s : %s\n", name, strings.Join(names, " "))
		} else {
			fmt.Fprintln(ctx.Stdout, strings.Join(names, " "))
		}
	}
	return rc
}
