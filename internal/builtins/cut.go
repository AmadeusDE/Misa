package builtins

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

const cutSpec = "f(fields):d(delimiter):b(bytes):c(characters):s(only-delimited)n(no-split-multibyte)z(zero-terminated)(complement)O(output-delimiter):D(nosort)F:w(whitespace-delimited):?"

var cutSpecParsed *Spec

func init() {
	var err error
	cutSpecParsed, err = ParseSpec(cutSpec)
	if err != nil {
		panic("builtins: bad cut spec: " + err.Error())
	}
	Register(Command{Name: "cut", Run: runCut})
}

func getEnv(ctx *Context, key string) string {
	return ctx.Getenv(key)
}

type cutRange struct{ lo, hi int }

func parseCutRanges(spec string) ([]cutRange, error) {
	var ranges []cutRange
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if idx := strings.IndexByte(part, '-'); idx >= 0 {
			loStr, hiStr := part[:idx], part[idx+1:]
			lo, hi := 1, int(^uint(0)>>1)
			if loStr != "" {
				n, err := strconv.Atoi(loStr)
				if err != nil || n < 1 {
					return nil, fmt.Errorf("invalid range '%s'", part)
				}
				lo = n
			}
			if hiStr != "" {
				n, err := strconv.Atoi(hiStr)
				if err != nil || n < 1 {
					return nil, fmt.Errorf("invalid range '%s'", part)
				}
				hi = n
			}
			if loStr != "" && hiStr != "" && lo > hi {
				return nil, fmt.Errorf("invalid decreasing range '%s'", part)
			}
			ranges = append(ranges, cutRange{lo, hi})
		} else {
			n, err := strconv.Atoi(part)
			if err != nil || n < 1 {
				return nil, fmt.Errorf("invalid range '%s'", part)
			}
			ranges = append(ranges, cutRange{n, n})
		}
	}
	return ranges, nil
}

func inRanges(pos int, ranges []cutRange, complement bool) bool {
	hit := false
	for _, r := range ranges {
		if pos >= r.lo && pos <= r.hi {
			hit = true
			break
		}
	}
	if complement {
		return !hit
	}
	return hit
}

func selectFieldsOrdered(fields []string, ranges []cutRange, complement bool) []string {
	if complement {
		var selected []string
		for i, f := range fields {
			if inRanges(i+1, ranges, complement) {
				selected = append(selected, f)
			}
		}
		return selected
	}
	var selected []string
	for _, r := range ranges {
		hi := r.hi
		if hi > len(fields) {
			hi = len(fields)
		}
		for p := r.lo; p <= hi; p++ {
			if p < 1 {
				continue
			}
			selected = append(selected, fields[p-1])
		}
	}
	return selected
}

func runCut(ctx *Context) int {
	flags, err := cutSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "cut:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, cutSpecParsed) {
		return 0
	}

	complement := flags.Bool("complement")
	onlyDelimited := flags.Bool("s")
	noSort := flags.Bool("D")

	fieldSpec, hasFields := flags.String("f")
	regexFieldSpec, hasRegexFields := flags.String("F")
	wsVal, hasWSOpt := flags.ExplicitString("whitespace-delimited")
	hasWSBool := flags.Bool("w") || flags.Bool("whitespace-delimited")
	hasWS := hasWSOpt || hasWSBool
	isTrimmed := hasWSOpt && (wsVal == "" || strings.HasPrefix("trimmed", wsVal))
	byteSpec, hasBytes := flags.String("b")
	charSpec, hasChars := flags.String("c")

	modes := 0
	if hasFields {
		modes++
	}
	if hasRegexFields {
		modes++
	}
	if hasBytes {
		modes++
	}
	if hasChars {
		modes++
	}
	if modes == 0 {
		fmt.Fprintln(ctx.Stderr, "cut: you must specify a list of bytes, characters, or fields")
		return 1
	}
	if modes > 1 {
		fmt.Fprintln(ctx.Stderr, "cut: only one list may be specified")
		return 1
	}

	if hasRegexFields {
		hasFields = true
		fieldSpec = regexFieldSpec
	}

	if hasWS && (hasBytes || hasChars) {
		fmt.Fprintln(ctx.Stderr, "cut: an input delimiter makes sense\n\tonly when operating on fields")
		return 1
	}

	if hasBytes || hasChars {
		if _, ok := flags.String("d"); ok {
			fmt.Fprintln(ctx.Stderr, "cut: an input delimiter may be specified only when operating on fields")
			return 1
		}
		if onlyDelimited {
			fmt.Fprintln(ctx.Stderr, "cut: suppressing non-delimited lines makes sense only when operating on fields")
			return 1
		}
	}

	if hasWS {
		if wsVal != "" && !strings.HasPrefix("trimmed", wsVal) {
			fmt.Fprintf(ctx.Stderr, "cut: invalid argument '%s' for '--whitespace-delimited'\n", wsVal)
			return 1
		}
		if _, ok := flags.String("d"); ok {
			fmt.Fprintln(ctx.Stderr, "cut: -d and -w are mutually exclusive")
			return 1
		}
	}

	delim := "\t"
	if d, ok := flags.String("d"); ok {
		if len(d) == 0 {
			delim = "\x00"
		} else {
			isCLocale := false
			if lc := getEnv(ctx, "LC_ALL"); lc == "C" || lc == "POSIX" {
				isCLocale = true
			} else if lc == "" {
				if lc = getEnv(ctx, "LC_CTYPE"); lc == "C" || lc == "POSIX" {
					isCLocale = true
				} else if lc == "" {
					if lc = getEnv(ctx, "LANG"); lc == "C" || lc == "POSIX" {
						isCLocale = true
					}
				}
			}
			if isCLocale && len(d) > 1 {
				fmt.Fprintf(ctx.Stderr, "cut: the delimiter must be a single character\n")
				return 1
			}
			if utf8.RuneCountInString(d) > 1 {
				fmt.Fprintf(ctx.Stderr, "cut: the delimiter must be a single character\n")
				return 1
			}
			delim = d
		}
	}

	outDelim := delim
	if hasRegexFields {
		outDelim = " "
	}
	if hasWS {
		outDelim = "\t"
	}
	if od, ok := flags.String("output-delimiter"); ok {
		outDelim = od
	}

	var rangeSpec string
	if hasFields {
		rangeSpec = fieldSpec
	} else if hasBytes {
		rangeSpec = byteSpec
	} else {
		rangeSpec = charSpec
	}

	ranges, err := parseCutRanges(rangeSpec)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "cut: %v\n", err)
		return 1
	}

	files := flags.Operands
	if len(files) == 0 {
		files = []string{"-"}
	}

	zeroTerm := flags.Bool("z")
	lineSep := byte('\n')
	if zeroTerm {
		lineSep = 0
	}

	exitCode := 0
	for _, f := range files {
		var r io.Reader
		if f == "-" {
			r = ctx.Stdin
		} else {
			path := resolvePath(ctx, f)
			fh, err := os.Open(path)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "cut: %s: %s\n", f, fmtOsErr(err))
				exitCode = 1
				continue
			}
			defer fh.Close()
			r = fh
		}

		if hasFields && delim == "\n" {
			cutNewlineDelim(ctx, r, ranges, complement, onlyDelimited, outDelim, lineSep, noSort)
			continue
		}

		br := bufio.NewReader(r)
		for {
			var line string
			if zeroTerm {
				line, err = br.ReadString(0)
				if len(line) > 0 && line[len(line)-1] == 0 {
					line = line[:len(line)-1]
				}
			} else {
				line, err = br.ReadString('\n')
				if len(line) > 0 && line[len(line)-1] == '\n' {
					line = line[:len(line)-1]
				}
			}
			if len(line) == 0 && err != nil {
				if err != io.EOF {
					fmt.Fprintf(ctx.Stderr, "cut: %s: %s\n", f, fmtOsErr(err))
					exitCode = 1
				}
				break
			}

			if hasWS {
				lineWork := line
				if isTrimmed {
					lineWork = strings.TrimSpace(line)
				}
				var fields []string
				if !isTrimmed && len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
					fields = append([]string{""}, strings.Fields(lineWork)...)
				} else {
					fields = strings.Fields(lineWork)
				}

				if len(fields) <= 1 {
					if !onlyDelimited {
						if isTrimmed {
							ctx.Stdout.Write([]byte(lineWork))
						} else {
							ctx.Stdout.Write([]byte(line))
						}
						ctx.Stdout.Write([]byte{lineSep})
					}
				} else {
					var selected []string
					if noSort {
						selected = selectFieldsOrdered(fields, ranges, complement)
					} else {
						for i, fld := range fields {
							if inRanges(i+1, ranges, complement) {
								selected = append(selected, fld)
							}
						}
					}
					ctx.Stdout.Write([]byte(strings.Join(selected, outDelim)))
					ctx.Stdout.Write([]byte{lineSep})
				}
			} else if hasRegexFields {
				var fields []string
				if _, ok := flags.String("d"); ok {
					fields = strings.Split(line, delim)
				} else {
					fields = strings.Fields(line)
				}
				var selected []string
				if noSort {
					selected = selectFieldsOrdered(fields, ranges, complement)
				} else {
					for i, fld := range fields {
						if inRanges(i+1, ranges, complement) {
							selected = append(selected, fld)
						}
					}
				}
				ctx.Stdout.Write([]byte(strings.Join(selected, outDelim)))
				ctx.Stdout.Write([]byte{lineSep})
			} else if hasFields {
				if !strings.Contains(line, delim) {
					if !onlyDelimited {
						ctx.Stdout.Write([]byte(line))
						ctx.Stdout.Write([]byte{lineSep})
					}
				} else {
					fields := strings.Split(line, delim)
					var selected []string
					if noSort {
						selected = selectFieldsOrdered(fields, ranges, complement)
					} else {
						for i, fld := range fields {
							if inRanges(i+1, ranges, complement) {
								selected = append(selected, fld)
							}
						}
					}
					ctx.Stdout.Write([]byte(strings.Join(selected, outDelim)))
					ctx.Stdout.Write([]byte{lineSep})
				}
			} else {
				_, hasOutDelim := flags.String("output-delimiter")
				if hasOutDelim {
					cutByteRangesWithDelim(ctx, line, ranges, complement, outDelim, lineSep)
				} else if hasChars {
					runes := []rune(line)
					var selected []rune
					for i, r := range runes {
						if inRanges(i+1, ranges, complement) {
							selected = append(selected, r)
						}
					}
					ctx.Stdout.Write([]byte(string(selected)))
					ctx.Stdout.Write([]byte{lineSep})
				} else {
					var selected []byte
					if flags.Bool("n") || flags.Bool("no-split-multibyte") {
						startByte := 1
						for _, r := range line {
							runeLen := utf8.RuneLen(r)
							endByte := startByte + runeLen - 1
							if inRanges(endByte, ranges, complement) {
								selected = append(selected, []byte(string(r))...)
							}
							startByte = endByte + 1
						}
					} else {
						for i, b := range []byte(line) {
							if inRanges(i+1, ranges, complement) {
								selected = append(selected, b)
							}
						}
					}
					ctx.Stdout.Write(selected)
					ctx.Stdout.Write([]byte{lineSep})
				}
			}

			if err != nil {
				if err != io.EOF {
					fmt.Fprintf(ctx.Stderr, "cut: %s: %s\n", f, fmtOsErr(err))
					exitCode = 1
				}
				break
			}
		}
	}
	return exitCode
}

func cutNewlineDelim(ctx *Context, r io.Reader, ranges []cutRange, complement, onlyDelimited bool, outDelim string, lineSep byte, noSort bool) {
	data, err := io.ReadAll(r)
	if err != nil || len(data) == 0 {
		return
	}
	content := string(data)
	if !strings.Contains(content, "\n") {
		if !onlyDelimited {
			ctx.Stdout.Write([]byte(content))
			ctx.Stdout.Write([]byte{lineSep})
		}
		return
	}
	if strings.HasSuffix(content, "\n") {
		content = content[:len(content)-1]
	}
	fields := strings.Split(content, "\n")
	var selected []string
	if noSort {
		selected = selectFieldsOrdered(fields, ranges, complement)
	} else {
		for i, fld := range fields {
			if inRanges(i+1, ranges, complement) {
				selected = append(selected, fld)
			}
		}
	}
	if len(selected) == 0 {
		if !onlyDelimited {
			ctx.Stdout.Write([]byte{lineSep})
		}
		return
	}
	ctx.Stdout.Write([]byte(strings.Join(selected, outDelim)))
	ctx.Stdout.Write([]byte{lineSep})
}

func cutByteRangesWithDelim(ctx *Context, line string, ranges []cutRange, complement bool, outDelim string, lineSep byte) {
	data := []byte(line)
	first := true
	for _, r := range ranges {
		var chunk []byte
		for i := r.lo; i <= r.hi && i <= len(data); i++ {
			if complement == inRanges(i, ranges, false) {
				continue
			}
			chunk = append(chunk, data[i-1])
		}
		if len(chunk) > 0 {
			if !first {
				ctx.Stdout.Write([]byte(outDelim))
			}
			ctx.Stdout.Write(chunk)
			first = false
		}
	}
	ctx.Stdout.Write([]byte{lineSep})
}
