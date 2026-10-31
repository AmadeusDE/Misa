// Tar command behavior informed by Toybox tar.c (0BSD).
package builtins

import (
	"archive/tar"
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AmadeusDE/Misa/internal/archive/tarx"
)

func tarPath(dir, name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(dir, name)
}
func tarSelected(name string, selections []string) bool {
	if len(selections) == 0 {
		return true
	}
	n := strings.TrimSuffix(strings.TrimPrefix(name, "./"), "/")
	for _, s := range selections {
		s = strings.TrimSuffix(strings.TrimPrefix(s, "./"), "/")
		if s == "" || s == "." || n == s || strings.HasPrefix(n, s+"/") {
			return true
		}
	}
	return false
}
func runTar(ctx *Context) (code int) {
	originalStdout := ctx.Stdout
	ctx, cancel := streamContext(ctx)
	defer cancel()
	defer streamResult(ctx, &code)
	fail := func(err error) int { fmt.Fprintln(ctx.Stderr, "tar:", err); return 2 }
	usage := func(err error) int { fmt.Fprintln(ctx.Stderr, "tar:", err); return 1 }
	if err := ctx.CancellationContext().Err(); err != nil {
		return fail(err)
	}
	ev, err := tarScan(ctx.Args)
	if err != nil {
		return usage(err)
	}
	mode := ""
	archive := "-"
	verbose, deref, numeric, null, toStdout := false, false, false, false, false
	gzipMode, keep, samePerm, noOwner, touch := false, false, false, false, false
	strip := 0
	var owner, group *int
	var excludes []string
	for _, e := range ev {
		switch e.kind {
		case "--help", "--version":
			tarHelpVersion(ctx, e.kind)
			return 0
		case "j", "J", "a":
			return usage(fmt.Errorf("-%s is not supported", e.kind))
		case "z":
			gzipMode = true
		case "k", "keep-old":
			keep = true
		case "overwrite":
			keep = false
		case "p":
			samePerm = true
		case "o":
			noOwner = true
		case "m":
			touch = true
		case "strip-components":
			strip, err = strconv.Atoi(e.value)
			if err != nil || strip < 0 {
				return usage(fmt.Errorf("invalid strip-components: %s", e.value))
			}
		case "c", "t", "x":
			if mode != "" && mode != e.kind {
				return usage(fmt.Errorf("conflicting operation modes"))
			}
			mode = e.kind
		case "f":
			archive = e.value
		case "v":
			verbose = true
		case "h":
			deref = true
		case "O":
			toStdout = true
		case "numeric-owner":
			numeric = true
		case "null":
			null = true
		case "exclude":
			excludes = append(excludes, e.value)
		case "owner", "group":
			n, err := strconv.Atoi(e.value)
			if err != nil || n < 0 {
				return usage(fmt.Errorf("--%s requires a numeric ID", e.kind))
			}
			if e.kind == "owner" {
				owner = &n
			} else {
				group = &n
			}
		}
	}
	if mode == "" {
		return usage(fmt.Errorf("choose -c, -t or -x"))
	}
	if mode != "c" && archive == "-" {
		for _, e := range ev {
			if e.kind == "T" && e.value == "-" {
				return usage(fmt.Errorf("cannot read both archive and file list from stdin"))
			}
		}
	}
	if mode == "c" && toStdout {
		return usage(fmt.Errorf("-O is not supported when creating"))
	}
	dir := ctx.Dir
	var sources []tarx.Source
	var selected []string
	partial := false
	hadFilesFrom := false
	add := func(name string) {
		if mode == "c" {
			sources = append(sources, tarx.Source{Dir: dir, Name: name})
		} else {
			selected = append(selected, name)
		}
	}
	for _, e := range ev {
		switch e.kind {
		case "C":
			dir = tarPath(dir, e.value)
			st, err := os.Stat(dir)
			if err != nil {
				return fail(err)
			}
			if !st.IsDir() {
				return fail(fmt.Errorf("%s: not a directory", dir))
			}
		case "operand":
			add(e.value)
		case "T":
			var r io.Reader
			var f *os.File
			if e.value == "-" {
				r = ctx.Stdin
			} else {
				f, err = os.Open(tarPath(ctx.Dir, e.value))
				if err == nil {
					r = f
				}
			}
			if err != nil || r == nil {
				if err == nil {
					err = fmt.Errorf("stdin unavailable")
				}
				fmt.Fprintln(ctx.Stderr, "tar:", err)
				partial = true
				err = nil
				continue
			}
			hadFilesFrom = true
			delim := byte('\n')
			if null {
				delim = 0
			}
			br := bufio.NewReader(ContextReader(ctx.CancellationContext(), r))
			for {
				line, re := br.ReadString(delim)
				line = strings.TrimSuffix(line, string(delim))
				if line != "" {
					add(line)
				}
				if re != nil {
					if re != io.EOF {
						fmt.Fprintln(ctx.Stderr, "tar:", re)
						partial = true
					}
					break
				}
			}
			if f != nil {
				if e := f.Close(); e != nil {
					fmt.Fprintln(ctx.Stderr, "tar:", e)
					partial = true
				}
			}
		}
	}
	if mode == "c" && len(sources) == 0 && !hadFilesFrom {
		return usage(fmt.Errorf("refusing to create an empty archive"))
	}
	if mode == "c" {
		var w io.Writer = ctx.Stdout
		var f *os.File
		var info os.FileInfo
		if stdout, ok := originalStdout.(*os.File); ok {
			info, _ = stdout.Stat()
		}
		if archive != "-" {
			f, err = os.Create(tarPath(ctx.Dir, archive))
			if err != nil {
				return fail(err)
			}
			w = ContextWriter(ctx.CancellationContext(), f)
			info, _ = f.Stat()
		}
		if w == nil {
			return fail(fmt.Errorf("stdout unavailable"))
		}
		var vf func(*tar.Header) error
		if verbose {
			vw := ctx.Stdout
			if archive == "-" {
				vw = ctx.Stderr
			}
			vf = func(h *tar.Header) error { _, e := fmt.Fprintln(vw, h.Name); return e }
		}
		var gzclose func() error
		if gzipMode {
			w, gzclose = tarGzipWriter(w)
		}
		err = tarx.Create(ctx.CancellationContext(), w, sources, tarx.CreateOptions{Dereference: deref, NumericOwner: numeric, Owner: owner, Group: group, Exclude: excludes, ExcludeFile: info, Verbose: vf, OnError: func(name string, e error) { fmt.Fprintf(ctx.Stderr, "tar: %s: %v\n", name, e); partial = true }})
		if gzclose != nil {
			if e := gzclose(); err == nil {
				err = e
			}
		}
		if f != nil {
			if e := f.Close(); err == nil {
				err = e
			}
		}
		if err != nil {
			return fail(err)
		}
	} else {
		err = tarReadArchive(ctx, tarReadOptions{archive: archive, dir: dir, mode: mode, gzip: gzipMode, verbose: verbose, numeric: numeric, owner: owner, group: group, selected: selected, excludes: excludes, strip: strip, keep: keep, samePerm: samePerm, noOwner: noOwner, touch: touch, toStdout: toStdout}, &partial)
		if err != nil {
			return fail(err)
		}
	}
	if partial {
		return 2
	}
	return 0
}
