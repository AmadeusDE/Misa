// Behaviour adapted from uutils util-linux rename (MIT), src/uu/rename at
// 413b20967dcf8cd41f095dabf4858d4f02c17401: subst.rs, rename.rs, errors.rs, prompt.rs.

package builtins

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// renameSpec is the option surface: -v/--verbose, -n/--no-act,
// -o/--no-overwrite, -s/--symlink, -a/--all, -l/--last, -i/--interactive,
// -h/--help and -V/--version. The operands are EXPRESSION REPLACEMENT FILE...
// and are not described by the spec.
const renameSpec = "v(verbose)n(no-act)o(no-overwrite)s(symlink)a(all)l(last)i(interactive)h(help)V(version)"

var renameSpecParsed *Spec

func init() {
	var err error
	renameSpecParsed, err = ParseSpec(renameSpec)
	if err != nil {
		panic("builtins: bad rename spec: " + err.Error())
	}
	Register(Command{Name: "rename", Run: runRename})
}

const renameHelp = `Usage: rename [options] EXPRESSION REPLACEMENT FILE...

Rename FILEs, replacing EXPRESSION with REPLACEMENT in their names.

  -v, --verbose        explain what is being done
  -n, --no-act         do not make any changes
  -o, --no-overwrite   don't overwrite existing files
  -s, --symlink        act on the target of symlinks
  -a, --all            replace all occurrences
  -l, --last           replace only the last occurrence
  -i, --interactive    prompt before overwrite
  -h, --help           display this help and exit
  -V, --version        output version information and exit

Only the first occurrence in the final path component (the basename) is
replaced, unless EXPRESSION or REPLACEMENT contains a slash, in which case the
whole path is rewritten and a rename may move a file between directories.

Exit status:
  0  at least one file was renamed and none failed
  1  nothing was renamed and at least one operand failed
  2  at least one file was renamed and at least one operand failed
  4  nothing was renamed and nothing failed
`

// renameMode is which occurrence of the needle a run replaces.
type renameMode int

const (
	renameFirst renameMode = iota
	renameAll
	renameLast
)

type renameOptions struct {
	verbose     bool
	symlink     bool
	noAct       bool
	noOverwrite bool
	interactive bool
	mode        renameMode
	needle      string
	replacement string
}

// renameOutcome is what one operand did. A skip, a no-match and an unchanged
// name are all renameNeither; the two counters cannot tell them apart either.
type renameOutcome int

const (
	renameNeither renameOutcome = iota
	renameDone
	renameFailed
)

func runRename(ctx *Context) int {
	flags, err := renameSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "rename: %s\nTry 'rename --help' for more information.\n", err)
		return 1
	}
	if flags.Bool("h") {
		fmt.Fprint(ctx.Stdout, renameHelp)
		return 0
	}
	if flags.Bool("V") {
		fmt.Fprint(ctx.Stdout, "rename (Misa) 0.1.0\n")
		return 0
	}

	opts := renameOptions{
		verbose:     flags.Bool("v"),
		symlink:     flags.Bool("s"),
		noAct:       flags.Bool("n"),
		noOverwrite: flags.Bool("o"),
		interactive: flags.Bool("i"),
		mode:        renameFirst,
	}

	// The only two mutually exclusive pairs.
	if flags.Bool("a") && flags.Bool("l") {
		return renameUsageError(ctx, "options -a and -l are mutually exclusive")
	}
	if opts.noOverwrite && opts.interactive {
		return renameUsageError(ctx, "options -o and -i are mutually exclusive")
	}

	if flags.Bool("a") {
		opts.mode = renameAll
	} else if flags.Bool("l") {
		opts.mode = renameLast
	}

	args := flags.Operands
	if len(args) < 3 {
		return renameUsageError(ctx, "missing operand")
	}
	opts.needle = args[0]
	opts.replacement = args[1]

	// An identical expression and replacement short-circuit the whole run, not
	// one operand: no filesystem syscall happens at all, even for operands that
	// do not exist.
	if opts.needle == opts.replacement {
		return 4
	}

	// One reader for the whole run, so a multi-character answer never spills
	// into the next prompt and a run that outlives its input reaches end of
	// input instead of re-reading an old answer.
	var answers *bufio.Reader
	if opts.interactive {
		var stdin io.Reader = ctx.Stdin
		if stdin == nil {
			stdin = strings.NewReader("")
		}
		answers = bufio.NewReader(stdin)
	}

	renamed, failed := 0, 0
	for _, operand := range args[2:] {
		switch renameOne(ctx, &opts, answers, operand) {
		case renameDone:
			renamed++
		case renameFailed:
			failed++
		}
	}

	switch {
	case renamed > 0 && failed > 0:
		return 2
	case renamed > 0:
		return 0
	case failed > 0:
		return 1
	}
	return 4
}

func renameUsageError(ctx *Context, msg string) int {
	fmt.Fprintf(ctx.Stderr, "rename: %s\nTry 'rename --help' for more information.\n", msg)
	return 1
}

// renameSysPath turns a name into the path handed to the syscalls: an absolute
// name is used as it is, and a relative one is prefixed with the context
// directory without being cleaned, so the operand's literal text (trailing
// separators and all) survives into the call.
func renameSysPath(ctx *Context, name string) string {
	if name == "" || strings.HasPrefix(name, "/") || ctx.Dir == "" {
		return name
	}
	return strings.TrimSuffix(ctx.Dir, "/") + "/" + name
}

// renameOne is one operand, start to finish, and the only place that touches
// the filesystem.
func renameOne(ctx *Context, opts *renameOptions, answers *bufio.Reader, operand string) renameOutcome {
	sysOperand := renameSysPath(ctx, operand)

	// The existence check keeps the operand exactly as it was typed, trailing
	// separators and all, which is why `d1/s1/` reports ENOTDIR instead of
	// renaming `d1/s1`. Everything after this point uses the stripped form.
	info, err := os.Lstat(sysOperand)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "rename: %s: not accessible: %s\n", operand, fmtOsErr(err))
		return renameFailed
	}

	// This precedes the match test, so a non-symlink fails even when the
	// expression appears nowhere in its name.
	if opts.symlink && info.Mode()&os.ModeSymlink == 0 {
		fmt.Fprintf(ctx.Stderr, "rename: %s: not a symbolic link\n", operand)
		return renameFailed
	}

	// Symlink mode rewrites the link's target text and never the link's own
	// name; the chain is not resolved, so a link to a link sees only the next
	// hop's text.
	source := operand
	if opts.symlink {
		target, err := os.Readlink(sysOperand)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "rename: %s: not accessible: %s\n", operand, fmtOsErr(err))
			return renameFailed
		}
		source = target
	}

	old, newName := renameRewrite(source, opts.needle, opts.replacement, opts.mode)
	if old == newName {
		return renameNeither
	}

	// Only the two safeguards ask, so only they look at the destination at all.
	taken := false
	if opts.noOverwrite || opts.interactive {
		if opts.symlink {
			// Is there an ENTRY at the new target path? A dangling entry counts
			// here and blocks the rewrite.
			_, statErr := os.Lstat(renameSysPath(ctx, newName))
			taken = statErr == nil
		} else {
			// Is anything reachable at the destination? A dangling symlink does
			// not count as existing, and a probe that cannot be performed at all
			// (EACCES on a parent) reads as absent.
			_, statErr := os.Stat(renameSysPath(ctx, newName))
			taken = statErr == nil
		}
	}

	skipped := false
	if taken {
		if opts.interactive && !opts.noAct {
			skipped = !renameAsk(ctx, answers, newName)
		} else {
			// -o, or -i under -n: -n never prompts and never reads stdin.
			skipped = true
		}
	}
	if skipped {
		renameReportSkip(ctx, opts, operand, old, newName)
	}

	if opts.noAct {
		// The verbose line prints even for an operand the guard skipped, so a
		// skipped operand under -n prints two lines and counts as neither.
		renameReport(ctx, opts, operand, old, newName)
		if skipped {
			return renameNeither
		}
		return renameDone
	}

	if skipped {
		return renameNeither
	}

	if opts.symlink {
		// Not atomic, and deliberately so: the link is unlinked and then
		// created, and a failure to create the replacement leaves the original
		// link gone.
		if err := os.Remove(sysOperand); err != nil {
			fmt.Fprintf(ctx.Stderr, "rename: %s: unlink failed: %s\n", operand, fmtOsErr(err))
			return renameFailed
		}
		if err := os.Symlink(newName, sysOperand); err != nil {
			fmt.Fprintf(ctx.Stderr, "rename: %s: symlinking to %s failed: %s\n", operand, newName, fmtOsErr(err))
			return renameFailed
		}
	} else {
		if err := os.Rename(renameSysPath(ctx, old), renameSysPath(ctx, newName)); err != nil {
			fmt.Fprintf(ctx.Stderr, "rename: %s: rename to %s failed: %s\n", old, newName, fmtOsErr(err))
			return renameFailed
		}
	}

	renameReport(ctx, opts, operand, old, newName)
	return renameDone
}

// renameAsk prompts on stdout and takes one answer. Accepted when it begins
// with `y` or `Y`; end of input declines and says so, closing the line the
// prompt left open.
func renameAsk(ctx *Context, answers *bufio.Reader, newName string) bool {
	fmt.Fprintf(ctx.Stdout, "rename: overwrite `%s'? ", newName)

	first, ok := renameReadAnswer(answers)
	if !ok {
		fmt.Fprint(ctx.Stdout, "n\n")
		return false
	}
	return first == 'y' || first == 'Y'
}

// renameReadAnswer consumes a whole line and returns its first byte. An answer
// the input ended without terminating is still an answer; an input that has
// nothing left is not.
func renameReadAnswer(answers *bufio.Reader) (byte, bool) {
	if answers == nil {
		return 0, false
	}
	for {
		line, err := answers.ReadString('\n')
		if len(line) > 0 {
			return line[0], true
		}
		if err != nil {
			return 0, false
		}
	}
}

// renameReport is the verbose line, printed for a rename that happened and for
// one that -n only described.
func renameReport(ctx *Context, opts *renameOptions, operand, old, newName string) {
	if !opts.verbose {
		return
	}
	if opts.symlink {
		fmt.Fprintf(ctx.Stdout, "%s: `%s' -> `%s'\n", operand, old, newName)
		return
	}
	fmt.Fprintf(ctx.Stdout, "`%s' -> `%s'\n", old, newName)
}

// renameReportSkip is the skip report, which unlike the prompt is printed only
// under -v.
func renameReportSkip(ctx *Context, opts *renameOptions, operand, old, newName string) {
	if !opts.verbose {
		return
	}
	if opts.symlink {
		// Symlink mode names the link and the target it still has.
		fmt.Fprintf(ctx.Stdout, "Skipping existing link: `%s' -> `%s'\n", operand, old)
		return
	}
	fmt.Fprintf(ctx.Stdout, "Skipping existing file: `%s'\n", newName)
}

// renameRewrite applies the substitution at the right scope.
//
// Normally only the final path component changes. If either argument contains a
// separator the whole path is in scope, which is what lets a rename move a file
// between directories. The rule is decided on the two strings themselves, so it
// holds even when the needle cannot match anything.
//
// It returns the old name as it should be reported and renamed - the trailing
// separators are stripped off outside whole-path mode - and the new name.
func renameRewrite(name, needle, replacement string, mode renameMode) (string, string) {
	if strings.ContainsRune(needle, '/') || strings.ContainsRune(replacement, '/') {
		return name, renameSubstitute(name, needle, replacement, mode)
	}
	prefix, component := renameSplitComponent(name, '/')
	return name[:len(prefix)+len(component)], prefix + renameSubstitute(component, needle, replacement, mode)
}

// renameSplitComponent splits a name into everything before its final
// component and the component itself. Trailing separators belong to neither and
// are dropped, which is what the -v line shows: `d1/` is reported as `d1`.
//
// A name that is nothing but separators is the exception: there is nothing to
// strip without emptying the name, so the component becomes the final separator
// alone and everything before it is the prefix.
func renameSplitComponent(name string, sep byte) (prefix, component string) {
	end := len(name)
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] != sep {
			end = i + 1
			break
		}
	}

	searchEnd := 0
	if end > 0 {
		searchEnd = end - 1
	}
	start := 0
	for i := searchEnd - 1; i >= 0; i-- {
		if name[i] == sep {
			start = i + 1
			break
		}
	}
	return name[:start], name[start:end]
}

// renameSubstitute replaces occurrences of needle in name with replacement. The
// unit is a byte: a needle may split a UTF-8 sequence and a name that is not
// valid UTF-8 goes through unchanged.
func renameSubstitute(name, needle, replacement string, mode renameMode) string {
	if needle == "" {
		return renameInterleave(name, replacement, mode)
	}

	switch mode {
	case renameLast:
		if at := strings.LastIndex(name, needle); at >= 0 {
			return name[:at] + replacement + name[at+len(needle):]
		}
		return name
	case renameAll:
		var out strings.Builder
		out.Grow(len(name))
		rest := name
		for {
			at := strings.Index(rest, needle)
			if at < 0 {
				break
			}
			out.WriteString(rest[:at])
			out.WriteString(replacement)
			rest = rest[at+len(needle):]
		}
		out.WriteString(rest)
		return out.String()
	default:
		if at := strings.Index(name, needle); at >= 0 {
			return name[:at] + replacement + name[at+len(needle):]
		}
		return name
	}
}

// renameInterleave handles the empty needle, which matches between every two
// bytes: renameFirst prepends, renameLast appends, and renameAll inserts at
// every boundary including both ends - n + 1 times for a name of n bytes.
func renameInterleave(name, replacement string, mode renameMode) string {
	switch mode {
	case renameFirst:
		return replacement + name
	case renameLast:
		return name + replacement
	default:
		var out strings.Builder
		out.Grow(len(replacement)*(len(name)+1) + len(name))
		for i := 0; i < len(name); i++ {
			out.WriteString(replacement)
			out.WriteByte(name[i])
		}
		out.WriteString(replacement)
		return out.String()
	}
}
