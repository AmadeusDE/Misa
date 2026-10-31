// F08 creation-mode behavior is checked against Toybox tests/cp.test
// at 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb (0BSD). Native Go implementation.
package builtins

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/charmbracelet/x/term"
)

// Real GNU cp/mv never let short "-b" absorb a value the way the generic
// optional-string short-flag parsing would (e.g. "-bl" must mean "-b -l",
// not "-b" with value "l"). So -b is a bare bool here, and any CONTROL
// word comes only from the long "--backup[=X]" form; backupControl below
// reassembles the two into one style.
const cpSpec = "r(recursive)Ra(archive)f(force)v(verbose)n(no-clobber)(preserve):?pdP(no-dereference)LHs(symbolic-link)l(link)i(interactive)T(no-target-directory)t(target-directory):(parents)(backup):?(suffix):Dbu(update):?(remove-destination)(attributes-only)(debug)(no-preserve):(strip-trailing-slashes)Z(context):g(progress)(sparse):?(reflink):?"

var cpSpecParsed *Spec

func isTerminal(r io.Reader) bool {
	if f, ok := r.(*os.File); ok {
		return term.IsTerminal(f.Fd())
	}
	return false
}

func init() {
	var err error
	cpSpecParsed, err = ParseSpec(cpSpec)
	if err != nil {
		panic("builtins: bad cp spec: " + err.Error())
	}
	Register(Command{Name: "cp", Run: runCp})
}

// isSubdirOrEqual reports whether target is src itself or lies inside src,
// used to refuse recursively copying a directory into itself.
func isSubdirOrEqual(src, target string) bool {
	srcAbs, err1 := filepath.Abs(src)
	targetAbs, err2 := filepath.Abs(target)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(srcAbs, targetAbs)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && rel != "")
}

// inodeKey identifies a file by device+inode, used to notice when two
// operands (or two files discovered while recursing) refer to the same
// underlying file so hard links can be preserved instead of duplicated.
type inodeKey struct{ dev, ino uint64 }

// preserveAttrs tracks which file attributes cp should carry over to the
// destination, as controlled by -p/--preserve/--no-preserve/-a.
type preserveAttrs struct {
	mode, ownership, timestamps, links bool
}

func parsePreserveList(list string) (preserveAttrs, error) {
	if list == "" {
		return preserveAttrs{mode: true, ownership: true, timestamps: true}, nil
	}
	var p preserveAttrs
	for _, part := range strings.Split(list, ",") {
		switch {
		case part == "mode" || (len(part) >= 1 && strings.HasPrefix("mode", part)):
			p.mode = true
		case part == "ownership" || (len(part) >= 1 && strings.HasPrefix("ownership", part)):
			p.ownership = true
		case part == "timestamps" || (len(part) >= 1 && strings.HasPrefix("timestamps", part)):
			p.timestamps = true
		case part == "links" || part == "link" || part == "li" || part == "l":
			p.links = true
		case len(part) >= 1 && strings.HasPrefix("all", part):
			p.mode, p.ownership, p.timestamps, p.links = true, true, true, true
		case part == "context" || part == "xattr":
			// ignored on systems without xattr/selinux support
		default:
			return p, fmt.Errorf("invalid argument '%s' for '--preserve'", part)
		}
	}
	return p, nil
}

func parseNoPreserveList(list string) preserveAttrs {
	var p preserveAttrs
	for _, part := range strings.Split(list, ",") {
		switch strings.TrimSpace(part) {
		case "mode":
			p.mode = true
		case "ownership":
			p.ownership = true
		case "timestamps":
			p.timestamps = true
		case "links":
			p.links = true
		case "all":
			p.mode, p.ownership, p.timestamps, p.links = true, true, true, true
		}
	}
	return p
}

func subtractPreserve(base, minus preserveAttrs) preserveAttrs {
	if minus.mode {
		base.mode = false
	}
	if minus.ownership {
		base.ownership = false
	}
	if minus.timestamps {
		base.timestamps = false
	}
	if minus.links {
		base.links = false
	}
	return base
}

// Backup control styles, resolved from --backup[=CONTROL] / -b / VERSION_CONTROL.

func backupSuffix(ctx *Context, flags *ParseResult) string {
	if s, ok := flags.String("suffix"); ok && s != "" {
		return s
	}
	if s := ctx.Getenv("SIMPLE_BACKUP_SUFFIX"); s != "" {
		return s
	}
	return "~"
}

func backupControl(ctx *Context, flags *ParseResult) (control string, given bool, err error) {
	longVal, longGiven := flags.String("backup")
	given = flags.Bool("b") || longGiven
	if !given {
		return "none", false, nil
	}
	val := longVal
	if val == "" {
		val = ctx.Getenv("VERSION_CONTROL")
		if val == "" {
			val = "existing"
		}
	}
	switch val {
	case "none", "off":
		return "none", true, nil
	case "simple", "never":
		return "simple", true, nil
	case "existing", "nil":
		return "existing", true, nil
	case "numbered", "t":
		return "numbered", true, nil
	default:
		return "", true, fmt.Errorf("invalid argument '%s' for 'backup type'", val)
	}
}

func nextNumberedBackup(target string) string {
	n := 1
	for {
		candidate := fmt.Sprintf("%s.~%d~", target, n)
		if _, err := os.Lstat(candidate); err != nil {
			return candidate
		}
		n++
	}
}

func existingNumberedBackup(target string) (string, bool) {
	dir := filepath.Dir(target)
	base := filepath.Base(target)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	prefix := base + ".~"
	found := false
	maxN := 0
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, "~") {
			numStr := strings.TrimSuffix(strings.TrimPrefix(name, prefix), "~")
			if n, err := strconv.Atoi(numStr); err == nil {
				found = true
				if n > maxN {
					maxN = n
				}
			}
		}
	}
	if !found {
		return "", false
	}
	return fmt.Sprintf("%s.~%d~", target, maxN+1), true
}

func computeBackupPath(target, suffix, control string) string {
	switch control {
	case "simple":
		return target + suffix
	case "numbered":
		return nextNumberedBackup(target)
	case "existing":
		if p, ok := existingNumberedBackup(target); ok {
			return p
		}
		return target + suffix
	}
	return ""
}

// performBackup renames an existing target out of the way, returning the
// backup path used (empty if there was nothing to back up).
func performBackup(target string, active bool, suffix, control string) (string, error) {
	if !active {
		return "", nil
	}
	// A trailing slash (e.g. from "-T dst/") would otherwise get a backup
	// suffix appended after it, naming a bogus path *inside* the directory.
	target = strings.TrimRight(target, "/")
	if target == "" {
		target = "/"
	}
	if _, err := os.Lstat(target); err != nil {
		return "", nil
	}
	bpath := computeBackupPath(target, suffix, control)
	if bpath == "" {
		return "", nil
	}
	if err := os.Rename(target, bpath); err != nil {
		return "", err
	}
	return bpath, nil
}

func displayBackupName(display, target, backupPath string) string {
	if backupPath == "" {
		return ""
	}
	target = strings.TrimRight(target, "/")
	return display + strings.TrimPrefix(backupPath, target)
}

func runCp(ctx *Context) (code int) {
	stdinTerminal := isTerminal(ctx.Stdin)
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	flags, err := cpSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "cp:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, cpSpecParsed) {
		return 0
	}
	ops := flags.Operands
	if len(ops) < 1 {
		fmt.Fprintln(ctx.Stderr, "cp: missing operand")
		return 1
	}

	targetDir := ""
	if t, ok := flags.String("t"); ok {
		targetDir = t
	}

	var sources []string
	var dst string

	if targetDir != "" {
		sources = ops
		dst = resolvePath(ctx, targetDir)
	} else {
		if len(ops) < 2 {
			fmt.Fprintln(ctx.Stderr, "cp: missing destination file operand")
			return 1
		}
		dst = resolvePath(ctx, ops[len(ops)-1])
		sources = ops[:len(ops)-1]
	}

	stripSlashes := flags.Bool("strip-trailing-slashes")
	if stripSlashes {
		dst = strings.TrimRight(dst, "/")
		for i := range sources {
			sources[i] = strings.TrimRight(sources[i], "/")
		}
	}

	archive := flags.Bool("a")
	recursive := flags.Bool("r") || flags.Bool("R") || archive
	verbose := flags.Bool("v")
	noClobber := flags.Bool("n")
	preserveVal, preserveGiven := flags.ExplicitString("preserve")
	preserveGiven = preserveGiven || flags.Bool("p") || flags.Bool("preserve")
	// Hard links between command-line operands are preserved by default
	// (independent of -p), unless explicitly disabled via
	// --no-preserve=links.
	preserve := preserveAttrs{links: true}
	if archive {
		preserve = preserveAttrs{mode: true, ownership: true, timestamps: true, links: true}
	}
	if preserveGiven {
		p2, perr := parsePreserveList(preserveVal)
		if perr != nil {
			fmt.Fprintf(ctx.Stderr, "cp: %s\n", perr)
			return 1
		}
		preserve.mode = preserve.mode || p2.mode
		preserve.ownership = preserve.ownership || p2.ownership
		preserve.timestamps = preserve.timestamps || p2.timestamps
		preserve.links = preserve.links || p2.links
	}
	if noPreserveVal, ok := flags.String("no-preserve"); ok {
		preserve = subtractPreserve(preserve, parseNoPreserveList(noPreserveVal))
	}

	noDeref := flags.Bool("d") || flags.Bool("P") || archive
	derefAll := flags.Bool("L")
	derefArgs := flags.Bool("H")
	makeSymlink := flags.Bool("s")
	makeHardlink := flags.Bool("l")
	interactive := flags.Bool("i")
	force := flags.Bool("f")
	noTargetDir := flags.Bool("T")
	parents := flags.Bool("parents")
	removeDestination := flags.Bool("remove-destination")
	attributesOnly := flags.Bool("attributes-only")
	debug := flags.Bool("debug")

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
		fmt.Fprintf(ctx.Stderr, "cp: %s\n", backupErr)
		return 1
	}
	backupActive := backupGiven && backupCtl != "none"
	suffix := backupSuffix(ctx, flags)

	if backupActive && (updateMode == "none" || updateMode == "none-fail") {
		fmt.Fprintln(ctx.Stderr, "cp: --backup is mutually exclusive with -n or --update=none-fail")
		return 1
	}

	scanner := bufio.NewScanner(ctx.Stdin)
	dstInfo, dstErr := os.Stat(dst)
	dstIsDir := dstErr == nil && dstInfo.IsDir() && !noTargetDir

	if targetDir != "" {
		if dstErr != nil {
			fmt.Fprintf(ctx.Stderr, "cp: target directory '%s': %s\n", targetDir, fmtOsErr(dstErr))
			return 1
		}
		if !dstInfo.IsDir() {
			fmt.Fprintf(ctx.Stderr, "cp: target '%s' is not a directory\n", targetDir)
			return 1
		}
	}

	if parents && (dstErr != nil || !dstInfo.IsDir()) {
		fmt.Fprintln(ctx.Stderr, "cp: with --parents, the destination must be a directory")
		return 1
	}

	if len(sources) > 1 && !dstIsDir {
		fmt.Fprintf(ctx.Stderr, "cp: target '%s' is not a directory\n", ops[len(ops)-1])
		return 1
	}

	if sparseVal, ok := flags.String("sparse"); ok {
		if sparseVal != "always" && sparseVal != "auto" && sparseVal != "never" {
			fmt.Fprintf(ctx.Stderr, "cp: invalid argument '%s' for '--sparse'\nValid arguments are:\n  - 'always'\n  - 'auto'\n  - 'never'\n", sparseVal)
			return 1
		}
	}

	copiedInodes := make(map[inodeKey]string)
	seenSourcePaths := make(map[string]bool)
	createdSymlinks := make(map[string]bool)
	justCreated := make(map[string]bool)

	exitCode := 0
	for _, src := range sources {
		if ctx.CancellationContext().Err() != nil {
			return 1
		}
		if src == "" {
			fmt.Fprintln(ctx.Stderr, "cp: cannot stat '': No such file or directory")
			exitCode = 1
			continue
		}

		srcPath := resolvePath(ctx, src)
		target := dst
		displayDst := ops[len(ops)-1]
		if targetDir != "" {
			displayDst = targetDir
		}

		displayTarget := displayDst
		flattenContents := false
		if parents {
			cleanSrc := strings.TrimRight(src, "/")
			if cleanSrc == "" {
				cleanSrc = "/"
			}
			relSrc := strings.TrimPrefix(cleanSrc, "/")
			target = filepath.Join(dst, relSrc)
			displayTarget = filepath.Join(displayDst, relSrc)
			isAbs := filepath.IsAbs(cleanSrc)
			parts := strings.Split(filepath.Dir(cleanSrc), string(filepath.Separator))
			currSrc := ""
			if isAbs {
				currSrc = "/"
			}
			currDst := dst
			currDisplayDst := displayDst
			createdParentDirs := make(map[string]bool)
			for _, part := range parts {
				if part == "." || part == "" {
					continue
				}
				if currSrc == "" {
					currSrc = part
				} else if currSrc == "/" {
					currSrc = "/" + part
				} else {
					currSrc = filepath.Join(currSrc, part)
				}
				currDst = filepath.Join(currDst, part)
				currDisplayDst = filepath.Join(currDisplayDst, part)
				createdParentDirs[filepath.Clean(currDst)] = true
				if _, err := os.Stat(currDst); os.IsNotExist(err) {
					mode := os.FileMode(0755)
					if preserve.mode {
						if sInfo, sErr := os.Stat(currSrc); sErr == nil {
							mode = sInfo.Mode().Perm()
						}
					}
					if err := os.Mkdir(currDst, mode|0700); err == nil {
						if preserve.mode {
							_ = os.Chmod(currDst, mode)
						}
						if verbose {
							fmt.Fprintf(ctx.Stdout, "%s -> %s\n", currSrc, currDisplayDst)
						}
					}
				} else if err == nil && preserve.mode {
					if sInfo, sErr := os.Stat(currSrc); sErr == nil {
						_ = os.Chmod(currDst, sInfo.Mode().Perm())
					}
				}
			}
			if createdParentDirs[filepath.Clean(target)] {
				fmt.Fprintf(ctx.Stderr, "cp: cannot make directory '%s': File exists\n", displayTarget)
				exitCode = 1
				continue
			}
		} else if dstIsDir {
			base := filepath.Base(srcPath)
			// A trailing "/." (or a bare ".") means "copy the directory's
			// contents in place", not "create a new subdirectory here" —
			// unlike a plain trailing slash, which is just a dereference
			// hint and doesn't change the destination basename.
			if strings.HasSuffix(src, "/.") || src == "." {
				base = "."
				flattenContents = true
			}
			target = filepath.Join(dst, base)
			displayTarget = filepath.Join(displayDst, base)
		}

		srcInfo, err := os.Lstat(srcPath)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "cp: cannot stat '%s': %s\n", src, fmtOsErr(err))
			exitCode = 1
			continue
		}

		if srcInfo.IsDir() && recursive && isIntoSelf(srcPath, target) {
			fmt.Fprintf(ctx.Stderr, "cp: cannot copy a directory, '%s', into itself, '%s'\n", src, displayTarget)
			exitCode = 1
			continue
		}

		derefThis := derefAll || derefArgs || (!recursive && !noDeref) || strings.HasSuffix(src, "/")
		isSymlink := srcInfo.Mode()&os.ModeSymlink != 0
		if isSymlink && derefThis {
			derefInfo, derefErr := os.Stat(srcPath)
			if derefErr != nil {
				if linkTarget, lerr := os.Readlink(srcPath); lerr == nil {
					targetPath := filepath.Join(ctx.Dir, linkTarget)
					if stInfo, stErr := os.Stat(targetPath); stErr == nil {
						derefInfo = stInfo
						derefErr = nil
					}
				}
			}
			if derefErr != nil {
				fmt.Fprintf(ctx.Stderr, "cp: cannot stat '%s': %s\n", src, fmtOsErr(derefErr))
				exitCode = 1
				continue
			}
			srcInfo = derefInfo
		}

		cleanSrc := resolvePath(ctx, src)
		if seenSourcePaths[cleanSrc] {
			kind := "file"
			if srcInfo.IsDir() {
				kind = "directory"
			}
			fmt.Fprintf(ctx.Stderr, "cp: warning: source %s '%s' specified more than once\n", kind, src)
			continue
		}
		seenSourcePaths[cleanSrc] = true

		if justCreated[filepath.Clean(target)] {
			if li, err := os.Lstat(target); err == nil && li.Mode()&os.ModeSymlink != 0 {
				fmt.Fprintf(ctx.Stderr, "cp: will not copy '%s' through just-created symlink '%s'\n", src, displayTarget)
			} else {
				fmt.Fprintf(ctx.Stderr, "cp: will not overwrite just-created '%s' with '%s'\n", displayTarget, src)
			}
			exitCode = 1
			continue
		}

		tInfo, tErr := os.Lstat(target)
		if noTargetDir && tErr == nil && tInfo.IsDir() && !srcInfo.IsDir() {
			fmt.Fprintf(ctx.Stderr, "cp: cannot overwrite directory '%s' with non-directory\n", displayTarget)
			exitCode = 1
			continue
		}
		if srcInfo.IsDir() && tErr == nil && !tInfo.IsDir() {
			fmt.Fprintf(ctx.Stderr, "cp: cannot overwrite non-directory with directory\n")
			exitCode = 1
			continue
		}

		// Preserve hard-link relationships between command-line operands:
		// once one operand's target has been written, any later operand
		// that resolves (after any dereferencing above) to the same
		// device+inode is hard-linked to it instead of copied again.
		// Real filesystem hard links (Nlink>1) get this even without
		// --preserve=links, matching cp's existing default behavior.
		targetExists := false
		if _, lerr := os.Lstat(target); lerr == nil {
			targetExists = true
		}

		if noClobber && targetExists {
			if debug {
				fmt.Fprintf(ctx.Stdout, "skipped '%s'\n", displayTarget)
			}
			continue
		}

		if st, ok := srcInfo.Sys().(*syscall.Stat_t); ok && !srcInfo.IsDir() && preserve.links {
			k := inodeKey{uint64(st.Dev), st.Ino}
			if prevDst, exists := copiedInodes[k]; exists {
				_ = os.Remove(target)
				if err := os.Link(prevDst, target); err == nil {
					if verbose {
						fmt.Fprintf(ctx.Stdout, "'%s' -> '%s'\n", src, displayTarget)
					}
					continue
				}
			}
			copiedInodes[k] = target
		}

		if filepath.Clean(srcPath) == filepath.Clean(target) && !(force && backupActive) && !makeHardlink {
			fmt.Fprintf(ctx.Stderr, "cp: '%s' and '%s' are the same file\n", src, displayTarget)
			exitCode = 1
			continue
		}

		skipSameFile := backupActive || makeHardlink ||
			(removeDestination && filepath.Clean(srcPath) != filepath.Clean(target))
		if !skipSameFile {
			var dstCmp os.FileInfo
			var dcErr error
			if !makeSymlink {
				dstCmp, dcErr = os.Stat(target)
				if dcErr != nil {
					dstCmp, dcErr = os.Lstat(target)
				}
			} else {
				dstCmp, dcErr = os.Lstat(target)
			}
			if dcErr == nil && os.SameFile(srcInfo, dstCmp) {
				fmt.Fprintf(ctx.Stderr, "cp: '%s' and '%s' are the same file\n", src, displayTarget)
				exitCode = 1
				continue
			}
		}

		if attributesOnly {
			if isSymlink && !derefThis {
				if !removeDestination {
					fmt.Fprintf(ctx.Stderr, "cp: not writing through symlink '%s' with --attributes-only; use --remove-destination\n", displayTarget)
					exitCode = 1
					continue
				}
				// fall through: removeDestination allows replacing dst with a symlink copy.
			} else {
				if aerr := applyAttributesOnly(target, displayTarget, srcInfo, preserve); aerr != nil {
					fmt.Fprintf(ctx.Stderr, "cp: %s\n", aerr.Error())
					exitCode = 1
					continue
				}
				if verbose {
					fmt.Fprintf(ctx.Stdout, "'%s' -> '%s'\n", src, displayTarget)
				}
				continue
			}
		}

		targetExists = false
		if _, lerr := os.Lstat(target); lerr == nil {
			targetExists = true
		}

		if backupActive && targetExists {
			backupPath := computeBackupPath(strings.TrimRight(target, "/"), suffix, backupCtl)
			if backupPath != "" && filepath.Clean(backupPath) == filepath.Clean(srcPath) {
				fmt.Fprintf(ctx.Stderr, "cp: backing up '%s' might destroy source;  '%s' not copied\n", displayTarget, displayBackupName(displayTarget, target, backupPath))
				exitCode = 1
				continue
			}
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
					if debug {
						fmt.Fprintf(ctx.Stdout, "skipped '%s'\n", displayTarget)
					}
					if failHard {
						fmt.Fprintf(ctx.Stderr, "cp: not replacing '%s'\n", displayTarget)
						exitCode = 1
					}
					continue
				}
			}
		}

		if interactive && !srcInfo.IsDir() && targetExists {
			if stdinTerminal {
				fmt.Fprintf(ctx.Stderr, "cp: overwrite '%s'? ", displayTarget)
			}
			answeredYes := true
			if scanner.Scan() {
				ans := strings.TrimSpace(scanner.Text())
				answeredYes = len(ans) > 0 && (strings.HasPrefix(ans, "y") || strings.HasPrefix(ans, "Y"))
			}
			if ctx.CancellationContext().Err() != nil {
				return 1
			}
			if !answeredYes {
				exitCode = 1
				continue
			}
		}

		if !backupActive && !removeDestination && targetExists && !srcInfo.IsDir() && !(isSymlink && !derefThis) {
			// Default protections against symlink attacks: refuse to write
			// through a symlink this same invocation just created, and
			// refuse to write through a dangling symlink unless the caller
			// opted into historical behavior via POSIXLY_CORRECT.
			if lt, lerr := os.Lstat(target); lerr == nil && lt.Mode()&os.ModeSymlink != 0 {
				if createdSymlinks[target] {
					fmt.Fprintf(ctx.Stderr, "cp: will not copy '%s' through just-created symlink '%s'\n", src, displayTarget)
					exitCode = 1
					continue
				}
				if _, serr := os.Stat(target); serr != nil {
					if isELOOP(serr) {
						_ = os.Remove(target)
					} else if _, ok := lookupEnv(ctx, "POSIXLY_CORRECT"); !ok {
						fmt.Fprintf(ctx.Stderr, "cp: not writing through dangling symlink '%s'\n", displayTarget)
						exitCode = 1
						continue
					}
				}
			}
		}

		if removeDestination && targetExists {
			if srcInfo.IsDir() {
				_ = os.RemoveAll(target)
			} else {
				_ = os.Remove(target)
			}
		}

		var backupDisplay string
		if makeSymlink {
			bpath := ""
			if backupActive {
				bp, berr := performBackup(target, true, suffix, backupCtl)
				if berr != nil {
					fmt.Fprintf(ctx.Stderr, "cp: %s\n", fmtOsErr(berr))
					exitCode = 1
					continue
				}
				bpath = bp
			} else {
				_ = os.Remove(target)
			}
			if err := os.Symlink(srcPath, target); err != nil {
				fmt.Fprintf(ctx.Stderr, "cp: cannot create symlink '%s' to '%s'\n", displayTarget, src)
				exitCode = 1
				continue
			}
			justCreated[filepath.Clean(target)] = true
			backupDisplay = displayBackupName(displayTarget, target, bpath)
		} else if makeHardlink && !srcInfo.IsDir() {
			if tInfo, terr := os.Lstat(target); terr == nil && os.SameFile(srcInfo, tInfo) {
				// Already hard-linked to the source; nothing to do.
			} else {
				bpath := ""
				if backupActive {
					bp, berr := performBackup(target, true, suffix, backupCtl)
					if berr != nil {
						fmt.Fprintf(ctx.Stderr, "cp: %s\n", fmtOsErr(berr))
						exitCode = 1
						continue
					}
					bpath = bp
				} else {
					_ = os.Remove(target)
				}
				if err := os.Link(srcPath, target); err != nil {
					fmt.Fprintf(ctx.Stderr, "cp: cannot create hard link '%s' to '%s'\n", displayTarget, src)
					exitCode = 1
					continue
				}
				justCreated[filepath.Clean(target)] = true
				backupDisplay = displayBackupName(displayTarget, target, bpath)
			}
		} else if srcInfo.IsDir() {
			if !recursive {
				fmt.Fprintf(ctx.Stderr, "cp: -r not specified; omitting directory '%s'\n", src)
				exitCode = 1
				continue
			}
			if isSubdirOrEqual(srcPath, target) {
				fmt.Fprintf(ctx.Stderr, "cp: cannot copy a directory, '%s', into itself, '%s'\n", src, displayTarget)
				exitCode = 1
				continue
			}
			copyDeref := derefAll
			if err := copyDirRecursiveLinked(srcPath, target, src, displayTarget, preserve, noDeref, copyDeref, interactive, ctx, scanner, backupActive, suffix, backupCtl, copiedInodes); err != nil {
				exitCode = 1
				continue
			}
		} else {
			bpath, err := copyOneFile(ctx, srcPath, target, srcInfo, preserve, force, src, displayTarget, backupActive, suffix, backupCtl)
			if err != nil {
				fmt.Fprintln(ctx.Stderr, "cp: "+err.Error())
				exitCode = 1
				continue
			}
			backupDisplay = displayBackupName(displayTarget, target, bpath)
			justCreated[filepath.Clean(target)] = true
			if isSymlink && !derefThis {
				createdSymlinks[target] = true
			}
			if debug {
				if _, werr := fmt.Fprintf(ctx.Stdout, "copy offload: unsupported, reflink: unsupported, sparse detection: no\n"); werr != nil {
					exitCode = 1
				}
			}
		}

		if verbose && !(flattenContents && srcInfo.IsDir()) {
			msg := fmt.Sprintf("'%s' -> '%s'", src, displayTarget)
			if backupDisplay != "" {
				msg += fmt.Sprintf(" (backup: '%s')", backupDisplay)
			}
			fmt.Fprintln(ctx.Stdout, msg)
		}
	}
	return exitCode
}

func shouldSkipForUpdate(mode string, srcInfo, dstInfo os.FileInfo) (skip bool, fail bool) {
	switch mode {
	case "all":
		return false, false
	case "none":
		return true, false
	case "none-fail":
		return true, true
	default: // "older"
		if !srcInfo.ModTime().After(dstInfo.ModTime()) {
			return true, false
		}
		return false, false
	}
}

func isELOOP(err error) bool {
	return strings.Contains(err.Error(), "too many levels of symbolic links")
}

// isIntoSelf reports whether copying/moving directory src to target would
// place src inside itself, by walking target's existing ancestors looking
// for src's device+inode.
func isIntoSelf(src, target string) bool {
	srcInfo, err := os.Stat(src)
	if err != nil || !srcInfo.IsDir() {
		return false
	}
	dir := filepath.Clean(target)
	for {
		if info, err := os.Lstat(dir); err == nil && os.SameFile(srcInfo, info) {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

func applyAttributesOnly(target, displayTarget string, srcInfo os.FileInfo, preserve preserveAttrs) error {
	if _, err := os.Lstat(target); err != nil {
		f, cerr := os.Create(target)
		if cerr != nil {
			return fmt.Errorf("cannot create regular file '%s': %s", displayTarget, fmtOsErr(cerr))
		}
		f.Close()
	}
	if preserve.mode {
		_ = os.Chmod(target, srcInfo.Mode().Perm())
	}
	if preserve.timestamps {
		_ = os.Chtimes(target, srcInfo.ModTime(), srcInfo.ModTime())
	}
	if preserve.ownership {
		if st, ok := srcInfo.Sys().(*syscall.Stat_t); ok {
			_ = os.Chown(target, int(st.Uid), int(st.Gid))
		}
	}
	return nil
}

// applyCopiedAttrs applies mode/timestamps/ownership to a freshly-created
// dst independently, per the requested preserve set — mixing e.g.
// --preserve=ownership with --no-preserve=mode must not force mode
// preservation too.
func applyCopiedAttrs(dst string, srcInfo os.FileInfo, preserve preserveAttrs) {
	if preserve.mode {
		os.Chmod(dst, srcInfo.Mode().Perm())
	}
	// Else: leave the destination mode as set by open (source mode masked
	// by umask for a new file, unchanged for an existing file).
	if preserve.timestamps {
		os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
	}
	if preserve.ownership {
		if st, ok := srcInfo.Sys().(*syscall.Stat_t); ok {
			os.Chown(dst, int(st.Uid), int(st.Gid))
		}
	}
}

// copyOneFile copies a single file (or recreates a symlink/fifo) at dst,
// backing up an existing dst first when backupActive. It returns the
// backup path used, if any.
func copyOneFile(ctx *Context, src, dst string, srcInfo os.FileInfo, preserve preserveAttrs, force bool, displaySrc, displayDst string, backupActive bool, suffix, control string) (string, error) {
	if srcInfo.Mode()&os.ModeSymlink != 0 {
		linkTarget, err := os.Readlink(src)
		if err != nil {
			return "", fmt.Errorf("cannot read symbolic link '%s': %s", displaySrc, fmtOsErr(err))
		}
		bpath := ""
		if backupActive {
			bp, berr := performBackup(dst, true, suffix, control)
			if berr != nil {
				return "", fmt.Errorf("%s", fmtOsErr(berr))
			}
			bpath = bp
		} else {
			_ = os.Remove(dst)
		}
		if err := os.Symlink(linkTarget, dst); err != nil {
			return bpath, fmt.Errorf("cannot create symlink '%s' to '%s'", displayDst, displaySrc)
		}
		return bpath, nil
	}

	// Named pipes (and other special files) can't be read like regular
	// data — opening one for reading blocks until a writer shows up, and
	// there's nothing meaningful to stream anyway. Recreate the node
	// itself instead of copying "through" it.
	if srcInfo.Mode()&os.ModeNamedPipe != 0 {
		bpath := ""
		if backupActive {
			bp, berr := performBackup(dst, true, suffix, control)
			if berr != nil {
				return "", fmt.Errorf("%s", fmtOsErr(berr))
			}
			bpath = bp
		} else {
			_ = os.Remove(dst)
		}
		// 0666: the umask still applies at mknod() time; applyCopiedAttrs
		// below corrects the mode explicitly when preserve.mode is set.
		if err := syscall.Mkfifo(dst, 0666); err != nil {
			return bpath, fmt.Errorf("cannot create fifo '%s': %s", displayDst, fmtOsErr(err))
		}
		applyCopiedAttrs(dst, srcInfo, preserve)
		return bpath, nil
	}

	sf, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("cannot open '%s' for reading: %s", displaySrc, strings.ToLower(fmtOsErr(err)))
	}
	defer sf.Close()

	bpath := ""
	if backupActive {
		bp, berr := performBackup(dst, true, suffix, control)
		if berr != nil {
			return "", fmt.Errorf("%s", fmtOsErr(berr))
		}
		bpath = bp
	} else if force {
		_ = os.Remove(dst)
	}
	df, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode().Perm())
	if err != nil && force && !backupActive {
		_ = os.Remove(dst)
		df, err = os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode().Perm())
	}
	if err != nil {
		return bpath, fmt.Errorf("cannot create regular file '%s': %s", displayDst, fmtOsErr(err))
	}
	defer df.Close()

	if _, err := CopyContext(ctx.CancellationContext(), df, sf); err != nil {
		return bpath, fmt.Errorf("error writing '%s': %s", displayDst, fmtOsErr(err))
	}

	applyCopiedAttrs(dst, srcInfo, preserve)
	return bpath, nil
}

func copyDirRecursive(src, dst string, preserve preserveAttrs, noDeref, deref, interactive bool, ctx *Context, scanner *bufio.Scanner, backupActive bool, suffix, control string) error {
	return copyDirRecursiveLinked(src, dst, src, dst, preserve, noDeref, deref, interactive, ctx, scanner, backupActive, suffix, control, nil)
}

// copyDirRecursiveLinked is copyDirRecursive with an optional shared
// device+inode map so hard-linked files discovered anywhere in the tree
// (or shared with another top-level operand) are hard-linked at the
// destination instead of duplicated, matching --preserve=links.
func copyDirRecursiveLinked(src, dst, displaySrc, displayDst string, preserve preserveAttrs, noDeref, deref, interactive bool, ctx *Context, scanner *bufio.Scanner, backupActive bool, suffix, control string, linkMap map[inodeKey]string) error {
	if err := ctx.CancellationContext().Err(); err != nil {
		return err
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "cp: cannot stat '%s': %s\n", displaySrc, fmtOsErr(err))
		return errors.New("cannot stat source directory")
	}

	// Create with a permissive mode and let the umask (or, below, an
	// explicit preserve.mode chmod) settle the final bits — using the
	// source's own perm here directly would let its specific bits survive
	// umask-masking in ways a fresh directory's wouldn't.
	if err := os.MkdirAll(dst, 0777); err != nil {
		fmt.Fprintf(ctx.Stderr, "cp: cannot create directory '%s': %s\n", displayDst, fmtOsErr(err))
		return errors.New("cannot create destination directory")
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "cp: cannot open directory '%s' for reading: %s\n", displaySrc, fmtOsErr(err))
		return errors.New("cannot read source directory")
	}

	var hasError bool
	for _, e := range entries {
		if err := ctx.CancellationContext().Err(); err != nil {
			return err
		}
		srcChild := filepath.Join(src, e.Name())
		dstChild := filepath.Join(dst, e.Name())
		displaySrcChild := filepath.Join(displaySrc, e.Name())
		displayDstChild := filepath.Join(displayDst, e.Name())

		info, err := os.Lstat(srcChild)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "cp: cannot stat '%s': %s\n", displaySrcChild, fmtOsErr(err))
			hasError = true
			continue
		}

		isLink := info.Mode()&os.ModeSymlink != 0
		if isLink && deref {
			statInfo, statErr := os.Stat(srcChild)
			if statErr != nil {
				if linkTarget, lerr := os.Readlink(srcChild); lerr == nil {
					targetPath := filepath.Join(ctx.Dir, linkTarget)
					if stInfo, stErr := os.Stat(targetPath); stErr == nil {
						statInfo = stInfo
						statErr = nil
					}
				}
			}
			if statErr != nil {
				fmt.Fprintf(ctx.Stderr, "cp: cannot stat '%s': %s\n", displaySrcChild, fmtOsErr(statErr))
				hasError = true
				continue
			}
			info = statInfo
		}

		if interactive && !info.IsDir() {
			if _, err := os.Lstat(dstChild); err == nil {
				fmt.Fprintf(ctx.Stderr, "cp: overwrite '%s'? ", displayDstChild)
				if scanner.Scan() {
					ans := strings.TrimSpace(scanner.Text())
					if len(ans) > 0 && (strings.HasPrefix(ans, "y") || strings.HasPrefix(ans, "Y")) {
						// proceed
					} else {
						continue
					}
				} else {
					return fmt.Errorf("interrupted")
				}
			}
		}

		if err := ctx.CancellationContext().Err(); err != nil {
			return err
		}

		if info.IsDir() {
			if err := copyDirRecursiveLinked(srcChild, dstChild, displaySrcChild, displayDstChild, preserve, noDeref, deref, interactive, ctx, scanner, backupActive, suffix, control, linkMap); err != nil {
				hasError = true
			}
			continue
		}

		leafInfo := info
		if isLink && noDeref {
			leafInfo, _ = os.Lstat(srcChild)
		}

		if linkMap != nil {
			if st, ok := leafInfo.Sys().(*syscall.Stat_t); ok && preserve.links {
				k := inodeKey{uint64(st.Dev), st.Ino}
				if prevDst, exists := linkMap[k]; exists {
					_ = os.Remove(dstChild)
					if err := os.Link(prevDst, dstChild); err == nil {
						continue
					}
				}
				linkMap[k] = dstChild
			}
		}

		if _, err := copyOneFile(ctx, srcChild, dstChild, leafInfo, preserve, false, displaySrcChild, displayDstChild, backupActive, suffix, control); err != nil {
			fmt.Fprintf(ctx.Stderr, "cp: %s\n", err)
			hasError = true
		}
	}

	if preserve.mode {
		os.Chmod(dst, srcInfo.Mode().Perm())
	}
	if preserve.timestamps {
		os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
	}
	if preserve.ownership {
		if st, ok := srcInfo.Sys().(*syscall.Stat_t); ok {
			os.Chown(dst, int(st.Uid), int(st.Gid))
		}
	}
	if hasError {
		return errors.New("directory copy error")
	}
	return nil
}
