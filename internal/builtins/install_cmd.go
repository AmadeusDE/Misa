// install_cmd.go is the install builtin: it copies files and sets their attributes.
//
// Behaviour follows toybox install (0BSD, toys/posix/cp.c at
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb) and uutils coreutils install (MIT,
// src/uu/install at fee61ff5d4f87e0f912a52e639942b5f7ad58663).
//
// This is not `misa --install`, which symlinks the multicall binary: the two
// share nothing but the name.
//
// Deliberately unsupported: -s/--strip is rejected instead of running an
// external strip program, and --strip-program, --backup/--suffix,
// --preserve-context/--context/-Z and -U/--unprivileged (SELinux, backups and
// privilege dropping are decisions this builtin does not make) are rejected as
// unknown options, so they fail instead of silently doing nothing.
//
// Differences from both references, chosen here: a regular destination is
// replaced atomically through a temporary file and rename (uutils truncates in
// place after unlinking), an existing non-regular destination (device, FIFO) is
// written into and keeps its own mode, and a -d operand whose last component is
// "." or ".." is created but not re-moded, so `install -d ..` cannot change the
// mode of the directory you are in.
package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const installSpec = "c(ignored)C(compare)d(directory)D(mkdir)g(group):m(mode):" +
	"o(owner):p(preserve-timestamps)s(strip)t(target-directory):T(no-target-directory)v(verbose)"

var installSpecParsed *Spec

func init() {
	var err error
	installSpecParsed, err = ParseSpec(installSpec)
	if err != nil {
		panic("builtins: bad install spec: " + err.Error())
	}
	Register(Command{Name: "install", Run: runInstall})
}

const installUsage = `Usage: install [OPTION]... SOURCE... DEST
       install -d [OPTION]... DIRECTORY...

Copy files and set their attributes. A destination is replaced atomically: the
copy is written to a temporary file in the destination's directory and renamed
over it, so a failed copy keeps the old destination and leaves no temporary
file behind. Symlink sources are followed; device and FIFO destinations are
written into directly rather than replaced.

  -c, --ignored                accepted and ignored (compatibility)
  -C, --compare                skip a copy whose destination already matches
  -d, --directory              create DIRECTORY... instead of copying
  -D, --mkdir                  create DEST's leading directories
  -g, --group=GROUP            set the installed file's group
  -m, --mode=MODE              octal or chmod-style symbolic mode (default 0755)
  -o, --owner=OWNER            set the installed file's owner
  -p, --preserve-timestamps    keep the source's access and modification times
  -s, --strip                  not supported: rejected, exit 1
  -t, --target-directory=DIR   copy every SOURCE into DIR (needs no DEST)
  -T, --no-target-directory    DEST always names the file, never a directory
  -v, --verbose                print each action
      --help                   display this help and exit
      --version                output version information and exit

An explicit -m is not limited by the umask; directories -d/-D create for a
missing parent get the default mode instead. -o/-g need the privilege to change
ownership and fail loudly without it. Misa never invokes an external strip
program: use -s nowhere, strip the binary yourself.
`

// installOptions carries the parsed request.
type installOptions struct {
	mode     os.FileMode // raw 0o7777 bits, applied exactly
	uid, gid int         // -1 leaves that field alone
	verbose  bool
	preserve bool // -p
	compare  bool // -C
}

func runInstall(ctx *Context) int {
	flags, err := installSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "install:", err)
		return 1
	}
	if flags.HelpRequested {
		fmt.Fprint(ctx.Stdout, installUsage)
		return 0
	}
	if flags.VersionRequested {
		fmt.Fprintf(ctx.Stdout, "install (Misa) 0.1.0\n")
		return 0
	}
	if flags.Bool("s") {
		fmt.Fprintln(ctx.Stderr, "install: stripping (-s) is not supported: Misa never runs an external strip program")
		return 1
	}

	targetDir, haveTargetDir := flags.String("t")
	noTargetDir := flags.Bool("T")
	if haveTargetDir && noTargetDir {
		fmt.Fprintln(ctx.Stderr, "install: --target-directory and --no-target-directory are mutually exclusive")
		return 1
	}

	opts := installOptions{uid: -1, gid: -1, mode: 0755,
		verbose:  flags.Bool("v"),
		preserve: flags.Bool("p"),
		compare:  flags.Bool("C"),
	}
	directory := flags.Bool("d")
	if mode, ok := flags.String("m"); ok {
		parsed, err := parseInstallMode(mode, directory)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "install: %v\n", err)
			return 1
		}
		opts.mode = parsed
	}
	if group, ok := flags.String("g"); ok {
		gid, err := parseGroupSpec(group)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "install: %v\n", err)
			return 1
		}
		opts.gid = gid
	}
	if owner, ok := flags.String("o"); ok {
		uid, err := parseUserSpec(owner)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "install: %v\n", err)
			return 1
		}
		opts.uid = uid
	}

	ops := flags.Operands
	if directory {
		if len(ops) == 0 {
			fmt.Fprintln(ctx.Stderr, "install: missing file operand")
			return 1
		}
		return installDirectories(ctx, ops, opts)
	}

	var sources []string
	var dest string
	if haveTargetDir {
		sources, dest = ops, targetDir
	} else {
		if len(ops) == 0 {
			fmt.Fprintln(ctx.Stderr, "install: missing file operand")
			return 1
		}
		if len(ops) == 1 {
			fmt.Fprintf(ctx.Stderr, "install: missing destination file operand after '%s'\n", ops[0])
			return 1
		}
		sources, dest = ops[:len(ops)-1], ops[len(ops)-1]
	}
	if noTargetDir && len(ops) > 2 {
		fmt.Fprintf(ctx.Stderr, "install: extra operand '%s'\n", ops[2])
		return 1
	}
	if len(sources) == 0 {
		fmt.Fprintln(ctx.Stderr, "install: missing file operand")
		return 1
	}

	destPath := resolvePath(ctx, dest)
	intoDir := haveTargetDir || len(sources) > 1 || strings.HasSuffix(dest, "/")
	if !noTargetDir {
		intoDir = intoDir || isDirPath(destPath)
	}

	if flags.Bool("D") {
		if code := installCreateLeading(ctx, dest, destPath, haveTargetDir, intoDir, opts); code != 0 {
			return code
		}
	}

	if intoDir {
		return installIntoDir(ctx, sources, dest, destPath, opts, haveTargetDir)
	}
	copied, err := installCopyFile(sources[0], resolvePath(ctx, sources[0]), dest, destPath, opts)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "install: %v\n", err)
		return 1
	}
	installReport(ctx, opts, copied, sources[0], dest)
	return 0
}

// installDirectories is the -d path: every operand becomes a directory, with
// missing parents created the way mkdir -p would.
func installDirectories(ctx *Context, names []string, opts installOptions) int {
	code := 0
	for _, name := range names {
		path := resolvePath(ctx, name)
		created, err := installMakeDirTree(path, name, opts)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "install: %v\n", err)
			code = 1
			continue
		}
		if created && opts.verbose {
			fmt.Fprintf(ctx.Stdout, "install: creating directory '%s'\n", installDirDisplay(name))
		}
	}
	return code
}

// installCreateLeading is the -D path. With -t the target directory itself is
// created; otherwise only a single file destination's leading directories are,
// because a missing directory target for several sources is an error rather
// than something to invent.
func installCreateLeading(ctx *Context, dest, destPath string, haveTargetDir bool, intoDir bool, opts installOptions) int {
	if haveTargetDir {
		if err := installLeadingTargetCheck(dest, destPath); err != nil {
			fmt.Fprintf(ctx.Stderr, "install: %v\n", err)
			return 1
		}
	}
	if intoDir && !haveTargetDir {
		return 0
	}
	path, display := filepath.Dir(destPath), filepath.Dir(dest)
	if haveTargetDir {
		path, display = destPath, dest
	}
	created, err := installMakeDirTree(path, display, installOptions{mode: 0755, uid: -1, gid: -1})
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "install: %v\n", err)
		return 1
	}
	if created && opts.verbose {
		fmt.Fprintf(ctx.Stdout, "install: creating directory '%s'\n", installDirDisplay(display))
	}
	return 0
}

// installIntoDir copies every source into the destination directory.
func installIntoDir(ctx *Context, sources []string, dest, destPath string, opts installOptions, haveTargetDir bool) int {
	if err := installCheckTargetDir(dest, destPath, haveTargetDir, len(sources) > 1); err != nil {
		fmt.Fprintf(ctx.Stderr, "install: %v\n", err)
		return 1
	}
	code := 0
	for _, source := range sources {
		name := installBase(source)
		dstPath := filepath.Join(destPath, name)
		dstDisplay := installDisplayJoin(dest, name)
		copied, err := installCopyFile(source, resolvePath(ctx, source), dstDisplay, dstPath, opts)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "install: %v\n", err)
			code = 1
			continue
		}
		installReport(ctx, opts, copied, source, dstDisplay)
	}
	return code
}

// installLeadingTargetCheck refuses a -D target that exists but cannot become a
// directory. A missing one is exactly what -D is for.
func installLeadingTargetCheck(dest, destPath string) error {
	info, err := os.Stat(destPath)
	switch {
	case err == nil && info.IsDir():
		return nil
	case err == nil:
		return fmt.Errorf("failed to access '%s': Not a directory", dest)
	case !os.IsNotExist(err):
		return fmt.Errorf("failed to access '%s': %s", dest, fmtOsErr(err))
	}
	if _, lerr := os.Lstat(destPath); lerr == nil {
		// A dangling symlink: the name exists but does not lead anywhere.
		return fmt.Errorf("failed to access '%s': No such file or directory", dest)
	}
	return nil
}

// installCheckTargetDir reports why a destination directory is unusable. The
// wording names the destination the way the user did: a -t target is "failed to
// access", another directory destination for several sources is "target", and a
// single file destination that cannot be created says so instead of inventing a
// directory.
func installCheckTargetDir(dest, destPath string, fromFlagT, multi bool) error {
	info, err := os.Stat(destPath)
	if err == nil && info.IsDir() {
		return nil
	}
	reason := "No such file or directory"
	switch {
	case info != nil:
		reason = "Not a directory"
	case err != nil && !os.IsNotExist(err):
		reason = fmtOsErr(err)
	}
	switch {
	case fromFlagT:
		return fmt.Errorf("failed to access '%s': %s", dest, reason)
	case multi:
		// uutils lowercases the errno text in this one message and its
		// install case asserts the lowercase form, so keep that wording.
		return fmt.Errorf("target '%s': %s", dest, strings.ToLower(reason[:1])+reason[1:])
	default:
		return fmt.Errorf("cannot create regular file '%s': %s", dest, reason)
	}
}

// installReport prints the -v line for one file that was actually installed.
// -C skips print nothing.
func installReport(ctx *Context, opts installOptions, copied bool, from, to string) {
	if opts.verbose && copied {
		fmt.Fprintf(ctx.Stdout, "'%s' -> '%s'\n", from, to)
	}
}

// parseInstallMode parses -m into raw 0o7777 permission bits. isDir selects
// directory semantics for the symbolic "X" clause and for a mode-less who.
func parseInstallMode(spec string, isDir bool) (os.FileMode, error) {
	if spec == "" {
		return 0, fmt.Errorf("invalid mode %q", spec)
	}
	if installOctalMode(spec) {
		return parseOctalMode(spec)
	}
	base := os.FileMode(0)
	if isDir {
		base = os.ModeDir
	}
	mode, err := parseSymbolicModeMasked(spec, base, func() (os.FileMode, error) {
		return 0, nil // an explicit -m ignores the umask
	})
	if err != nil {
		return 0, err
	}
	return installModeBits(mode), nil
}

// installModeBits converts a parsed os.FileMode into raw 0o7777 bits.
func installModeBits(mode os.FileMode) os.FileMode {
	bits := mode.Perm()
	if mode&os.ModeSetuid != 0 {
		bits |= 04000
	}
	if mode&os.ModeSetgid != 0 {
		bits |= 02000
	}
	if mode&os.ModeSticky != 0 {
		bits |= 01000
	}
	return bits
}

// installFileMode is the reverse: raw 0o7777 bits as os.Chmod understands them.
func installFileMode(bits os.FileMode) os.FileMode {
	mode := bits.Perm()
	if bits&04000 != 0 {
		mode |= os.ModeSetuid
	}
	if bits&02000 != 0 {
		mode |= os.ModeSetgid
	}
	if bits&01000 != 0 {
		mode |= os.ModeSticky
	}
	return mode
}

func installOctalMode(spec string) bool {
	for i := 0; i < len(spec); i++ {
		if spec[i] < '0' || spec[i] > '7' {
			return false
		}
	}
	return true
}
