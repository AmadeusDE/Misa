package builtins

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// sedInPlace holds -i settings. Each file is written to a temporary file in
// its own directory, which replaces the original only once the whole file
// has been written; on any error the temporary file is removed and the
// original is left alone.
type sedInPlace struct {
	suffix string
	follow bool
}

func (r *sedRun) editAll() {
	names := r.in.names
	for _, name := range names {
		if r.quitting || r.fatal != nil {
			return
		}
		r.editFile(name)
	}
}

func (r *sedRun) editFile(name string) {
	path, info, ok := r.editTarget(name)
	if !ok {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "sed")
	if err != nil {
		r.editError("couldn't open temporary file %s: %s", filepath.Join(filepath.Dir(path), "sedXXXXXX"), grepErrText(err))
		return
	}
	tmp.Chmod(info.Mode().Perm())
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		tmp.Chown(int(st.Uid), int(st.Gid))
	}

	r.out = &sedOutput{w: bufio.NewWriter(tmp), delim: r.delim}
	r.in = &sedInput{ctx: r.ctx, names: []string{name}, delim: r.delim, separate: true}
	r.lineNo = 0
	r.in.fill(true)
	r.process()
	if err := r.out.w.Flush(); err != nil && r.out.err == nil {
		r.out.err = err
	}
	writeErr := r.out.err
	r.out = r.stdout
	if closeErr := tmp.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr == nil && r.in.failed {
		writeErr = fmt.Errorf("read error")
	}
	if writeErr != nil || r.fatal != nil {
		os.Remove(tmp.Name())
		if writeErr != nil {
			r.editError("couldn't write %s: %s", name, grepErrText(writeErr))
		}
		return
	}
	if err := r.replace(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		r.editError("%v", err)
	}
}

// editTarget resolves the file to rewrite. Without --follow-symlinks a
// symlink is replaced by a regular file, as GNU sed does.
func (r *sedRun) editTarget(name string) (string, os.FileInfo, bool) {
	if name == "-" {
		r.editError("couldn't edit -: not a regular file")
		return "", nil, false
	}
	path := resolvePath(r.ctx, name)
	if r.inPlace.follow {
		if real, err := filepath.EvalSymlinks(path); err == nil {
			path = real
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(r.ctx.Stderr, "sed: can't read %s: %s\n", name, grepErrText(err))
		r.readFailed = true
		return "", nil, false
	}
	if !info.Mode().IsRegular() {
		r.editError("couldn't edit %s: not a regular file", name)
		return "", nil, false
	}
	return path, info, true
}

func (r *sedRun) replace(tmp, path string) error {
	if r.inPlace.suffix != "" {
		backup := r.backupName(path)
		if err := os.Rename(path, backup); err != nil {
			return fmt.Errorf("cannot rename %s: %s", path, grepErrText(err))
		}
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("cannot rename %s: %s", tmp, grepErrText(err))
	}
	return nil
}

// backupName appends the suffix, or, when it contains '*', uses it as a
// template where each '*' is the file's base name. A template without a
// '/' names a file next to the original.
func (r *sedRun) backupName(path string) string {
	suffix := r.inPlace.suffix
	if !strings.Contains(suffix, "*") {
		return path + suffix
	}
	name := strings.ReplaceAll(suffix, "*", filepath.Base(path))
	if strings.Contains(name, "/") {
		return resolvePath(r.ctx, name)
	}
	return filepath.Join(filepath.Dir(path), name)
}

func (r *sedRun) editError(format string, args ...any) {
	fmt.Fprintf(r.ctx.Stderr, "sed: "+format+"\n", args...)
	r.editFailed = true
}
