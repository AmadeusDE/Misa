package builtins

import (
	"bufio"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
)

const checksumSpec = "c(check)"

var checksumSpecParsed *Spec

func init() {
	var err error
	checksumSpecParsed, err = ParseSpec(checksumSpec)
	if err != nil {
		panic("builtins: bad checksum spec: " + err.Error())
	}
	Register(Command{Name: "md5sum", Run: makeChecksumRunner("md5sum", md5.New)})
	Register(Command{Name: "sha1sum", Run: makeChecksumRunner("sha1sum", sha1.New)})
	Register(Command{Name: "sha256sum", Run: makeChecksumRunner("sha256sum", sha256.New)})
	Register(Command{Name: "sha512sum", Run: makeChecksumRunner("sha512sum", sha512.New)})
}

func makeChecksumRunner(name string, newHash func() hash.Hash) func(*Context) int {
	return func(ctx *Context) int {
		return runChecksumWith(ctx, name, newHash)
	}
}

func runChecksumWith(ctx *Context, cmdName string, newHash func() hash.Hash) (code int) {
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	flags, err := checksumSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "%s: %v\n", cmdName, err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, checksumSpecParsed) {
		return 0
	}

	if flags.Bool("c") {
		return checksumCheck(ctx, cmdName, newHash, flags.Operands)
	}

	files := flags.Operands
	if len(files) == 0 {
		files = []string{"-"}
	}

	exitCode := 0
	for _, f := range files {
		h := newHash()
		var displayName string

		if f == "-" {
			if ctx.Stdin == nil {
				fmt.Fprintf(ctx.Stderr, "%s: stdin not available\n", cmdName)
				exitCode = 1
				continue
			}
			io.Copy(h, ctx.Stdin)
			displayName = "-"
		} else {
			path := resolvePath(ctx, f)
			fh, err := os.Open(path)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "%s: %v\n", cmdName, err)
				exitCode = 1
				continue
			}
			CopyContext(ctx.CancellationContext(), h, fh)
			fh.Close()
			displayName = f
		}
		fmt.Fprintf(ctx.Stdout, "%x  %s\n", h.Sum(nil), displayName)
	}
	return exitCode
}

func checksumCheck(ctx *Context, cmdName string, newHash func() hash.Hash, files []string) int {
	if len(files) == 0 {
		files = []string{"-"}
	}

	exitCode := 0
	for _, f := range files {
		var scanner *bufio.Scanner
		if f == "-" {
			if ctx.Stdin == nil {
				fmt.Fprintf(ctx.Stderr, "%s: stdin not available\n", cmdName)
				return 1
			}
			scanner = bufio.NewScanner(ctx.Stdin)
		} else {
			path := resolvePath(ctx, f)
			fh, err := os.Open(path)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "%s: %v\n", cmdName, err)
				return 1
			}
			defer fh.Close()
			scanner = bufio.NewScanner(ContextReader(ctx.CancellationContext(), fh))
		}

		for scanner.Scan() {
			line := strings.TrimRight(scanner.Text(), "\r\n")
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			idx := strings.IndexAny(line, " \t")
			if idx == -1 {
				fmt.Fprintf(ctx.Stderr, "%s: invalid line: %s\n", cmdName, line)
				exitCode = 1
				continue
			}
			expectedHash := line[:idx]
			rest := line[idx:]
			if len(rest) > 0 && (rest[0] == ' ' || rest[0] == '\t') {
				rest = rest[1:]
			}
			if len(rest) > 0 && (rest[0] == ' ' || rest[0] == '*' || rest[0] == '?') {
				rest = rest[1:]
			}
			fileName := rest
			if fileName == "" {
				fmt.Fprintf(ctx.Stderr, "%s: invalid line: %s\n", cmdName, line)
				exitCode = 1
				continue
			}

			h := newHash()
			path := resolvePath(ctx, fileName)
			fh, err := os.Open(path)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "%s: %s: %v\n", cmdName, fileName, err)
				exitCode = 1
				continue
			}
			CopyContext(ctx.CancellationContext(), h, fh)
			fh.Close()

			actual := fmt.Sprintf("%x", h.Sum(nil))
			if strings.EqualFold(actual, expectedHash) {
				fmt.Fprintf(ctx.Stdout, "%s: OK\n", fileName)
			} else {
				fmt.Fprintf(ctx.Stdout, "%s: FAILED\n", fileName)
				exitCode = 1
			}
		}
	}
	return exitCode
}
