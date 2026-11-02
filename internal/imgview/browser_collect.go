package imgview

import (
	"path/filepath"
	"strings"
)

// browserPathBytes leaves room for the environment and other launch arguments.
const browserPathBytes = 64 << 10

// BrowserCollectionWarning is retained across the standalone launch boundary.
const BrowserCollectionWarning = "image collection truncated; limit reached"

// CollectBrowserPaths filters paths in browser display order without scanning
// directories. A bounded window around the selected image retains lexical
// order. Hidden siblings are skipped; a hidden selection is retained. A selected
// path that is not a candidate returns ErrNoImages and index -1.
func CollectBrowserPaths(orderedPaths []string, selected string, opts CollectOptions, limits Limits) (Collection, int, error) {
	ok, err := CheckImageFile(selected, limits)
	if err != nil {
		return Collection{}, -1, err
	}
	if !ok {
		return Collection{}, -1, ErrNoImages
	}
	opts = opts.normalized()
	selectedAt := -1
	for i, path := range orderedPaths {
		if path == selected {
			selectedAt = i
			break
		}
	}
	// Browser entries are already in lexical display order. Insert a missing
	// selection at its name's position without copying or sorting the input.
	absent := selectedAt < 0
	total := len(orderedPaths)
	if absent {
		selectedAt = len(orderedPaths)
		name := filepath.Base(selected)
		for i, path := range orderedPaths {
			if filepath.Base(path) > name {
				selectedAt = i
				break
			}
		}
		total++
	}
	pathAt := func(i int) string {
		if absent && i > selectedAt {
			i--
		}
		return orderedPaths[i]
	}
	start, end := 0, total
	if end > opts.MaxEntriesScanned {
		start = max(0, min(selectedAt-opts.MaxEntriesScanned/2, end-opts.MaxEntriesScanned))
		end = start + opts.MaxEntriesScanned
	}
	limited := start != 0 || end != total
	left, right := selectedAt-1, selectedAt+1
	var before, after []string
	// The launch also passes selected as the value of --start.
	count, size := 1, 2*(len(selected)+1)
	// Alternating sides favors the images nearest the selection. Stop at the
	// first candidate that cannot fit, keeping a contiguous candidate window.
	for left >= start || right < end {
		for side := 0; side < 2; side++ {
			var path string
			if side == 0 && left >= start {
				path = pathAt(left)
				left--
			} else if side == 1 && right < end {
				path = pathAt(right)
				right++
			} else {
				continue
			}
			if path == selected || strings.HasPrefix(filepath.Base(path), ".") || !IsImageFile(path, limits) {
				continue
			}
			if count >= opts.MaxFiles || size+len(path)+1 > browserPathBytes {
				limited = true
				goto done
			}
			count++
			size += len(path) + 1
			if side == 0 {
				before = append(before, path)
			} else {
				after = append(after, path)
			}
		}
	}
done:
	paths := make([]string, 0, count)
	for i := len(before) - 1; i >= 0; i-- {
		paths = append(paths, before[i])
	}
	index := len(paths)
	paths = append(paths, selected)
	paths = append(paths, after...)
	col := Collection{Paths: paths}
	if limited {
		col.Warning = BrowserCollectionWarning
	}
	return col, index, nil
}
