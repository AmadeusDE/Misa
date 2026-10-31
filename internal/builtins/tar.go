// Tar command behavior informed by Toybox tar.c (0BSD).
package builtins

import (
	"archive/tar"
	"bufio"
	"context"
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
func runTar(ctx *Context) int {
	fail := func(err error) int { fmt.Fprintln(ctx.Stderr, "tar:", err); return 2 }
	ev, err := tarScan(ctx.Args)
	if err != nil {
		return fail(err)
	}
	mode := ""
	archive := "-"
	verbose, deref, numeric, null, toStdout := false, false, false, false, false
	var owner, group *int
	var excludes []string
	for _, e := range ev {
		switch e.kind {
		case "--help", "--version":
			tarHelpVersion(ctx, e.kind)
			return 0
		case "x", "z", "j", "J", "a":
			return fail(fmt.Errorf("-%s is not supported until U33", e.kind))
		case "c", "t":
			if mode != "" && mode != e.kind {
				return fail(fmt.Errorf("conflicting operation modes"))
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
				return fail(fmt.Errorf("--%s requires a numeric ID", e.kind))
			}
			if e.kind == "owner" {
				owner = &n
			} else {
				group = &n
			}
		}
	}
	if mode == "" {
		return fail(fmt.Errorf("choose -c or -t"))
	}
	if mode == "t" && archive == "-" {
		for _, e := range ev {
			if e.kind == "T" && e.value == "-" {
				return fail(fmt.Errorf("cannot read both archive and file list from stdin"))
			}
		}
	}
	if mode == "c" && toStdout {
		return fail(fmt.Errorf("-O is only supported when listing"))
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
			br := bufio.NewReader(r)
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
		return fail(fmt.Errorf("refusing to create an empty archive"))
	}
	if mode == "c" {
		var w io.Writer = ctx.Stdout
		var f *os.File
		var info os.FileInfo
		if stdout, ok := w.(*os.File); ok {
			info, _ = stdout.Stat()
		}
		if archive != "-" {
			f, err = os.Create(tarPath(ctx.Dir, archive))
			if err != nil {
				return fail(err)
			}
			w = f
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
		err = tarx.Create(context.Background(), w, sources, tarx.CreateOptions{Dereference: deref, NumericOwner: numeric, Owner: owner, Group: group, Exclude: excludes, ExcludeFile: info, Verbose: vf, OnError: func(name string, e error) { fmt.Fprintf(ctx.Stderr, "tar: %s: %v\n", name, e); partial = true }})
		if f != nil {
			if e := f.Close(); err == nil {
				err = e
			}
		}
		if err != nil {
			return fail(err)
		}
	} else {
		var r io.Reader = ctx.Stdin
		var f *os.File
		if archive != "-" {
			f, err = os.Open(tarPath(ctx.Dir, archive))
			if err != nil {
				return fail(err)
			}
			r = f
		}
		if r == nil {
			return fail(fmt.Errorf("stdin unavailable"))
		}
		seen := make([]bool, len(selected))
		err = tarx.Read(r, func(h *tar.Header, _ io.Reader) error {
			for i, name := range selected {
				if tarSelected(h.Name, []string{name}) {
					seen[i] = true
				}
			}
			if tarx.Excluded(h.Name, excludes) || !tarSelected(h.Name, selected) {
				return nil
			}
			return tarPrintHeader(ctx.Stdout, h, verbose, numeric, owner, group)
		})
		if f != nil {
			if e := f.Close(); err == nil {
				err = e
			}
		}
		if err != nil {
			return fail(err)
		}
		for i, ok := range seen {
			if !ok {
				fmt.Fprintf(ctx.Stderr, "tar: %s: not found in archive\n", selected[i])
				partial = true
			}
		}
	}
	if partial {
		return 2
	}
	return 0
}
