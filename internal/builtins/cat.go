package builtins

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const catSpec = "n(number)b(number-nonblank)s(squeeze-blank)eE(show-ends)tT(show-tabs)v(show-nonprinting)A(show-all)u"

var catSpecParsed *Spec

func init() {
	var err error
	catSpecParsed, err = ParseSpec(catSpec)
	if err != nil {
		panic("builtins: bad cat spec: " + err.Error())
	}

	Register(Command{Name: "cat", Run: runCat})
}

func runCat(ctx *Context) int {
	flags, err := catSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "cat:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, catSpecParsed) {
		return 0
	}

	showAll := flags.Bool("A")
	smallE := flags.Bool("e")
	smallT := flags.Bool("t")
	number := flags.Bool("n")
	numberNonBlank := flags.Bool("b")
	squeeze := flags.Bool("s")
	showEnds := flags.Bool("E") || showAll || smallE
	showTabs := flags.Bool("T") || showAll || smallT
	showNonPrint := flags.Bool("v") || showAll || smallE || smallT

	if numberNonBlank {
		number = false
	}

	files := flags.Operands
	if len(files) == 0 {
		if ctx.Stdin == nil {
			fmt.Fprintln(ctx.Stderr, "cat: reading from stdin is not supported in this shell")
			return 1
		}
		files = []string{"-"}
	}

	lineNum := 1
	exitCode := 0
	prevBlank := false
	pendingCR := false

	for _, f := range files {
		var r io.Reader

		if f == "-" {
			if ctx.Stdin == nil {
				fmt.Fprintln(ctx.Stderr, "cat: reading from stdin is not supported in this shell")
				exitCode = 1
				continue
			}
			r = ctx.Stdin
		} else {
			path := f
			if !filepath.IsAbs(path) {
				path = filepath.Join(ctx.Dir, path)
			}
			fh, err := os.Open(path)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "cat: %s: %s\n", f, fmtOsErr(err))
				exitCode = 1
				continue
			}
			defer fh.Close()
			r = fh
		}

		if !number && !numberNonBlank && !squeeze && !showEnds && !showTabs && !showNonPrint {
			io.Copy(ctx.Stdout, r)
			continue
		}

		br := bufio.NewReader(r)
		for {
			rawLine, readErr := br.ReadBytes('\n')
			if len(rawLine) == 0 && readErr != nil {
				break
			}

			if pendingCR {
				if len(rawLine) > 0 && rawLine[0] == '\n' {
					if showEnds || showNonPrint {
						ctx.Stdout.Write([]byte("^M"))
					} else {
						ctx.Stdout.Write([]byte{'\r'})
					}
				} else {
					if showNonPrint {
						ctx.Stdout.Write([]byte("^M"))
					} else {
						ctx.Stdout.Write([]byte{'\r'})
					}
				}
				pendingCR = false
			}

			hasNewline := len(rawLine) > 0 && rawLine[len(rawLine)-1] == '\n'
			content := rawLine
			if hasNewline {
				content = rawLine[:len(rawLine)-1]
			}

			hasCR := len(content) > 0 && content[len(content)-1] == '\r'

			isBlank := len(content) == 0 || (len(content) == 1 && content[0] == '\r')

			if squeeze && isBlank && prevBlank {
				if readErr != nil {
					break
				}
				continue
			}
			prevBlank = isBlank

			var out strings.Builder

			if numberNonBlank {
				if !isBlank {
					fmt.Fprintf(&out, "%6d\t", lineNum)
					lineNum++
				}
			} else if number {
				fmt.Fprintf(&out, "%6d\t", lineNum)
				lineNum++
			}

			lineContent := content
			if hasCR {
				if hasNewline {
					if showEnds || showNonPrint {
						lineContent = content[:len(content)-1]
					}
				} else {
					lineContent = content[:len(content)-1]
					pendingCR = true
				}
			}

			for _, c := range lineContent {
				if showTabs && c == '\t' {
					out.WriteString("^I")
				} else if showNonPrint && c != '\t' && c != '\n' {
					if c < 32 {
						out.WriteByte('^')
						out.WriteByte(byte(c + 64))
					} else if c == 127 {
						out.WriteString("^?")
					} else if c > 127 {
						out.WriteString("M-")
						if c-128 < 32 {
							out.WriteByte('^')
							out.WriteByte(byte(c - 128 + 64))
						} else if c-128 == 127 {
							out.WriteString("^?")
						} else {
							out.WriteByte(byte(c - 128))
						}
					} else {
						out.WriteByte(byte(c))
					}
				} else {
					out.WriteByte(byte(c))
				}
			}

			if hasCR && hasNewline && (showEnds || showNonPrint) {
				out.WriteString("^M")
			}

			if showEnds && hasNewline {
				out.WriteByte('$')
			}

			if hasNewline {
				out.WriteByte('\n')
			}
			ctx.Stdout.Write([]byte(out.String()))

			if readErr != nil {
				break
			}
		}
	}

	if pendingCR {
		if showNonPrint {
			ctx.Stdout.Write([]byte("^M"))
		} else {
			ctx.Stdout.Write([]byte{'\r'})
		}
		pendingCR = false
	}

	return exitCode
}
