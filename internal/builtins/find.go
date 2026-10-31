// Walk orchestration inspired by Toybox find.c, copyright 2014 Rob Landley,
// commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb (0BSD).
package builtins

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/AmadeusDE/Misa/internal/walk"
)

var findQuit = errors.New("find quit")

func init() { Register(Command{Name: "find", Run: runFind}) }

func runFind(ctx *Context) int {
	args := ctx.Args
	opt := findOptions{}
	for len(args) > 0 {
		switch args[0] {
		case "-H":
			opt.follow = false
			opt.followRoot = true
		case "-L":
			opt.follow = true
			opt.followRoot = false
		case "-P":
			opt.follow = false
			opt.followRoot = false
		default:
			goto roots
		}
		args = args[1:]
	}
roots:
	var paths []string
	for len(args) > 0 && !strings.HasPrefix(args[0], "-") && args[0] != "!" && args[0] != "(" && args[0] != ")" && args[0] != "," {
		paths = append(paths, args[0])
		args = args[1:]
	}
	if len(paths) == 0 {
		paths = []string{"."}
	}
	p := findParser{ctx: ctx, args: args, options: opt, now: ctx.Now()}
	node, err := p.parse()
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "find: %v\n", err)
		return 1
	}
	if !p.action {
		node = findAnd(node, func(e *findEntry) bool {
			_, err := fmt.Fprintln(e.ctx.Stdout, e.path)
			if err != nil {
				return e.fail(err)
			}
			return true
		})
	}
	status := 0
	for _, root := range paths {
		if root == "" {
			fmt.Fprintln(ctx.Stderr, "find: '': empty path")
			status = 1
			continue
		}
		err = walk.Walk(ctx.DoneContext(), root, walk.Options{Base: ctx.Dir, FollowLinks: p.options.follow, FollowRoot: p.options.followRoot, PostOrder: p.options.depth, MaxDepth: p.options.maxDepth, SameDevice: p.options.xdev, DanglingLinks: true}, func(w walk.Entry) error {
			if w.Err != nil {
				fmt.Fprintf(ctx.Stderr, "find: %s: %v\n", w.Path, w.Err)
				status = 1
				return nil
			}
			if w.Depth < p.options.minDepth {
				return nil
			}
			full := w.Path
			if !filepath.IsAbs(full) {
				full = filepath.Join(ctx.Dir, full)
			}
			e := &findEntry{ctx: ctx, path: w.Path, full: full, root: root, info: w.Info, depth: w.Depth, now: p.now, follow: p.options.follow || (w.Depth == 0 && p.options.followRoot)}
			node(e)
			if e.err != nil {
				fmt.Fprintf(ctx.Stderr, "find: %s: %v\n", w.Path, e.err)
				status = 1
			}
			if e.quit {
				return findQuit
			}
			if e.prune && !p.options.depth {
				return walk.SkipDir
			}
			return nil
		})
		if errors.Is(err, findQuit) {
			break
		}
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "find: %v\n", err)
			status = 1
			break
		}
	}
	return status
}
