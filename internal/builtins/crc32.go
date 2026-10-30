// Behaviour adapted from toybox toys/posix/cksum.c, where `crc32` is declared
// as an alias of cksum (0BSD, commit 6a0c0ef). crc32 is that alias with -H -N
// -P -L forced: a reflected (little endian) CRC-32 with pre- and post-inversion
// in %08x, which is exactly the standard IEEE CRC-32 (Go's hash/crc32.IEEE).
//
// Known intentional difference: toybox decrements its operand count in
// crc32_main, so it drops the file name when there is exactly one operand. Misa
// prints "%08x name" for every explicit operand, which is toybox's own output
// for two or more operands and cksum's behaviour; implicit stdin prints the
// checksum alone. Unknown options are rejected instead of being treated as a
// file name.
package builtins

import (
	"fmt"
	"hash/crc32"
	"io"
	"os"
)

var crc32SpecParsed *Spec

func init() {
	spec, err := ParseSpec("")
	if err != nil {
		panic("builtins: bad crc32 spec: " + err.Error())
	}
	crc32SpecParsed = spec
	Register(Command{Name: "crc32", Run: runCrc32})
}

func runCrc32(ctx *Context) int {
	flags, err := crc32SpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "crc32: %v\n", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, crc32SpecParsed) {
		return 0
	}

	files := flags.Operands
	if len(files) == 0 {
		files = []string{"-"}
	}

	code := 0
	for _, name := range files {
		var r io.Reader
		var f *os.File
		if name == "-" {
			if ctx.Stdin == nil {
				fmt.Fprintln(ctx.Stderr, "crc32: stdin not available")
				code = 1
				continue
			}
			r = ctx.Stdin
		} else {
			opened, err := os.Open(resolvePath(ctx, name))
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "crc32: %s: %s\n", name, fmtOsErr(err))
				code = 1
				continue
			}
			f = opened
			r = f
		}

		h := crc32.NewIEEE()
		_, readErr := io.Copy(h, r)
		if f != nil {
			f.Close()
		}
		if readErr != nil {
			fmt.Fprintf(ctx.Stderr, "crc32: %s: %s\n", name, fmtOsErr(readErr))
			code = 1
			continue
		}

		if name == "-" {
			fmt.Fprintf(ctx.Stdout, "%08x\n", h.Sum32())
		} else {
			fmt.Fprintf(ctx.Stdout, "%08x %s\n", h.Sum32(), name)
		}
	}
	return code
}
