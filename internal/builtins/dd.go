// dd copies blocks of data from an input to an output, reporting how many
// records and bytes it handled.
//
// Behaviour adapted from toybox dd (toys/posix/dd.c, 0BSD) and uutils coreutils
// dd (src/uu/dd/src/{dd,parseargs,progress,numbers}.rs, MIT), both at the
// commits pinned in docs/CREDITS.md: the copy loop, skip=/seek= handling and
// short reads follow toybox while the size suffixes, the "bytes copied" line
// and the status= levels follow uutils.
package builtins

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

func init() { Register(Command{Name: "dd", Run: runDD}) }

// errInvalidSeek is how dd reports an output that cannot be seeked.
var errInvalidSeek = errors.New("Invalid argument")

// ddElapsed is the clock the summary uses; tests replace it.
var ddElapsed = func(start time.Time) time.Duration { return time.Since(start) }

const ddUsage = `usage: dd [OPERAND]...

Copy blocks of data from an input to an output.

  if=FILE   read FILE instead of standard input
  of=FILE   write FILE instead of standard output
  bs=N      read and write blocks of N bytes (overrides ibs= and obs=)
  ibs=N     input block size (default 512)
  obs=N     output block size (default 512)
  count=N   copy N input blocks
  skip=N    skip N input blocks before copying
  seek=N    skip N output blocks before writing

N takes the suffixes c (1 byte), w (2), b (512), k/K (1024), M, G, T, P, E,
kB/MB/GB/TB/PB/EB (powers of 1000), KiB/MiB/GiB/TiB/PiB/EiB (powers of 1024),
and x products such as 2x512. A 0x prefix is hexadecimal. A count=/skip=/seek=
value with a B suffix counts bytes instead of blocks. iseek= and oseek= are
aliases of skip= and seek=.

  conv=notrunc|sync|noerror|fsync|fdatasync|excl|nocreat
  iflag=count_bytes|skip_bytes|fullblock
  oflag=seek_bytes|append
  status=none|noxfer

fdatasync is served by fsync. Not supported in this version: conv=ascii,
ebcdic, ibm, lcase, ucase, block, unblock, swab, sparse, cbs=, the platform
flags of iflag=/oflag= (direct, sync, nocache, …) and status=progress.

Records and the bytes copied are reported on standard error.

  --help     display this help and exit
  --version  output version information and exit
`

// ddCmdName is the name dd calls itself, set when the multicall binary runs it
// as a symlink or a subcommand.
func ddCmdName(ctx *Context) string {
	if ctx.CmdName == "" {
		return "dd"
	}
	return ctx.CmdName
}

func runDD(ctx *Context) int {
	for _, arg := range ctx.Args {
		switch arg {
		case "--help":
			fmt.Fprint(ctx.Stdout, ddUsage)
			return 0
		case "--version":
			fmt.Fprintf(ctx.Stdout, "%s (Misa) 0.1.0\n", ddCmdName(ctx))
			return 0
		}
	}

	cfg, err := parseDDArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "dd: %v\n", err)
		var usageErr *ddUsageError
		if errors.As(err, &usageErr) {
			fmt.Fprintf(ctx.Stderr, "Try '%s --help' for more information.\n", ddCmdName(ctx))
		}
		return 1
	}

	in, inFile, inName, inClose, ok := ddInput(ctx, cfg)
	if !ok {
		return 1
	}
	if inClose != nil {
		defer inClose.Close()
	}
	out, outFile, outName, outClose, ok := ddOutput(ctx, cfg)
	if !ok {
		return 1
	}
	if outClose != nil {
		defer outClose.Close()
	}

	start := time.Now()

	if cfg.skip > 0 && !ddSkip(in, inFile, cfg.skip, cfg.ibs) {
		fmt.Fprintf(ctx.Stderr, "dd: '%s': cannot skip to specified offset\n", inName)
	}
	if cfg.seek > 0 {
		if err := ddSeek(outFile, cfg.seek); err != nil {
			fmt.Fprintf(ctx.Stderr, "dd: '%s': cannot seek: %v\n", outName, err)
			ddPrintSummary(ctx.Stderr, ddStats{}, ddElapsed(start), cfg.status)
			return 1
		}
		if !cfg.conv.notrunc && outFile != nil && ddIsRegular(outFile) {
			if err := outFile.Truncate(int64(cfg.seek)); err != nil {
				fmt.Fprintf(ctx.Stderr, "dd: '%s': cannot truncate: %s\n", outName, fmtOsErr(err))
			}
		}
	}

	copier := ddCopier{
		in:         in,
		out:        out,
		ibs:        cfg.ibs,
		obs:        cfg.obs,
		unbuffered: cfg.ddUnbuffered(),
		sync:       cfg.conv.sync,
		noerror:    cfg.conv.noerror,
		fullblock:  cfg.iflag.fullblock,
		remaining:  -1,
		inName:     inName,
		outName:    outName,
		stderr:     ctx.Stderr,
	}
	if cfg.countSet {
		copier.remaining = int64(cfg.count)
	}

	status := 0
	if err := copier.run(); err != nil {
		var ioErr *ddIOError
		if errors.As(err, &ioErr) {
			fmt.Fprintf(ctx.Stderr, "dd: '%s': %s error: %s\n", ioErr.name, ioErr.op, fmtOsErr(ioErr.err))
		} else {
			fmt.Fprintf(ctx.Stderr, "dd: %s\n", err)
		}
		status = 1
	}

	if outFile != nil && (cfg.conv.fsync || cfg.conv.fdatasync) {
		// fdatasync is served by fsync: a superset of what it guarantees.
		if err := outFile.Sync(); err != nil {
			fmt.Fprintf(ctx.Stderr, "dd: '%s': fsync: %s\n", outName, fmtOsErr(err))
			status = 1
		}
	}

	ddPrintSummary(ctx.Stderr, copier.stats, ddElapsed(start), cfg.status)
	return status
}

// ddInput opens if= or falls back to the standard input.
func ddInput(ctx *Context, cfg *ddConfig) (io.Reader, *os.File, string, *os.File, bool) {
	if cfg.inSet {
		file, err := os.Open(ddPath(ctx, cfg.inName))
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "dd: failed to open '%s': %s\n", cfg.inName, fmtOsErr(err))
			return nil, nil, "", nil, false
		}
		return file, file, cfg.inName, file, true
	}
	if ctx.Stdin == nil {
		return strings.NewReader(""), nil, "standard input", nil, true
	}
	inFile, _ := ctx.Stdin.(*os.File)
	return ctx.Stdin, inFile, "standard input", nil, true
}

// ddPath resolves a relative operand against the shell's directory, leaving an
// empty name empty so that it fails to open rather than naming the directory.
func ddPath(ctx *Context, name string) string {
	if name == "" {
		return ""
	}
	return resolvePath(ctx, name)
}

// ddOutput opens of= or falls back to the standard output. The output is
// truncated at open unless notrunc or seek= moved the start.
func ddOutput(ctx *Context, cfg *ddConfig) (io.Writer, *os.File, string, *os.File, bool) {
	if !cfg.outSet {
		if ctx.Stdout == nil {
			return nil, nil, "", nil, false
		}
		outFile, _ := ctx.Stdout.(*os.File)
		return ctx.Stdout, outFile, "standard output", nil, true
	}
	flags := os.O_WRONLY
	if !cfg.conv.nocreat {
		flags |= os.O_CREATE
	}
	if cfg.conv.excl {
		flags |= os.O_EXCL
	}
	if cfg.oflag.append {
		flags |= os.O_APPEND
	}
	if cfg.seek == 0 && !cfg.conv.notrunc {
		flags |= os.O_TRUNC
	}
	file, err := os.OpenFile(ddPath(ctx, cfg.outName), flags, 0o666)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "dd: failed to open '%s': %s\n", cfg.outName, fmtOsErr(err))
		return nil, nil, "", nil, false
	}
	return file, file, cfg.outName, file, true
}

func ddIsRegular(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode().IsRegular()
}

// ddSkip discards n bytes of input. A seekable input seeks; anything else is
// read and thrown away. It reports whether the whole skip was possible.
func ddSkip(in io.Reader, inFile *os.File, n uint64, ibs int) bool {
	if inFile != nil {
		if _, err := inFile.Seek(int64(n), io.SeekCurrent); err == nil {
			if !ddIsRegular(inFile) {
				return true
			}
			info, err := inFile.Stat()
			if err != nil {
				return true
			}
			pos, err := inFile.Seek(0, io.SeekCurrent)
			if err != nil {
				return true
			}
			return uint64(pos) <= uint64(info.Size())
		}
	}
	buf := make([]byte, min(ibs, 1<<16))
	for n > 0 {
		chunk := int(min(uint64(len(buf)), n))
		read, err := in.Read(buf[:chunk])
		n -= uint64(read)
		if err != nil || read == 0 {
			return n == 0
		}
	}
	return true
}

// ddSeek moves the output start to byte offset n. Only a seekable output can
// be moved; a pipe or a memory buffer cannot.
func ddSeek(outFile *os.File, n uint64) error {
	if outFile == nil {
		return errInvalidSeek
	}
	if _, err := outFile.Seek(int64(n), io.SeekStart); err != nil {
		return errInvalidSeek
	}
	return nil
}
