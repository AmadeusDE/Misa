// F08 move regressions follow MIT uutils/coreutils tests/by-util/test_mv.rs
// at fee61ff5d4f87e0f912a52e639942b5f7ad58663. Native Go implementation.
package builtins

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const mvSpec = "i(interactive)f(force)v(verbose)n(no-clobber)t(target-directory):T(no-target-directory)" +
	"(backup):?(suffix):bu(update):?(strip-trailing-slashes)(debug)(progress)(exchange)"

var mvSpecParsed *Spec

func init() {
	var err error
	mvSpecParsed, err = ParseSpec(mvSpec)
	if err != nil {
		panic("builtins: bad mv spec: " + err.Error())
	}
	Register(Command{Name: "mv", Run: runMv})
}

// statOrLstat follows symlinks when possible, falling back to the link's
// own stat if the target can't be resolved (e.g. dangling). Used so the
// "same file" safety check catches writes that would go through a
// symlink on either side.
func statOrLstat(path string) (os.FileInfo, error) {
	if fi, err := os.Stat(path); err == nil {
		return fi, nil
	}
	return os.Lstat(path)
}

func runMv(ctx *Context) (code int) {
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	flags, err := mvSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "mv:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, mvSpecParsed) {
		return 0
	}

	ops := flags.Operands
	if len(ops) < 1 {
		fmt.Fprintln(ctx.Stderr, "mv: missing file operand")
		fmt.Fprintln(ctx.Stderr, "Try 'mv --help' for more information.")
		return 1
	}

	verbose := flags.Bool("v")
	debug := flags.Bool("debug")
	noTargetDir := flags.Bool("T")
	exchange := flags.Bool("exchange")

	mode := "force"
	for _, k := range flags.Order() {
		switch k {
		case "i", "interactive":
			mode = "interactive"
		case "f", "force":
			mode = "force"
		case "n", "no-clobber":
			mode = "noClobber"
		}
	}
	noClobber := mode == "noClobber"
	interactive := mode == "interactive"
	force := mode == "force"

	updateVal, updateGiven := flags.String("u")
	updateMode := ""
	if updateGiven {
		updateMode = updateVal
		if updateMode == "" {
			updateMode = "older"
		}
	}

	backupCtl, backupGiven, backupErr := backupControl(ctx, flags)
	if backupErr != nil {
		fmt.Fprintf(ctx.Stderr, "mv: %s\n", backupErr)
		return 1
	}
	backupActive := backupGiven && backupCtl != "none"
	suffix := backupSuffix(ctx, flags)

	if backupActive && noClobber {
		fmt.Fprintln(ctx.Stderr, "mv: cannot combine --backup with -n")
		return 1
	}
	if backupActive && (updateMode == "none" || updateMode == "none-fail") {
		fmt.Fprintln(ctx.Stderr, "mv: cannot combine --backup with --update=none")
		return 1
	}

	targetDir := ""
	if t, ok := flags.String("t"); ok {
		targetDir = t
	}

	var sources []string
	var dst string
	var displayDst string

	if targetDir != "" {
		sources = ops
		dst = resolvePath(ctx, targetDir)
		displayDst = targetDir
	} else {
		if len(ops) < 2 {
			fmt.Fprintf(ctx.Stderr, "mv: missing destination file operand after '%s'\n", ops[len(ops)-1])
			fmt.Fprintln(ctx.Stderr, "Try 'mv --help' for more information.")
			return 1
		}
		dst = resolvePath(ctx, ops[len(ops)-1])
		displayDst = ops[len(ops)-1]
		sources = ops[:len(ops)-1]
	}

	dstInfo, dstErr := os.Stat(dst)
	dstIsDir := dstErr == nil && dstInfo.IsDir() && !noTargetDir

	if exchange {
		for _, src := range sources {
			if ctx.CancellationContext().Err() != nil {
				return 1
			}
			srcPath := resolvePath(ctx, src)
			target := dst
			displayTarget := displayDst
			if dstIsDir && noTargetDir {
				fmt.Fprintln(ctx.Stderr, "mv: target is not a directory")
				return 1
			}
			if dstIsDir {
				target = filepath.Join(dst, filepath.Base(srcPath))
				displayTarget = filepath.Join(displayDst, filepath.Base(src))
			}
			srcInfo, err1 := os.Lstat(srcPath)
			tInfo, err2 := os.Lstat(target)
			if err1 != nil {
				fmt.Fprintf(ctx.Stderr, "mv: cannot stat '%s': No such file or directory\n", src)
				return 1
			}
			if err2 != nil {
				fmt.Fprintf(ctx.Stderr, "mv: cannot stat '%s': No such file or directory\n", displayTarget)
				return 1
			}
			if os.SameFile(srcInfo, tInfo) {
				fmt.Fprintf(ctx.Stderr, "mv: '%s' and '%s' are the same file\n", src, displayTarget)
				return 1
			}
			err = renameExchange(srcPath, target)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "mv: exchange '%s' and '%s' failed: %v\n", src, displayTarget, err)
				return 1
			}
			if verbose || debug {
				fmt.Fprintf(ctx.Stdout, "exchanged '%s' <-> '%s'\n", src, displayTarget)
			}
		}
		return 0
	}

	if targetDir != "" {
		if dstErr != nil {
			fmt.Fprintf(ctx.Stderr, "mv: target directory '%s': %s\n", targetDir, fmtOsErr(dstErr))
			return 1
		}
		if !dstInfo.IsDir() {
			fmt.Fprintf(ctx.Stderr, "mv: target directory '%s': Not a directory\n", targetDir)
			return 1
		}
	}

	if len(sources) > 1 && !dstIsDir {
		reason := "Not a directory"
		if dstErr != nil {
			reason = fmtOsErr(dstErr)
		}
		fmt.Fprintf(ctx.Stderr, "mv: target '%s': %s\n", ops[len(ops)-1], reason)
		return 1
	}

	// Track paths to detect duplicate source operands.
	seenSources := map[string]bool{}

	// Track destinations that have been successfully written in this invocation
	// to prevent "will not overwrite just-created" errors
	justCreated := map[string]bool{}

	exitCode := 0
	for _, src := range sources {
		if ctx.CancellationContext().Err() != nil {
			return 1
		}
		srcPath := resolvePath(ctx, src)

		srcInfo, err := os.Lstat(srcPath)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "mv: cannot stat '%s': %s\n", src, fmtOsErr(err))
			exitCode = 1
			continue
		}

		canon := filepath.Clean(srcPath)
		if seenSources[canon] {
			kind := "file"
			if srcInfo.IsDir() {
				kind = "directory"
			}
			fmt.Fprintf(ctx.Stderr, "mv: warning: source %s '%s' specified more than once\n", kind, src)
			continue
		}
		seenSources[canon] = true

		target := dst
		if dstIsDir {
			target = filepath.Join(dst, filepath.Base(src))
		}

		displayTarget := displayDst
		if dstIsDir {
			base := filepath.Base(src)
			if displayDst == "." && base != "." {
				displayTarget = "./" + base
			} else {
				displayTarget = filepath.Join(displayDst, base)
			}
		}

		if !backupActive {
			srcL, slErr := os.Lstat(srcPath)
			dstL, dlErr := os.Lstat(target)
			if slErr == nil && dlErr == nil {
				if os.SameFile(srcL, dstL) {
					fmt.Fprintf(ctx.Stderr, "mv: '%s' and '%s' are the same file\n", src, displayTarget)
					exitCode = 1
					continue
				}
				if srcL.Mode()&os.ModeSymlink != 0 && dstL.Mode()&os.ModeSymlink == 0 {
					srcStat, sErr := os.Stat(srcPath)
					dstStat, dErr := os.Stat(target)
					if sErr == nil && dErr == nil && os.SameFile(srcStat, dstStat) {
						fmt.Fprintf(ctx.Stderr, "mv: '%s' and '%s' are the same file\n", src, displayTarget)
						exitCode = 1
						continue
					}
				}
			}
		}

		if srcInfo.IsDir() && isIntoSelf(srcPath, target) {
			fmt.Fprintf(ctx.Stderr, "mv: cannot move '%s' to a subdirectory of itself, '%s'\n", src, displayTarget)
			exitCode = 1
			continue
		}

		if tInfo, tErr := os.Lstat(target); tErr == nil && srcInfo.IsDir() && !tInfo.IsDir() {
			fmt.Fprintf(ctx.Stderr, "mv: cannot overwrite non-directory '%s' with directory '%s'\n", displayTarget, src)
			exitCode = 1
			continue
		}

		targetExists := false
		if tInfo, lerr := os.Lstat(target); lerr == nil {
			targetExists = true
			if !backupActive && srcInfo.IsDir() && tInfo.IsDir() {
				if err := os.Remove(target); err != nil {
					fmt.Fprintf(ctx.Stderr, "mv: cannot overwrite '%s': %s\n", displayTarget, fmtOsErr(err))
					exitCode = 1
					continue
				}
				targetExists = false
			}
		}

		// Check if this destination was just-created in this invocation
		if justCreated[filepath.Clean(target)] {
			fmt.Fprintf(ctx.Stderr, "mv: will not overwrite just-created '%s' with '%s'\n", displayTarget, src)
			exitCode = 1
			continue
		}

		if noClobber && targetExists {
			if debug {
				fmt.Fprintf(ctx.Stdout, "skipped '%s'\n", displayTarget)
			}
			continue
		}

		if updateMode != "" && targetExists {
			if dstStat, derr := os.Stat(target); derr == nil {
				skip, failHard := shouldSkipForUpdate(updateMode, srcInfo, dstStat)
				if skip {
					if failHard {
						exitCode = 1
					}
					continue
				}
			}
		}

		if interactive && !force && targetExists {
			fmt.Fprintf(ctx.Stderr, "mv: overwrite '%s'? ", displayTarget)
			var resp string
			if ctx.Stdin != nil {
				fmt.Fscanln(ctx.Stdin, &resp)
			}
			if ctx.CancellationContext().Err() != nil {
				return 1
			}
			resp = strings.TrimSpace(resp)
			if len(resp) == 0 || (resp[0] != 'y' && resp[0] != 'Y') {
				exitCode = 1
				continue
			}
		}

		var backupDisplay string
		if backupActive && targetExists {
			backupPath := computeBackupPath(strings.TrimRight(target, "/"), suffix, backupCtl)
			simpleBackup := strings.TrimRight(target, "/") + suffix
			isSimpleConflict := (backupCtl != "numbered" && backupCtl != "t") && filepath.Clean(simpleBackup) == filepath.Clean(srcPath)
			if srcInfo.Mode()&os.ModeSymlink == 0 && (filepath.Clean(backupPath) == filepath.Clean(srcPath) || isSimpleConflict) {
				fmt.Fprintf(ctx.Stderr, "mv: backing up '%s' might destroy source;  '%s' not moved\n", displayTarget, displayBackupName(displayTarget, target, backupPath))
				exitCode = 1
				continue
			}
			bpath, berr := performBackup(target, true, suffix, backupCtl)
			if berr != nil {
				fmt.Fprintf(ctx.Stderr, "mv: %s\n", fmtOsErr(berr))
				exitCode = 1
				continue
			}
			backupDisplay = displayBackupName(displayTarget, target, bpath)
		}

		err = os.Rename(srcPath, target)
		if err != nil {
			var linkErr *os.LinkError
			if errors.As(err, &linkErr) && errors.Is(linkErr.Err, syscall.EXDEV) {
				if cpErr := crossDeviceMove(ctx, srcPath, target); cpErr != nil {
					fmt.Fprintf(ctx.Stderr, "mv: %v\n", cpErr)
					exitCode = 1
					continue
				}
			} else {
				fmt.Fprintf(ctx.Stderr, "mv: cannot move '%s' to '%s': %s\n", src, displayTarget, fmtOsErr(err))
				exitCode = 1
				continue
			}
		}

		// Mark this destination as just-created in this invocation
		justCreated[filepath.Clean(target)] = true

		if verbose || debug {
			msg := fmt.Sprintf("renamed '%s' -> '%s'", src, displayTarget)
			if backupDisplay != "" {
				msg += fmt.Sprintf(" (backup: '%s')", backupDisplay)
			}
			fmt.Fprintln(ctx.Stdout, msg)
		}
	}
	return exitCode
}

func crossDeviceMove(ctx *Context, src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeNamedPipe != 0 {
		os.Remove(dst)
		if err := syscall.Mkfifo(dst, uint32(info.Mode().Perm())); err != nil {
			return err
		}
		return os.Remove(src)
	}
	if info.Mode()&os.ModeSocket != 0 || info.Mode()&os.ModeDevice != 0 {
		os.Remove(dst)
		return os.Remove(src)
	}
	if info.IsDir() {
		if err := copyDirRecursive(src, dst, preserveAttrs{}, true, false, false, ctx, nil, false, "", ""); err != nil {
			return err
		}
	} else {
		if _, err := copyOneFile(ctx, src, dst, info, preserveAttrs{}, false, src, dst, false, "", ""); err != nil {
			return err
		}
	}
	if err := ctx.CancellationContext().Err(); err != nil {
		return err
	}
	return os.RemoveAll(src)
}
