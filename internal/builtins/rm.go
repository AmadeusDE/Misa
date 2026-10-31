package builtins

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/x/term"
)

const rmSpec = "(interactive):?(one-file-system)(-presume-input-tty)r(recursive)f(force)v(verbose)d(dir)RiI(preserve-root):?(no-preserve-root)g(progress)"

var rmSpecParsed *Spec

func init() {
	var err error
	rmSpecParsed, err = ParseSpec(rmSpec)
	if err != nil {
		panic("builtins: bad rm spec: " + err.Error())
	}
	Register(Command{Name: "rm", Run: runRm})
}

type rmInteractiveMode int

const (
	rmAskDefault rmInteractiveMode = iota
	rmAskNever
	rmAskOnce
	rmAskAlways
)

type rmState struct {
	ctx          *Context
	in           *bufio.Reader
	mode         rmInteractiveMode
	force        bool
	ttyEffective bool
	verbose      bool
	exitCode     int
}

func runRm(ctx *Context) int {
	var rawArgs []string
	presumeTTY := false
	for _, a := range ctx.Args {
		if a == "---presume-input-tty" {
			presumeTTY = true
			continue
		}
		if a == "---presume-input-pipe" {
			continue
		}
		rawArgs = append(rawArgs, a)
	}

	flags, err := rmSpecParsed.ParseArgs(rawArgs)
	if err != nil {
		if tryRmDashHint(ctx, err) {
			return 1
		}
		fmt.Fprintln(ctx.Stderr, "rm:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, rmSpecParsed) {
		return 0
	}

	if prVal, ok := flags.String("preserve-root"); ok && prVal != "" && prVal != "all" {
		fmt.Fprintf(ctx.Stderr, "rm: invalid value '%s' for option '--preserve-root'\n", prVal)
		return 1
	}
	preserveRoot := !flags.Bool("no-preserve-root")

	recursive := flags.Bool("r") || flags.Bool("R")
	verbose := flags.Bool("v")
	rmDirFlag := flags.Bool("d")

	mode, force := rmComputeMode(ctx.Args)

	if len(flags.Operands) == 0 {
		if force {
			return 0
		}
		fmt.Fprintln(ctx.Stderr, "rm: missing operand")
		return 1
	}

	st := &rmState{
		ctx:          ctx,
		in:           bufio.NewReader(ctx.Stdin),
		mode:         mode,
		force:        force,
		ttyEffective: presumeTTY || isTerminalReader(ctx.Stdin),
		verbose:      verbose,
	}

	if mode == rmAskOnce {
		n := len(flags.Operands)
		ask := false
		msg := ""
		switch {
		case recursive:
			ask = true
			if n == 1 {
				msg = "rm: remove 1 argument recursively? "
			} else {
				msg = fmt.Sprintf("rm: remove %d arguments recursively? ", n)
			}
		case n > 3:
			ask = true
			msg = fmt.Sprintf("rm: remove %d arguments? ", n)
		}
		if ask && !st.confirmRaw(msg) {
			return st.exitCode
		}
	}

	for _, name := range flags.Operands {
		if name == "" {
			fmt.Fprintln(ctx.Stderr, "rm: cannot remove '': No such file or directory")
			st.exitCode = 1
			continue
		}
		if refused, msg := rmRefuseDotDot(name); refused {
			fmt.Fprintln(ctx.Stderr, msg)
			st.exitCode = 1
			continue
		}

		path := resolvePath(ctx, name)
		clean := filepath.Clean(path)
		if isFilesystemRoot(clean) && preserveRoot {
			fmt.Fprintln(ctx.Stderr, "rm: it is dangerous to operate recursively on '/'")
			fmt.Fprintln(ctx.Stderr, "rm: use --no-preserve-root to override this failsafe")
			st.exitCode = 1
			continue
		}
		display := collapseTrailingSlashes(name)

		info, lerr := os.Lstat(path)
		if lerr != nil {
			if !force || !os.IsNotExist(lerr) {
				fmt.Fprintf(ctx.Stderr, "rm: cannot remove '%s': %s\n", display, fmtOsErr(lerr))
				st.exitCode = 1
			}
			continue
		}

		if info.IsDir() {
			if recursive {
				st.removeNode(path, display)
				continue
			}
			if !rmDirFlag {
				fmt.Fprintf(ctx.Stderr, "rm: cannot remove '%s': Is a directory\n", display)
				st.exitCode = 1
				continue
			}
			st.removeLeaf(path, info, display, true)
			continue
		}

		st.removeLeaf(path, info, display, false)
	}
	return st.exitCode
}

// removeNode recursively removes path (which may be a directory), prompting
// as required by the current interactive mode. It reports its own errors and
// tracks exitCode; the return value only indicates whether the node (and, for
// directories, all of its contents) were fully removed.
func (s *rmState) removeNode(path, display string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if !s.force {
				s.reportErr(display, err)
			}
			return false
		}
		s.reportErr(display, err)
		return false
	}

	if !info.IsDir() {
		return s.removeLeaf(path, info, display, false)
	}

	entries, rerr := os.ReadDir(path)
	nonEmpty := rerr == nil && len(entries) > 0
	if s.mode == rmAskAlways && nonEmpty {
		if !s.confirmf("rm: descend into directory '%s'? ", display) {
			return false
		}
	}
	if rerr != nil {
		// With -f, if we can't list the directory (e.g. mode 0333: wx but no read),
		// attempt to remove it directly; it may be empty or we may have delete rights.
		// If the direct remove fails, report the original permission error.
		if s.force && os.IsPermission(rerr) {
			if removeErr := os.Remove(path); removeErr == nil {
				if s.verbose {
					fmt.Fprintf(s.ctx.Stdout, "removed directory '%s'\n", display)
				}
				return true
			}
			// Direct remove failed - report original EACCES, not the removal error.
			s.reportErr(display, rerr)
			return false
		}
		s.reportErr(display, rerr)
		return false
	}

	allOK := true
	for _, e := range entries {
		childPath := filepath.Join(path, e.Name())
		childDisplay := strings.TrimRight(display, "/") + "/" + e.Name()
		if !s.removeNode(childPath, childDisplay) {
			allOK = false
		}
	}
	if !allOK {
		// A child failed or was declined; either way its real error (if
		// any) has already been reported and exitCode set by reportErr —
		// a plain decline is not itself a failure. Skip trying to remove
		// this now-non-empty directory.
		return false
	}

	return s.removeLeaf(path, info, display, true)
}

// removeLeaf removes a single filesystem entry (a file, symlink, or an
// already-empty/childless directory), after any interactive confirmation.
func (s *rmState) removeLeaf(path string, info os.FileInfo, display string, isDir bool) bool {
	if !s.confirmRemoval(path, info, display) {
		return false
	}
	if err := os.Remove(path); err != nil {
		s.reportErr(display, err)
		return false
	}
	if s.verbose {
		if isDir {
			fmt.Fprintf(s.ctx.Stdout, "removed directory '%s'\n", display)
		} else {
			fmt.Fprintf(s.ctx.Stdout, "removed '%s'\n", display)
		}
	}
	return true
}

func (s *rmState) confirmRemoval(path string, info os.FileInfo, display string) bool {
	switch s.mode {
	case rmAskNever:
		return true
	case rmAskAlways:
		return s.confirmDir(path, info, display)
	default:
		if !s.ttyEffective {
			return true
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return true
		}
		if isWritableAccess(path) {
			return true
		}
		return s.confirmDir(path, info, display)
	}
}

// confirmDir prompts for removal; uses "attempt removal of inaccessible directory" phrasing for
// dirs that lack execute permission (matching uutils/GNU rm behavior).
func (s *rmState) confirmDir(path string, info os.FileInfo, display string) bool {
	if info.IsDir() && !canSearch(path) {
		return s.confirmf("rm: attempt removal of inaccessible directory '%s'? ", display)
	}
	return s.confirmf("rm: remove %s '%s'? ", rmClassifyFile(path, info), display)
}

func (s *rmState) confirmf(format string, args ...interface{}) bool {
	return s.confirmRaw(fmt.Sprintf(format, args...))
}

func (s *rmState) confirmRaw(msg string) bool {
	fmt.Fprint(s.ctx.Stderr, msg)
	line, _ := s.in.ReadString('\n')
	line = strings.TrimSpace(line)
	return len(line) > 0 && (line[0] == 'y' || line[0] == 'Y')
}

func (s *rmState) reportErr(display string, err error) {
	fmt.Fprintf(s.ctx.Stderr, "rm: cannot remove '%s': %s\n", display, fmtOsErr(err))
	s.exitCode = 1
}

func rmClassifyFile(path string, info os.FileInfo) string {
	if info.Mode()&os.ModeSymlink != 0 {
		return "symbolic link"
	}
	if info.IsDir() {
		if isWritableAccess(path) {
			return "directory"
		}
		// If we can't execute (traverse) the directory at all, it's inaccessible.
		if !canSearch(path) {
			return "inaccessible directory"
		}
		return "write-protected directory"
	}
	kind := "regular file"
	if info.Mode().IsRegular() && info.Size() == 0 {
		kind = "regular empty file"
	}
	if isWritableAccess(path) {
		return kind
	}
	return "write-protected " + kind
}

func isWritableAccess(path string) bool {
	return canWrite(path)
}

func isTerminalReader(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(f.Fd())
}

// rmComputeMode scans the raw argument list (independently of the option
// parser) so that -f/-i/-I/--interactive can override one another based on
// which appears last on the command line, matching GNU rm.
func rmComputeMode(argv []string) (rmInteractiveMode, bool) {
	mode := rmAskDefault
	force := false
	for _, a := range argv {
		if a == "--" {
			break
		}
		if len(a) < 2 || a[0] != '-' {
			continue
		}
		if a[1] == '-' {
			name := a[2:]
			val := ""
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				val = name[eq+1:]
				name = name[:eq]
			}
			switch name {
			case "force":
				mode = rmAskNever
				force = true
			case "interactive":
				switch val {
				case "", "always", "yes":
					mode = rmAskAlways
				case "once":
					mode = rmAskOnce
				case "never", "no", "none":
					mode = rmAskNever
				}
			}
			continue
		}
		for i := 1; i < len(a); i++ {
			switch a[i] {
			case 'f':
				mode = rmAskNever
				force = true
			case 'i':
				mode = rmAskAlways
			case 'I':
				mode = rmAskOnce
			}
		}
	}
	return mode, force
}

func rmRefuseDotDot(name string) (bool, string) {
	trimmed := strings.TrimRight(name, "/")
	if trimmed == "" {
		return false, ""
	}
	base := trimmed
	if idx := strings.LastIndexByte(trimmed, '/'); idx >= 0 {
		base = trimmed[idx+1:]
	}
	if base == "." || base == ".." {
		return true, fmt.Sprintf("rm: refusing to remove '.' or '..' directory: skipping '%s'", name)
	}
	return false, ""
}

func collapseTrailingSlashes(name string) string {
	trimmed := strings.TrimRight(name, "/")
	if trimmed == name {
		return name
	}
	if trimmed == "" {
		return "/"
	}
	return trimmed + "/"
}

func tryRmDashHint(ctx *Context, err error) bool {
	for _, a := range ctx.Args {
		if a == "--" {
			break
		}
		if !isOption(a) {
			continue
		}
		if _, statErr := os.Lstat(resolvePath(ctx, a)); statErr == nil {
			fmt.Fprintln(ctx.Stderr, "rm:", err)
			fmt.Fprintf(ctx.Stderr, "Try 'rm ./%s' to remove the file %s.\n", shellQuoteIfNeeded(a), shellQuoteAlways(a))
			fmt.Fprintln(ctx.Stderr, "Try 'rm --help' for more information.")
			return true
		}
	}
	return false
}

func shellSafeChar(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	}
	switch r {
	case '-', '_', '.', '/', ',', ':', '@', '%', '+', '=':
		return true
	}
	return false
}

func shellQuoteIfNeeded(s string) string {
	for _, r := range s {
		if !shellSafeChar(r) {
			return shellQuoteAlways(s)
		}
	}
	return s
}

func shellQuoteAlways(s string) string {
	var b strings.Builder
	inQuote := false
	open := func() {
		if !inQuote {
			b.WriteByte('\'')
			inQuote = true
		}
	}
	closeQ := func() {
		if inQuote {
			b.WriteByte('\'')
			inQuote = false
		}
	}
	for _, r := range s {
		switch {
		case r == '\'':
			closeQ()
			b.WriteString(`\'`)
		case r == '\n':
			closeQ()
			b.WriteString(`$'\n'`)
		case r == '\t':
			closeQ()
			b.WriteString(`$'\t'`)
		case r == '\r':
			closeQ()
			b.WriteString(`$'\r'`)
		case r == '\a':
			closeQ()
			b.WriteString(`$'\a'`)
		case r == '\b':
			closeQ()
			b.WriteString(`$'\b'`)
		case r == '\f':
			closeQ()
			b.WriteString(`$'\f'`)
		case r == '\v':
			closeQ()
			b.WriteString(`$'\v'`)
		case r < 0x20 || r == 0x7f:
			closeQ()
			fmt.Fprintf(&b, `$'\%03o'`, r)
		default:
			open()
			b.WriteRune(r)
		}
	}
	closeQ()
	if b.Len() == 0 {
		return "''"
	}
	return b.String()
}
