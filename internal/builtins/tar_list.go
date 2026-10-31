// Tar listing behavior informed by Toybox tar.c (0BSD).
package builtins

import (
	"archive/tar"
	"fmt"
	"io"
	"strconv"
)

func tarPrintHeader(w io.Writer, h *tar.Header, verbose, numeric bool, owner, group *int) error {
	if owner != nil {
		h.Uid = *owner
		h.Uname = ""
	}
	if group != nil {
		h.Gid = *group
		h.Gname = ""
	}
	suffix := ""
	if h.Typeflag == tar.TypeSymlink {
		suffix = " -> " + h.Linkname
	} else if h.Typeflag == tar.TypeLink {
		suffix = " link to " + h.Linkname
	}
	if verbose {
		u, g := strconv.Itoa(h.Uid), strconv.Itoa(h.Gid)
		if !numeric {
			if h.Uname != "" {
				u = h.Uname
			}
			if h.Gname != "" {
				g = h.Gname
			}
		}
		_, err := fmt.Fprintf(w, "%s %s/%s %9d %s %s%s\n", h.FileInfo().Mode(), u, g, h.Size, h.ModTime.Format("2006-01-02 15:04"), h.Name, suffix)
		return err
	}
	_, err := fmt.Fprintln(w, h.Name+suffix)
	return err
}
