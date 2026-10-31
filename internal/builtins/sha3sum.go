// Behaviour adapted from toybox sha3sum.c (0BSD).
package builtins

import (
	"crypto/sha3"
	"fmt"
	"hash"
	"io"
	"os"
	"strconv"
)

func init() { Register(Command{Name: "sha3sum", Run: runSha3sum}) }

func runSha3sum(ctx *Context) (code int) {
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	spec, _ := ParseSpec("bSa:")
	flags, err := spec.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "sha3sum:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, spec) {
		return 0
	}
	bits := 224
	if value, ok := flags.String("a"); ok {
		bits, err = strconv.Atoi(value)
	}
	shake := flags.Bool("S")
	if err != nil || (!shake && bits != 224 && bits != 256 && bits != 384 && bits != 512) || (shake && bits != 128 && bits != 256) {
		fmt.Fprintln(ctx.Stderr, "sha3sum: unsupported hash size")
		return 1
	}
	files := flags.Operands
	if len(files) == 0 {
		files = []string{"-"}
	}
	status := 0
	for _, name := range files {
		var input io.Reader = ctx.Stdin
		var file *os.File
		if name != "-" {
			file, err = os.Open(resolvePath(ctx, name))
			input = ContextReader(ctx.CancellationContext(), file)
		} else if input == nil {
			err = fmt.Errorf("stdin not available")
		} else {
			err = nil
		}
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "sha3sum: %s: %s\n", name, fmtOsErr(err))
			status = 1
			continue
		}
		var digest []byte
		if shake {
			var h *sha3.SHAKE
			if bits == 128 {
				h = sha3.NewSHAKE128()
			} else {
				h = sha3.NewSHAKE256()
			}
			_, err = io.Copy(h, input)
			if err == nil {
				digest = make([]byte, bits/8)
				_, err = io.ReadFull(h, digest)
			}
		} else {
			var h hash.Hash
			switch bits {
			case 224:
				h = sha3.New224()
			case 256:
				h = sha3.New256()
			case 384:
				h = sha3.New384()
			case 512:
				h = sha3.New512()
			}
			_, err = io.Copy(h, input)
			digest = h.Sum(nil)
		}
		if file != nil {
			file.Close()
		}
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "sha3sum: %s: %s\n", name, fmtOsErr(err))
			status = 1
			continue
		}
		fmt.Fprintf(ctx.Stdout, "%x", digest)
		if !flags.Bool("b") {
			fmt.Fprintf(ctx.Stdout, "  %s", name)
		}
		fmt.Fprintln(ctx.Stdout)
	}
	return status
}
