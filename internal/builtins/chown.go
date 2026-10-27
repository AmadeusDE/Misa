package builtins

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const chownSpec = "R(recursive)v(verbose)f(silent)h(no-dereference)(dereference)LHP"

var chownSpecParsed *Spec

func init() {
	var err error
	chownSpecParsed, err = ParseSpec(chownSpec)
	if err != nil {
		panic("builtins: bad chown spec: " + err.Error())
	}
	Register(Command{Name: "chown", Run: runChown})
}

func runChown(ctx *Context) int {
	flags, err := chownSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "chown:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, chownSpecParsed) {
		return 0
	}
	if len(flags.Operands) < 2 {
		fmt.Fprintln(ctx.Stderr, "chown: missing operand")
		return 1
	}

	ownerSpec := flags.Operands[0]
	targets := flags.Operands[1:]
	recursive := flags.Bool("R")
	verbose := flags.Bool("v")
	silent := flags.Bool("f")
	noDereference := flags.Bool("h")
	followAll := flags.Bool("L")

	uid, gid, err := parseOwnerGroup(ownerSpec)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "chown: %v\n", err)
		return 1
	}

	chownFn := os.Chown
	if noDereference {
		chownFn = os.Lchown
	}

	exitCode := 0

	applyOwner := func(path string) error {
		if err := chownFn(path, uid, gid); err != nil {
			return err
		}
		if verbose {
			fmt.Fprintf(ctx.Stdout, "changed ownership of '%s'\n", path)
		}
		return nil
	}

	for _, name := range targets {
		path := resolvePath(ctx, name)

		if !noDereference {
			if info, lerr := os.Lstat(path); lerr == nil && info.Mode()&os.ModeSymlink != 0 {
				if _, serr := os.Stat(path); serr != nil {
					fmt.Fprintf(ctx.Stderr, "chown: cannot dereference '%s': %s\n", name, fmtOsErr(serr))
					exitCode = 1
					continue
				}
			}
		}

		if recursive && followAll {
			visited := map[string]bool{}
			chownRecursiveL(ctx, path, name, uid, gid, verbose, silent, visited, &exitCode)
			continue
		}

		if recursive {
			err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
				if err != nil {
					if !silent {
						fmt.Fprintf(ctx.Stderr, "chown: %v\n", err)
					}
					return nil
				}
				return applyOwner(p)
			})
			if err != nil {
				if !silent {
					fmt.Fprintf(ctx.Stderr, "chown: %v\n", err)
				}
				exitCode = 1
			}
		} else {
			if err := applyOwner(path); err != nil {
				if !silent {
					fmt.Fprintf(ctx.Stderr, "chown: %v\n", err)
				}
				exitCode = 1
			}
		}
	}
	return exitCode
}

func chownRecursiveL(ctx *Context, path, display string, uid, gid int, verbose, silent bool, visited map[string]bool, exitCode *int) {
	info, err := os.Stat(path)
	if err != nil {
		if !silent {
			fmt.Fprintf(ctx.Stderr, "chown: %v\n", err)
		}
		*exitCode = 1
		return
	}

	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		key := fmt.Sprintf("%d:%d", st.Dev, st.Ino)
		if visited[key] {
			return
		}
		visited[key] = true
	}

	if err := os.Chown(path, uid, gid); err != nil {
		if !silent {
			fmt.Fprintf(ctx.Stderr, "chown: %v\n", err)
		}
		*exitCode = 1
	} else if verbose {
		fmt.Fprintf(ctx.Stdout, "changed ownership of '%s'\n", display)
	}

	if !info.IsDir() {
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		if !silent {
			fmt.Fprintf(ctx.Stderr, "chown: %v\n", err)
		}
		*exitCode = 1
		return
	}
	for _, e := range entries {
		chownRecursiveL(ctx, filepath.Join(path, e.Name()), display+"/"+e.Name(), uid, gid, verbose, silent, visited, exitCode)
	}
}

func parseOwnerGroup(spec string) (uid, gid int, err error) {
	uid, gid = -1, -1
	if spec == "" {
		return uid, gid, nil
	}

	sep := strings.IndexAny(spec, ":.")
	hasSep := sep >= 0
	var userPart, groupPart string
	if !hasSep {
		userPart = spec
	} else {
		userPart = spec[:sep]
		groupPart = spec[sep+1:]
	}

	var resolvedUser *user.User
	if userPart != "" {
		if u, lerr := user.Lookup(userPart); lerr == nil {
			resolvedUser = u
			uid, _ = strconv.Atoi(u.Uid)
		} else if n, cerr := strconv.Atoi(userPart); cerr == nil {
			uid = n
		} else {
			return -1, -1, fmt.Errorf("invalid user: %q", spec)
		}
	}

	if hasSep {
		if groupPart != "" {
			if n, cerr := strconv.Atoi(groupPart); cerr == nil {
				gid = n
			} else if g, lerr := user.LookupGroup(groupPart); lerr == nil {
				gid, _ = strconv.Atoi(g.Gid)
			} else {
				return -1, -1, fmt.Errorf("invalid group: %q", spec)
			}
		} else if userPart != "" {
			if resolvedUser == nil {
				return -1, -1, fmt.Errorf("invalid spec: %q", spec)
			}
			gid, _ = strconv.Atoi(resolvedUser.Gid)
		}
	}

	return uid, gid, nil
}
