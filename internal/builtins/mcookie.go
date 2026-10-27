// Behaviour adapted from toybox mcookie.c (0BSD) and uutils util-linux mcookie (MIT).
package builtins

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func init() { Register(Command{Name: "mcookie", Run: runMcookie}) }

func runMcookie(ctx *Context) int {
	max := int64(4096)
	verbose := false
	var files []string
	for i := 0; i < len(ctx.Args); i++ {
		a := ctx.Args[i]
		switch a {
		case "-v", "--verbose":
			verbose = true
		case "-f", "--file":
			i++
			if i == len(ctx.Args) {
				fmt.Fprintln(ctx.Stderr, "mcookie: missing file operand")
				return 1
			}
			files = append(files, ctx.Args[i])
		case "-m", "--max-size":
			i++
			if i == len(ctx.Args) {
				fmt.Fprintln(ctx.Stderr, "mcookie: missing max-size")
				return 1
			}
			number, multiplier := parseSizeSuffix(ctx.Args[i])
			n, err := strconv.ParseInt(number, 10, 64)
			if err != nil || n < 0 || (multiplier > 1 && n > int64(^uint64(0)>>1)/multiplier) {
				fmt.Fprintln(ctx.Stderr, "mcookie: Failed to parse max-size value")
				return 1
			}
			max = n * multiplier
		default:
			if strings.HasPrefix(a, "-m") && len(a) > 2 {
				n, err := strconv.ParseInt(a[2:], 10, 64)
				if err != nil || n < 0 {
					fmt.Fprintln(ctx.Stderr, "mcookie: Failed to parse max-size value")
					return 1
				}
				max = n
			} else if strings.HasPrefix(a, "-") {
				fmt.Fprintf(ctx.Stderr, "mcookie: invalid option -- '%s'\n", strings.TrimLeft(a, "-"))
				return 1
			} else {
				files = append(files, a)
			}
		}
	}
	h := md5.New()
	buf := make([]byte, 128)
	if _, err := rand.Read(buf); err != nil {
		fmt.Fprintln(ctx.Stderr, "mcookie:", err)
		return 1
	}
	h.Write(buf)
	if verbose {
		fmt.Fprintln(ctx.Stderr, "Got 128 bytes from randomness source")
	}
	for _, name := range files {
		var r io.Reader
		if name == "-" {
			r = ctx.Stdin
		} else {
			f, err := os.Open(resolvePath(ctx, name))
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "mcookie: cannot open %s: %v\n", name, err)
				continue
			}
			defer f.Close()
			r = f
		}
		n, err := io.CopyN(h, r, max)
		if err != nil && err != io.EOF {
			fmt.Fprintf(ctx.Stderr, "mcookie: %v\n", err)
			return 1
		}
		if verbose {
			label := name
			if name == "-" {
				label = "stdin"
			}
			fmt.Fprintf(ctx.Stderr, "Got %d bytes from %s\n", n, label)
		}
	}
	fmt.Fprintln(ctx.Stdout, hex.EncodeToString(h.Sum(nil)))
	return 0
}
