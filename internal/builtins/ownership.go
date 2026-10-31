// Ownership traversal shared by chown and chgrp.
//
// One walker covers every -R case: symlink policy is decided per entry instead
// of by picking filepath.WalkDir or a hand-written recursion. Behaviour adapted
// from toybox chgrp/chown (0BSD, toys/posix/chgrp.c at
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb) and uutils coreutils chgrp (MIT,
// src/uu/chgrp at fee61ff5d4f87e0f912a52e639942b5f7ad58663).
//
// Going from two walks to one settles the places chown's -R (filepath.WalkDir)
// and chown's -R -L (its own recursion) disagreed, always in favour of -L or of
// the rule above: a failing file is reported and the walk continues (plain -R
// used to abandon the rest of the tree, and even exited 0 when the operand
// itself was missing); -v names files as walked ("d/f") rather than by resolved
// path; and a symlink that cannot be dereferenced reports the chown/chgrp call's
// error instead of the pre-stat's.
package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// ownershipErrStyle selects the per-file error format.
type ownershipErrStyle int

const (
	// ownershipErrRaw prints the raw error text: "chown: chown /x: operation
	// not permitted".
	ownershipErrRaw ownershipErrStyle = iota
	// ownershipErrContext names the operation: "chgrp: changing group of
	// '/x': Operation not permitted".
	ownershipErrContext
)

// ownershipOp describes one chown/chgrp run.
type ownershipOp struct {
	uid, gid int    // -1 leaves that field alone (a gid of -1 makes chgrp a no-op)
	verb     string // "ownership" for chown, "group" for chgrp

	verbose       bool // -v: report every file the operation is applied to
	changes       bool // -c: report (and act) only when the id actually differs
	silent        bool // -f: suppress error messages (the exit status still reports them)
	noDereference bool // -h/--no-dereference: act on the symlink itself
	followAll     bool // -L: follow every symlink met while recursing
	followCmdline bool // -H: follow symlinks named on the command line only
	recursive     bool // -R

	errStyle ownershipErrStyle
	// danglingCheck adds chown's non-recursive pre-check for a command-line
	// symlink whose target does not exist ("cannot dereference"). It is off
	// for chgrp, which reports the kernel's error for the file instead.
	danglingCheck bool
}

// ownershipWalk carries the state of one run: the options, the command name
// used for messages, and the exit status.
type ownershipWalk struct {
	ctx      *Context
	cmdName  string
	op       ownershipOp
	exitCode int
	visited  map[string]bool // "dev:ino" of the directories already entered
}

// applyOwnership runs the operation over every command-line operand and
// returns the exit status (1 if anything failed).
func applyOwnership(ctx *Context, cmdName string, op ownershipOp, targets []string) int {
	w := &ownershipWalk{ctx: ctx, cmdName: cmdName, op: op}
	for _, name := range targets {
		w.operand(name)
	}
	return w.exitCode
}

// operand applies the operation to one command-line operand.
func (w *ownershipWalk) operand(name string) {
	op := w.op
	path := resolvePath(w.ctx, name)

	// chown keeps its old pre-check: a non-recursive operand that is a symlink
	// is dereferenced up front so a dangling one is reported by the name the
	// user typed. Deliberately before the walk, and only for chown.
	if op.danglingCheck && !op.recursive && !op.noDereference {
		if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
			if _, err := os.Stat(path); err != nil {
				fmt.Fprintf(w.ctx.Stderr, "%s: cannot dereference '%s': %s\n", w.cmdName, name, fmtOsErr(err))
				w.exitCode = 1
				return
			}
		}
	}

	// A command-line symlink is followed unless -h is given (-H and -L also
	// ask for it). With -R the default (-P) does not follow it either: -h is
	// implied by -R, as in toybox and GNU (chgrp -R G link changes the link
	// itself, not its target). Without -R the plain dereference stays.
	dereference := (op.followCmdline || op.followAll || !op.noDereference) &&
		!(op.recursive && !op.followCmdline && !op.followAll)
	descend := op.followCmdline || op.followAll

	w.visited = map[string]bool{}
	w.walk(path, name, dereference, descend)
}

// walk applies the operation to path (reported to the user as display) and,
// with -R, to everything below it. dereference says whether path itself is
// followed when it is a symlink; descend says whether -R may enter it when it
// is a symlinked directory.
func (w *ownershipWalk) walk(path, display string, dereference, descend bool) {
	info, isLink := w.lookup(path, dereference)

	recurse := false
	if info != nil {
		isDir := info.IsDir()
		// Loop protection: a tree can reach the same directory twice through
		// symlinks, so enter each "dev:ino" at most once. Only directories are
		// tracked: hard-linked files are separate paths and stay separate.
		if isDir {
			if st, ok := info.Sys().(*syscall.Stat_t); ok {
				key := fmt.Sprintf("%d:%d", st.Dev, st.Ino)
				if w.visited[key] {
					return
				}
				w.visited[key] = true
			}
		}
		recurse = isDir && (!isLink || descend)
	}

	w.apply(path, display, info, dereference)

	if !w.op.recursive || !recurse {
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		w.report(display, err)
		return
	}
	for _, e := range entries {
		childPath := filepath.Join(path, e.Name())
		childDisplay := display + "/" + e.Name()
		isSymlink := e.Type()&os.ModeSymlink != 0
		if isSymlink && !w.op.followAll {
			// Not descended into, but the operation still applies: to the
			// symlink itself with -h or a plain -R (-h is implied by -R),
			// to its target with -H, which follows every symlink it meets.
			childDeref := !w.op.noDereference && (w.op.followCmdline || w.op.followAll)
			child, _ := w.lookup(childPath, childDeref)
			w.apply(childPath, childDisplay, child, childDeref)
			continue
		}
		childDeref := !w.op.noDereference
		if isSymlink {
			childDeref = true // -L follows everything
		}
		w.walk(childPath, childDisplay, childDeref, w.op.followAll)
	}
}

// apply performs the chown/chgrp call for one file. info is the pre-stat used
// for -c (nil when it failed; the operation is still attempted and the kernel
// error is reported).
func (w *ownershipWalk) apply(path, display string, info os.FileInfo, dereference bool) {
	op := w.op

	changed, known := true, false
	if info != nil {
		if st, ok := info.Sys().(*syscall.Stat_t); ok {
			changed, known = ownershipDiffers(st, op.uid, op.gid), true
		}
	}
	if op.changes && !op.verbose && known && !changed {
		return
	}

	chownFn := os.Chown
	if !dereference {
		chownFn = os.Lchown
	}
	if err := chownFn(path, op.uid, op.gid); err != nil {
		w.report(display, err)
		return
	}
	if op.verbose || op.changes {
		fmt.Fprintf(w.ctx.Stdout, "changed %s of '%s'\n", op.verb, display)
	}
}

// report prints one per-file error and marks the run as failed. -f silences
// the message but not the exit status.
func (w *ownershipWalk) report(display string, err error) {
	w.exitCode = 1
	if w.op.silent {
		return
	}
	if w.op.errStyle == ownershipErrContext {
		fmt.Fprintf(w.ctx.Stderr, "%s: changing %s of '%s': %s\n", w.cmdName, w.op.verb, display, fmtOsErr(err))
		return
	}
	fmt.Fprintf(w.ctx.Stderr, "%s: %v\n", w.cmdName, err)
}

// lookup returns the file information used for -c and to decide recursion,
// plus whether path is a symlink. A symlink is dereferenced when dereference is
// set; a stat that fails then yields nil so the operation itself runs and
// reports the error the kernel gives.
func (w *ownershipWalk) lookup(path string, dereference bool) (os.FileInfo, bool) {
	linfo, err := os.Lstat(path)
	if err != nil {
		return nil, false
	}
	if linfo.Mode()&os.ModeSymlink == 0 {
		return linfo, false
	}
	if !dereference {
		return linfo, true
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, true
	}
	return info, true
}

// ownershipDiffers reports whether the ids in st differ from the requested
// ones; -1 means "leave alone".
func ownershipDiffers(st *syscall.Stat_t, uid, gid int) bool {
	if uid >= 0 && int(st.Uid) != uid {
		return true
	}
	if gid >= 0 && int(st.Gid) != gid {
		return true
	}
	return false
}
