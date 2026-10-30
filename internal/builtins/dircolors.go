// dircolors — print commands to set LS_COLORS.
//
// Adapted from uutils coreutils (MIT license):
//
//	src/uu/dircolors/src/dircolors.rs
//	src/uucore/src/lib/features/colors.rs
//
// pinned at commit fee61ff5d4f87e0f912a52e639942b5f7ad58663.
// See docs/CREDITS.md and THIRD_PARTY_NOTICES.md.
//
// Supported: -b/--sh/--bourne-shell, -c/--csh/--c-shell,
// -p/--print-database, --print-ls-colors and an optional FILE ("-" = stdin).
// Behaviour kept from the reference: the shell type defaults from the
// basename of $SHELL ("csh"/"tcsh" -> C shell, anything else -> Bourne
// shell), the last of -b/-c wins, and a run without a FILE prints the
// built-in database without filtering it by TERM/COLORTERM (only a FILE
// operand is matched against the environment).
//
// Exit statuses: 0 on success; 1 for an unknown option, mutually exclusive
// options, a missing $SHELL, an unreadable FILE, an extra operand, an
// invalid line ("FILE:LINE: invalid line;  missing second token") or an
// unrecognized keyword; 2 for a FILE that is a directory. Unsupported from
// GNU dircolors: nothing beyond that option set (no experimental long
// options), and TERM/COLORTERM are read from the invocation environment
// (Context.Env when the caller supplies one, else the process).
package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const dircolorsSpec = "b(sh)c(csh)p(print-database)(print-ls-colors)(bourne-shell)(c-shell)"

var dircolorsSpecParsed *Spec

func init() {
	var err error
	dircolorsSpecParsed, err = ParseSpec(dircolorsSpec)
	if err != nil {
		panic("builtins: bad dircolors spec: " + err.Error())
	}
	Register(Command{Name: "dircolors", Run: runDircolors})
}

// dircolorsFmt is the output syntax.
type dircolorsFmt int

const (
	dcShell   dircolorsFmt = iota // LS_COLORS='...';\nexport LS_COLORS
	dcCShell                      // setenv LS_COLORS '...'
	dcDisplay                     // one coloured "code<TAB>value" line per entry
)

// dircolorsColors returns the prefix and suffix that wrap the colour list.
func dircolorsColors(f dircolorsFmt) (string, string) {
	switch f {
	case dcShell:
		return "LS_COLORS='", "';\nexport LS_COLORS"
	case dcCShell:
		return "setenv LS_COLORS '", "'"
	}
	return "", ""
}

// dircolorsShellFlags reports which shell syntax option came last, so that
// "-bc" and "-cb" behave like the reference (the later option wins).
func dircolorsShellFlags(args []string) (sh, csh bool) {
scan:
	for _, a := range args {
		switch {
		case a == "--":
			break scan
		case strings.HasPrefix(a, "--"):
			switch a {
			case "--sh", "--bourne-shell":
				sh, csh = true, false
			case "--csh", "--c-shell":
				sh, csh = false, true
			}
		case len(a) > 1 && a[0] == '-':
			for i := 1; i < len(a); i++ {
				switch a[i] {
				case 'b':
					sh, csh = true, false
				case 'c':
					sh, csh = false, true
				}
			}
		}
	}
	return sh, csh
}

// dircolorsGuessSyntax picks the syntax from the basename of $SHELL.
func dircolorsGuessSyntax(shell string, ok bool) (dircolorsFmt, bool) {
	if !ok || shell == "" {
		return 0, false
	}
	switch filepath.Base(shell) {
	case "csh", "tcsh":
		return dcCShell, true
	}
	return dcShell, true
}

// dircolorsQuote wraps s in single quotes for the diagnostics.
func dircolorsQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func runDircolors(ctx *Context) int {
	flags, err := dircolorsSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "dircolors:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, dircolorsSpecParsed) {
		return 0
	}

	sh, csh := dircolorsShellFlags(ctx.Args)
	printDatabase := flags.Bool("print-database")
	printLSCcolors := flags.Bool("print-ls-colors")
	files := flags.Operands

	switch {
	case (sh || csh) && (printDatabase || printLSCcolors):
		fmt.Fprintln(ctx.Stderr, "dircolors: the options to output non shell syntax, and to select a shell syntax are mutually exclusive")
		return 1
	case printDatabase && printLSCcolors:
		fmt.Fprintln(ctx.Stderr, "dircolors: options --print-database and --print-ls-colors are mutually exclusive")
		return 1
	case printDatabase && len(files) > 0:
		fmt.Fprintf(ctx.Stderr, "dircolors: extra operand %s\n  file operands cannot be combined with --print-database (-p)\n", dircolorsQuote(files[0]))
		return 1
	}

	if printDatabase {
		fmt.Fprintln(ctx.Stdout, dircolorsDatabase)
		return 0
	}

	format := dcShell
	var ok bool
	switch {
	case csh:
		format = dcCShell
	case sh:
		format = dcShell
	case printLSCcolors:
		format = dcDisplay
	default:
		shell, found := lookupEnv(ctx, "SHELL")
		if format, ok = dircolorsGuessSyntax(shell, found); !ok {
			fmt.Fprintln(ctx.Stderr, "dircolors: no SHELL environment variable, and no shell type option given")
			return 1
		}
	}

	switch len(files) {
	case 0:
		fmt.Fprintln(ctx.Stdout, dircolorsDefaultColors(format))
		return 0
	case 1:
	}
	if len(files) > 1 {
		fmt.Fprintf(ctx.Stderr, "dircolors: extra operand %s\n", dircolorsQuote(files[1]))
		return 1
	}

	name := files[0]
	path := name
	if name != "-" && !filepath.IsAbs(path) {
		path = filepath.Join(ctx.Dir, path)
	}
	if name != "-" {
		if st, err := os.Stat(path); err == nil && st.IsDir() {
			fmt.Fprintf(ctx.Stderr, "dircolors: expected file, got directory %s\n", dircolorsQuote(name))
			return 2
		}
	}
	lines, err := readDatabaseLines(ctx, name, path)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "dircolors: %v\n", err)
		return 1
	}

	out, msg := dircolorsParse(lines, format, name, ctx)
	if msg != "" {
		fmt.Fprintf(ctx.Stderr, "dircolors: %s\n", msg)
		return 1
	}
	fmt.Fprintln(ctx.Stdout, out)
	return 0
}

// dircolorsDefaultColors renders the built-in database in the output syntax.
// It is used when no FILE operand is given, where the reference emits its
// compiled-in tables without consulting TERM or COLORTERM.
func dircolorsDefaultColors(f dircolorsFmt) string {
	types := make([]string, 0, len(dircolorsTypes))
	exts := make([]string, 0, len(dircolorsExts))
	if f == dcDisplay {
		for _, t := range dircolorsTypes {
			types = append(types, fmt.Sprintf("\x1b[%sm%s\t%s\x1b[0m", t.value, t.code, t.value))
		}
		for _, e := range dircolorsExts {
			exts = append(exts, fmt.Sprintf("\x1b[%sm%s\t%s\x1b[0m", e[1], e[0], e[1]))
		}
		return strings.Join(append(types, exts...), "\n")
	}
	for _, t := range dircolorsTypes {
		types = append(types, t.code+"="+t.value)
	}
	for _, e := range dircolorsExts {
		exts = append(exts, e[0]+"="+e[1])
	}
	prefix, suffix := dircolorsColors(f)
	return prefix + strings.Join(types, ":") + ":" + strings.Join(exts, ":") + ":" + suffix
}
