// Extraction behavior is informed by Toybox tar.c (0BSD); see NOTICE.
package tarx

import (
	"archive/tar"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

func (x *extractor) entry(ctx context.Context, tr *tar.Reader, h *tar.Header, name string) (bool, error) {
	p, base, err := x.parent(name, true)
	if err != nil {
		return false, err
	}
	defer p.Close()
	info, err := p.Lstat(base)
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if h.Typeflag == tar.TypeDir {
		if exists && !info.IsDir() {
			return false, fmt.Errorf("not a directory")
		}
		if !exists {
			if err := p.Mkdir(base, 0700); err != nil {
				return false, err
			}
		}
		info, err = p.Lstat(base)
		if err != nil || !info.IsDir() {
			return false, fmt.Errorf("directory changed: %s", base)
		}
		d, err := p.OpenRoot(base)
		if err != nil {
			return false, err
		}
		opened, err := d.Stat(".")
		if err != nil || !os.SameFile(info, opened) {
			d.Close()
			return false, fmt.Errorf("directory changed: %s", base)
		}
		d.Close()
		x.dirs = append(x.dirs, dirMetadata{name, opened, *h, h.Name, strings.Count(name, "/")})
		return false, nil
	}
	if base == "." {
		return false, fmt.Errorf("invalid destination")
	}
	if exists && x.opt.KeepOld {
		return false, fmt.Errorf("file exists")
	}
	switch h.Typeflag {
	case tar.TypeReg, tar.TypeRegA:
		return x.regular(ctx, tr, p, base, h)
	case tar.TypeSymlink:
		if exists {
			if err := p.Remove(base); err != nil {
				return false, err
			}
		}
		if err := p.Symlink(h.Linkname, base); err != nil {
			return false, err
		}
		if err := x.symlinkOwner(p, base, h); err != nil {
			return false, err
		}
		if !x.opt.Touch && !h.ModTime.IsZero() {
			return false, symlinkTimes(p, base, h)
		}
		return false, nil
	case tar.TypeLink:
		target, ok, err := x.name(h.Linkname, false)
		if err != nil {
			return false, err
		}
		if !ok || target == "." {
			return false, fmt.Errorf("invalid hardlink target")
		}
		src, sbase, err := x.parent(target, false)
		if err != nil {
			return false, err
		}
		defer src.Close()
		st, err := src.Lstat(sbase)
		if err != nil {
			return false, err
		}
		if !st.Mode().IsRegular() {
			return false, fmt.Errorf("hardlink target is not regular")
		}
		if exists && os.SameFile(info, st) {
			return false, nil
		}
		if exists {
			if err := p.Remove(base); err != nil {
				return false, err
			}
		}
		if err := linkRoots(src, sbase, p, base); err != nil {
			return false, err
		}
		linked, err := p.Lstat(base)
		if err != nil || !linked.Mode().IsRegular() || !os.SameFile(st, linked) {
			p.Remove(base)
			return false, fmt.Errorf("hardlink target changed")
		}
		return false, nil
	case tar.TypeFifo, tar.TypeChar, tar.TypeBlock:
		if os.Geteuid() != 0 {
			return false, fmt.Errorf("special file requires root")
		}
		if exists {
			if err := p.Remove(base); err != nil {
				return false, err
			}
		}
		if err := makeSpecial(p, base, h); err != nil {
			return false, err
		}
		return false, x.specialMetadata(p, base, h)
	default:
		return false, fmt.Errorf("unsupported entry type %q", h.Typeflag)
	}
}
func (x *extractor) regular(ctx context.Context, tr *tar.Reader, p *os.Root, base string, h *tar.Header) (bool, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return false, err
	}
	tmp := ".tarx-" + hex.EncodeToString(nonce[:])
	f, err := p.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return false, err
	}
	defer p.Remove(tmp)
	buf := make([]byte, 32*1024)
	var diskErr error
	for {
		if err := ctx.Err(); err != nil {
			f.Close()
			return true, err
		}
		n, rerr := tr.Read(buf)
		if n > 0 && diskErr == nil {
			var written int
			written, diskErr = f.Write(buf[:n])
			if written != n && diskErr == nil {
				diskErr = io.ErrShortWrite
			}
		}
		if rerr != nil {
			if rerr != io.EOF {
				f.Close()
				return true, rerr
			}
			break
		}
	}
	if diskErr == nil {
		diskErr = x.metadata(f, h)
	}
	if err := f.Close(); diskErr == nil {
		diskErr = err
	}
	if diskErr != nil {
		return false, diskErr
	}
	if err := ctx.Err(); err != nil {
		return true, err
	}
	if x.opt.KeepOld { // Link is an atomic no-replace installation.
		if err := p.Link(tmp, base); err != nil {
			return false, err
		}
		return false, nil
	}
	return false, p.Rename(tmp, base)
}
