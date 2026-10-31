// Behaviour follows toybox grep.c (0BSD) and its tests; uutils/grep (MIT) is a secondary reference.
package builtins

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/AmadeusDE/Misa/internal/posixre"
)

const grepSpec = "e(regexp):f(file):E(extended-regexp)F(fixed-strings)G(basic-regexp)P(perl-regexp)" +
	"i(ignore-case)yv(invert-match)w(word-regexp)x(line-regexp)(no-ignore-case)" +
	"c(count)l(files-with-matches)L(files-without-match)q(quiet)(silent)s(no-messages)" +
	"h(no-filename)H(with-filename)n(line-number)b(byte-offset)o(only-matching)" +
	"m#(max-count)A#(after-context)B#(before-context)C#(context)" +
	"z(null-data)Z(null)a(text)r(recursive)R(dereference-recursive)" +
	"(include):(exclude):(exclude-from):(exclude-dir):" +
	"(label):(color):?(colour):?(line-buffered)(group-separator):(no-group-separator)"

var grepSpecParsed = mustParseSpec("grep", grepSpec)

func mustParseSpec(name, spec string) *Spec {
	s, err := ParseSpec(spec)
	if err != nil {
		panic("builtins: bad " + name + " spec: " + err.Error())
	}
	return s
}

func init() {
	for _, name := range []string{"grep", "egrep", "fgrep"} {
		Register(Command{Name: name, Run: func(ctx *Context) int { return runGrep(ctx, name) }})
	}
}

type grepOptions struct {
	invert, count, filesWith, filesWithout, quiet, noMessages bool
	withFilename, lineNumber, byteOffset, onlyMatching        bool
	nullData, nullName, lineBuffered, color, contextSet       bool
	recursive, dereference, noFilename                        bool
	maxCount                                                  int64
	after, before                                             int
	label                                                     string
	groupSep                                                  *string
}

type grepUsageError struct{ msg string }

func (e grepUsageError) Error() string { return e.msg }

func runGrep(ctx *Context, name string) int {
	flags, err := grepSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "grep: %v\n", err)
		return 2
	}
	if HandleHelpVersion(ctx, flags, grepSpecParsed) {
		return 0
	}
	opts, err := grepParseOptions(flags)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "grep: %v\n", err)
		return 2
	}
	patterns, files, err := grepPatterns(ctx, flags)
	if err != nil {
		if _, usage := err.(grepUsageError); usage {
			fmt.Fprintln(ctx.Stderr, "Usage: grep [OPTION]... PATTERNS [FILE]...")
		} else {
			fmt.Fprintf(ctx.Stderr, "grep: %v\n", err)
		}
		return 2
	}
	matcher, err := grepCompile(patterns, grepRegexOptions(name, flags))
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "grep: %v\n", err)
		return 2
	}
	filter, err := grepFilters(ctx, flags)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "grep: %v\n", err)
		return 2
	}
	switch {
	case len(files) > 0:
	case opts.recursive:
		files = []string{""}
	default:
		files = []string{"-"}
	}
	if !opts.noFilename && (flags.Bool("H") || len(files) > 1) {
		opts.withFilename = true
	}
	g := newGrepRun(ctx, opts, matcher)
	g.filter = filter
	return g.files(files)
}

func grepParseOptions(flags *ParseResult) (*grepOptions, error) {
	if flags.Bool("P") {
		return nil, errors.New("-P: Perl regular expressions are not supported")
	}
	o := &grepOptions{
		invert: flags.Bool("v"), count: flags.Bool("c"), quiet: flags.Bool("q") || flags.Bool("silent"),
		filesWith: flags.Bool("l"), filesWithout: flags.Bool("L"), noMessages: flags.Bool("s"),
		lineNumber: flags.Bool("n"), byteOffset: flags.Bool("b"), onlyMatching: flags.Bool("o"),
		nullData: flags.Bool("z"), nullName: flags.Bool("Z"), lineBuffered: flags.Bool("line-buffered"),
		recursive: flags.Bool("r") || flags.Bool("R"), dereference: flags.Bool("R"), noFilename: flags.Bool("h"),
		maxCount: -1, label: "(standard input)",
	}
	if n, ok := flags.Num("m"); ok {
		o.maxCount = n
	}
	if n, ok := flags.Num("C"); ok {
		o.after, o.before, o.contextSet = int(n), int(n), true
	}
	if n, ok := flags.Num("A"); ok {
		o.after, o.contextSet = int(n), true
	}
	if n, ok := flags.Num("B"); ok {
		o.before, o.contextSet = int(n), true
	}
	if o.after < 0 || o.before < 0 {
		return nil, errors.New("invalid context length argument")
	}
	if label, ok := flags.ExplicitString("label"); ok {
		o.label = label
	}
	sep := "--"
	o.groupSep = &sep
	if s, ok := flags.ExplicitString("group-separator"); ok {
		o.groupSep = &s
	}
	if flags.Bool("no-group-separator") {
		o.groupSep = nil
	}
	color, err := grepColorMode(flags)
	o.color = color
	return o, err
}

func grepColorMode(flags *ParseResult) (bool, error) {
	for _, name := range []string{"color", "colour"} {
		if _, ok := flags.String(name); !ok {
			continue
		}
		when, _ := flags.ExplicitString(name)
		switch when {
		case "always", "yes", "force":
			return true, nil
		case "", "auto", "tty", "if-tty", "never", "no", "none":
			return false, nil
		}
		return false, fmt.Errorf("invalid argument '%s' for '--%s'", when, name)
	}
	return false, nil
}

func grepRegexOptions(cmdName string, flags *ParseResult) posixre.Options {
	opt := posixre.Options{GNU: true, Line: flags.Bool("x"), Word: flags.Bool("w")}
	switch cmdName {
	case "egrep":
		opt.Syntax = posixre.Extended
	case "fgrep":
		opt.Literal = true
	}
	for _, name := range flags.Order() {
		switch name {
		case "E":
			opt.Syntax, opt.Literal = posixre.Extended, false
		case "G":
			opt.Syntax, opt.Literal = posixre.Basic, false
		case "F":
			opt.Literal = true
		case "i", "y":
			opt.IgnoreCase = true
		case "no-ignore-case":
			opt.IgnoreCase = false
		}
	}
	return opt
}

// grepPatterns collects -e and -f patterns, or takes the first operand.
// Every pattern source is split at newlines, even with -z.
func grepPatterns(ctx *Context, flags *ParseResult) ([]string, []string, error) {
	operands := flags.Operands
	exprs := flags.AllStrings("e")
	files := flags.AllStrings("f")
	if len(exprs) == 0 && len(files) == 0 {
		if len(operands) == 0 {
			return nil, nil, grepUsageError{}
		}
		exprs, operands = operands[:1], operands[1:]
	}
	var patterns []string
	for _, e := range exprs {
		patterns = append(patterns, strings.Split(e, "\n")...)
	}
	for _, name := range files {
		data, err := grepReadPatternFile(ctx, name)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %s", name, grepErrText(err))
		}
		if len(data) > 0 {
			patterns = append(patterns, strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")...)
		}
	}
	return patterns, operands, nil
}

func grepReadPatternFile(ctx *Context, name string) ([]byte, error) {
	if name == "-" {
		return io.ReadAll(ctx.Stdin)
	}
	return os.ReadFile(resolvePath(ctx, name))
}

func grepErrText(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		err = pe.Err
	}
	msg := err.Error()
	if msg == "" {
		return msg
	}
	return strings.ToUpper(msg[:1]) + msg[1:]
}
