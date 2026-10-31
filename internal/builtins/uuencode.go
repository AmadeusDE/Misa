// Behaviour adapted from toybox uuencode.c (0BSD).
package builtins

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
)

func init() {
	Register(Command{Name: "uuencode", Run: runUUEncode})
}

func runUUEncode(ctx *Context) (code int) {
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	if ctx.CancellationContext().Err() != nil {
		return 0
	}
	spec, _ := ParseSpec("m")
	flags, err := spec.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "uuencode:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, spec) {
		return 0
	}
	if len(flags.Operands) < 1 || len(flags.Operands) > 2 {
		fmt.Fprintln(ctx.Stderr, "uuencode: expected [INFILE] ENCODE_FILENAME")
		return 1
	}
	input := ctx.Stdin
	mode := os.FileMode(0744)
	if len(flags.Operands) == 2 {
		file, err := os.Open(resolvePath(ctx, flags.Operands[0]))
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "uuencode: %s: %s\n", flags.Operands[0], fmtOsErr(err))
			return 1
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			fmt.Fprintln(ctx.Stderr, "uuencode:", fmtOsErr(err))
			return 1
		}
		mode, input = info.Mode().Perm(), file
	}
	if input == nil {
		input = strings.NewReader("")
	}
	input = ContextReader(ctx.CancellationContext(), input)
	output := bufio.NewWriter(ctx.Stdout)
	header, terminator, width := "begin", "end", 45
	if flags.Bool("m") {
		header, terminator, width = "begin-base64", "====", 57
	}
	fmt.Fprintf(output, "%s %o %s\n", header, mode, flags.Operands[len(flags.Operands)-1])
	buf := make([]byte, width)
	for {
		n, readErr := io.ReadFull(input, buf)
		if n > 0 {
			var line string
			if flags.Bool("m") {
				line = base64.StdEncoding.EncodeToString(buf[:n])
			} else {
				line = uuEncodeLine(buf[:n])
			}
			if _, err = fmt.Fprintln(output, line); err != nil {
				break
			}
		}
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
		if readErr != nil {
			err = readErr
			break
		}
	}
	if err == nil {
		_, err = fmt.Fprintln(output, terminator)
	}
	if flushErr := output.Flush(); err == nil {
		err = flushErr
	}
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "uuencode:", fmtOsErr(err))
		return 1
	}
	return 0
}

func uuEncodeLine(data []byte) string {
	out := make([]byte, 1, 1+4*((len(data)+2)/3))
	out[0] = byte(len(data)) + 32
	for len(data) > 0 {
		n := min(3, len(data))
		var bits uint32
		for i, b := range data[:n] {
			bits |= uint32(b) << (16 - 8*i)
		}
		for shift := 18; shift >= 0; shift -= 6 {
			c := byte(bits>>shift) & 63
			if c == 0 {
				out = append(out, '`')
			} else {
				out = append(out, c+32)
			}
		}
		data = data[n:]
	}
	return string(out)
}
