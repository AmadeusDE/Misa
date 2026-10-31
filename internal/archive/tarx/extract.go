// Extraction behavior is informed by Toybox tar.c (0BSD); see NOTICE.
package tarx

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type ExtractOptions struct {
	StripComponents                              int
	KeepOld, SamePermissions, NoSameOwner, Touch bool
	Umask                                        os.FileMode
	Owner, Group                                 *int
	NumericOwner                                 bool
	Select                                       func(*tar.Header) bool
	Verbose                                      func(*tar.Header) error
	OnError                                      func(string, error)
	Warn                                         func(string)
	ToStdout                                     io.Writer
}

type extractor struct {
	root   *os.Root
	opt    ExtractOptions
	warned bool
	dirs   []dirMetadata
}
type dirMetadata struct {
	path  string
	info  os.FileInfo
	h     tar.Header
	name  string
	depth int
}

// Extract processes entries in archive order. Entry errors are reported and skipped;
// malformed input, cancellation and output callback failures are returned.
func Extract(ctx context.Context, r io.Reader, dir string, opt ExtractOptions) error {
	if opt.StripComponents < 0 {
		return fmt.Errorf("negative strip components")
	}
	x := extractor{opt: opt}
	if opt.ToStdout == nil {
		root, err := os.OpenRoot(dir)
		if err != nil {
			return err
		}
		x.root = root
		defer root.Close()
	}
	defer x.finishDirs(ctx)
	tr := tar.NewReader(&contextReader{ctx: ctx, r: r})
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if opt.Select != nil && !opt.Select(h) {
			continue
		}
		name, ok, err := x.name(h.Name, true)
		if err != nil {
			x.report(h.Name, err)
			continue
		}
		if !ok {
			continue
		}
		if opt.Verbose != nil {
			if err := opt.Verbose(h); err != nil {
				return err
			}
		}
		if opt.ToStdout != nil {
			if h.Typeflag == tar.TypeReg || h.Typeflag == tar.TypeRegA {
				if _, err := io.Copy(opt.ToStdout, &contextReader{ctx: ctx, r: tr}); err != nil {
					return err
				}
			}
			continue
		}
		fatal, err := x.entry(ctx, tr, h, name)
		if err != nil {
			if fatal {
				return err
			}
			x.report(h.Name, err)
		}
	}
}
func (x *extractor) report(name string, err error) {
	if x.opt.OnError != nil {
		x.opt.OnError(name, err)
	}
}
func (x *extractor) name(n string, warn bool) (string, bool, error) {
	if filepath.Separator == '\\' && strings.Contains(n, "\\") {
		return "", false, fmt.Errorf("unsafe path %q", n)
	}
	if strings.HasPrefix(n, "/") {
		if warn && !x.warned {
			x.warned = true
			if x.opt.Warn != nil {
				x.opt.Warn("removing leading '/' from member names")
			}
		}
		n = strings.TrimLeft(n, "/")
	}
	var parts []string
	for _, p := range strings.Split(n, "/") {
		if p == "" || p == "." {
			continue
		}
		if p == ".." {
			return "", false, fmt.Errorf("unsafe path %q", n)
		}
		parts = append(parts, p)
	}
	if len(parts) == 0 && x.opt.StripComponents == 0 {
		return ".", true, nil
	}
	if len(parts) <= x.opt.StripComponents {
		return "", false, nil
	}
	return path.Join(parts[x.opt.StripComponents:]...), true, nil
}

// parent opens each component separately, refusing symlinks even within the root.
func (x *extractor) parent(name string, create bool) (*os.Root, string, error) {
	parts := strings.Split(name, "/")
	cur, err := x.root.OpenRoot(".")
	if err != nil {
		return nil, "", err
	}
	for _, p := range parts[:len(parts)-1] {
		info, e := cur.Lstat(p)
		if os.IsNotExist(e) && create {
			e = cur.Mkdir(p, 0755)
			if e == nil {
				info, e = cur.Lstat(p)
			}
		}
		if e != nil {
			cur.Close()
			return nil, "", e
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			cur.Close()
			return nil, "", fmt.Errorf("unsafe parent %q", p)
		}
		next, e := cur.OpenRoot(p)
		if e != nil {
			cur.Close()
			return nil, "", e
		} // Reject a directory swapped for a symlink between Lstat and OpenRoot.
		opened, e := next.Stat(".")
		if e != nil || !os.SameFile(info, opened) {
			next.Close()
			cur.Close()
			return nil, "", fmt.Errorf("parent changed: %s", p)
		}
		cur.Close()
		cur = next
	}
	return cur, parts[len(parts)-1], nil
}
