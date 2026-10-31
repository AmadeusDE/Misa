// Behaviour adapted from uutils coreutils shred (MIT), src/uu/shred and
// tests/by-util/test_shred.rs at fee61ff5d4f87e0f912a52e639942b5f7ad58663;
// option defaults checked against Toybox toys/other/shred.c (0BSD,
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb), which differs (default -n 1, -o OFFSET).

package builtins

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// shredBlockSize is the size of a single write, matching GNU/uutils (2^16).
	shredBlockSize = 1 << 16
	// shredAlign is the block the byte count is rounded up to unless -x or -s.
	shredAlign = 1 << 12
	// shredDefaultPasses is the default for -n.
	shredDefaultPasses = 3
	// shredNameCharset is the alphabet the replacement names for --remove=wipe
	// are built from, in the order the reference enumerates them.
	shredNameCharset = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_."
)

const shredHelp = `Usage: shred [OPTION]... FILE...

Overwrite FILE(s) with random data, and optionally delete them, so that the
old contents are harder to recover even with expensive hardware probing.

  -n, --iterations=N   overwrite N times instead of the default (3); 0 skips
                       the overwriting entirely
  -s, --size=N         shred this many bytes instead of the file length
                       (suffixes K, M, G, ... are accepted, and 0x hex);
                       implies --exact
  -u                   like --remove=wipesync
      --remove[=HOW]   remove the file after overwriting; HOW is one of
                       unlink   just remove the name
                       wipe     also rename the file through shorter and
                                shorter names first
                       wipesync wipe, syncing after every rename
                       A bare --remove (or -u) means wipesync, and an
                       unambiguous abbreviation is accepted.
  -f, --force          change permissions to allow writing if necessary
  -v, --verbose        show progress on standard error
  -x, --exact          do not round the byte count up to the next full block
                       (4096 bytes)
  -z, --zero           add a final overwrite with zeros to hide shredding
      --random-source=FILE
                       take the random bytes from FILE instead of the system
                       random generator; it is read sequentially
  -h, --help           display this help and exit
  -V, --version        output version information and exit

Exit status:
  0  every FILE was overwritten (and removed, if asked) successfully
  1  something failed: a bad option, a missing file operand, a file that
     could not be opened or written, or a removal that did not work

NOTES
  "-" names standard output. Unlike coreutils, shred accepts it: if standard
  output is a regular file, that file is overwritten in place (the descriptor
  is seeked to 0 and written through, never truncated, and never unlinked or
  renamed). When standard output is a pipe, socket or terminal there is
  nothing to overwrite, so shred refuses with "standard output is not a
  regular file". --remove together with "-" is always an error, because shred
  will not remove the file behind standard output.

CAUTION: shred relies on one big assumption -- that the filesystem overwrites
data in place. That is the traditional way of doing things, but many modern
designs break it, so overwriting only helps on filesystems that really write
over the old blocks. shred does NOT guarantee erasure on:

  - log-structured or journaled filesystems (in particular any filesystem
    running with data journaling, such as ext3/ext4 data=journal), where the
    data may be written to the journal instead of over the old blocks;
  - copy-on-write (COW) filesystems such as btrfs, ZFS, APFS or bcachefs,
    which never modify a block in place and simply point at a new one;
  - SSDs, flash and other media with wear levelling and spare blocks, whose
    controller remaps writes and keeps the old data around;
  - filesystems with snapshots, which keep the previous contents reachable;
  - RAID with write caches or redundancy, where copies live on other disks;
  - compressed or deduplicating filesystems, which store shared or shortened
    representations of the data.

In addition, backups, remote mirrors, temporary caches (NFS clients, editor
swap files, ...) and bad-block relocation may hold copies of the data that
shred cannot reach. shred does not verify the result: it cannot tell you that
the data is gone. --remove only unlinks the name (after optionally renaming
it), which does not erase anything by itself.
`

// shredOptions is the shared configuration for every operand.
type shredOptions struct {
	iterations int64
	size       int64
	haveSize   bool
	exact      bool
	zero       bool
	verbose    bool
	force      bool
	remove     string // "", "unlink", "wipe" or "wipesync"
}

var shredSpecParsed *Spec

func init() {
	var err error
	// -u is paired with --remove: bare they both mean wipesync, and the value
	// is only accepted attached or after '=' (see shredNormalizeArgs).
	shredSpecParsed, err = ParseSpec("n(iterations)#s(size):u(remove):?f(force)v(verbose)x(exact)z(zero)(random-source):V")
	if err != nil {
		panic("builtins: bad shred spec: " + err.Error())
	}
	Register(Command{Name: "shred", Run: runShred})
}

func runShred(ctx *Context) int {
	flags, err := shredSpecParsed.ParseArgs(shredNormalizeArgs(ctx.Args))
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "shred: %v\nTry 'shred --help' for more information.\n", err)
		return 1
	}
	if flags.HelpRequested {
		fmt.Fprint(ctx.Stdout, shredHelp)
		return 0
	}
	if flags.VersionRequested || flags.Bool("V") {
		fmt.Fprint(ctx.Stdout, "shred (Misa) 0.1.0\n")
		return 0
	}

	opt := &shredOptions{iterations: shredDefaultPasses}
	if n, ok := flags.Num("n"); ok {
		if n < 0 {
			fmt.Fprintf(ctx.Stderr, "shred: invalid number of passes: '%d'\n", n)
			return 1
		}
		opt.iterations = n
	}
	if s, ok := flags.String("s"); ok {
		size, err := shredParseSize(s)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "shred: invalid file size: '%s'\n", s)
			return 1
		}
		opt.size, opt.haveSize = size, true
	}
	opt.exact = flags.Bool("x") || opt.haveSize
	opt.zero = flags.Bool("z")
	opt.verbose = flags.Bool("v")
	opt.force = flags.Bool("f")
	if how, ok := flags.String("remove"); ok {
		method, err := shredRemoveHow(how)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "shred: %v\nValid arguments are 'unlink', 'wipe' and 'wipesync'.\n", err)
			return 1
		}
		opt.remove = method
	}
	var randomSource *os.File
	if src, ok := flags.String("random-source"); ok {
		randomSource, err = os.Open(resolvePath(ctx, src))
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "shred: %s: %s\n", src, fmtOsErr(err))
			return 1
		}
		defer randomSource.Close()
	}

	if len(flags.Operands) == 0 {
		fmt.Fprint(ctx.Stderr, "shred: missing file operand\nTry 'shred --help' for more information.\n")
		return 1
	}

	status := 0
	for _, name := range flags.Operands {
		if !shredOne(ctx, name, opt, randomSource) {
			status = 1
		}
	}
	return status
}

// shredNormalizeArgs rewrites -uHOW into --remove=HOW, and -uXYZ (anything
// else attached to -u, also inside a group such as -zuv) into -u -XYZ. The
// option parser hands the rest of a short-option group to an optional-value
// flag, so without this the common "shred -uv file" would set the removal
// method to "v" instead of enabling verbose mode.
func shredNormalizeArgs(args []string) []string {
	out := make([]string, 0, len(args))
	operandsOnly := false
	for _, a := range args {
		if operandsOnly || a == "--" {
			operandsOnly = true
			out = append(out, a)
			continue
		}
		if len(a) > 2 && a[0] == '-' && a[1] != '-' {
			group := a[1:]
			if i := strings.IndexByte(group, 'u'); i >= 0 && i < len(group)-1 {
				prefix, rest := group[:i], group[i+1:]
				out = append(out, "-"+prefix+"u")
				if shredRemoveMethod(rest) {
					out = append(out, "--remove="+rest)
				} else {
					out = append(out, "-"+rest)
				}
				continue
			}
		}
		out = append(out, a)
	}
	return out
}

func shredRemoveMethod(how string) bool {
	return how == "unlink" || how == "wipe" || how == "wipesync"
}

// shredRemoveHow resolves the --remove/--remove=HOW argument. A bare option
// means wipesync, and like the reference an unambiguous abbreviation is
// accepted (--remove=u, --remove=wipes).
func shredRemoveHow(how string) (string, error) {
	if how == "" {
		return "wipesync", nil
	}
	if shredRemoveMethod(how) {
		return how, nil
	}
	var matches []string
	for _, method := range []string{"unlink", "wipe", "wipesync"} {
		if strings.HasPrefix(method, how) {
			matches = append(matches, method)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("invalid argument '%s' for '--remove'", how)
	default:
		return "", fmt.Errorf("ambiguous argument '%s' for '--remove' (%s)",
			how, strings.Join(matches, ", "))
	}
}

// shredParseSize parses the -s/--size argument: a plain decimal number, a
// 0x-prefixed hex number, or a decimal number with a K/M/G/... suffix.
func shredParseSize(s string) (int64, error) {
	t := strings.TrimSpace(s)
	if strings.HasPrefix(t, "-") {
		return 0, fmt.Errorf("negative size")
	}
	t = strings.TrimPrefix(t, "+")
	if t == "" {
		return 0, fmt.Errorf("empty size")
	}
	if len(t) > 2 && t[0] == '0' && (t[1] == 'x' || t[1] == 'X') {
		u, err := strconv.ParseUint(t[2:], 16, 64)
		if err != nil || u > math.MaxInt64 {
			return 0, fmt.Errorf("invalid hex size")
		}
		return int64(u), nil
	}
	numStr, mult := parseSizeSuffix(t)
	n, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid size")
	}
	if mult > 1 && n > math.MaxInt64/mult {
		return 0, fmt.Errorf("size too large")
	}
	return n * mult, nil
}

// shredOne overwrites a single operand. It reports its own diagnostics and
// returns false when the operand failed.
func shredOne(ctx *Context, name string, opt *shredOptions, randomSource *os.File) bool {
	var (
		file *os.File
		size int64
		path string
	)

	if name == "-" {
		if opt.remove != "" {
			fmt.Fprint(ctx.Stderr, "shred: -: cannot remove standard output\n")
			return false
		}
		f, ok := ctx.Stdout.(*os.File)
		if !ok {
			fmt.Fprint(ctx.Stderr, "shred: -: standard output is not a regular file\n")
			return false
		}
		st, err := f.Stat()
		if err != nil || !st.Mode().IsRegular() {
			fmt.Fprint(ctx.Stderr, "shred: -: standard output is not a regular file\n")
			return false
		}
		file, size = f, st.Size()
	} else {
		path = resolvePath(ctx, name)
		st, err := os.Stat(path)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "shred: %s: %s\n", name, fmtOsErr(err))
			return false
		}
		switch {
		case st.IsDir():
			fmt.Fprintf(ctx.Stderr, "shred: %s: Is a directory\n", name)
			return false
		case !st.Mode().IsRegular():
			fmt.Fprintf(ctx.Stderr, "shred: %s: Not a file\n", name)
			return false
		}
		size = st.Size()
		if opt.force && st.Mode().Perm()&0200 == 0 {
			if err := os.Chmod(path, st.Mode().Perm()|0200); err != nil {
				fmt.Fprintf(ctx.Stderr, "shred: %s: %s\n", name, fmtOsErr(err))
				return false
			}
		}
		// Open for writing without truncation and write through this handle, so
		// hard links see the new data and the inode keeps its identity.
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "shred: %s: %s\n", name, fmtOsErr(err))
			return false
		}
		defer f.Close()
		file = f
	}

	count := size
	if opt.haveSize {
		count = opt.size
	}
	if !opt.exact {
		count = (count + shredAlign - 1) / shredAlign * shredAlign
	}

	// A zero length means nothing to overwrite, and then no pass is reported
	// either (as in the reference).
	total := int64(0)
	if count > 0 {
		total = opt.iterations
		if opt.zero {
			total++
		}
	}
	for i := int64(1); i <= total; i++ {
		passName := "random"
		zeros := opt.zero && i == total
		if zeros {
			passName = "000000"
		}
		if opt.verbose {
			fmt.Fprintf(ctx.Stderr, "shred: %s: pass %d/%d (%s)...\n", name, i, total, passName)
		}
		if err := shredPass(file, count, randomSource, zeros); err != nil {
			fmt.Fprintf(ctx.Stderr, "shred: %s: %s\n", name, fmtOsErr(err))
			return false
		}
	}

	if opt.remove != "" {
		if err := shredRemove(ctx, path, name, opt, shredDisplayDir(name)); err != nil {
			fmt.Fprintf(ctx.Stderr, "shred: %s: %v\n", name, err)
			return false
		}
	}
	return true
}

// shredPass writes count bytes to f, starting at offset 0: random bytes (from
// randomSource, or from the system generator when it is nil) or zeros. It
// syncs the file at the end of the pass.
func shredPass(f *os.File, count int64, randomSource *os.File, zeros bool) error {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	buf := make([]byte, shredBlockSize) // already zeroed, reused for -z
	for count > 0 {
		n := int64(len(buf))
		if n > count {
			n = count
		}
		chunk := buf[:n]
		if !zeros {
			var err error
			if randomSource != nil {
				_, err = io.ReadFull(randomSource, chunk)
			} else {
				_, err = rand.Read(chunk)
			}
			if err != nil {
				return err
			}
		}
		if _, err := f.Write(chunk); err != nil {
			return err
		}
		count -= n
	}
	return f.Sync()
}

// shredRemove removes the file after overwriting it, optionally renaming it
// through shorter and shorter names first.
func shredRemove(ctx *Context, path, name string, opt *shredOptions, displayDir string) error {
	if opt.verbose {
		fmt.Fprintf(ctx.Stderr, "shred: %s: removing\n", name)
	}
	target := path
	if opt.remove != "unlink" {
		var err error
		target, err = shredWipeName(ctx, path, opt, displayDir)
		if err != nil {
			return err
		}
	}
	if err := os.Remove(target); err != nil {
		return errors.New(fmtOsErr(err))
	}
	if opt.verbose {
		fmt.Fprintf(ctx.Stderr, "shred: %s: removed\n", name)
	}
	return nil
}

// shredWipeName renames path to shorter and shorter names taken from
// shredNameCharset, skipping names that already exist, and returns the last
// name used.
func shredWipeName(ctx *Context, path string, opt *shredOptions, displayDir string) (string, error) {
	dir, base := filepath.Split(path)
	cur := path
	for length := len(base); length >= 1; length-- {
		next := ""
		var renameErr error
		shredEachName(length, func(newName string) bool {
			candidate := dir + newName
			// Never reuse a name that is taken (a dangling symlink counts too).
			if _, err := os.Lstat(candidate); err == nil {
				return true
			}
			if err := os.Rename(cur, candidate); err != nil {
				renameErr = fmt.Errorf("could not rename to %s: %s", candidate, fmtOsErr(err))
				return false
			}
			next = candidate
			return false
		})
		if renameErr != nil {
			return cur, renameErr
		}
		if next == "" {
			continue // every name of this length is taken
		}
		if opt.verbose {
			fmt.Fprintf(ctx.Stderr, "shred: %s: renamed to %s\n", displayDir+filepath.Base(cur), displayDir+filepath.Base(next))
		}
		if opt.remove == "wipesync" {
			if err := shredSyncName(next); err != nil {
				return next, err
			}
		}
		cur = next
	}
	return cur, nil
}

// shredDisplayDir is the directory prefix of the operand as it was typed, so
// the rename reports of --remove=wipe name paths the way the user wrote them
// instead of the resolved ones.
func shredDisplayDir(name string) string {
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		return name[:i+1]
	}
	return ""
}

// shredSyncName opens a renamed file and syncs it, so the rename survives a
// crash (--remove=wipesync).
func shredSyncName(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("could not sync %s: %s", path, fmtOsErr(err))
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		return fmt.Errorf("could not sync %s: %s", path, fmtOsErr(err))
	}
	return nil
}

// shredEachName calls fn with every name of exactly length characters over
// shredNameCharset, in the order the reference enumerates them. It stops as
// soon as fn returns false.
func shredEachName(length int, fn func(string) bool) {
	digits := make([]int, length)
	buf := make([]byte, length)
	for {
		for i, d := range digits {
			buf[i] = shredNameCharset[d]
		}
		if !fn(string(buf)) {
			return
		}
		i := length - 1
		for ; i >= 0; i-- {
			if digits[i] < len(shredNameCharset)-1 {
				digits[i]++
				break
			}
			digits[i] = 0
		}
		if i < 0 {
			return // all names of this length were tried
		}
	}
}
