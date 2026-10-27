// Behaviour adapted from uutils coreutils b2sum (MIT).
package builtins

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func init() { Register(Command{Name: "b2sum", Run: runB2sum}) }

func runB2sum(ctx *Context) int {
	spec, _ := ParseSpec("b(binary)t(text)c(check)l(length):z(zero)(tag)(quiet)(status)(strict)w(warn)")
	flags, err := spec.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "b2sum:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, spec) {
		return 0
	}
	bits := 0
	if value, ok := flags.String("l"); ok {
		n, err := strconv.ParseUint(value, 10, 64)
		if err != nil || n > 512 || n%8 != 0 {
			fmt.Fprintf(ctx.Stderr, "b2sum: invalid length: '%s'\n", value)
			if n > 512 {
				fmt.Fprintln(ctx.Stderr, "b2sum: maximum digest length for 'BLAKE2b' is 512 bits")
			}
			return 1
		}
		if n < 512 {
			bits = int(n)
		}
	}
	if flags.Bool("c") && (flags.Bool("tag") || flags.Bool("z")) {
		fmt.Fprintln(ctx.Stderr, "b2sum: --tag and --zero are not supported when verifying checksums")
		return 1
	}
	files := flags.Operands
	if len(files) == 0 {
		files = []string{"-"}
	}
	if flags.Bool("c") {
		return b2sumCheck(ctx, files, bits, flags.Bool("quiet"), flags.Bool("status"), flags.Bool("strict"), flags.Bool("w"))
	}
	status := 0
	for _, name := range files {
		data, err := b2sumRead(ctx, name)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "b2sum: %s: %s\n", name, fmtOsErr(err))
			status = 1
			continue
		}
		digest, label, _ := computeDigest("blake2b", data, bits)
		end := "\n"
		if flags.Bool("z") {
			end = "\x00"
		}
		if flags.Bool("tag") {
			fmt.Fprintf(ctx.Stdout, "%s (%s) = %x%s", label, name, digest, end)
		} else {
			marker := " "
			if flags.Bool("b") {
				marker = "*"
			}
			display := name
			escaped := !flags.Bool("z") && strings.ContainsAny(name, "\\\n\r")
			if escaped {
				display = strings.NewReplacer("\\", "\\\\", "\n", "\\n", "\r", "\\r").Replace(name)
				fmt.Fprint(ctx.Stdout, "\\")
			}
			fmt.Fprintf(ctx.Stdout, "%x %s%s%s", digest, marker, display, end)
		}
	}
	return status
}

func b2sumRead(ctx *Context, name string) ([]byte, error) {
	if name != "-" {
		return os.ReadFile(resolvePath(ctx, name))
	}
	if ctx.Stdin == nil {
		return nil, fmt.Errorf("stdin not available")
	}
	return io.ReadAll(ctx.Stdin)
}

func b2sumCheck(ctx *Context, files []string, bits int, quiet, status, strict, warn bool) int {
	result := 0
	for _, name := range files {
		data, err := b2sumRead(ctx, name)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "b2sum: %s: %s\n", name, fmtOsErr(err))
			result = 1
			continue
		}
		valid, invalid := 0, 0
		for index, line := range strings.Split(string(data), "\n") {
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			digest, filename := "", ""
			lineBits := bits
			if match := bsdTagRe.FindStringSubmatch(line); match != nil && isValidCheckAlgo(match[1], "blake2b") {
				digest, filename = match[3], match[2]
				if strings.Contains(match[1], "-") {
					lineBits, _ = strconv.Atoi(match[1][strings.LastIndexByte(match[1], '-')+1:])
				}
			} else if len(line) > 3 {
				if i := strings.IndexByte(line, ' '); i > 0 && i+2 < len(line) && (line[i+1] == ' ' || line[i+1] == '*') {
					digest, filename = line[:i], line[i+2:]
				}
			}
			decoded, err := hex.DecodeString(digest)
			if lineBits == 0 {
				lineBits = len(decoded) * 8
			}
			if err != nil || len(decoded) == 0 || lineBits > 512 || len(decoded)*8 != lineBits || filename == "" {
				invalid++
				if warn {
					fmt.Fprintf(ctx.Stderr, "b2sum: %s: %d: improperly formatted BLAKE2 checksum line\n", name, index+1)
				}
				continue
			}
			valid++
			var diagnostics bytes.Buffer
			checkCtx := *ctx
			checkCtx.Stdin = strings.NewReader(digest + "  " + filename + "\n")
			checkCtx.Stderr = &diagnostics
			if runCheckMode(&checkCtx, []string{"-"}, "blake2b", lineBits, quiet, status) != 0 {
				result = 1
			}
			fmt.Fprint(ctx.Stderr, strings.ReplaceAll(diagnostics.String(), "cksum:", "b2sum:"))
		}
		if valid == 0 {
			display := name
			if name == "-" {
				display = "'standard input'"
			}
			fmt.Fprintf(ctx.Stderr, "b2sum: %s: no properly formatted checksum lines found\n", display)
			result = 1
		} else if invalid > 0 {
			if !status {
				fmt.Fprintf(ctx.Stderr, "b2sum: WARNING: %d lines are improperly formatted\n", invalid)
			}
			if strict {
				result = 1
			}
		}
	}
	return result
}
