package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	realpathSpec = "e(canonicalize-existing)s(strip)(no-symlinks)m(canonicalize-missing)z(zero)(relative-to):(relative-base):qLPE"
	readlinkSpec = "f(canonicalize)e(canonicalize-existing)m(canonicalize-missing)n(no-newline)v(verbose)z(zero)"
)

var (
	realpathSpecParsed *Spec
	readlinkSpecParsed *Spec
)

func init() {
	var err error
	realpathSpecParsed, err = ParseSpec(realpathSpec)
	if err != nil {
		panic("builtins: bad realpath spec: " + err.Error())
	}
	readlinkSpecParsed, err = ParseSpec(readlinkSpec)
	if err != nil {
		panic("builtins: bad readlink spec: " + err.Error())
	}
	Register(Command{Name: "realpath", Run: runRealpath})
	Register(Command{Name: "readlink", Run: runReadlink})
}

// joinRawPath concatenates dir and rel without lexically cleaning the
// result (unlike filepath.Join/resolvePath), so that ".." components survive
// intact for canonicalizePath to resolve against the real, symlink-aware
// path rather than against the textual one.
func joinRawPath(dir, rel string) string {
	if filepath.IsAbs(rel) {
		return rel
	}
	if rel == "" {
		return dir
	}
	return strings.TrimRight(dir, "/") + "/" + rel
}

// canonicalize modes, mirroring GNU's canonicalize_filename_mode.
const (
	canonExisting = iota // every component must exist (readlink -e / realpath -e)
	canonDefault         // all but the final component must exist (readlink -f / realpath default)
	canonMissing         // nothing needs to exist; unresolved tails are appended literally (readlink -m / realpath -m)
)

func runRealpath(ctx *Context) int {
	flags, err := realpathSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "realpath:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, realpathSpecParsed) {
		return 0
	}
	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "realpath: missing operand")
		return 1
	}

	terminator := "\n"
	if flags.Bool("z") {
		terminator = "\x00"
	}

	strip := flags.Bool("s") || flags.Bool("no-symlinks")
	logical := flags.Bool("L")

	mode := canonDefault
	if flags.Bool("e") {
		mode = canonExisting
	}
	if flags.Bool("m") {
		mode = canonMissing
	}

	relTo, hasRelTo := flags.String("relative-to")
	relBase, hasRelBase := flags.String("relative-base")

	if hasRelTo && relTo == "" {
		fmt.Fprintln(ctx.Stderr, "realpath: : No such file or directory")
		return 1
	}
	if hasRelBase && relBase == "" {
		fmt.Fprintln(ctx.Stderr, "realpath: : No such file or directory")
		return 1
	}

	if hasRelTo && !filepath.IsAbs(relTo) {
		relTo = filepath.Join(ctx.Dir, relTo)
	}
	if hasRelBase && !filepath.IsAbs(relBase) {
		relBase = filepath.Join(ctx.Dir, relBase)
	}

	requireDirs := mode == canonExisting && !flags.Bool("E")
	if requireDirs {
		for _, d := range []struct {
			has bool
			val string
		}{{hasRelTo, relTo}, {hasRelBase, relBase}} {
			if !d.has {
				continue
			}
			info, serr := os.Stat(d.val)
			if serr != nil || !info.IsDir() {
				fmt.Fprintf(ctx.Stderr, "realpath: %s: Not a directory\n", d.val)
				return 1
			}
		}
	}

	disableRelative := false
	if hasRelBase && hasRelTo {
		cleanBase := filepath.Clean(relBase)
		cleanTo := filepath.Clean(relTo)
		if cleanBase != cleanTo && cleanBase != "/" && !strings.HasPrefix(cleanTo, cleanBase+"/") {
			disableRelative = true
		}
	}

	exitCode := 0
	for _, arg := range flags.Operands {
		if arg == "" {
			fmt.Fprintln(ctx.Stderr, "realpath: : No such file or directory")
			exitCode = 1
			continue
		}
		path := joinRawPath(ctx.Dir, arg)
		if logical {
			path = filepath.Clean(path)
		}

		var result string
		if strip {
			result = filepath.Clean(path)
			if mode == canonExisting {
				if _, serr := os.Stat(result); serr != nil {
					fmt.Fprintf(ctx.Stderr, "realpath: %s: No such file or directory\n", arg)
					exitCode = 1
					continue
				}
			}
		} else {
			resolved, cerr := canonicalizePath(path, mode)
			if cerr != nil {
				fmt.Fprintf(ctx.Stderr, "realpath: %s: %s\n", arg, fmtOsErr(cerr))
				exitCode = 1
				continue
			}
			result = resolved
		}

		if (hasRelTo || hasRelBase) && !disableRelative {
			result = makeRelative(result, relTo, relBase, hasRelTo, hasRelBase)
		}

		fmt.Fprint(ctx.Stdout, result)
		fmt.Fprint(ctx.Stdout, terminator)
	}
	return exitCode
}

func makeRelative(path, relTo, relBase string, hasRelTo, hasRelBase bool) string {
	if hasRelBase && !hasRelTo {
		relTo = relBase
	}

	if hasRelBase {
		cleanBase := filepath.Clean(relBase)
		cleanPath := filepath.Clean(path)
		if cleanBase != "/" {
			if cleanPath != cleanBase && !strings.HasPrefix(cleanPath, cleanBase+"/") {
				return path
			}
		}
	}

	rel, err := filepath.Rel(relTo, path)
	if err != nil {
		return path
	}
	return rel
}

// canonicalizePath resolves an absolute path, following symlinks as
// encountered and applying ".." against the real (already-resolved) path —
// i.e. "physical" resolution. mode controls how missing components are
// handled: canonExisting requires every component to exist; canonDefault
// tolerates a missing final component (but not a non-directory blocking one
// of its own); canonMissing tolerates anything, appending unresolved tails
// literally.
func canonicalizePath(path string, mode int) (string, error) {
	hasTrailingSlash := len(path) > 1 && strings.HasSuffix(path, "/")
	trimmed := strings.TrimRight(path, "/")
	if trimmed == "" {
		trimmed = "/"
	}

	pending := strings.Split(strings.TrimPrefix(trimmed, "/"), "/")

	resolved := "/"
	tailMissing := false
	steps := 0

	for len(pending) > 0 {
		c := pending[0]
		pending = pending[1:]
		isLast := len(pending) == 0

		if c == "" || c == "." {
			continue
		}
		if c == ".." {
			resolved = filepath.Dir(resolved)
			continue
		}
		if tailMissing {
			resolved = filepath.Join(resolved, c)
			continue
		}

		candidate := filepath.Join(resolved, c)
		info, err := os.Lstat(candidate)
		if err != nil {
			if mode == canonExisting {
				return "", err
			}
			if mode == canonDefault && !isLast {
				return "", err
			}
			tailMissing = true
			resolved = candidate
			continue
		}

		if info.Mode()&os.ModeSymlink != 0 {
			steps++
			if steps > 100 {
				return "", fmt.Errorf("Too many levels of symbolic links")
			}
			target, rerr := os.Readlink(candidate)
			if rerr != nil {
				return "", rerr
			}
			isAbsTarget := strings.HasPrefix(target, "/")
			trimmedTarget := strings.TrimPrefix(strings.TrimRight(target, "/"), "/")
			var targetComps []string
			if trimmedTarget != "" {
				targetComps = strings.Split(trimmedTarget, "/")
			}
			if isAbsTarget {
				resolved = "/"
			}
			pending = append(targetComps, pending...)
			continue
		}

		if !info.IsDir() && !isLast {
			if mode == canonMissing {
				resolved = candidate
				continue
			}
			return "", fmt.Errorf("not a directory")
		}

		resolved = candidate
	}

	if resolved == "" {
		resolved = "/"
	}

	if hasTrailingSlash && !tailMissing && mode != canonMissing {
		if info, err := os.Stat(resolved); err != nil {
			if mode == canonExisting {
				return "", err
			}
		} else if !info.IsDir() {
			return "", fmt.Errorf("not a directory")
		}
	}

	return resolved, nil
}

func runReadlink(ctx *Context) int {
	flags, err := readlinkSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "readlink:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, readlinkSpecParsed) {
		return 0
	}
	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "readlink: error: the following required arguments were not provided:\n  <FILE>...")
		return 1
	}

	noNewline := flags.Bool("n")
	zeroTerm := flags.Bool("z")
	terminator := "\n"
	if zeroTerm {
		terminator = "\x00"
	}
	if noNewline && len(flags.Operands) > 1 {
		fmt.Fprintln(ctx.Stderr, "readlink: ignoring --no-newline with multiple arguments")
		noNewline = false
	}
	if noNewline {
		terminator = ""
	}

	canon := flags.Bool("f") || flags.Bool("e") || flags.Bool("m")
	mode := canonDefault
	if flags.Bool("e") {
		mode = canonExisting
	}
	if flags.Bool("m") {
		mode = canonMissing
	}

	exitCode := 0
	for _, arg := range flags.Operands {
		if canon {
			resolved, cerr := canonicalizePath(joinRawPath(ctx.Dir, arg), mode)
			if cerr != nil {
				fmt.Fprintf(ctx.Stderr, "readlink: %s: %s\n", arg, fmtOsErr(cerr))
				exitCode = 1
				continue
			}
			fmt.Fprint(ctx.Stdout, resolved)
			fmt.Fprint(ctx.Stdout, terminator)
			continue
		}

		target, rerr := os.Readlink(resolvePath(ctx, arg))
		if rerr != nil {
			fmt.Fprintf(ctx.Stderr, "readlink: %s: %s\n", arg, fmtOsErr(rerr))
			exitCode = 1
			continue
		}
		fmt.Fprint(ctx.Stdout, target)
		fmt.Fprint(ctx.Stdout, terminator)
	}
	return exitCode
}
