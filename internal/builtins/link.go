package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	lnSpec    = "s(symbolic)f(force)v(verbose)r(relative)t(target-directory):T(no-target-directory)(backup):?S(suffix):bL(dereference)P(no-dereference)i(interactive)n"
	rmdirSpec = "p(parents)v(verbose)(ignore-fail-on-non-empty)"
)

var (
	lnSpecParsed    *Spec
	rmdirSpecParsed *Spec
)

func init() {
	var err error
	lnSpecParsed, err = ParseSpec(lnSpec)
	if err != nil {
		panic("builtins: bad ln spec: " + err.Error())
	}
	rmdirSpecParsed, err = ParseSpec(rmdirSpec)
	if err != nil {
		panic("builtins: bad rmdir spec: " + err.Error())
	}
	Register(Command{Name: "ln", Run: runLn})
	Register(Command{Name: "rmdir", Run: runRmdir})
}

func runLn(ctx *Context) int {
	flags, err := lnSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "ln:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, lnSpecParsed) {
		return 0
	}
	ops := flags.Operands

	sym := flags.Bool("s")
	force := flags.Bool("f")
	verbose := flags.Bool("v")
	relative := flags.Bool("r")
	noTarget := flags.Bool("T")

	backup := flags.Bool("b")
	backupSuffix := ""
	hasSuffix := false
	if s, ok := flags.String("backup"); ok {
		backup = true
		_ = s
	}
	if s, ok := flags.String("S"); ok {
		backupSuffix = s
		hasSuffix = true
	}
	suffix := lnBackupSuffix(ctx, backupSuffix, hasSuffix)

	targetDir := ""
	if td, ok := flags.String("t"); ok {
		targetDir = resolvePath(ctx, td)
	}

	var sources []string
	var target string

	if targetDir != "" {
		sources = ops
		target = targetDir
	} else if len(ops) < 2 {
		if len(ops) == 0 {
			fmt.Fprintln(ctx.Stderr, "ln: missing operand")
			return 1
		}
		if noTarget {
			fmt.Fprintf(ctx.Stderr, "ln: missing destination file operand after '%s'\n", ops[0])
			return 1
		}
		target = resolvePath(ctx, ".")
		sources = ops
	} else {
		target = resolvePath(ctx, ops[len(ops)-1])
		sources = ops[:len(ops)-1]
	}

	info, statErr := os.Stat(target)
	targetIsDir := statErr == nil && info.IsDir()
	if noTarget {
		targetIsDir = false
	} else if targetDir != "" {
		targetIsDir = true
	}

	if len(sources) > 1 && !targetIsDir {
		fmt.Fprintln(ctx.Stderr, "ln: target is not a directory")
		return 1
	}

	exitCode := 0
	for _, src := range sources {
		if src == "" {
			fmt.Fprintln(ctx.Stderr, "ln: failed to create link: No such file or directory")
			exitCode = 1
			continue
		}
		srcPath := resolvePath(ctx, src)
		dst := target
		dstDisplay := ""
		if len(ops) >= 2 {
			dstDisplay = ops[len(ops)-1]
		} else {
			dstDisplay = "."
		}
		if targetIsDir {
			dst = filepath.Join(target, filepath.Base(srcPath))
			dstDisplay = filepath.Join(dstDisplay, filepath.Base(srcPath))
		}

		cleanSrc := filepath.Clean(srcPath)
		cleanDst := filepath.Clean(dst)
		isSame := false
		if sym {
			if !force && cleanSrc == cleanDst {
				if _, lerr := os.Lstat(dst); lerr == nil {
					isSame = true
				}
			}
		} else {
			if cleanSrc == cleanDst {
				isSame = true
			} else if !backup && sameFile(srcPath, dst) {
				isSame = true
			}
		}

		if isSame {
			fmt.Fprintf(ctx.Stderr, "ln: '%s' and '%s' are the same file\n", src, dstDisplay)
			exitCode = 1
			continue
		}

		if _, lerr := os.Lstat(dst); lerr == nil {
			if backup {
				backupPath := dst + suffix
				if sameFile(dst, backupPath) {
					// dst is already a hard link to backupPath: the backup
					// exists implicitly, so just drop the dst name.
					os.Remove(dst)
				} else {
					os.Rename(dst, backupPath)
				}
			} else if force {
				os.Remove(dst)
			}
		}

		var linkErr error
		if sym {
			linkTarget := src
			if relative {
				srcAbs := resolveSymlinksBestEffort(srcPath)
				dstDir := resolveSymlinksBestEffort(filepath.Dir(dst))
				if rel, err := filepath.Rel(dstDir, srcAbs); err == nil {
					linkTarget = rel
				}
			}
			linkErr = os.Symlink(linkTarget, dst)
		} else {
			linkErr = os.Link(srcPath, dst)
		}
		if linkErr != nil {
			if sym {
				fmt.Fprintf(ctx.Stderr, "ln: failed to create symbolic link '%s': %s\n", dstDisplay, fmtOsErr(linkErr))
			} else {
				fmt.Fprintf(ctx.Stderr, "ln: failed to create hard link '%s': %s\n", dstDisplay, fmtOsErr(linkErr))
			}
			exitCode = 1
			continue
		}
		if verbose {
			fmt.Fprintf(ctx.Stdout, "'%s' -> '%s'\n", dst, src)
		}
	}
	return exitCode
}

func sameFile(a, b string) bool {
	ai, aerr := os.Stat(a)
	bi, berr := os.Stat(b)
	if aerr != nil || berr != nil {
		return false
	}
	return os.SameFile(ai, bi)
}

func resolveSymlinksBestEffort(path string) string {
	for i := 0; i < 40; i++ {
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			return path
		}
		target, rerr := os.Readlink(path)
		if rerr != nil {
			return path
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		path = filepath.Clean(target)
	}
	return path
}

func lnBackupSuffix(ctx *Context, explicit string, hasExplicit bool) string {
	suffix := "~"
	if env := ctx.Getenv("SIMPLE_BACKUP_SUFFIX"); env != "" && !strings.Contains(env, "/") {
		suffix = env
	}
	if hasExplicit && explicit != "" && !strings.Contains(explicit, "/") {
		suffix = explicit
	}
	return suffix
}

func removeOneDir(ctx *Context, dir string) (string, error) {
	hasTrailingSlash := strings.HasSuffix(dir, "/") || strings.HasSuffix(dir, string(filepath.Separator))
	cleanDir := filepath.Clean(dir)
	path := resolvePath(ctx, cleanDir)

	info, lerr := os.Lstat(path)
	if lerr != nil {
		return dir, lerr
	}

	if info.Mode()&os.ModeSymlink != 0 {
		if hasTrailingSlash {
			targetInfo, statErr := os.Stat(path)
			if statErr == nil && !targetInfo.IsDir() {
				return dir, fmt.Errorf("Not a directory")
			}
			return dir, fmt.Errorf("Symbolic link not followed")
		}
		return dir, fmt.Errorf("Not a directory")
	}

	if !info.IsDir() {
		return dir, fmt.Errorf("Not a directory")
	}

	err := syscall.Rmdir(path)
	if err != nil {
		return dir, err
	}
	return dir, nil
}

func runRmdir(ctx *Context) int {
	flags, err := rmdirSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "rmdir:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, rmdirSpecParsed) {
		return 0
	}
	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "rmdir: missing operand")
		return 1
	}

	parents := flags.Bool("p")
	verbose := flags.Bool("v")
	ignoreFail := flags.Bool("ignore-fail-on-non-empty")
	exitCode := 0

	for _, dir := range flags.Operands {
		d, rerr := removeOneDir(ctx, dir)
		if rerr != nil {
			if ignoreFail && (isDirNotEmpty(rerr) || dirIsNonEmpty(resolvePath(ctx, dir))) {
				continue
			}
			fmt.Fprintf(ctx.Stderr, "rmdir: failed to remove '%s': %s\n", d, fmtOsErr(rerr))
			exitCode = 1
			continue
		}
		if verbose {
			fmt.Fprintf(ctx.Stdout, "rmdir: removing directory, '%s'\n", d)
		}
		if !parents {
			continue
		}
		cleanDir := filepath.Clean(dir)
		for {
			parent := filepath.Dir(cleanDir)
			if parent == "." || parent == "/" || parent == cleanDir {
				break
			}
			cleanDir = parent
			d, rerr := removeOneDir(ctx, cleanDir)
			if rerr != nil {
				if ignoreFail && (isDirNotEmpty(rerr) || dirIsNonEmpty(resolvePath(ctx, cleanDir))) {
					break
				}
				fmt.Fprintf(ctx.Stderr, "rmdir: failed to remove '%s': %s\n", d, fmtOsErr(rerr))
				exitCode = 1
				break
			}
			if verbose {
				fmt.Fprintf(ctx.Stdout, "rmdir: removing directory, '%s'\n", d)
			}
		}
	}
	return exitCode
}
