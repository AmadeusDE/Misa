// Package tarx provides streaming tar creation and reading.
// Its baseline behavior was informed by Toybox tar.c and tar.test
// (Toybox contributors, 0BSD); see NOTICE.
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
	"time"

	"github.com/AmadeusDE/Misa/internal/walk"
)

type Source struct{ Dir, Name string }
type CreateOptions struct {
	Dereference, NumericOwner bool
	Owner, Group              *int
	Exclude                   []string
	ExcludeFile               os.FileInfo
	Verbose                   func(*tar.Header) error
	OnError                   func(name string, err error)
}

// Excluded matches unanchored shell patterns, including across slashes, and
// treats a matching directory prefix as excluding all its descendants.
func Excluded(name string, patterns []string) bool {
	name = strings.TrimPrefix(name, "./")
	for _, pattern := range patterns {
		pattern = strings.TrimPrefix(pattern, "./")
		for s := name; ; {
			for end := len(s); ; {
				ok, _ := path.Match(strings.ReplaceAll(pattern, "/", "\x00"), strings.ReplaceAll(s[:end], "/", "\x00"))
				if ok {
					return true
				}
				i := strings.LastIndexByte(s[:end], '/')
				if i < 0 {
					break
				}
				end = i
			}
			i := strings.IndexByte(s, '/')
			if i < 0 {
				break
			}
			s = s[i+1:]
		}
	}
	return false
}

// Create writes sources in order, recursively visiting each in sorted order.
// Source errors are reported to OnError and skipped; output errors are returned.
func Create(ctx context.Context, w io.Writer, sources []Source, opt CreateOptions) error {
	tw := tar.NewWriter(w)
	c := creator{ctx: ctx, tw: tw, opt: opt, links: make(map[fileID]string), names: make(map[string]fileID)}
	for _, src := range sources {
		c.base = src.Dir
		err := walk.Walk(ctx, src.Name, walk.Options{Base: src.Dir, FollowLinks: opt.Dereference}, c.entry)
		if err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return tw.Close()
}

type creator struct {
	ctx   context.Context
	tw    *tar.Writer
	opt   CreateOptions
	base  string
	links map[fileID]string
	names map[string]fileID
}

func (c *creator) report(name string, err error) {
	if c.opt.OnError != nil {
		c.opt.OnError(name, err)
	}
}
func safeName(n string) string {
	n = filepath.ToSlash(n)
	n = strings.TrimLeft(n, "/")
	for {
		switch {
		case strings.HasPrefix(n, "../"):
			n = n[3:]
		case strings.HasPrefix(n, "./"):
			n = n[2:]
		default:
			goto done
		}
	}
done:
	n = path.Clean("/" + n)
	n = strings.TrimPrefix(n, "/")
	if n == "" {
		return "."
	}
	return n
}
func (c *creator) entry(e walk.Entry) error {
	if e.Err != nil {
		c.report(e.Path, e.Err)
		return nil
	}
	name := safeName(e.Path)
	if Excluded(name, c.opt.Exclude) {
		if e.Info.IsDir() {
			return walk.SkipDir
		}
		return nil
	}
	if c.opt.ExcludeFile != nil && os.SameFile(e.Info, c.opt.ExcludeFile) {
		return nil
	}
	mode := e.Info.Mode()
	if !mode.IsRegular() && !mode.IsDir() && mode&os.ModeSymlink == 0 {
		c.report(e.Path, fmt.Errorf("unsupported file type %s", mode))
		return nil
	}
	fsname := e.Path
	if !filepath.IsAbs(fsname) {
		fsname = filepath.Join(c.base, fsname)
	}
	var link string
	var err error
	if mode&os.ModeSymlink != 0 {
		link, err = os.Readlink(fsname)
		if err != nil {
			c.report(e.Path, err)
			return nil
		}
	}
	h, err := tar.FileInfoHeader(e.Info, link)
	if err != nil {
		c.report(e.Path, err)
		return nil
	}
	h.Name = name
	if mode.IsDir() {
		h.Name += "/"
	}
	h.AccessTime = time.Time{}
	h.ChangeTime = time.Time{}
	h.ModTime = h.ModTime.Truncate(time.Second)
	if c.opt.NumericOwner {
		h.Uname = ""
		h.Gname = ""
	}
	if c.opt.Owner != nil {
		h.Uid = *c.opt.Owner
		h.Uname = ""
	}
	if c.opt.Group != nil {
		h.Gid = *c.opt.Group
		h.Gname = ""
	}
	var f *os.File
	var id fileID
	var hasID bool
	if mode.IsRegular() {
		id, hasID = idOf(e.Info)
		if old, exists := c.links[id]; hasID && exists && old != name {
			h.Typeflag = tar.TypeLink
			h.Linkname = old
			h.Size = 0
		}
		if h.Typeflag != tar.TypeLink {
			f, err = os.Open(fsname)
			if err != nil {
				c.report(e.Path, err)
				return nil
			}
			defer f.Close()
		}
	}
	// A later entry with the same name replaces the earlier extraction target.
	// Never refer a subsequent hardlink to that stale name.
	if oldID, exists := c.names[name]; exists {
		delete(c.links, oldID)
		delete(c.names, name)
	}
	if f != nil && hasID {
		c.links[id] = name
		c.names[name] = id
	}
	if err = c.tw.WriteHeader(h); err != nil {
		return err
	}
	if c.opt.Verbose != nil {
		if err = c.opt.Verbose(h); err != nil {
			return err
		}
	}
	if f != nil {
		_, err = io.Copy(c.tw, &contextReader{ctx: c.ctx, r: f})
		if err != nil {
			return err
		}
	}
	return nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

// Read visits each decoded USTAR, PAX, or GNU entry. The reader is valid only
// during visit; unread contents are skipped before the next callback.
func Read(r io.Reader, visit func(*tar.Header, io.Reader) error) error {
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err = visit(h, tr); err != nil {
			return err
		}
	}
}
