package builtins

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/AmadeusDE/Misa/internal/walk"
)

// grepFilter holds --include/--exclude (in command-line order, since the
// last matching one wins) and --exclude-dir globs.
type grepFilter struct {
	files []grepGlob
	dirs  []string
}

type grepGlob struct {
	pattern string
	include bool
}

func grepFilters(ctx *Context, flags *ParseResult) (*grepFilter, error) {
	f := &grepFilter{dirs: flags.AllStrings("exclude-dir")}
	seen := map[string]int{}
	for _, name := range flags.Order() {
		if name != "include" && name != "exclude" && name != "exclude-from" {
			continue
		}
		value := flags.AllStrings(name)[seen[name]]
		seen[name]++
		if name != "exclude-from" {
			f.files = append(f.files, grepGlob{value, name == "include"})
			continue
		}
		data, err := grepReadPatternFile(ctx, value)
		if err != nil {
			return nil, fmt.Errorf("%s: %s", value, grepErrText(err))
		}
		for _, line := range strings.Split(string(data), "\n") {
			if line != "" {
				f.files = append(f.files, grepGlob{line, false})
			}
		}
	}
	return f, nil
}

// skipFile reports whether a file is filtered out. With no matching glob a
// file is kept, unless the first glob is an --include.
func (f *grepFilter) skipFile(name string, operand bool) bool {
	skip := len(f.files) > 0 && f.files[0].include
	for _, g := range f.files {
		if grepGlobMatch(g.pattern, name, operand) {
			skip = !g.include
		}
	}
	return skip
}

func (f *grepFilter) skipDir(name string, operand bool) bool {
	for _, pattern := range f.dirs {
		if grepGlobMatch(pattern, name, operand) {
			return true
		}
	}
	return false
}

// grepGlobMatch matches a base name found while recursing, or any suffix of
// a command-line name that starts right after a '/', as GNU grep does.
func grepGlobMatch(pattern, name string, operand bool) bool {
	name = strings.TrimRight(name, "/")
	if !operand {
		ok, _ := path.Match(pattern, path.Base(name))
		return ok
	}
	for {
		if ok, _ := path.Match(pattern, name); ok {
			return true
		}
		i := strings.IndexByte(name, '/')
		if i < 0 {
			return false
		}
		name = strings.TrimLeft(name[i+1:], "/")
	}
}

var errGrepStop = errors.New("grep: stop")

// walk searches root and, for a directory, everything below it. Symlinks
// met while recursing are followed only with -R; devices, FIFOs and
// sockets met while recursing are skipped so a search never blocks.
func (g *grepRun) walk(root string) {
	opt := walk.Options{Base: g.ctx.Dir, FollowRoot: true, FollowLinks: g.o.dereference}
	walk.Walk(context.Background(), root, opt, func(e walk.Entry) error {
		if g.stop {
			return errGrepStop
		}
		operand := e.Depth == 0
		switch {
		case errors.Is(e.Err, walk.ErrLoop):
			if !g.o.noMessages {
				fmt.Fprintf(g.ctx.Stderr, "grep: %s: warning: recursive directory loop\n", e.Path)
			}
		case e.Err != nil:
			g.fail(e.Path, e.Err)
		case e.Info.IsDir():
			if e.Path != "" && g.filter.skipDir(e.Path, operand) {
				return walk.SkipDir
			}
			if !g.o.noFilename {
				g.o.withFilename = true
			}
		case operand || e.Info.Mode().IsRegular():
			if !g.filter.skipFile(e.Path, operand) {
				g.file(e.Path)
			}
		}
		return nil
	})
}
