package imgview

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CollectOptions bounds directory expansion. Zero fields use defaults.
type CollectOptions struct {
	Recursive         bool
	MaxFiles          int // images collected from directory operands
	MaxDepth          int // directory levels scanned below an operand
	MaxEntriesScanned int // directory entries read while walking
}

// Collection is the ordered result of expanding imgview operands. Warning is a
// single status-line notice, set only when a bound stopped the walk.
type Collection struct {
	Paths   []string
	Warning string
}

// ErrNoImages reports that the operands expanded to no supported image files.
var ErrNoImages = errors.New("no images found")

func defaultCollectOptions() CollectOptions {
	return CollectOptions{MaxFiles: 10000, MaxDepth: 32, MaxEntriesScanned: 200000}
}

func CollectPaths(operands []string, opts CollectOptions) (Collection, error) {
	return CollectPathsContext(context.Background(), operands, opts)
}

// CollectPathsContext expands directory operands in place and keeps explicit
// file operands, including missing or unusually named ones, in their original
// position. Directory contents are filtered by extension, hidden names are
// skipped and each directory's images are naturally sorted by relative path so
// recursive results stay grouped per directory.
func CollectPathsContext(ctx context.Context, operands []string, opts CollectOptions) (Collection, error) {
	c := &collector{ctx: ctx, opts: opts.normalized()}
	for _, operand := range operands {
		if err := ctx.Err(); err != nil {
			return Collection{}, err
		}
		if info, err := os.Stat(operand); err != nil || !info.IsDir() {
			c.paths = append(c.paths, operand)
			continue
		}
		if c.stopped {
			continue
		}
		paths, err := c.dir(operand)
		if err != nil {
			return Collection{}, err
		}
		c.paths = append(c.paths, paths...)
	}
	if err := ctx.Err(); err != nil {
		return Collection{}, err
	}
	col := c.result()
	if len(col.Paths) == 0 {
		return col, ErrNoImages
	}
	return col, nil
}

type collector struct {
	ctx     context.Context
	opts    CollectOptions
	paths   []string
	scanned int  // directory entries read, across operands
	files   int  // images collected from directories, across operands
	stopped bool // a global bound was hit: no further directory is scanned
	limited bool // some bound was hit, so the warning applies
}

func (o CollectOptions) normalized() CollectOptions {
	d := defaultCollectOptions()
	if o.MaxFiles <= 0 {
		o.MaxFiles = d.MaxFiles
	}
	if o.MaxDepth <= 0 {
		o.MaxDepth = d.MaxDepth
	}
	if o.MaxEntriesScanned <= 0 {
		o.MaxEntriesScanned = d.MaxEntriesScanned
	}
	return o
}

func (c *collector) result() Collection {
	col := Collection{Paths: c.paths}
	if c.limited {
		col.Warning = fmt.Sprintf("showing first %d images; limit reached", len(col.Paths))
	}
	return col
}

// dir returns the naturally sorted images of one directory operand.
func (c *collector) dir(root string) ([]string, error) {
	var paths []string
	if err := c.walk(root, 0, &paths); err != nil {
		return nil, err
	}
	sort.SliceStable(paths, func(i, j int) bool {
		return naturalLess(relativeSlash(root, paths[i]), relativeSlash(root, paths[j]))
	})
	return paths, nil
}

// walk streams one directory in bounded batches so a huge directory never
// loads all of its entries at once.
func (c *collector) walk(dir string, depth int, paths *[]string) error {
	f, err := os.Open(dir)
	if err != nil {
		return nil // unreadable directories are skipped quietly
	}
	defer f.Close()
	for {
		if c.stopped {
			return nil
		}
		batch, readErr := f.ReadDir(min(256, max(1, c.opts.MaxEntriesScanned-c.scanned)))
		for _, entry := range batch {
			if c.stopped {
				return nil
			}
			if err := c.ctx.Err(); err != nil {
				return err
			}
			c.scanned++
			if err := c.visit(dir, depth, entry, paths); err != nil {
				return err
			}
			if c.scanned >= c.opts.MaxEntriesScanned {
				c.stopped, c.limited = true, true
				return nil
			}
		}
		if readErr != nil {
			return nil // io.EOF or a read error ends this directory
		}
	}
}

func (c *collector) visit(dir string, depth int, entry os.DirEntry, paths *[]string) error {
	name := entry.Name()
	if strings.HasPrefix(name, ".") {
		return nil
	}
	path := filepath.Join(dir, name)
	if entry.IsDir() {
		if !c.opts.Recursive {
			return nil
		}
		if depth+1 >= c.opts.MaxDepth {
			c.limited = true
			return nil
		}
		return c.walk(path, depth+1, paths)
	}
	if !isImageName(name) {
		return nil
	}
	// Symlinked directories never reach this point (ReadDir does not follow
	// them); a symlinked file is accepted when it resolves to a regular file.
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	*paths = append(*paths, path)
	c.files++
	if c.files >= c.opts.MaxFiles {
		c.stopped, c.limited = true, true
	}
	return nil
}

func relativeSlash(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

// naturalLess orders paths case-insensitively, comparing digit runs by value so
// img2 sorts before img10.
func naturalLess(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	for i, j := 0, 0; ; {
		if i == len(a) || j == len(b) {
			return len(a)-i < len(b)-j
		}
		if isDigit(a[i]) && isDigit(b[j]) {
			ai, bj := i, j
			for i < len(a) && isDigit(a[i]) {
				i++
			}
			for j < len(b) && isDigit(b[j]) {
				j++
			}
			x, y := strings.TrimLeft(a[ai:i], "0"), strings.TrimLeft(b[bj:j], "0")
			if len(x) != len(y) {
				return len(x) < len(y)
			}
			if x != y {
				return x < y
			}
			if i-ai != j-bj {
				return i-ai < j-bj
			}
			continue
		}
		if a[i] != b[j] {
			return a[i] < b[j]
		}
		i++
		j++
	}
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
