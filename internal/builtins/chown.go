package builtins

import (
	"fmt"
	"os/user"
	"strconv"
	"strings"
)

const chownSpec = "R(recursive)v(verbose)f(silent)h(no-dereference)(dereference)LHP"

var chownSpecParsed *Spec

func init() {
	var err error
	chownSpecParsed, err = ParseSpec(chownSpec)
	if err != nil {
		panic("builtins: bad chown spec: " + err.Error())
	}
	Register(Command{Name: "chown", Run: runChown})
}

// runChown shares the walk and the apply step with chgrp (see ownership.go).
// Its -c and --reference are deliberately absent (chgrp has them); -P and
// --dereference are accepted and ignored because dereferencing is the default.
func runChown(ctx *Context) int {
	flags, err := chownSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "chown:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, chownSpecParsed) {
		return 0
	}
	if len(flags.Operands) < 2 {
		fmt.Fprintln(ctx.Stderr, "chown: missing operand")
		return 1
	}

	ownerSpec := flags.Operands[0]
	targets := flags.Operands[1:]

	uid, gid, err := parseOwnerGroup(ownerSpec)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "chown: %v\n", err)
		return 1
	}

	op := ownershipOp{
		uid:           uid,
		gid:           gid,
		verb:          "ownership",
		verbose:       flags.Bool("v"),
		silent:        flags.Bool("f"),
		noDereference: flags.Bool("h"),
		followAll:     flags.Bool("L"),
		followCmdline: flags.Bool("H"),
		recursive:     flags.Bool("R"),
		errStyle:      ownershipErrRaw,
		danglingCheck: true,
	}
	return applyOwnership(ctx, "chown", op, targets)
}

func parseOwnerGroup(spec string) (uid, gid int, err error) {
	uid, gid = -1, -1
	if spec == "" {
		return uid, gid, nil
	}

	sep := strings.IndexAny(spec, ":.")
	hasSep := sep >= 0
	var userPart, groupPart string
	if !hasSep {
		userPart = spec
	} else {
		userPart = spec[:sep]
		groupPart = spec[sep+1:]
	}

	var resolvedUser *user.User
	if userPart != "" {
		if u, lerr := user.Lookup(userPart); lerr == nil {
			resolvedUser = u
			uid, _ = strconv.Atoi(u.Uid)
		} else if n, cerr := strconv.Atoi(userPart); cerr == nil {
			uid = n
		} else {
			return -1, -1, fmt.Errorf("invalid user: %q", spec)
		}
	}

	if hasSep {
		if groupPart != "" {
			if n, cerr := strconv.Atoi(groupPart); cerr == nil {
				gid = n
			} else if g, lerr := user.LookupGroup(groupPart); lerr == nil {
				gid, _ = strconv.Atoi(g.Gid)
			} else {
				return -1, -1, fmt.Errorf("invalid group: %q", spec)
			}
		} else if userPart != "" {
			if resolvedUser == nil {
				return -1, -1, fmt.Errorf("invalid spec: %q", spec)
			}
			gid, _ = strconv.Atoi(resolvedUser.Gid)
		}
	}

	return uid, gid, nil
}
