// Behaviour adapted from toybox uudecode.c (0BSD).
package builtins

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func init() {
	Register(Command{Name: "uudecode", Run: runUUDecode})
}

func runUUDecode(ctx *Context) int {
	spec, _ := ParseSpec("o:")
	flags, err := spec.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "uudecode:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, spec) {
		return 0
	}
	if len(flags.Operands) > 1 {
		fmt.Fprintln(ctx.Stderr, "uudecode: expected [INFILE]")
		return 1
	}
	input := ctx.Stdin
	if len(flags.Operands) == 1 {
		file, err := os.Open(resolvePath(ctx, flags.Operands[0]))
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "uudecode: %s: %s\n", flags.Operands[0], fmtOsErr(err))
			return 1
		}
		defer file.Close()
		input = file
	}
	if input == nil {
		input = strings.NewReader("")
	}
	err = uuDecode(ctx, bufio.NewReader(input), flags)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "uudecode:", fmtOsErr(err))
		return 1
	}
	return 0
}

func uuDecode(ctx *Context, input *bufio.Reader, flags *ParseResult) error {
	var name string
	var mode uint64
	var isBase64 bool
	for name == "" {
		line, err := input.ReadString('\n')
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		header, rest, found := strings.Cut(line, " ")
		if found && (header == "begin" || header == "begin-base64") {
			modeText, filename, ok := strings.Cut(strings.TrimLeft(rest, " "), " ")
			if ok {
				var modeErr error
				mode, modeErr = strconv.ParseUint(modeText, 8, 12)
				if modeErr != nil {
					return fmt.Errorf("invalid mode %q", modeText)
				}
				name = strings.TrimLeft(filename, " ")
				isBase64 = header == "begin-base64"
			}
		}
		if err != nil && name == "" {
			if err == io.EOF {
				return fmt.Errorf("no begin line")
			}
			return err
		}
	}
	output := ctx.Stdout
	if override, ok := flags.String("o"); ok {
		name = override
	} else if name == "-" {
		name = "./-"
	}
	var file *os.File
	if name != "-" {
		var err error
		file, err = os.OpenFile(resolvePath(ctx, name), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(mode)&0777)
		if err != nil {
			return err
		}
		output = file
	}
	err := uuDecodeBody(input, output, isBase64)
	if file != nil {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}
	return err
}

func uuDecodeBody(input *bufio.Reader, output io.Writer, isBase64 bool) error {
	terminator := "end"
	if isBase64 {
		terminator = "===="
	}
	for {
		line, readErr := input.ReadString('\n')
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if line == terminator {
			return nil
		}
		var data []byte
		if isBase64 {
			clean := strings.Map(func(c rune) rune {
				if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '+' || c == '/' || c == '=' {
					return c
				}
				return -1
			}, line)
			var err error
			data, err = decodeBase64Stream(clean)
			if err != nil {
				return fmt.Errorf("bad %s", line)
			}
		} else if len(line) > 0 {
			remaining := int((line[0] - 32) & 63)
			var bits uint32
			count := 0
			for i := 1; i < len(line) && remaining > 0; i++ {
				bits = bits<<6 | uint32((line[i]-32)&63)
				count += 6
				if count >= 8 {
					count -= 8
					data = append(data, byte(bits>>count))
					remaining--
				}
			}
		}
		if _, err := output.Write(data); err != nil {
			return err
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}
