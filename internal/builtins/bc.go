package builtins

// bc - an arbitrary-precision decimal calculator with the POSIX bc language.
//
// Behaviour follows POSIX bc as implemented by Toybox toys/pending/bc.c at
// commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb (0BSD). Differences and the
// deliberately unsupported parts (-l, -s, -w, read(), array parameters) are
// recorded in docs/plans/bc.md.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const bcSpec = "i(interactive)l(mathlib)q(quiet)s(standard)w(warn)"

// bcVersion is reported by --version and the -i banner.
const bcVersion = "0.1.0"

const bcUsage = `usage: bc [-ilqsw] [file ...]

Arbitrary-precision decimal calculator with the POSIX bc language.

options:
  -i, --interactive  force interactive mode (print the banner)
  -q, --quiet        don't print the banner
  -l, --mathlib      load the math library (not implemented)
  -s, --standard     error on non-POSIX extensions (not implemented)
  -w, --warn         warn on non-POSIX extensions (not implemented)

files are read in order, then standard input. BC_LINE_LENGTH sets the
output wrap column (default 69). See docs/plans/bc.md for the supported
language and known differences.
`

var bcSpecParsed *Spec

func init() {
	var err error
	bcSpecParsed, err = ParseSpec(bcSpec)
	if err != nil {
		panic("builtins: bad bc spec: " + err.Error())
	}
	Register(Command{Name: "bc", Run: runBc})
}

func runBc(ctx *Context) int {
	flags, err := bcSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "bc:", err)
		return 1
	}
	if flags.HelpRequested {
		fmt.Fprint(ctx.Stdout, bcUsage)
		return 0
	}
	if flags.VersionRequested {
		fmt.Fprintf(ctx.Stdout, "bc (Misa) %s\n", bcVersion)
		return 0
	}
	if flags.Bool("l") {
		fmt.Fprintln(ctx.Stderr, "bc: -l (math library): not implemented")
		return 1
	}
	if flags.Bool("s") || flags.Bool("w") {
		fmt.Fprintln(ctx.Stderr, "bc: -s/-w (POSIX extension checks): not implemented")
		return 1
	}

	// Toybox ignores absurd line lengths (it stores the value in 16 bits), so
	// only 2..65535 is honoured.
	lineLen := 69
	if v := getEnv(ctx, "BC_LINE_LENGTH"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 2 && n <= 65535 {
			lineLen = n
		}
	}
	out := newBcOutput(ctx.Stdout, lineLen)
	run := &bcRun{
		ctx: ctx,
		out: out,
		in:  newBcInterp(ctx, out),
	}

	if flags.Bool("i") && !flags.Bool("q") {
		fmt.Fprintf(ctx.Stdout, "bc (Misa) %s\n", bcVersion)
	}
	if flags.Bool("i") {
		run.interactive = true
	}

	for _, name := range flags.Operands {
		if err := run.runFile(name); err != nil {
			return run.report(name, err)
		}
	}
	if ctx.Stdin != nil {
		if err := run.runStdin(ctx.Stdin); err != nil {
			return run.report("<stdin>", err)
		}
	}
	if out.err != nil {
		fmt.Fprintf(ctx.Stderr, "bc: write error: %s\n", fmtOsErr(out.err))
		return 1
	}
	return 0
}

type bcRun struct {
	ctx         *Context
	out         *bcOutput
	in          *bcInterp
	interactive bool
}

// report prints an error the way the other builtins do and maps it to a status.
func (r *bcRun) report(name string, err error) int {
	switch {
	case err == nil, errors.Is(err, errBcQuit), errors.Is(err, errBcHalt):
		return 0
	}
	r.diagnose(name, err)
	return 1
}

// diagnose prints one diagnostic line. Parse errors carry file:line.
func (r *bcRun) diagnose(name string, err error) {
	if r.out.err != nil {
		fmt.Fprintf(r.ctx.Stderr, "bc: write error: %s\n", fmtOsErr(r.out.err))
		return
	}
	var perr *bcParseError
	if errors.As(err, &perr) {
		fmt.Fprintf(r.ctx.Stderr, "bc: %s:%d: %s\n", name, perr.line, perr.msg)
		return
	}
	if errors.Is(err, errBcIncomplete) {
		fmt.Fprintf(r.ctx.Stderr, "bc: %s: unexpected end of input\n", name)
		return
	}
	fmt.Fprintf(r.ctx.Stderr, "bc: %s\n", err)
}

func (r *bcRun) runFile(name string) error {
	var data []byte
	var err error
	if name == "-" {
		if r.ctx.Stdin == nil {
			return fmt.Errorf("stdin not available")
		}
		data, err = io.ReadAll(r.ctx.Stdin)
		if err != nil {
			return fmt.Errorf("standard input: %s", fmtOsErr(err))
		}
		return r.runUnit(name, string(data))
	}
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(r.ctx.Dir, path)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s: %s", name, fmtOsErr(err))
	}
	return r.runUnit(name, string(data))
}

// runStdin feeds standard input one logical line at a time, holding back lines
// until they parse (so multi-line blocks and continuations still work). With
// -i an error is reported and reading continues, as Toybox does.
func (r *bcRun) runStdin(rd io.Reader) error {
	br := bufio.NewReader(rd)
	var buf strings.Builder
	for {
		line, err := br.ReadString('\n')
		buf.WriteString(line)
		atEOF := err == io.EOF
		if err != nil && !atEOF {
			return err
		}
		if buf.Len() == 0 && atEOF {
			return nil
		}
		done, perr := r.unitStatus(buf.String())
		if done {
			stmts, parseErr := r.parseUnit(buf.String())
			if parseErr == nil {
				parseErr = r.execUnit(stmts)
			}
			switch {
			case parseErr == nil:
			case errors.Is(parseErr, errBcHalt), errors.Is(parseErr, errBcQuit):
				return parseErr
			case r.interactive:
				r.diagnose("<stdin>", parseErr)
				r.in.frames = nil
			default:
				return parseErr
			}
			buf.Reset()
		} else if perr != nil && !errors.Is(perr, errBcIncomplete) {
			if !r.interactive {
				return perr
			}
			r.diagnose("<stdin>", perr)
			buf.Reset()
		} else if atEOF {
			return perr
		}
		if atEOF {
			return nil
		}
	}
}

// unitStatus reports whether the buffered text is a complete parse unit. The
// error is returned either way so the caller can report a real parse error at
// once and an incomplete unit when the input ends.
func (r *bcRun) unitStatus(src string) (bool, error) {
	_, err := r.parseUnit(src)
	if errors.Is(err, errBcIncomplete) {
		return false, err
	}
	return err == nil, err
}

func (r *bcRun) parseUnit(src string) ([]bcStmt, error) {
	toks, err := bcLex(src)
	if err != nil {
		return nil, err
	}
	return bcParse(toks)
}

func (r *bcRun) execUnit(stmts []bcStmt) error {
	sig, err := r.in.runStmts(stmts)
	if err != nil {
		if errors.Is(err, errBcQuit) || errors.Is(err, errBcHalt) {
			return err
		}
		return err
	}
	if sig == bcSigHalt {
		return errBcHalt
	}
	return nil
}

// runUnit parses and runs a whole file (or a `-` operand) as one unit, which is
// what Toybox does for file operands.
func (r *bcRun) runUnit(name, src string) error {
	stmts, err := r.parseUnit(src)
	if err != nil {
		return err
	}
	return r.execUnit(stmts)
}
