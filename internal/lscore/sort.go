package lscore

import (
	"path/filepath"
	"sort"
	"strings"
)

type SortField int

const (
	SortName SortField = iota
	SortSize
	SortModTime
	SortExtension
	SortWidth
	SortNone
)

type SortOpts struct {
	Field     SortField
	Reverse   bool
	DirsFirst bool
}

func Sort(files []*FileInfo, opts SortOpts) {
	if opts.Field == SortNone && !opts.DirsFirst {
		return
	}

	sort.SliceStable(files, func(i, j int) bool {
		a, b := files[i], files[j]

		if opts.DirsFirst && groupsAsDir(a) != groupsAsDir(b) {
			return groupsAsDir(a)
		}

		// less is the complete, tie-broken (by name) ordering decision
		// for the field in its GNU default direction (e.g. -t is
		// newest-first, -S is largest-first, by design of
		// compareField). -r then reverses that whole decision
		// uniformly, tie-break included, rather than only the primary
		// field comparison — matching GNU ls's actual "-r reverses the
		// final order" semantics.
		less := lessByField(a, b, opts.Field)
		if opts.Reverse {
			return !less
		}
		return less
	})

	for _, fi := range files {
		if len(fi.Children) > 0 {
			Sort(fi.Children, opts)
		}
	}
}

// groupsAsDir reports whether fi should be grouped with directories under
// --group-directories-first: real directories, and symlinks that resolve
// to a directory (GNU groups those together too).
func groupsAsDir(fi *FileInfo) bool {
	return fi.IsDir || (fi.IsSymlink && fi.LinksToDir && !fi.LinkBroken)
}

func lessByField(a, b *FileInfo, field SortField) bool {
	cmp := compareField(a, b, field)
	if cmp != 0 {
		return cmp < 0
	}
	return strings.ToLower(a.Name) < strings.ToLower(b.Name)
}

// compareField compares two entries by field, in that field's GNU
// "default" direction: name/extension/width sort ascending, while size
// and modification time sort with the largest/newest first (matching
// plain -S and -t without -r).
func compareField(a, b *FileInfo, field SortField) int {
	switch field {
	case SortSize:
		return cmpInt(b.Size, a.Size)
	case SortModTime:
		if a.ModTime.Before(b.ModTime) {
			return 1
		}
		if a.ModTime.After(b.ModTime) {
			return -1
		}
		return 0
	case SortExtension:
		ea := filepath.Ext(a.Name)
		eb := filepath.Ext(b.Name)
		return strings.Compare(strings.ToLower(ea), strings.ToLower(eb))
	case SortWidth:
		return cmpInt(int64(len([]rune(a.Name))), int64(len([]rune(b.Name))))
	case SortNone:
		return 0
	default:
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	}
}

func cmpInt(a, b int64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
