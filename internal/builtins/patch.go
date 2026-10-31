// Behaviour modelled on toybox toys/posix/patch.c,
// landley/toybox@6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb (0BSD).
// Native Go implementation; see docs/CREDITS.md and THIRD_PARTY_NOTICES.md.
//
// patch applies unified diffs. Scope, policies and intentional differences:
//
// Supported: -p N, -i FILE, -R, -s/--quiet, -u (unified is the only format),
// -d DIR, -l/--ignore-whitespace, -F N/--fuzz N, -v/--verbose, -x N/--debug N,
// -E/--remove-empty-files, --dry-run, --no-backup-if-mismatch (accepted: Misa
// never writes backups), --help, --version. Operands are [FILE [PATCHFILE]].
//
// Not supported, and refused instead of silently ignored: -b/--backup, -N
// (ignore already applied), -D define, -o outfile, -c context diffs, -e ed
// diffs, -g. Context, ed, normal and binary patches are rejected with a clear
// message; only unified hunks are understood.
//
// Exit status is 0 only when every hunk of every file applied, and 1 for
// usage errors, I/O errors, unsafe paths, malformed patches and rejected
// hunks (toybox's status).
//
// Policies:
//   - No -p strips every leading directory (toybox's default), so a/b/c.diff
//     patches ./c.diff; -p N strips exactly N components. Absolute paths and
//     ".." components left after stripping are refused, and nothing is
//     written for that file.
//   - A file is only rewritten when all of its hunks apply: the new content
//     goes to a temp file in the same directory and is renamed over the
//     (symlink-resolved) target, so a failure never leaves a partial file.
//   - Hunks may apply at an offset (the search moves forward only, like
//     toybox). Fuzz drops at most the leading/trailing context lines the -F
//     value allows, and needs at least two substantial (>= 2 bytes trimmed)
//     matched lines; the default allowance is leading context lines - 1.
//   - Rejected hunks go to <target>.rej (a valid unified patch) and the target
//     is left untouched; a stale .rej is removed when the patch fully applies.
//     -R reverses names, ranges and +/- lines before applying.
//   - Creating (/dev/null old side) over an existing file is refused rather
//     than overwritten; deletion (+++ /dev/null) unlinks the file only when the
//     hunks leave it empty, otherwise the target is left unchanged.
//   - Line matching also compares the "no newline at end of file" state, so a
//     patch produced by misa diff applies byte-exactly.
//
// Known differences from toybox patch: rejects are written to .rej files
// instead of being dumped on stderr mid-stream; a whole patch is parsed before
// anything is written, so a malformed patch changes nothing; -p with fewer
// components than requested strips what is there instead of skipping the file;
// a "declared deleted but not emptied" file is an error instead of an unlink.
package builtins

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// patchSpec is the option surface. Two spellings differ from the U36 spec
// literal so the long forms work: a value-taking short flag must be written
// "c(long)#", so -F/--fuzz and -x/--debug are grouped that way (the spec's
// "F#(fuzz)" would make --fuzz a separate boolean). -V is appended for the
// documented "--version/-V" pair.
const patchSpec = "F(fuzz)#v(verbose)E(remove-empty-files)ul(ignore-whitespace)p#d:i:Rs(quiet)x(debug)#(dry-run)(no-backup-if-mismatch)V"

var patchSpecParsed *Spec

func init() {
	var err error
	patchSpecParsed, err = ParseSpec(patchSpec)
	if err != nil {
		panic("builtins: bad patch spec: " + err.Error())
	}
	Register(Command{Name: "patch", Run: runPatch})
}

// What a patch file section does to its target.
const (
	patchKindPatch = iota
	patchKindCreate
	patchKindDelete
)

// patchOptions carries the parsed command line plus the output streams the
// apply engine uses (so its signature stays data-only).
type patchOptions struct {
	stripAll    bool // no -p: drop every leading directory
	strip       int  // -p N
	reverse     bool // -R
	quiet       bool // -s
	loose       bool // -l
	fuzzSet     bool // -F given
	fuzz        int
	verbose     int // -v count
	debug       int // max(-v count, -x N)
	dryRun      bool
	removeEmpty bool // -E

	stdout io.Writer
	stderr io.Writer

	dosNotice bool // "Removing DOS newlines" already printed
}

func runPatch(ctx *Context) int {
	flags, err := patchSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "patch: %v\nTry 'patch --help' for more information.\n", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, patchSpecParsed) {
		return 0
	}
	if flags.Bool("V") {
		fmt.Fprintf(ctx.Stdout, "%s (Misa) 0.1.0\n", ctx.CmdName)
		return 0
	}

	verbose := 0
	for _, k := range flags.Order() {
		if k == "v" {
			verbose++
		}
	}
	quiet := flags.Bool("s")
	if quiet && verbose > 0 {
		fmt.Fprintln(ctx.Stderr, "patch: -s and -v are mutually exclusive")
		return 1
	}

	opts := &patchOptions{
		reverse:     flags.Bool("R"),
		quiet:       quiet,
		loose:       flags.Bool("l"),
		verbose:     verbose,
		debug:       verbose,
		dryRun:      flags.Bool("dry-run"),
		removeEmpty: flags.Bool("E"),
		stdout:      ctx.Stdout,
		stderr:      ctx.Stderr,
	}
	if n, ok := flags.Num("x"); ok && int(n) > opts.debug {
		opts.debug = int(n)
	}
	if n, ok := flags.Num("F"); ok {
		opts.fuzzSet = true
		if n < 0 {
			n = 0
		}
		opts.fuzz = int(n)
	}
	if n, ok := flags.Num("p"); ok {
		if n < 0 {
			n = 0
		}
		opts.strip = int(n)
	} else {
		opts.stripAll = true
	}

	operands := flags.Operands
	if len(operands) > 2 {
		fmt.Fprintf(ctx.Stderr, "patch: extra operand '%s'\nTry 'patch --help' for more information.\n", operands[2])
		return 1
	}
	override := ""
	if len(operands) > 0 {
		override = operands[0]
	}
	patchName := ""
	if len(operands) > 1 {
		patchName = operands[1]
	} else if s, ok := flags.String("i"); ok {
		patchName = s
	}

	base := ctx.Dir
	if d, ok := flags.String("d"); ok && d != "" {
		base = d
		if !filepath.IsAbs(base) {
			base = filepath.Join(ctx.Dir, d)
		}
		if fi, serr := os.Stat(base); serr != nil || !fi.IsDir() {
			msg := "Not a directory"
			if serr != nil {
				msg = fmtOsErr(serr)
			}
			fmt.Fprintf(ctx.Stderr, "patch: %s: %s\n", d, msg)
			return 1
		}
	}

	// Toybox reads the patch before it changes directory (-d), so the patch
	// file name is resolved against the caller's directory.
	if patchName == "" || patchName == "-" {
		if ctx.Stdin == nil {
			fmt.Fprintln(ctx.Stderr, "patch: stdin not available")
			return 1
		}
	}
	data, err := patchReadInput(ctx, ctx.Dir, patchName)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "patch: %s: %s\n", patchName, fmtOsErr(err))
		return 1
	}

	files, err := parsePatch(data, opts, patchName)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "patch: %v\n", err)
		return 1
	}

	status := 0
	for i := range files {
		if !patchApplyFile(opts, base, &files[i], override) {
			status = 1
		}
	}
	return status
}

// patchReadInput reads the patch text from -i, the second operand or stdin.
// runPatch has already refused a missing stdin.
func patchReadInput(ctx *Context, base, name string) ([]byte, error) {
	if name == "" || name == "-" {
		return io.ReadAll(ctx.Stdin)
	}
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	return os.ReadFile(path)
}

// patchApplyFile resolves one patch section's target and applies it. It
// reports whether everything went through; failures never touch the target.
func patchApplyFile(opts *patchOptions, base string, pf *patchFile, override string) bool {
	name, kind, safe := patchTargetName(pf, override, opts)
	if !safe {
		fmt.Fprintf(opts.stderr, "patch: %s: refusing unsafe path (absolute or '..' after -p stripping)\n", name)
		return false
	}
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}

	fi, err := os.Stat(path)
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(opts.stderr, "patch: %s: %s\n", name, fmtOsErr(err))
		return false
	}

	var fromFile bool
	switch kind {
	case patchKindCreate:
		if exists {
			fmt.Fprintf(opts.stderr, "patch: %s: file already exists\n", name)
			return false
		}
		if !opts.quiet {
			fmt.Fprintf(opts.stdout, "creating %s\n", name)
		}
	case patchKindDelete:
		if !opts.quiet {
			fmt.Fprintf(opts.stdout, "removing %s\n", name)
		}
		fromFile = true
	default:
		if !opts.quiet {
			fmt.Fprintf(opts.stdout, "patching %s\n", name)
		}
		fromFile = true
	}
	if kind != patchKindCreate && !exists {
		fmt.Fprintf(opts.stderr, "patch: %s: No such file or directory\n", name)
		return false
	}

	var content []byte
	if fromFile {
		content, err = os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(opts.stderr, "patch: %s: %s\n", name, fmtOsErr(err))
			return false
		}
	}

	out, failed, unapplied := applyPatchHunks(patchSplitLines(content), pf.hunks, *opts)
	if len(failed) > 0 {
		for _, hi := range failed {
			h := &pf.hunks[hi]
			fmt.Fprintf(opts.stderr, "Hunk %d FAILED %d/%d.\n", hi+1, h.oldStart, h.newStart)
		}
		rejPath := path + ".rej"
		if !opts.dryRun {
			mode := os.FileMode(0644)
			if rfi, rerr := os.Stat(rejPath); rerr == nil {
				mode = rfi.Mode().Perm()
			}
			if werr := patchWriteFileAtomic(rejPath, patchRejectText(pf, unapplied), mode); werr != nil {
				fmt.Fprintf(opts.stderr, "patch: %s: %s\n", rejPath, fmtOsErr(werr))
			}
		}
		fmt.Fprintf(opts.stderr, "patch: %s: %d hunk(s) rejected, %s\n", name, len(unapplied), patchRejectNote(opts, rejPath))
		return false
	}

	// Everything applied: a stale reject file no longer describes this target.
	if !opts.dryRun {
		if _, serr := os.Lstat(path + ".rej"); serr == nil {
			os.Remove(path + ".rej")
		}
	}

	result := patchJoinLines(out)
	if kind == patchKindDelete {
		if len(result) != 0 {
			// The header says the file goes away, but the hunks did not empty
			// it. Refuse rather than leave a half-deleted target behind.
			fmt.Fprintf(opts.stderr, "patch: %s: patch declared the file deleted but left it non-empty; target unchanged\n", name)
			return false
		}
		return patchRemoveTarget(opts, path, name)
	}
	if opts.removeEmpty && len(result) == 0 {
		return patchRemoveTarget(opts, path, name)
	}
	if !opts.dryRun {
		if werr := patchWriteResult(path, result, exists, fi); werr != nil {
			fmt.Fprintf(opts.stderr, "patch: %s: %s\n", name, fmtOsErr(werr))
			return false
		}
	}
	return true
}

// patchRemoveTarget unlinks a target the patch emptied or deleted.
func patchRemoveTarget(opts *patchOptions, path, name string) bool {
	if opts.dryRun {
		return true
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(opts.stderr, "patch: %s: %s\n", name, fmtOsErr(err))
		return false
	}
	return true
}

// patchWriteResult writes a patched file, keeping the old mode for existing
// files and 0666&^umask for new ones.
func patchWriteResult(path string, data []byte, exists bool, fi os.FileInfo) error {
	mode := os.FileMode(0666) &^ os.FileMode(currentUmask())
	if exists && fi != nil {
		mode = fi.Mode().Perm()
	}
	return patchWriteFileAtomic(path, data, mode)
}

// patchRejectNote says where the rejects went without claiming a write that
// --dry-run never makes.
func patchRejectNote(opts *patchOptions, rejPath string) string {
	if opts.dryRun {
		return "not written (--dry-run): " + rejPath
	}
	return "written to " + rejPath
}

// patchTargetName picks the path a patch section talks about and classifies
// the action. Reverse patches are already normalised, so this is forward-only.
func patchTargetName(pf *patchFile, override string, opts *patchOptions) (string, int, bool) {
	name, kind := pf.newName, patchKindPatch
	switch {
	case pf.newName == "/dev/null":
		name, kind = pf.oldName, patchKindDelete
	case pf.oldName == "/dev/null":
		kind = patchKindCreate
	}
	if override != "" {
		// A FILE operand is typed by the caller and used literally, so it is
		// never stripped and may be absolute; only an empty name is refused.
		return override, kind, override != ""
	}
	name = patchStripName(name, opts)
	return name, kind, !patchUnsafePath(name)
}

// patchStripName drops leading path components: -p N exactly N, otherwise
// every one of them (the toybox default).
func patchStripName(name string, opts *patchOptions) string {
	if name == "/dev/null" {
		return name
	}
	stripped := name
	i, pos := 0, 0
	for pos < len(name) {
		if !opts.stripAll && i == opts.strip {
			break
		}
		if name[pos] != '/' {
			pos++
			continue
		}
		pos++
		for pos < len(name) && name[pos] == '/' {
			pos++
		}
		stripped = name[pos:]
		i++
	}
	return stripped
}

// patchUnsafePath rejects anything that could escape the working directory.
func patchUnsafePath(name string) bool {
	if name == "" || name == "." {
		return true
	}
	if strings.HasPrefix(name, "/") {
		return true
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

// patchRejectText renders the .rej file: the file header lines plus every hunk
// that was not applied, verbatim, so the reject file is itself a valid patch.
func patchRejectText(pf *patchFile, unapplied []int) []byte {
	var b strings.Builder
	oldHeader, newHeader := pf.oldHeader, pf.newHeader
	if oldHeader == "" {
		oldHeader = "--- " + pf.oldName
	}
	if newHeader == "" {
		newHeader = "+++ " + pf.newName
	}
	b.WriteString(oldHeader)
	b.WriteByte('\n')
	b.WriteString(newHeader)
	b.WriteByte('\n')
	for _, hi := range unapplied {
		h := &pf.hunks[hi]
		if len(h.raw) == 0 {
			continue
		}
		b.WriteString(h.raw[0])
		b.WriteByte('\n')
		for i, hl := range h.lines {
			if 1+i < len(h.raw) {
				b.WriteString(h.raw[1+i])
				b.WriteByte('\n')
			} else {
				b.WriteByte(hl.kind)
				b.WriteString(hl.text)
				b.WriteByte('\n')
			}
			if hl.noNL {
				b.WriteString("\\ No newline at end of file\n")
			}
		}
	}
	return []byte(b.String())
}

// patchWriteFileAtomic replaces path with data through a temp file in the same
// directory, so a failure never leaves a half-written target behind.
func patchWriteFileAtomic(path string, data []byte, mode os.FileMode) error {
	resolved := path
	if fi, err := os.Lstat(path); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		if real, rerr := filepath.EvalSymlinks(path); rerr == nil {
			resolved = real
		}
	}
	dir := filepath.Dir(resolved)
	if _, err := os.Stat(dir); err != nil {
		if merr := os.MkdirAll(dir, 0777); merr != nil {
			return merr
		}
	}

	tmp, err := os.CreateTemp(dir, ".patch-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	fail := func(err error) error {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return fail(err)
	}
	if err := tmp.Sync(); err != nil {
		return fail(err)
	}
	if err := tmp.Chmod(mode); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, resolved); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
