package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func diffDirectory(ctx *Context, a, b string, ai, bi os.FileInfo, ae, be error, o diffOptions, seen map[string]bool) int {
	leftMissing := os.IsNotExist(ae) && (o.newFile || o.unidirectional)
	rightMissing := os.IsNotExist(be) && o.newFile
	if leftMissing {
		ae = nil
	}
	if rightMissing {
		be = nil
	}
	if ae != nil || be != nil {
		if ae != nil {
			diffReadError(ctx, a, ae)
		}
		if be != nil {
			diffReadError(ctx, b, be)
		}
		return 2
	}
	if ai != nil && !ai.IsDir() || bi != nil && !bi.IsDir() {
		left, right := "a directory", "a regular file"
		if bi != nil && bi.IsDir() {
			left, right = right, left
		}
		fmt.Fprintf(ctx.Stdout, "File %s is %s while file %s is %s\n", a, left, b, right)
		return 1
	}
	ap, bp := resolvePath(ctx, a), resolvePath(ctx, b)
	var errA, errB error
	if !leftMissing {
		ap, errA = filepath.EvalSymlinks(ap)
	}
	if !rightMissing {
		bp, errB = filepath.EvalSymlinks(bp)
	}
	if errA != nil || errB != nil {
		if errA != nil {
			diffReadError(ctx, a, errA)
		}
		if errB != nil {
			diffReadError(ctx, b, errB)
		}
		return 2
	}
	key := ap + "\x00" + bp
	if seen[key] {
		fmt.Fprintln(ctx.Stderr, "diff: recursive directory loop")
		return 2
	}
	seen[key] = true
	defer delete(seen, key)
	var as, bs []os.DirEntry
	if !leftMissing {
		as, ae = os.ReadDir(ap)
	}
	if !rightMissing {
		bs, be = os.ReadDir(bp)
	}
	if ae != nil || be != nil {
		if ae != nil {
			diffReadError(ctx, a, ae)
		}
		if be != nil {
			diffReadError(ctx, b, be)
		}
		return 2
	}
	names := map[string]uint8{}
	for _, e := range as {
		names[e.Name()] |= 1
	}
	for _, e := range bs {
		names[e.Name()] |= 2
	}
	sorted := make([]string, 0, len(names))
	for name := range names {
		excluded := false
		for _, pattern := range o.excludes {
			if matched, _ := filepath.Match(pattern, name); matched {
				excluded = true
				break
			}
		}
		if !excluded {
			sorted = append(sorted, name)
		}
	}
	sort.Strings(sorted)
	result := 0
	for _, name := range sorted {
		if names[name] != 3 && !o.newFile && !(o.unidirectional && names[name] == 2) {
			dir := a
			if names[name] == 2 {
				dir = b
			}
			fmt.Fprintf(ctx.Stdout, "Only in %s: %s\n", dir, name)
			result = max(result, 1)
			continue
		}
		left, right := filepath.Join(a, name), filepath.Join(b, name)
		li, le := os.Stat(resolvePath(ctx, left))
		ri, re := os.Stat(resolvePath(ctx, right))
		if !o.recursive && (le == nil && li.IsDir() || re == nil && ri.IsDir()) && (le != nil || li.IsDir()) && (re != nil || ri.IsDir()) {
			fmt.Fprintf(ctx.Stdout, "Common subdirectories: %s and %s\n", left, right)
			continue
		}
		result = max(result, diffCompare(ctx, left, right, o, true, seen))
	}
	return result
}

func diffDirectoryFlags(o diffOptions) string {
	if o.commandFlags != "" {
		return o.commandFlags
	}
	var flags []string
	if o.recursive {
		flags = append(flags, "-r")
	}
	switch o.style {
	case "unified":
		flags = append(flags, fmt.Sprintf("-U %d", o.context))
	case "context":
		flags = append(flags, fmt.Sprintf("-C %d", o.context))
	case "ed":
		flags = append(flags, "-e")
	case "side":
		flags = append(flags, "-y")
	}
	if o.ignoreCase {
		flags = append(flags, "-i")
	}
	if o.ignoreSpace {
		flags = append(flags, "-b")
	}
	if o.ignoreAllSpace {
		flags = append(flags, "-w")
	}
	if len(flags) == 0 {
		return ""
	}
	return strings.Join(flags, " ") + " "
}
