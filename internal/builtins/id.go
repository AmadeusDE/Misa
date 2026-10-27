package builtins

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"
)

const idSpec = "u(user)g(group)G(groups)n(name)r(real)z(zero-terminated)apP"

var idSpecParsed *Spec

func init() {
	var err error
	idSpecParsed, err = ParseSpec(idSpec)
	if err != nil {
		panic("builtins: bad id spec: " + err.Error())
	}
	Register(Command{Name: "id", Run: runId})
}

func runId(ctx *Context) int {
	flags, err := idSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "id:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, idSpecParsed) {
		return 0
	}

	hasU := flags.Bool("u")
	hasG := flags.Bool("g")
	hasBigG := flags.Bool("G")
	showName := flags.Bool("n")
	hasReal := flags.Bool("r")
	zeroTerm := flags.Bool("z")
	hasP := flags.Bool("p")
	hasBigP := flags.Bool("P")

	if hasP && hasBigP {
		fmt.Fprintln(ctx.Stderr, "id: the argument '-p' cannot be used with '-P'")
		return 1
	}

	if !hasU && !hasG && !hasBigG {
		if showName || hasReal {
			fmt.Fprintln(ctx.Stderr, "id: printing only names or real IDs requires -u, -g, or -G")
			return 1
		}
		if zeroTerm {
			fmt.Fprintln(ctx.Stderr, "id: option --zero not permitted in default format")
			return 1
		}
	}

	uid := os.Getuid()
	gid := os.Getgid()
	if hasReal {
		// keep real uid/gid
	} else {
		uid = syscall.Geteuid()
		gid = syscall.Getegid()
	}

	var userObj *user.User
	if len(flags.Operands) > 0 {
		username := flags.Operands[0]
		u, err := user.Lookup(username)
		if err != nil {
			if _, numErr := strconv.Atoi(username); numErr == nil {
				u, err = user.LookupId(username)
			}
		}
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "id: '%s': no such user\n", username)
			return 1
		} else if u != nil {
			userObj = u
			uid, _ = strconv.Atoi(u.Uid)
			gid, _ = strconv.Atoi(u.Gid)
		}
	}

	terminator := "\n"
	if zeroTerm {
		terminator = "\x00"
	}

	if hasU {
		if showName {
			if u, err := user.LookupId(strconv.Itoa(uid)); err == nil {
				fmt.Fprint(ctx.Stdout, u.Username, terminator)
			} else {
				fmt.Fprint(ctx.Stdout, uid, terminator)
			}
		} else {
			fmt.Fprint(ctx.Stdout, uid, terminator)
		}
		return 0
	}

	if hasG {
		if showName {
			if g, err := user.LookupGroupId(strconv.Itoa(gid)); err == nil {
				fmt.Fprint(ctx.Stdout, g.Name, terminator)
			} else {
				fmt.Fprint(ctx.Stdout, gid, terminator)
			}
		} else {
			fmt.Fprint(ctx.Stdout, gid, terminator)
		}
		return 0
	}

	if hasBigG {
		var groups []int
		if userObj != nil {
			gids, err := userObj.GroupIds()
			if err == nil {
				for _, g := range gids {
					n, _ := strconv.Atoi(g)
					groups = append(groups, n)
				}
			}
			if len(groups) == 0 {
				groups = []int{gid}
			}
		} else {
			var err error
			groups, err = syscall.Getgroups()
			if err != nil || len(groups) == 0 {
				groups = []int{gid}
			}
		}
		var parts []string
		for _, g := range groups {
			if showName {
				if gr, err := user.LookupGroupId(strconv.Itoa(g)); err == nil {
					parts = append(parts, gr.Name)
				} else {
					parts = append(parts, strconv.Itoa(g))
				}
			} else {
				parts = append(parts, strconv.Itoa(g))
			}
		}
		delim := " "
		if zeroTerm {
			delim = "\x00"
		}
		fmt.Fprint(ctx.Stdout, strings.Join(parts, delim), terminator)
		return 0
	}

	uidName := strconv.Itoa(uid)
	if u, err := user.LookupId(uidName); err == nil {
		uidName = u.Username
	}
	gidName := strconv.Itoa(gid)
	if g, err := user.LookupGroupId(gidName); err == nil {
		gidName = g.Name
	}

	out := fmt.Sprintf("uid=%d(%s) gid=%d(%s)", uid, uidName, gid, gidName)

	var groups []int
	if userObj != nil {
		gids, err := userObj.GroupIds()
		if err == nil {
			for _, g := range gids {
				n, _ := strconv.Atoi(g)
				groups = append(groups, n)
			}
		}
		if len(groups) == 0 {
			groups = []int{gid}
		}
	} else {
		var err error
		groups, err = syscall.Getgroups()
		if err != nil || len(groups) == 0 {
			groups = []int{gid}
		}
	}

	if len(groups) > 0 {
		var parts []string
		for _, g := range groups {
			name := strconv.Itoa(g)
			if gr, err := user.LookupGroupId(name); err == nil {
				name = gr.Name
			}
			parts = append(parts, fmt.Sprintf("%d(%s)", g, name))
		}
		out += " groups=" + strings.Join(parts, ",")
	}

	fmt.Fprint(ctx.Stdout, out, terminator)
	return 0
}
