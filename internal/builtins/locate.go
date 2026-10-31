// locate reads the database written by updatedb.
//
// Argv, matching rules and exit statuses follow uutils/findutils
// src/locate/mod.rs, commit d71f3242b0bc3b21bb311f04b23ae62e5f3fed67 (MIT).
// The database container is Misa's own (locate_db.go, docs/plans/locate.md);
// GNU LOCATE02 databases are not readable and are rejected as "not a Misa
// locate database".
package builtins

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/AmadeusDE/Misa/internal/posixre"
)

const locateSpec = "d(database):i(ignore-case)c(count)l(limit)#0(null)b(basename)a(all)r(regex)e(existing)w(wholename)"

var locateSpecParsed *Spec

func init() {
	var err error
	if locateSpecParsed, err = ParseSpec(locateSpec); err != nil {
		panic("builtins: bad locate spec: " + err.Error())
	}
	Register(Command{Name: "locate", Run: runLocate})
}

// locateUnsupported lists the findutils options this locate does not implement.
// They fail with a message naming the option instead of being ignored.
var locateUnsupported = map[string]string{
	"-E":                 "--non-existing is not supported (locate always requires an existing path)",
	"--non-existing":     "--non-existing is not supported (locate always requires an existing path)",
	"-L":                 "-L/--follow is not supported (indexed paths are never symlink targets)",
	"--follow":           "-L/--follow is not supported (indexed paths are never symlink targets)",
	"-H":                 "-H/--nofollow is not supported (symlinks are never followed)",
	"-P":                 "-P/--nofollow is not supported (symlinks are never followed)",
	"--nofollow":         "-H/--nofollow is not supported (symlinks are never followed)",
	"-m":                 "-m/--mmap is not supported (the database is read sequentially)",
	"--mmap":             "-m/--mmap is not supported (the database is read sequentially)",
	"-s":                 "-s/--stdio is not supported (the database is read sequentially)",
	"--stdio":            "-s/--stdio is not supported (the database is read sequentially)",
	"-S":                 "-S/--statistics is not supported",
	"--statistics":       "-S/--statistics is not supported",
	"-p":                 "-p/--print is not supported (matching paths are printed unless -c is given)",
	"--print":            "-p/--print is not supported (matching paths are printed unless -c is given)",
	"--max-database-age": "--max-database-age is not supported (the database is never treated as stale)",
	"--regextype":        "--regextype is not supported (Misa pattern matching uses POSIX extended syntax)",
}

// locateUnsupportedOption reports the first unsupported option in args.
func locateUnsupportedOption(args []string) (string, bool) {
	for _, a := range args {
		if a == "--" {
			return "", false
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			continue
		}
		name, _, _ := strings.Cut(a, "=")
		if msg, ok := locateUnsupported[name]; ok {
			return msg, true
		}
		if strings.HasPrefix(a, "--") || len(a) == 2 {
			continue
		}
		// A cluster of short flags: check each letter.
		for _, c := range a[1:] {
			if msg, ok := locateUnsupported["-"+string(c)]; ok {
				return msg, true
			}
		}
	}
	return "", false
}

type locateOptions struct {
	ignoreCase bool
	count      bool
	null       bool
	basename   bool
	all        bool
	regex      bool
	limit      int64
}

// locateMatcher decides whether an indexed path matches the operands.
type locateMatcher struct {
	patterns []string
	regexes  []*posixre.Regexp
	opts     locateOptions
}

func newLocateMatcher(patterns []string, opts locateOptions) (*locateMatcher, error) {
	m := &locateMatcher{opts: opts}
	for _, p := range patterns {
		if opts.regex {
			re, err := posixre.Compile(p, posixre.Options{Syntax: posixre.Extended, IgnoreCase: opts.ignoreCase})
			if err != nil {
				return nil, fmt.Errorf("invalid regular expression %q: %v", p, err)
			}
			m.regexes = append(m.regexes, re)
			continue
		}
		if opts.ignoreCase {
			p = strings.ToLower(p)
		}
		if locateIsGlob(p) {
			if _, err := locateGlobMatch(p, ""); err != nil {
				return nil, fmt.Errorf("invalid pattern %q: %v", p, err)
			}
		}
		m.patterns = append(m.patterns, p)
	}
	return m, nil
}

// locateIsGlob reports whether findutils would treat the pattern as a glob.
func locateIsGlob(p string) bool {
	return strings.ContainsAny(p, "*?[")
}

// locateGlobMatch matches the whole path as a glob. The separator is replaced
// with a NUL so '*' and '?' may span it, which is how findutils' fnmatch(3)
// call (no FNM_PATHNAME) behaves.
func locateGlobMatch(pattern, subject string) (bool, error) {
	return filepath.Match(strings.ReplaceAll(pattern, "/", "\x00"), strings.ReplaceAll(subject, "/", "\x00"))
}

// Match reports whether one indexed path matches.
func (m *locateMatcher) Match(path string) bool {
	subject := path
	if m.opts.basename {
		subject = filepath.Base(path)
	}
	if m.opts.ignoreCase {
		subject = strings.ToLower(subject)
	}
	if m.opts.regex {
		got := 0
		for _, re := range m.regexes {
			ok, err := re.Match([]byte(subject))
			if err != nil || !ok {
				continue
			}
			got++
			if !m.opts.all {
				return true
			}
		}
		return got == len(m.regexes)
	}
	got := 0
	for _, p := range m.patterns {
		var ok bool
		if locateIsGlob(p) {
			ok, _ = locateGlobMatch(p, subject)
		} else {
			ok = strings.Contains(subject, p)
		}
		if ok {
			got++
			if !m.opts.all {
				return true
			}
		}
	}
	return got == len(m.patterns)
}

// locateVisible applies the lookup-time access checks: the path itself must
// still exist (lstat, so a dangling symlink still counts as a name) and every
// ancestor directory must be searchable by this user. A database built by
// somebody else therefore cannot disclose names the caller cannot reach.
func locateVisible(path string) bool {
	if _, err := os.Lstat(path); err != nil {
		return false
	}
	dir := filepath.Dir(path)
	for {
		if !locateSearchable(dir) {
			return false
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return true
		}
		dir = parent
	}
}

// locateResolve joins a path onto the invocation's directory so relative
// arguments still resolve without touching process state.
func locateResolve(dir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if dir == "" {
		return path
	}
	return filepath.Join(dir, path)
}

// locateDefaultDBPath is $XDG_CACHE_HOME/misa/locatedb, or
// ~/.cache/misa/locatedb. Unlike findutils it never defaults under /var, so
// updatedb needs no root.
func locateDefaultDBPath(ctx *Context) (string, error) {
	if xdg, ok := lookupEnv(ctx, "XDG_CACHE_HOME"); ok && xdg != "" {
		return filepath.Join(xdg, "misa", "locatedb"), nil
	}
	if home, ok := lookupEnv(ctx, "HOME"); ok && home != "" {
		return filepath.Join(home, ".cache", "misa", "locatedb"), nil
	}
	return "", errors.New("cannot determine the database path: set XDG_CACHE_HOME or HOME, or pass -d")
}

// locateDatabasePath picks -d, then LOCATE_PATH, then the default.
func locateDatabasePath(ctx *Context, flags *ParseResult) (string, error) {
	if db, ok := flags.ExplicitString("d"); ok {
		return db, nil
	}
	if db, ok := flags.ExplicitString("database"); ok {
		return db, nil
	}
	if env, ok := lookupEnv(ctx, "LOCATE_PATH"); ok && env != "" {
		if strings.ContainsRune(env, filepath.ListSeparator) {
			return "", errors.New("LOCATE_PATH with several databases is not supported; pass one -d")
		}
		return env, nil
	}
	return locateDefaultDBPath(ctx)
}

func runLocate(ctx *Context) int {
	if msg, ok := locateUnsupportedOption(ctx.Args); ok {
		fmt.Fprintf(ctx.Stderr, "locate: %s\nTry 'locate --help' for more information.\n", msg)
		return 2
	}
	flags, err := locateSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "locate: %v\nTry 'locate --help' for more information.\n", err)
		return 2
	}
	if HandleHelpVersion(ctx, flags, locateSpecParsed) {
		return 0
	}
	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "locate: missing pattern\nTry 'locate --help' for more information.")
		return 2
	}
	opts := locateOptions{
		ignoreCase: flags.Bool("i") || flags.Bool("ignore-case"),
		count:      flags.Bool("c") || flags.Bool("count"),
		null:       flags.Bool("0") || flags.Bool("null"),
		basename:   flags.Bool("b") || flags.Bool("basename"),
		all:        flags.Bool("a") || flags.Bool("all"),
		regex:      flags.Bool("r") || flags.Bool("regex"),
	}
	// -e/--existing asks for the existence check that is always on, so it is
	// accepted and needs no further work.
	// -b and -w are opposites; the last one on the command line wins.
	for _, a := range ctx.Args {
		switch {
		case a == "-w", a == "--wholename":
			opts.basename = false
		case a == "-b", a == "--basename":
			opts.basename = true
		}
	}
	if n, ok := flags.Num("l"); ok {
		if n < 0 {
			fmt.Fprintf(ctx.Stderr, "locate: invalid limit %d\nTry 'locate --help' for more information.\n", n)
			return 2
		}
		opts.limit = n
	}
	matcher, err := newLocateMatcher(flags.Operands, opts)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "locate: %v\n", err)
		return 2
	}
	dbPath, err := locateDatabasePath(ctx, flags)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "locate: %v\n", err)
		return 1
	}
	dbPath = locateResolve(ctx.Dir, dbPath)

	f, err := os.Open(dbPath)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "locate: cannot open database %s: %s\n", dbPath, fmtOsErr(err))
		return 1
	}
	defer f.Close()
	rd, err := openLocateDB(f)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "locate: %s: %v\n", dbPath, err)
		return 1
	}

	sep := "\n"
	if opts.null {
		sep = "\x00"
	}
	matched := 0
	scanned := 0
	for {
		scanned++
		if scanned%1024 == 0 {
			if cerr := ctx.DoneContext().Err(); cerr != nil {
				fmt.Fprintf(ctx.Stderr, "locate: interrupted: %v\n", cerr)
				return 1
			}
		}
		path, ok, err := rd.Next()
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "locate: %s: %v\n", dbPath, err)
			return 1
		}
		if !ok {
			break
		}
		if opts.limit > 0 && int64(matched) >= opts.limit {
			break
		}
		if !matcher.Match(path) {
			continue
		}
		if !locateVisible(path) {
			continue
		}
		matched++
		if opts.count {
			continue
		}
		if _, err := io.WriteString(ctx.Stdout, path+sep); err != nil {
			fmt.Fprintf(ctx.Stderr, "locate: write error: %v\n", err)
			return 1
		}
	}
	if opts.count {
		if _, err := fmt.Fprintf(ctx.Stdout, "%d\n", matched); err != nil {
			fmt.Fprintf(ctx.Stderr, "locate: write error: %v\n", err)
			return 1
		}
	}
	if matched == 0 {
		return 1
	}
	return 0
}
