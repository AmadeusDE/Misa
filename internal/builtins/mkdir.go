package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

const mkdirSpec = "p(parents)v(verbose)m(mode):"

var mkdirSpecParsed *Spec

func init() {
	var err error
	mkdirSpecParsed, err = ParseSpec(mkdirSpec)
	if err != nil {
		panic("builtins: bad mkdir spec: " + err.Error())
	}
	Register(Command{Name: "mkdir", Run: runMkdir})
}

func runMkdir(ctx *Context) int {
	flags, err := mkdirSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "mkdir:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, mkdirSpecParsed) {
		return 0
	}
	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "mkdir: error: the following required arguments were not provided:\n  <DIRECTORY>...")
		return 1
	}

	perm := os.FileMode(0777)
	hasMode := false
	if modeStr, ok := flags.String("m"); ok {
		p, err := parseMkdirMode(modeStr)
		if err != nil {
			fmt.Fprintln(ctx.Stderr, "mkdir:", err)
			return 1
		}
		perm = p
		hasMode = true
	}

	parents := flags.Bool("p")
	verbose := flags.Bool("v")
	exitCode := 0

	for _, dir := range flags.Operands {
		if dir == "" {
			fmt.Fprintf(ctx.Stderr, "mkdir: cannot create directory '': No such file or directory\n")
			exitCode = 1
			continue
		}
		path := resolvePath(ctx, dir)
		if parents {
			if err := mkdirParents(ctx, dir, path, perm, hasMode, verbose); err != nil {
				fmt.Fprintf(ctx.Stderr, "mkdir: cannot create directory '%s': %s\n", dir, fmtOsErr(err))
				exitCode = 1
			}
		} else {
			err := os.Mkdir(path, perm)
			if err == nil && hasMode {
				os.Chmod(path, perm)
			}
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "mkdir: cannot create directory '%s': %s\n", dir, fmtOsErr(err))
				exitCode = 1
				continue
			}
			if verbose {
				fmt.Fprintf(ctx.Stdout, "mkdir: created directory '%s'\n", dir)
			}
		}
	}
	return exitCode
}

func parseMkdirMode(spec string) (os.FileMode, error) {
	isOctal := true
	for _, c := range spec {
		if c < '0' || c > '7' {
			isOctal = false
			break
		}
	}
	if isOctal {
		return parseOctalMode(spec)
	}
	return parseSymbolicMode(spec, os.ModeDir|0777)
}

// mkdirParents creates every missing component of path, walking with
// directory descriptors so an ancestor that loses its search bit after we
// pass it doesn't block what is below (as GNU mkdir -p), without touching
// the process's working directory. The final directory is never opened, so
// a -m mode without search permission still succeeds.
func mkdirParents(ctx *Context, rawDir, path string, perm os.FileMode, hasMode, verbose bool) error {
	start, components := ctx.Dir, strings.Split(rawDir, "/")
	absolute := filepath.IsAbs(rawDir)
	if absolute {
		start, components = "/", strings.Split(strings.TrimPrefix(path, "/"), "/")
	}
	dirfd, err := unix.Open(start, dirOpenFlags, 0)
	if err != nil {
		return &os.PathError{Op: "open", Path: start, Err: err}
	}
	defer func() { unix.Close(dirfd) }()

	lastRealIdx := -1
	for i, c := range components {
		if c != "" && c != "." && c != ".." {
			lastRealIdx = i
		}
	}
	parentPerm := (os.FileMode(0777) &^ os.FileMode(currentUmask())) | 0700

	var displayParts []string
	for idx, c := range components {
		if c == "" || c == "." {
			continue
		}
		displayParts = append(displayParts, c)
		isLast := idx == lastRealIdx
		if c != ".." {
			created, err := mkdirAt(dirfd, c)
			if err != nil {
				return err
			}
			if created {
				mode := parentPerm
				if isLast {
					mode = perm
				}
				if !isLast || hasMode {
					unix.Fchmodat(dirfd, c, unixMode(mode), 0)
				}
				if verbose {
					disp := strings.Join(displayParts, "/")
					if absolute {
						disp = "/" + disp
					}
					fmt.Fprintf(ctx.Stdout, "mkdir: created directory '%s'\n", disp)
				}
			}
		}
		if isLast || idx > lastRealIdx {
			return nil
		}
		next, err := unix.Openat(dirfd, c, dirOpenFlags, 0)
		if err != nil {
			return &os.PathError{Op: "open", Path: c, Err: err}
		}
		unix.Close(dirfd)
		dirfd = next
	}
	return nil
}

// mkdirAt creates name under dirfd, reporting whether it was new. An existing
// directory is fine; an existing non-directory is "File exists".
func mkdirAt(dirfd int, name string) (bool, error) {
	err := unix.Mkdirat(dirfd, name, 0777)
	if err == nil {
		return true, nil
	}
	if err != unix.EEXIST {
		return false, &os.PathError{Op: "mkdir", Path: name, Err: err}
	}
	var st unix.Stat_t
	if unix.Fstatat(dirfd, name, &st, 0) == nil && st.Mode&unix.S_IFMT != unix.S_IFDIR {
		return false, fmt.Errorf("File exists")
	}
	return false, nil
}

func unixMode(m os.FileMode) uint32 {
	mode := uint32(m.Perm())
	if m&os.ModeSetuid != 0 {
		mode |= unix.S_ISUID
	}
	if m&os.ModeSetgid != 0 {
		mode |= unix.S_ISGID
	}
	if m&os.ModeSticky != 0 {
		mode |= unix.S_ISVTX
	}
	return mode
}
