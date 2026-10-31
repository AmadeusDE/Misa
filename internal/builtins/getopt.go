// getopt: parse command-line options. Behaviour ported from Toybox
// toys/other/getopt.c (0BSD, commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb).
//
// usage: getopt [-aTu] [-lo OPTIONS] [-n NAME] [OPTIONS] ARG...
//
// Recognised options (Toybox spec "^a(alternative)n:(name)o:(options)l*(long)(longoptions)Tu";
// the leading ^ means getopt stops scanning its own options at the first
// non-option argument, and everything from there on is an operand):
//
//	-a, --alternative    allow long options introduced by a single '-'
//	-n, --name NAME      command name used in the error messages (default "getopt")
//	-o, --options OPTSTR short option string (instead of the first operand)
//	-l, --long LIST      long option declarations; repeatable, each value may be
//	    --longoptions    comma separated. "name", "name:" (required argument)
//	                     and "name::" (optional argument, must be attached).
//	-T                   test for a modern getopt: print nothing, exit 4
//	-u                   unquoted output (the default when getopt gets no options)
//	-h, --help           print this usage text and exit 0
//
// getopt's own options accept the usual short cluster/attached forms (-onal:,
// -nfoo, -n foo) and unambiguous long prefixes (--alt, --nam foo). "--" ends
// getopt's own options. A long form that Toybox does not declare (--test,
// --unquoted) is an unknown option. Errors are one line each on stderr,
// prefixed "getopt: ": "unknown option '-q'", "unknown option '--bogus'",
// "ambiguous option '--l'", "option '-n' requires an argument", "bad -l 'X'",
// "no OPTSTR", and "option '--alternative' takes no argument".
//
// The caller's command line is then parsed GNU-style (options may follow
// operands; operands keep their relative order) and printed as
//
//	-a 'value' --long 'value' -- operand operand
//
// each option/value quoted the way Toybox's out() does it, unless unquoted
// output was asked for (or getopt was invoked with no options at all, which is
// Toybox's legacy mode). Errors while parsing the caller's line go to stderr as
// "NAME: invalid option 'x'" (NAME from -n) and only change the exit status;
// parsing continues and the line is still printed.
//
// Deliberate deviations from Toybox (all of them cases where Toybox prints the
// NUL byte that glibc leaves in optopt, which is not reproducible text):
//
//   - an unknown long option prints "NAME: invalid option '--name'" instead of
//     "NAME: invalid option '\0'";
//   - an ambiguous long option abbreviation prints "NAME: ambiguous option
//     '--name'" instead of the same NUL message;
//   - a long option with a required argument that has no value (a dangling
//     "--name", or "--name" at the end) and a long option that takes no
//     argument but was given "=value" both print "NAME: invalid option
//     '--name'";
//   - a -l entry whose name still contains a ':' once the trailing colons are
//     stripped ("bad:::name") is rejected with "bad -l 'bad:::name'". Toybox
//     only strips trailing colons, so it would declare the long option
//     "bad:::name" instead of reporting it.
//
// Exit status: 0 when everything parsed, 1 for a rejected option in the
// caller's line *and* for getopt's own usage errors (Toybox routes both through
// error_exit, which exits 1), and 4 for -T, which is its purpose. Toybox has no
// exit 2 for this command; keep that distinction in mind when comparing with
// other getopt implementations.
//
// Two further notes on fidelity:
//
//   - in -a (alternative) mode every token starting with a single '-' is tried
//     as a long option first, even when its first character is in OPTSTR, so
//     "-a -l a" makes "-a" the long option "a". glibc only tries the long match
//     when the token is longer than two characters or the character is not in
//     OPTSTR;
//   - the glibc OPTSTR modifiers ('+', '-' and a leading ':') are not
//     implemented; those characters are parsed as ordinary option characters.
//     Permutation is always GNU-style, as Toybox's own usage documents, and the
//     POSIXLY_CORRECT environment variable (which glibc would use to disable
//     permutation) is not consulted.
package builtins

import (
	"fmt"
	"strings"
)

func init() { Register(Command{Name: "getopt", Run: runGetopt}) }

const getoptUsage = `usage: getopt [-aTu] [-lo OPTIONS] [-n NAME] [OPTIONS] ARG...

Outputs command line with recognized OPTIONS character arguments moved to
front, then "--", then non-option arguments. Returns 1 if unknown options.
OPTIONS followed by : take an argument, or :: for optional arguments (which
must be attached, ala -xblah or --long=blah).

 -a	Allow long options starting with a single -
 -l	Long OPTIONS (repeated or comma separated)
 -n	Command NAME for error messages
 -o	Short OPTIONS (instead of using first argument)
 -T	Test whether this is a modern getopt
 -u	Unquoted output (default if no other options set)

Example:
  $ getopt -l long:,arg:: abc command --long -b there --arg
  --long '-b' --arg '' -- 'command' 'there'
`

// getoptLong is one -l declaration. hasArg is 0 (no argument), 1 (required) or
// 2 (optional, and then the value must be attached).
type getoptLong struct {
	name   string
	hasArg int
}

// getoptOwnLongs maps getopt's own long option names onto their short letters.
var getoptOwnLongs = []struct {
	name  string
	short byte
}{
	{"alternative", 'a'},
	{"help", 'h'},
	{"name", 'n'},
	{"options", 'o'},
	{"long", 'l'},
	{"longoptions", 'l'},
}

type getoptState struct {
	alternative bool
	test        bool
	unquoted    bool
	name        string // -n, argv[0] of the parsed line
	optstr      string
	optstrSet   bool
	longSpecs   []string // raw -l values, in order
	longs       []getoptLong
}

// getoptErrorf writes one error line to stderr and returns the failing status.
func getoptErrorf(ctx *Context, format string, a ...any) int {
	fmt.Fprintf(ctx.Stderr, "getopt: "+format+"\n", a...)
	return 1
}

// matchLongName resolves name against names: an exact match wins, otherwise a
// unique prefix match. It returns the matching index, or -1 with amb true when
// the prefix matches more than one distinct name, or -1 with amb false when
// nothing matches.
func matchLongName(names []string, name string) (idx int, amb bool) {
	if name == "" {
		return -1, false
	}
	for i, n := range names {
		if n == name {
			return i, false
		}
	}
	idx = -1
	for i, n := range names {
		if strings.HasPrefix(n, name) {
			if idx >= 0 && names[idx] != n {
				return -1, true
			}
			idx = i
		}
	}
	return idx, false
}

func runGetopt(ctx *Context) int {
	st := &getoptState{name: "getopt"}
	anyOption := false
	help := false

	apply := func(c byte, val string) {
		anyOption = true
		switch c {
		case 'a':
			st.alternative = true
		case 'T':
			st.test = true
		case 'u':
			st.unquoted = true
		case 'h':
			help = true
		case 'n':
			st.name = val
		case 'o':
			st.optstr, st.optstrSet = val, true
		case 'l':
			st.longSpecs = append(st.longSpecs, val)
		}
	}

	// getopt's own options: Toybox's "^" stops at the first non-option.
	args := ctx.Args
	ownNames := make([]string, len(getoptOwnLongs))
	for i, o := range getoptOwnLongs {
		ownNames[i] = o.name
	}
	i := 0
ownOptions:
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--":
			i++
			break ownOptions
		case len(a) < 2 || a[0] != '-':
			break ownOptions
		case a[1] == '-':
			body := a[2:]
			name, val, hasVal := body, "", false
			if j := strings.IndexByte(body, '='); j >= 0 {
				name, val, hasVal = body[:j], body[j+1:], true
			}
			idx, amb := matchLongName(ownNames, name)
			if idx < 0 {
				if amb {
					return getoptErrorf(ctx, "ambiguous option '--%s'", name)
				}
				return getoptErrorf(ctx, "unknown option '--%s'", name)
			}
			c := getoptOwnLongs[idx].short
			if c == 'n' || c == 'o' || c == 'l' {
				if !hasVal {
					i++
					if i >= len(args) {
						return getoptErrorf(ctx, "option '--%s' requires an argument", name)
					}
					val = args[i]
				}
				apply(c, val)
			} else if hasVal {
				return getoptErrorf(ctx, "option '--%s' takes no argument", name)
			} else {
				apply(c, "")
			}
			i++
		default:
			cluster := a[1:]
			for k := 0; k < len(cluster); k++ {
				c := cluster[k]
				if c == 'n' || c == 'o' || c == 'l' {
					val := cluster[k+1:]
					if val == "" {
						i++
						if i >= len(args) {
							return getoptErrorf(ctx, "option '-%s' requires an argument", cluster[k:k+1])
						}
						val = args[i]
					}
					apply(c, val)
					break
				}
				switch c {
				case 'a', 'T', 'u', 'h':
					apply(c, "")
				default:
					return getoptErrorf(ctx, "unknown option '-%s'", cluster[k:k+1])
				}
			}
			i++
		}
	}
	operands := args[i:]

	if help {
		fmt.Fprint(ctx.Stdout, getoptUsage)
		return 0
	}
	// Toybox checks -T before it looks for an OPTSTR or validates -l.
	if st.test {
		return 4
	}
	for _, spec := range st.longSpecs {
		for _, entry := range strings.Split(spec, ",") {
			name, hasArg := entry, 0
			for strings.HasSuffix(name, ":") {
				hasArg++
				name = name[:len(name)-1]
			}
			if name == "" || hasArg > 2 || strings.Contains(name, ":") {
				return getoptErrorf(ctx, "bad -l '%s'", entry)
			}
			st.longs = append(st.longs, getoptLong{name: name, hasArg: hasArg})
		}
	}

	if !st.optstrSet {
		if len(operands) == 0 {
			fmt.Fprintln(ctx.Stderr, "getopt: no OPTSTR")
			return 1
		}
		st.optstr, operands = operands[0], operands[1:]
	}
	if st.optstr == "" {
		fmt.Fprintln(ctx.Stderr, "getopt: no OPTSTR")
		return 1
	}
	// Toybox: if (!toys.optflags) toys.optflags = FLAG_u;
	if !anyOption {
		st.unquoted = true
	}

	line, status := st.parseLine(ctx, operands)
	fmt.Fprint(ctx.Stdout, line)
	return status
}

// putValue appends one value in Toybox's out() format.
func (st *getoptState) putValue(b *strings.Builder, v string) {
	if st.unquoted {
		b.WriteByte(' ')
		b.WriteString(v)
		return
	}
	b.WriteString(" '")
	b.WriteString(strings.ReplaceAll(v, "'", `'\''`))
	b.WriteByte('\'')
}

// shortKind reports whether c is in OPTSTR and whether it takes an argument
// (0 none, 1 required, 2 optional).
func (st *getoptState) shortKind(c byte) (kind int, known bool) {
	for i := 0; i < len(st.optstr); i++ {
		if st.optstr[i] != c {
			continue
		}
		if i+1 < len(st.optstr) && st.optstr[i+1] == ':' {
			if i+2 < len(st.optstr) && st.optstr[i+2] == ':' {
				return 2, true
			}
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

func (st *getoptState) longNames() []string {
	names := make([]string, len(st.longs))
	for i, l := range st.longs {
		names[i] = l.name
	}
	return names
}

// parseLine parses the caller's arguments and returns the output line plus the
// exit status (1 if anything was rejected, 0 otherwise).
func (st *getoptState) parseLine(ctx *Context, params []string) (string, int) {
	var out strings.Builder
	status := 0
	badf := func(format string, a ...any) {
		fmt.Fprintf(ctx.Stderr, format, a...)
		status = 1
	}

	var operands []string
	names := st.longNames()
	for i := 0; i < len(params); i++ {
		a := params[i]
		if a == "--" {
			operands = append(operands, params[i+1:]...)
			break
		}
		if len(a) < 2 || a[0] != '-' {
			operands = append(operands, a)
			continue
		}

		long := a[1] == '-'
		if long || st.alternative {
			text := a[1:]
			if long {
				text = a[2:]
			}
			name, val, hasVal := text, "", false
			if j := strings.IndexByte(text, '='); j >= 0 {
				name, val, hasVal = text[:j], text[j+1:], true
			}
			idx, amb := matchLongName(names, name)
			switch {
			case amb:
				badf("%s: ambiguous option '--%s'\n", st.name, name)
				continue
			case idx >= 0:
				opt := st.longs[idx]
				switch opt.hasArg {
				case 0:
					if hasVal {
						badf("%s: invalid option '--%s'\n", st.name, name)
						continue
					}
				case 1:
					if !hasVal {
						if i+1 >= len(params) {
							badf("%s: invalid option '--%s'\n", st.name, name)
							continue
						}
						i++
						val = params[i]
					}
				}
				out.WriteString(" --")
				out.WriteString(opt.name)
				if opt.hasArg > 0 {
					st.putValue(&out, val)
				}
				continue
			case long:
				badf("%s: invalid option '--%s'\n", st.name, name)
				continue
			}
			// -a mode: not a long option, parse it as a short cluster.
		}

		for k := 1; k < len(a); k++ {
			c := a[k]
			kind, known := st.shortKind(c)
			if !known {
				badf("%s: invalid option '%s'\n", st.name, a[k:k+1])
				continue
			}
			switch kind {
			case 0:
				out.WriteString(" -")
				out.WriteByte(c)
			case 1, 2:
				if k+1 < len(a) {
					out.WriteString(" -")
					out.WriteByte(c)
					st.putValue(&out, a[k+1:])
					k = len(a)
					continue
				}
				if kind == 2 {
					// Optional arguments must be attached: emit no value.
					out.WriteString(" -")
					out.WriteByte(c)
					continue
				}
				if i+1 >= len(params) {
					badf("%s: invalid option '%s'\n", st.name, a[k:k+1])
					continue
				}
				i++
				out.WriteString(" -")
				out.WriteByte(c)
				st.putValue(&out, params[i])
			}
		}
	}

	out.WriteString(" --")
	for _, o := range operands {
		st.putValue(&out, o)
	}
	out.WriteByte('\n')
	return out.String(), status
}
