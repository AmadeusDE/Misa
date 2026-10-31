// Behaviour follows toybox chgrp/chown (0BSD, toys/posix/chgrp.c at 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb) and uutils coreutils chgrp (MIT, src/uu/chgrp at fee61ff5d4f87e0f912a52e639942b5f7ad58663).
package builtins

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"syscall"
)

// chgrp [-RHLPhvcf] [--reference=RFILE] GROUP FILE...
//
// Deliberately unsupported: --from and --preserve-root/--no-preserve-root (they
// need a policy decision this builtin does not make), and --quiet as a synonym
// for --silent. The option parser rejects all of them as unknown options, which
// is honest: `chgrp --from=1000 998 f` fails instead of silently changing every
// matching file.
const chgrpSpec = "R(recursive)v(verbose)c(changes)f(silent)h(no-dereference)(reference):LHP"

var chgrpSpecParsed *Spec

func init() {
	var err error
	chgrpSpecParsed, err = ParseSpec(chgrpSpec)
	if err != nil {
		panic("builtins: bad chgrp spec: " + err.Error())
	}
	Register(Command{Name: "chgrp", Run: runChgrp})
}

func runChgrp(ctx *Context) int {
	flags, err := chgrpSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "chgrp:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, chgrpSpecParsed) {
		return 0
	}

	// With --reference the first operand is a file, so the group comes from it
	// and one operand is enough; otherwise the first operand is the group.
	refFile, haveRef := flags.String("reference")
	minOperands := 2
	if haveRef {
		minOperands = 1
	}
	if len(flags.Operands) < minOperands {
		fmt.Fprintln(ctx.Stderr, "chgrp: missing operand")
		return 1
	}

	gid := -1
	var targets []string
	if haveRef {
		info, err := os.Stat(resolvePath(ctx, refFile))
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "chgrp: failed to get attributes of '%s': %s\n", refFile, fmtOsErr(err))
			return 1
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			fmt.Fprintf(ctx.Stderr, "chgrp: failed to get attributes of '%s': unsupported file information\n", refFile)
			return 1
		}
		gid = int(st.Gid)
		targets = flags.Operands
	} else {
		gid, err = parseGroupSpec(flags.Operands[0])
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "chgrp: %v\n", err)
			return 1
		}
		targets = flags.Operands[1:]
	}

	op := ownershipOp{
		uid:           -1,
		gid:           gid,
		verb:          "group",
		verbose:       flags.Bool("v"),
		changes:       flags.Bool("c"),
		silent:        flags.Bool("f"),
		noDereference: flags.Bool("h"),
		followAll:     flags.Bool("L"),
		followCmdline: flags.Bool("H"),
		recursive:     flags.Bool("R"),
		errStyle:      ownershipErrContext,
	}
	return applyOwnership(ctx, "chgrp", op, targets)
}

// parseGroupSpec maps the GROUP operand to a gid. Digits are a numeric gid,
// anything else must be a group name. An empty spec means "leave the group
// alone": it yields -1, which the walk treats as a successful no-op (this is
// uutils' test_no_change).
func parseGroupSpec(spec string) (int, error) {
	if spec == "" {
		return -1, nil
	}
	if allDigits(spec) { // head.go's helper: non-empty and all ASCII digits
		if n, err := strconv.Atoi(spec); err == nil {
			return n, nil
		}
	} else if g, err := user.LookupGroup(spec); err == nil {
		if n, err := strconv.Atoi(g.Gid); err == nil {
			return n, nil
		}
	}
	return -1, fmt.Errorf("invalid group: '%s'", spec)
}
