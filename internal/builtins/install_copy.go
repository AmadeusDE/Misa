// Copy and directory creation for the install builtin (see install_cmd.go for
// the option set).
//
// Behaviour follows toybox install (0BSD, toys/posix/cp.c at
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb) and uutils coreutils install (MIT,
// src/uu/install at fee61ff5d4f87e0f912a52e639942b5f7ad58663).
package builtins

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// installMakeDirTree creates every missing component of path (an already
// resolved path). Newly created parents get 0755 and the last component opts.mode,
// so neither the umask nor -m leaks into the parents; an existing path is
// required to be a directory and then has the requested attributes applied.
// It reports whether anything was created.
func installMakeDirTree(path, displayName string, opts installOptions) (created bool, err error) {
	display := installDirDisplay(displayName)
	cleaned := filepath.Clean(path)
	namedDot := installLastComponent(path) == "." || installLastComponent(path) == ".."

	if info, statErr := os.Stat(cleaned); statErr == nil {
		if !info.IsDir() {
			return false, fmt.Errorf("cannot create directory '%s': File exists", display)
		}
		if !namedDot {
			if err := installApplyAttributes(cleaned, display, opts); err != nil {
				return false, err
			}
		}
		return false, nil
	} else if !os.IsNotExist(statErr) {
		return false, fmt.Errorf("cannot create directory '%s': %s", display, fmtOsErr(statErr))
	}

	prefix := ""
	if filepath.IsAbs(cleaned) {
		prefix = string(filepath.Separator)
	}
	parts := strings.Split(strings.TrimPrefix(cleaned, prefix), string(filepath.Separator))
	for i, part := range parts {
		prefix = filepath.Join(prefix, part)
		last := i == len(parts)-1
		if info, statErr := os.Stat(prefix); statErr == nil {
			if !info.IsDir() {
				return created, fmt.Errorf("cannot create directory '%s': Not a directory", display)
			}
			continue
		}
		if mkErr := os.Mkdir(prefix, 0755); mkErr != nil {
			return created, fmt.Errorf("cannot create directory '%s': %s", display, fmtOsErr(mkErr))
		}
		created = true
		mode := os.FileMode(0755)
		if last {
			mode = opts.mode
		}
		if err := installApplyAttributes(prefix, display, installOptions{mode: mode, uid: opts.uid, gid: opts.gid}); err != nil {
			return created, err
		}
	}
	return created, nil
}

// installApplyAttributes sets ownership first and the mode second: chown clears
// setuid/setgid, so the mode must win.
func installApplyAttributes(path, display string, opts installOptions) error {
	if opts.uid >= 0 || opts.gid >= 0 {
		if err := os.Chown(path, opts.uid, opts.gid); err != nil {
			return fmt.Errorf("cannot change ownership of '%s': %s", display, fmtOsErr(err))
		}
	}
	if err := os.Chmod(path, installFileMode(opts.mode)); err != nil {
		return fmt.Errorf("cannot change permissions of '%s': %s", display, fmtOsErr(err))
	}
	return nil
}

// installCopyFile installs one source file as dest. It reports whether a copy
// happened: -C skips one whose destination already matches.
func installCopyFile(sourceDisplay, source, destDisplay, dest string, opts installOptions) (copied bool, err error) {
	srcInfo, err := os.Stat(source)
	if err != nil {
		return false, fmt.Errorf("cannot stat '%s': %s", sourceDisplay, fmtOsErr(err))
	}
	if srcInfo.IsDir() {
		return false, fmt.Errorf("omitting directory '%s'", sourceDisplay)
	}

	if destInfo, err := os.Lstat(dest); err == nil {
		if destInfo.IsDir() {
			return false, fmt.Errorf("cannot overwrite directory '%s' with non-directory", destDisplay)
		}
		if os.SameFile(srcInfo, destInfo) {
			return false, fmt.Errorf("'%s' and '%s' are the same file", sourceDisplay, destDisplay)
		}
		if destInfo.Mode()&os.ModeSymlink == 0 && !destInfo.Mode().IsRegular() {
			copied, err := installWriteIntoSource(source, destDisplay, dest, srcInfo, opts)
			return copied, err
		}
	}

	if opts.compare && !installNeedsCopy(source, dest, srcInfo, opts) {
		return false, nil
	}
	if err := installReplaceAtomic(source, dest, destDisplay, srcInfo, opts); err != nil {
		return false, err
	}
	return true, nil
}

// installReplaceAtomic writes the copy to a temporary file in dest's directory
// and renames it over dest, so a reader never sees a half-written destination
// and a failure leaves dest (and no temporary file) alone.
func installReplaceAtomic(source, dest, destDisplay string, srcInfo os.FileInfo, opts installOptions) error {
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".install-*")
	if err != nil {
		return fmt.Errorf("cannot create regular file '%s': %s", destDisplay, fmtOsErr(err))
	}
	tmpName := tmp.Name()
	discard := func() {
		tmp.Close()
		os.Remove(tmpName)
	}

	in, err := os.Open(source)
	if err != nil {
		discard()
		return fmt.Errorf("cannot open '%s': %s", source, fmtOsErr(err))
	}
	_, copyErr := io.Copy(tmp, in)
	in.Close()
	if copyErr != nil {
		discard()
		return fmt.Errorf("cannot write '%s': %s", destDisplay, fmtOsErr(copyErr))
	}
	if err := installApplyAttributes(tmpName, destDisplay, opts); err != nil {
		discard()
		return err
	}
	if opts.preserve {
		if err := installPreserveTimes(tmpName, destDisplay, srcInfo); err != nil {
			discard()
			return err
		}
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("cannot write '%s': %s", destDisplay, fmtOsErr(err))
	}
	if err := os.Rename(tmpName, dest); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("cannot create regular file '%s': %s", destDisplay, fmtOsErr(err))
	}
	return nil
}

// installWriteIntoSource copies into an existing non-regular destination (a
// device or FIFO) instead of replacing it: renaming over a device node would
// destroy it. Its mode and ownership are not touched, since they belong to the
// node rather than to the installed file.
func installWriteIntoSource(source, destDisplay, dest string, srcInfo os.FileInfo, opts installOptions) (copied bool, err error) {
	if opts.compare && !installNeedsCopy(source, dest, srcInfo, opts) {
		return false, nil
	}
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return false, fmt.Errorf("cannot create regular file '%s': %s", destDisplay, fmtOsErr(err))
	}
	in, err := os.Open(source)
	if err != nil {
		out.Close()
		return false, fmt.Errorf("cannot open '%s': %s", source, fmtOsErr(err))
	}
	_, copyErr := io.Copy(out, in)
	in.Close()
	if closeErr := out.Close(); copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		return false, fmt.Errorf("cannot write '%s': %s", destDisplay, fmtOsErr(copyErr))
	}
	return true, nil
}

// installPreserveTimes copies the source's access and modification times.
func installPreserveTimes(path, display string, srcInfo os.FileInfo) error {
	times := []time.Time{srcInfo.ModTime(), srcInfo.ModTime()}
	if st, ok := srcInfo.Sys().(*syscall.Stat_t); ok {
		times[0] = time.Unix(atim(st).Sec, atim(st).Nsec)
		times[1] = time.Unix(mtim(st).Sec, mtim(st).Nsec)
	}
	if err := os.Chtimes(path, times[0], times[1]); err != nil {
		return fmt.Errorf("cannot preserve times of '%s': %s", display, fmtOsErr(err))
	}
	return nil
}

// installNeedsCopy is -C: false only when dest is a regular file that already
// has the requested mode, ownership and contents.
func installNeedsCopy(source, dest string, srcInfo os.FileInfo, opts installOptions) bool {
	destInfo, err := os.Stat(dest)
	if err != nil || !destInfo.Mode().IsRegular() {
		return true
	}
	const special = os.ModeSetuid | os.ModeSetgid | os.ModeSticky
	if destInfo.Mode().Perm() != opts.mode.Perm() || destInfo.Mode()&special != installFileMode(opts.mode)&special {
		return true
	}
	if destInfo.Size() != srcInfo.Size() {
		return true
	}
	if opts.preserve && !destInfo.ModTime().Equal(srcInfo.ModTime()) {
		return true
	}
	if st, ok := destInfo.Sys().(*syscall.Stat_t); ok {
		if opts.uid >= 0 && int(st.Uid) != opts.uid {
			return true
		}
		if opts.gid >= 0 && int(st.Gid) != opts.gid {
			return true
		}
	}
	same, err := installSameContents(source, dest, srcInfo.Size())
	if err != nil || !same {
		return true
	}
	return false
}

// installSameContents compares source and dest byte for byte.
func installSameContents(source, dest string, size int64) (bool, error) {
	a, err := os.Open(source)
	if err != nil {
		return false, err
	}
	defer a.Close()
	b, err := os.Open(dest)
	if err != nil {
		return false, err
	}
	defer b.Close()

	bufA := make([]byte, 32*1024)
	bufB := make([]byte, 32*1024)
	var done int64
	for done < size {
		nA, errA := io.ReadFull(a, bufA)
		nB, errB := io.ReadFull(b, bufB)
		if nA != nB || !bytes.Equal(bufA[:nA], bufB[:nB]) {
			return false, nil
		}
		if errA != nil || errB != nil {
			return errA == errB, nil
		}
		done += int64(nA)
	}
	return true, nil
}

// installBase is the name a source gets inside a destination directory.
func installBase(source string) string {
	return filepath.Base(filepath.Clean(source))
}

// installDisplayJoin names a file inside the destination the way the user did.
func installDisplayJoin(dest, name string) string {
	return strings.TrimSuffix(dest, "/") + "/" + name
}

// installDirDisplay drops trailing slashes and "/." components from a directory
// operand for messages: "dir3/." is reported as "dir3".
func installDirDisplay(path string) string {
	display := path
	for {
		switch {
		case strings.HasSuffix(display, "/.") && len(display) > 2:
			display = strings.TrimSuffix(display, "/.")
		case len(display) > 1 && strings.HasSuffix(display, "/"):
			display = strings.TrimSuffix(display, "/")
		default:
			return display
		}
	}
}

// installLastComponent returns the final path element, which is "." or ".."
// when the operand named an existing directory indirectly.
func installLastComponent(path string) string {
	trimmed := strings.TrimRight(path, "/")
	if trimmed == "" {
		return string(filepath.Separator)
	}
	if i := strings.LastIndex(trimmed, "/"); i >= 0 {
		return trimmed[i+1:]
	}
	return trimmed
}

// isDirPath reports whether path is an existing directory.
func isDirPath(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
