// Package walk visits a directory tree in sorted order for the builtins that
// recurse (grep -r, and later du, chmod -R and friends), with GNU-style
// symlink choices and directory loop detection.
package walk

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Options says where relative roots resolve and which symlinks to follow.
type Options struct {
	Base        string // directory relative roots resolve against; "" is the process's
	FollowLinks bool   // follow every symlink, the root included (-R, -L)
	FollowRoot  bool   // follow only a symlink given as the root (-r, -H)
}

// Entry is one visited file. Path is the root joined with the names below
// it, so an empty root yields bare relative names. When Err is set the
// entry could not be read or, for a directory, listed; Info is then the
// symlink's own information, or nil when nothing could be read.
type Entry struct {
	Path  string
	Info  fs.FileInfo
	Depth int
	Err   error
}

// SkipDir returned for a directory prunes it; for anything else it is
// ignored.
var SkipDir = fs.SkipDir

// ErrLoop marks a directory that is one of its own ancestors, or, where
// files have no inode to compare, one nested deeper than maxDepth.
var ErrLoop = errors.New("recursive directory loop")

const maxDepth = 2048

// Walk calls fn for root and then, depth first and sorted by name, for
// everything below it. A directory is passed to fn before its contents. An
// error from fn other than SkipDir stops the walk and is returned, as is
// the context's error once it is done.
func Walk(ctx context.Context, root string, opt Options, fn func(Entry) error) error {
	w := &walker{ctx: ctx, opt: opt, fn: fn}
	info, err := os.Lstat(w.fsPath(root))
	if err == nil && info.Mode()&fs.ModeSymlink != 0 && (opt.FollowRoot || opt.FollowLinks) {
		var target fs.FileInfo
		if target, err = os.Stat(w.fsPath(root)); err == nil {
			info = target
		}
	}
	if err != nil {
		return w.report(root, info, 0, err)
	}
	return w.visit(root, info, 0)
}

type walker struct {
	ctx       context.Context
	opt       Options
	fn        func(Entry) error
	ancestors []fileID
}

func (w *walker) visit(path string, info fs.FileInfo, depth int) error {
	id, hasID := idOf(info)
	if info.IsDir() && (hasID && slices.Contains(w.ancestors, id) || !hasID && depth > maxDepth) {
		return w.report(path, info, depth, ErrLoop)
	}
	if err := w.call(Entry{Path: path, Info: info, Depth: depth}); err != nil || !info.IsDir() {
		return skipped(err)
	}
	if hasID {
		w.ancestors = append(w.ancestors, id)
		defer func() { w.ancestors = w.ancestors[:len(w.ancestors)-1] }()
	}
	entries, err := os.ReadDir(w.fsPath(path))
	if err != nil {
		if err := w.report(path, info, depth, err); err != nil {
			return err
		}
	}
	for _, d := range entries {
		child := join(path, d.Name())
		childInfo, err := w.childInfo(child, d)
		if err != nil {
			err = w.report(child, childInfo, depth+1, err)
		} else {
			err = w.visit(child, childInfo, depth+1)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *walker) childInfo(path string, d fs.DirEntry) (fs.FileInfo, error) {
	info, err := d.Info()
	if err != nil || !w.opt.FollowLinks || info.Mode()&fs.ModeSymlink == 0 {
		return info, err
	}
	target, err := os.Stat(w.fsPath(path))
	if err != nil {
		return info, err
	}
	return target, nil
}

func (w *walker) report(path string, info fs.FileInfo, depth int, err error) error {
	return skipped(w.call(Entry{Path: path, Info: info, Depth: depth, Err: err}))
}

func (w *walker) call(e Entry) error {
	if err := w.ctx.Err(); err != nil {
		return err
	}
	return w.fn(e)
}

func skipped(err error) error {
	if err == SkipDir {
		return nil
	}
	return err
}

func (w *walker) fsPath(path string) string {
	switch {
	case path == "":
		path = "."
	case filepath.IsAbs(path):
		return path
	}
	if w.opt.Base == "" {
		return path
	}
	return filepath.Join(w.opt.Base, path)
}

func join(dir, name string) string {
	if dir == "" || strings.HasSuffix(dir, "/") {
		return dir + name
	}
	return dir + "/" + name
}
