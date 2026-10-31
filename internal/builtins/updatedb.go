// updatedb builds the locate database.
//
// Argv and the default output location's role follow uutils/findutils
// src/updatedb/mod.rs, commit d71f3242b0bc3b21bb311f04b23ae62e5f3fed67 (MIT);
// the database written is Misa's own format (locate_db.go). Exclusions are
// prune paths instead of find expressions, and the database is rebuilt in full
// on every run: there is no incremental mode.
package builtins

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AmadeusDE/Misa/internal/walk"
)

const updatedbSpec = "(localpaths):(prunepaths):(output):(dbformat):(findoptions):(prunefs):(localuser):(netpaths):(netuser)"

var updatedbSpecParsed *Spec

func init() {
	var err error
	if updatedbSpecParsed, err = ParseSpec(updatedbSpec); err != nil {
		panic("builtins: bad updatedb spec: " + err.Error())
	}
	Register(Command{Name: "updatedb", Run: runUpdatedb})
}

// updatedbUnsupported lists the findutils options this updatedb does not
// implement. They fail with a message naming the option instead of silently
// indexing more or less than asked.
var updatedbUnsupported = map[string]string{
	"--findoptions": "--findoptions is not supported (this updatedb prunes by path only)",
	"--prunefs":     "--prunefs is not supported (prune filesystems by path with --prunepaths instead)",
	"--dbformat":    "--dbformat is not supported (updatedb always writes the Misa locate format)",
	"--localuser":   "--localuser is not supported (updatedb does not switch user)",
	"--netpaths":    "--netpaths is not supported (index network mounts as local paths)",
	"--netuser":     "--netuser is not supported (updatedb does not switch user)",
}

func updatedbUnsupportedOption(args []string) (string, bool) {
	for _, a := range args {
		if a == "--" {
			return "", false
		}
		if !strings.HasPrefix(a, "--") {
			continue
		}
		name, _, _ := strings.Cut(a, "=")
		if msg, ok := updatedbUnsupported[name]; ok {
			return msg, true
		}
	}
	return "", false
}

func runUpdatedb(ctx *Context) int {
	if msg, ok := updatedbUnsupportedOption(ctx.Args); ok {
		fmt.Fprintf(ctx.Stderr, "updatedb: %s\nTry 'updatedb --help' for more information.\n", msg)
		return 2
	}
	flags, err := updatedbSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "updatedb: %v\nTry 'updatedb --help' for more information.\n", err)
		return 2
	}
	if HandleHelpVersion(ctx, flags, updatedbSpecParsed) {
		return 0
	}
	if len(flags.Operands) > 0 {
		fmt.Fprintf(ctx.Stderr, "updatedb: extra operand %q\nTry 'updatedb --help' for more information.\n", flags.Operands[0])
		return 2
	}

	var rawRoots []string
	if v, ok := flags.ExplicitString("localpaths"); ok {
		rawRoots = strings.Fields(v)
	}
	if len(rawRoots) == 0 {
		rawRoots = []string{"/"}
	}
	roots := make([]string, 0, len(rawRoots))
	for _, root := range rawRoots {
		roots = append(roots, locateResolve(ctx.Dir, root))
	}
	var prunes []string
	if v, ok := flags.ExplicitString("prunepaths"); ok {
		for _, p := range strings.Fields(v) {
			prunes = append(prunes, filepath.Clean(locateResolve(ctx.Dir, p)))
		}
	}

	output, ok := flags.ExplicitString("output")
	if !ok {
		// The default location lives in the user's cache, so create it. An
		// explicit --output must name a directory that already exists, as with
		// findutils.
		if output, err = locateDefaultDBPath(ctx); err != nil {
			fmt.Fprintf(ctx.Stderr, "updatedb: %v\n", err)
			return 1
		}
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
			fmt.Fprintf(ctx.Stderr, "updatedb: cannot create %q: %s\n", filepath.Dir(output), fmtOsErr(err))
			return 1
		}
	}
	output = locateResolve(ctx.Dir, output)

	status := 0
	err = writeLocateDBFile(output, ctx.Now().Unix(), roots, func(add func(string) error) error {
		for _, root := range roots {
			if locatePruned(root, prunes) {
				continue
			}
			err := walk.Walk(ctx.DoneContext(), root, walk.Options{}, func(e walk.Entry) error {
				if e.Err != nil {
					fmt.Fprintf(ctx.Stderr, "updatedb: %s: %s\n", e.Path, fmtOsErr(e.Err))
					status = 1
					return nil
				}
				if locatePruned(e.Path, prunes) {
					if e.Info != nil && e.Info.IsDir() {
						return walk.SkipDir
					}
					return nil
				}
				return add(e.Path)
			})
			if err != nil {
				if cerr := ctx.DoneContext().Err(); cerr != nil {
					return fmt.Errorf("interrupted: %v", cerr)
				}
				return err
			}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "updatedb: %v\n", err)
		return 1
	}
	return status
}

// locatePruned reports whether path is one of the excluded subtrees.
func locatePruned(path string, prunes []string) bool {
	clean := filepath.Clean(path)
	for _, p := range prunes {
		if clean == p || strings.HasPrefix(clean, p+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// writeLocateDBFile writes the database to a temporary file next to path and
// renames it into place, so a failure or a cancellation leaves the previous
// database untouched and no half-written database is ever visible.
func writeLocateDBFile(path string, created int64, roots []string, fill func(add func(string) error) error) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".locatedb-*")
	if err != nil {
		return fmt.Errorf("cannot create %q: %s", path, fmtOsErr(err))
	}
	tmpName := tmp.Name()
	fail := func(err error) error {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("cannot write %q: %s", path, fmtOsErr(err))
	}
	w := bufio.NewWriterSize(tmp, 256<<10)
	lw, err := newLocateWriter(w, created, roots)
	if err != nil {
		return fail(err)
	}
	if err := fill(lw.Add); err != nil {
		return fail(err)
	}
	if err := lw.Finish(); err != nil {
		return fail(err)
	}
	if err := w.Flush(); err != nil {
		return fail(err)
	}
	if err := tmp.Sync(); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("cannot write %q: %s", path, fmtOsErr(err))
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("cannot write %q: %s", path, fmtOsErr(err))
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("cannot replace %q: %s", path, fmtOsErr(err))
	}
	return nil
}
