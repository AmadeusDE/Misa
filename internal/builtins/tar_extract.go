// Tar extraction dispatch informed by Toybox tar.c (0BSD).
package builtins

import (
	"archive/tar"
	"fmt"
	"io"
	"os"

	"github.com/AmadeusDE/Misa/internal/archive/tarx"
)

type tarReadOptions struct {
	archive, dir, mode                       string
	gzip, verbose, numeric                   bool
	owner, group                             *int
	selected, excludes                       []string
	strip                                    int
	keep, samePerm, noOwner, touch, toStdout bool
}

func tarReadArchive(ctx *Context, opt tarReadOptions, partial *bool) (err error) {
	var r io.Reader = ctx.Stdin
	if opt.archive != "-" {
		f, e := os.Open(tarPath(ctx.Dir, opt.archive))
		if e != nil {
			return e
		}
		defer func() {
			if e := f.Close(); err == nil {
				err = e
			}
		}()
		r = f
	}
	if r == nil {
		return fmt.Errorf("stdin unavailable")
	}
	r, cleanup, compressed, err := tarInput(ctx, r, opt.mode == "x", opt.gzip)
	if err != nil {
		return err
	}
	defer cleanup()
	seen := make([]bool, len(opt.selected))
	selectHeader := func(h *tar.Header) bool {
		for i, name := range opt.selected {
			if tarSelected(h.Name, []string{name}) {
				seen[i] = true
			}
		}
		return !tarx.Excluded(h.Name, opt.excludes) && tarSelected(h.Name, opt.selected)
	}
	if opt.mode == "t" {
		err = tarx.Read(r, func(h *tar.Header, _ io.Reader) error {
			if !selectHeader(h) {
				return nil
			}
			return tarPrintHeader(ctx.Stdout, h, opt.verbose, opt.numeric, opt.owner, opt.group)
		})
		// Drain gzip to validate its checksum even if tar ended before the trailer.
		if err == nil && compressed {
			_, err = CopyContext(ctx.CancellationContext(), io.Discard, r)
		}
	} else {
		var verbose func(*tar.Header) error
		if opt.verbose {
			w := ctx.Stdout
			if opt.toStdout {
				w = ctx.Stderr
			}
			verbose = func(h *tar.Header) error { _, e := fmt.Fprintln(w, h.Name); return e }
		}
		var out io.Writer
		if opt.toStdout {
			out = ctx.Stdout
		}
		err = tarx.Extract(ctx.CancellationContext(), r, opt.dir, tarx.ExtractOptions{StripComponents: opt.strip, KeepOld: opt.keep, SamePermissions: opt.samePerm, NoSameOwner: opt.noOwner, Touch: opt.touch, Umask: os.FileMode(currentUmask()), Owner: opt.owner, Group: opt.group, NumericOwner: opt.numeric, Select: selectHeader, Verbose: verbose, OnError: func(name string, e error) { fmt.Fprintf(ctx.Stderr, "tar: %s: %v\n", name, e); *partial = true }, Warn: func(s string) { fmt.Fprintln(ctx.Stderr, "tar:", s) }, ToStdout: out})
	}
	if err != nil {
		return err
	}
	for i, ok := range seen {
		if !ok {
			fmt.Fprintf(ctx.Stderr, "tar: %s: not found in archive\n", opt.selected[i])
			*partial = true
		}
	}
	return nil
}
