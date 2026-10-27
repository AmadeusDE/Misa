package lscore

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// ErrDerefFailed is returned by Collect when the root argument is a
// symlink that could not be dereferenced (broken target or a loop) while
// dereferencing was required (-H/-L on a command-line argument).
var ErrDerefFailed = errors.New("lscore: symlink could not be dereferenced")

type FileInfo struct {
	Name       string
	Path       string
	Size       int64
	Mode       os.FileMode
	ModTime    time.Time
	AccessTime time.Time
	ChangeTime time.Time
	IsDir      bool
	IsSymlink  bool
	LinkTarget string
	LinkBroken bool
	LinksToDir bool
	Owner      string
	Group      string
	UID        uint32
	GID        uint32
	Inode      uint64
	Dev        uint64
	Nlink      uint64
	Blocks     int64
	GitStatus  string
	Children   []*FileInfo
	Depth      int
	Err        error
}

type CollectOpts struct {
	All         bool
	AlmostAll   bool
	Recursive   bool
	TreeMode    bool
	MaxDepth    int
	FollowLinks bool
	DerefRoot   bool
	DirsOnly    bool
	DirItself   bool
	GitStatus   bool
}

// DirIdent identifies a directory by device+inode, for symlink-loop
// detection while recursing.
type DirIdent struct {
	Dev, Ino uint64
}

// Identify returns the (dev, inode) pair of path, dereferencing symlinks.
func Identify(path string) (DirIdent, bool) {
	st, err := os.Stat(path)
	if err != nil {
		return DirIdent{}, false
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return DirIdent{}, false
	}
	return DirIdent{Dev: uint64(sys.Dev), Ino: sys.Ino}, true
}

func Collect(root string, opts CollectOpts) ([]*FileInfo, error) {
	if opts.DirItself {
		fi := statFile(root, filepath.Base(root), opts.DerefRoot)
		return []*FileInfo{fi}, nil
	}

	lst, lerr := os.Lstat(root)
	if lerr != nil {
		return nil, lerr
	}

	st, serr := os.Stat(root)
	isDir := serr == nil && st.IsDir()
	isSymlink := lst.Mode()&os.ModeSymlink != 0

	if !isDir {
		if isSymlink && serr != nil && opts.DerefRoot {
			return nil, ErrDerefFailed
		}
		fi := statFile(root, filepath.Base(root), opts.DerefRoot)
		return []*FileInfo{fi}, nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var gitMap map[string]string
	if opts.GitStatus {
		gitMap, _ = CollectGitStatus(root)
	}

	var files []*FileInfo

	// os.ReadDir never yields "." or "..": GNU ls synthesizes them
	// itself for -a (but not -A/--almost-all).
	if opts.All {
		files = append(files,
			statFile(root, ".", opts.FollowLinks),
			statFile(filepath.Dir(root), "..", opts.FollowLinks),
		)
	}

	for _, e := range entries {
		name := e.Name()
		if !opts.All && !opts.AlmostAll && name[0] == '.' {
			continue
		}
		if opts.AlmostAll && (name == "." || name == "..") {
			continue
		}

		fi := statFile(filepath.Join(root, name), name, opts.FollowLinks)

		if opts.DirsOnly && !fi.IsDir {
			continue
		}

		if gitMap != nil {
			fi.GitStatus = gitMap[name]
		}

		if opts.TreeMode && fi.IsDir {
			fi.Children = collectTree(filepath.Join(root, name), opts, gitMap, 1)
		}

		files = append(files, fi)
	}

	return files, nil
}

func collectTree(dir string, opts CollectOpts, gitMap map[string]string, depth int) []*FileInfo {
	if opts.MaxDepth > 0 && depth >= opts.MaxDepth {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var files []*FileInfo
	for _, e := range entries {
		name := e.Name()
		if !opts.All && !opts.AlmostAll && name[0] == '.' {
			continue
		}

		fi := statFile(filepath.Join(dir, name), name, opts.FollowLinks)
		fi.Depth = depth

		if gitMap != nil {
			rel, _ := filepath.Rel(filepath.Dir(dir), filepath.Join(dir, name))
			fi.GitStatus = gitMap[rel]
		}

		if fi.IsDir {
			fi.Children = collectTree(filepath.Join(dir, name), opts, gitMap, depth+1)
		}

		files = append(files, fi)
	}

	return files
}

func statFile(path, name string, followLinks bool) *FileInfo {
	fi := &FileInfo{
		Name: name,
		Path: path,
	}

	linfo, err := os.Lstat(path)
	if err != nil {
		fi.Err = err
		return fi
	}

	info := linfo
	fi.IsSymlink = linfo.Mode()&os.ModeSymlink != 0

	if fi.IsSymlink {
		target, err := os.Readlink(path)
		if err == nil {
			fi.LinkTarget = target
		}
		resolved, rerr := os.Stat(path)
		if rerr != nil {
			fi.LinkBroken = true
		} else {
			fi.LinksToDir = resolved.IsDir()
			if followLinks {
				info = resolved
			}
		}
	}

	fi.Size = info.Size()
	fi.Mode = info.Mode()
	fi.ModTime = info.ModTime()
	fi.IsDir = info.IsDir()

	if sys, ok := info.Sys().(*syscall.Stat_t); ok {
		fi.UID = sys.Uid
		fi.GID = sys.Gid
		fi.Inode = sys.Ino
		fi.Dev = uint64(sys.Dev)
		fi.Nlink = uint64(sys.Nlink)
		fi.Blocks = sys.Blocks
		fi.AccessTime, fi.ChangeTime = statTimes(sys)

		if u, err := user.LookupId(strconv.Itoa(int(sys.Uid))); err == nil {
			fi.Owner = u.Username
		} else {
			fi.Owner = strconv.Itoa(int(sys.Uid))
		}
		if g, err := user.LookupGroupId(strconv.Itoa(int(sys.Gid))); err == nil {
			fi.Group = g.Name
		} else {
			fi.Group = strconv.Itoa(int(sys.Gid))
		}
	}

	return fi
}

func HumanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return strconv.FormatInt(b, 10)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return strconv.FormatFloat(float64(b)/float64(div), 'f', 1, 64) + string("KMGTPE"[exp])
}
