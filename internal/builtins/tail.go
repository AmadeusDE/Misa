package builtins

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func init() {
	Register(Command{Name: "tail", Run: runTail})
}

func tailParseCount(s string) (int64, bool, error) {
	fromStart := false
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "+") {
		fromStart = true
		s = s[1:]
	} else if strings.HasPrefix(s, "-") {
		s = s[1:]
	}
	s = strings.TrimSuffix(s, "c")
	s = strings.TrimSuffix(s, "n")
	numStr, mult := parseSizeSuffix(s)
	if numStr == "" {
		numStr = "0"
	}
	n, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		var ne *strconv.NumError
		if ok := isErrRange(err, &ne); ok {
			return math.MaxInt64, fromStart, nil
		}
		return 0, false, err
	}
	if n > 0 && mult > 1 {
		if n > math.MaxInt64/mult {
			return math.MaxInt64, fromStart, nil
		}
	}
	return n * mult, fromStart, nil
}

func isErrRange(err error, ne **strconv.NumError) bool {
	if e, ok := err.(*strconv.NumError); ok {
		*ne = e
		return e.Err == strconv.ErrRange
	}
	return false
}

func runTail(ctx *Context) (code int) {
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	mode := "lines"
	count := int64(10)
	fromStart := false
	quiet := false
	verbose := false
	zeroTerm := false
	follow := false
	var files []string

	i := 0
	for i < len(ctx.Args) {
		a := ctx.Args[i]
		if a == "--" {
			files = append(files, ctx.Args[i+1:]...)
			break
		}
		if a == "---presume-input-pipe" || a == "---disable-inotify" || a == "--debug" {
			i++
			continue
		}
		if a == "-n" || a == "--lines" || a == "-c" || a == "--bytes" {
			if a == "-c" || a == "--bytes" {
				mode = "bytes"
			} else {
				mode = "lines"
			}
			i++
			if i >= len(ctx.Args) {
				fmt.Fprintf(ctx.Stderr, "tail: option '%s' requires an argument\n", a)
				return 1
			}
			n, fs, err := tailParseCount(ctx.Args[i])
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "tail: invalid number of %s: '%s'\n", mode, ctx.Args[i])
				return 1
			}
			count = n
			fromStart = fs
		} else if strings.HasPrefix(a, "--lines=") {
			mode = "lines"
			val := a[len("--lines="):]
			n, fs, err := tailParseCount(val)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "tail: invalid number of lines: '%s'\n", val)
				return 1
			}
			count = n
			fromStart = fs
		} else if strings.HasPrefix(a, "--bytes=") {
			mode = "bytes"
			val := a[len("--bytes="):]
			n, fs, err := tailParseCount(val)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "tail: invalid number of bytes: '%s'\n", val)
				return 1
			}
			count = n
			fromStart = fs
		} else if a == "-s" || a == "--sleep-interval" || strings.HasPrefix(a, "--sleep-interval=") {
			var sval string
			if strings.HasPrefix(a, "--sleep-interval=") {
				sval = a[len("--sleep-interval="):]
			} else {
				i++
				if i < len(ctx.Args) {
					sval = ctx.Args[i]
				}
			}
			if sval != "" {
				f, err := strconv.ParseFloat(sval, 64)
				if err != nil {
					fmt.Fprintf(ctx.Stderr, "tail: invalid number of seconds: '%s'\n", sval)
					return 1
				}
				if f < 0 {
					fmt.Fprintf(ctx.Stderr, "tail: invalid number of seconds: '%s'\n", sval)
					return 1
				}
			}
		} else if len(a) >= 2 && a[0] == '-' && a[1] == 's' && len(a) > 2 {
			sval := a[2:]
			f, err := strconv.ParseFloat(sval, 64)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "tail: invalid number of seconds: '%s'\n", sval)
				return 1
			}
			if f < 0 {
				fmt.Fprintf(ctx.Stderr, "tail: invalid number of seconds: '%s'\n", sval)
				return 1
			}
		} else if a == "-f" || a == "--follow" || strings.HasPrefix(a, "--follow=") {
			follow = true
		} else if strings.HasPrefix(a, "--pid=") {
			pidStr := a[len("--pid="):]
			if _, err := strconv.Atoi(pidStr); err != nil {
				fmt.Fprintf(ctx.Stderr, "tail: invalid PID: '%s': invalid digit found in string\n", pidStr)
				return 1
			}
		} else if a == "--pid" {
			i++
			if i < len(ctx.Args) {
				pidStr := ctx.Args[i]
				if _, err := strconv.Atoi(pidStr); err != nil {
					fmt.Fprintf(ctx.Stderr, "tail: invalid PID: '%s': invalid digit found in string\n", pidStr)
					return 1
				}
			}
		} else if a == "--retry" {
			// retry: no-op (we don't implement inotify-based following)
		} else if strings.HasPrefix(a, "--max-unchanged-stats") {
			if a == "--max-unchanged-stats" {
				i++
			}
		} else if a == "-F" {
			follow = true
		} else if len(a) >= 2 && (a[0] == '-' || a[0] == '+') && a[1] >= '0' && a[1] <= '9' {
			j := 1
			for j < len(a) && a[j] >= '0' && a[j] <= '9' {
				j++
			}
			num := a[:j]
			trail := a[j:]
			if len(trail) > 0 && (trail[0] == 'c' || trail[0] == 'n') {
				if trail[0] == 'c' {
					mode = "bytes"
				} else {
					mode = "lines"
				}
				trail = trail[1:]
			}
			n, fs, err := tailParseCount(num)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "tail: invalid number %q\n", a)
				return 1
			}
			count = n
			fromStart = fs
			for _, ch := range trail {
				switch ch {
				case 'q':
					quiet = true
					verbose = false
				case 'v':
					verbose = true
					quiet = false
				case 'z':
					zeroTerm = true
				}
			}
		} else if len(a) >= 2 && a[0] == '-' && a[1] != '-' {
			flags := a[1:]
			for fi := 0; fi < len(flags); fi++ {
				switch flags[fi] {
				case 'q':
					quiet = true
					verbose = false
				case 'v':
					verbose = true
					quiet = false
				case 'z':
					zeroTerm = true
				case 'n', 'c':
					if flags[fi] == 'c' {
						mode = "bytes"
					} else {
						mode = "lines"
					}
					rest := flags[fi+1:]
					if rest != "" {
						n, fs, err := tailParseCount(rest)
						if err != nil {
							displayRest := rest
							if len(displayRest) > 0 && (displayRest[0] == '+' || displayRest[0] == '-') {
								displayRest = displayRest[1:]
							}
							fmt.Fprintf(ctx.Stderr, "tail: invalid number of %s: '%s'\n", mode, displayRest)
							return 1
						}
						count = n
						fromStart = fs
					} else {
						i++
						if i >= len(ctx.Args) {
							fmt.Fprintf(ctx.Stderr, "tail: option requires an argument -- '%c'\n", flags[fi])
							return 1
						}
						n, fs, err := tailParseCount(ctx.Args[i])
						if err != nil {
							fmt.Fprintf(ctx.Stderr, "tail: invalid number of %s: '%s'\n", mode, ctx.Args[i])
							return 1
						}
						count = n
						fromStart = fs
					}
					fi = len(flags)
				default:
					fmt.Fprintf(ctx.Stderr, "tail: unknown option -%c\n", flags[fi])
					return 1
				}
			}
		} else if strings.HasPrefix(a, "--") && len(a) > 2 {
			fmt.Fprintf(ctx.Stderr, "tail: unknown option %s\n", a)
			return 1
		} else {
			files = append(files, a)
		}
		i++
	}

	lineSep := byte('\n')
	if zeroTerm {
		lineSep = 0
	}

	if len(files) == 0 {
		if ctx.Stdin == nil {
			fmt.Fprintln(ctx.Stderr, "tail: missing operand")
			return 1
		}
		files = []string{"-"}
	}

	multi := len(files) > 1
	exitCode := 0

	for fi, name := range files {
		if err := ctx.CancellationContext().Err(); err != nil {
			return ExitStatus(err)
		}
		showHeader := (multi && !quiet) || verbose
		if showHeader {
			if fi > 0 {
				if _, err := fmt.Fprintln(ctx.Stdout); err != nil {
					return 1
				}
			}
			hdr := name
			if hdr == "-" {
				hdr = "standard input"
			} else {
				hdr = headQuoteString(hdr)
			}
			if _, err := fmt.Fprintf(ctx.Stdout, "==> %s <==\n", hdr); err != nil {
				return 1
			}
		}

		var data []byte
		if name == "-" {
			if ctx.Stdin == nil {
				fmt.Fprintln(ctx.Stderr, "tail: stdin not available")
				exitCode = 1
				continue
			}
			var err error
			data, err = io.ReadAll(ctx.Stdin)
			if err != nil {
				return 1
			}
		} else {
			path := name
			if !filepath.IsAbs(path) {
				path = filepath.Join(ctx.Dir, path)
			}
			st, statErr := os.Stat(path)
			if statErr != nil {
				fmt.Fprintf(ctx.Stderr, "tail: cannot open '%s' for reading: %s\n", name, fmtOsErr(statErr))
				exitCode = 1
				continue
			}
			if st.IsDir() {
				fmt.Fprintf(ctx.Stderr, "tail: error reading '%s': Is a directory\n", name)
				exitCode = 1
				continue
			}
			var readErr error
			f, openErr := os.Open(path)
			if openErr != nil {
				readErr = openErr
			} else {
				data, readErr = io.ReadAll(ContextReader(ctx.CancellationContext(), f))
				f.Close()
			}
			if readErr != nil {
				fmt.Fprintf(ctx.Stderr, "tail: error reading '%s': %s\n", name, fmtOsErr(readErr))
				exitCode = 1
				continue
			}
		}

		var result []byte
		if mode == "bytes" {
			if fromStart {
				offset := int(count) - 1
				if offset < 0 {
					offset = 0
				}
				if offset < len(data) {
					result = data[offset:]
				}
			} else {
				if count >= int64(len(data)) {
					result = data
				} else {
					offset := int64(len(data)) - count
					if offset < 0 {
						offset = 0
					}
					result = data[offset:]
				}
			}
		} else {
			if fromStart {
				lineIdx := int(count) - 1
				if lineIdx < 0 {
					lineIdx = 0
				}
				cur := 0
				for j := 0; j < lineIdx && cur < len(data); j++ {
					if err := ctx.CancellationContext().Err(); err != nil {
						return ExitStatus(err)
					}
					nl := bytes.IndexByte(data[cur:], lineSep)
					if nl < 0 {
						cur = len(data)
						break
					}
					cur += nl + 1
				}
				if cur < len(data) {
					result = data[cur:]
				}
			} else {
				lines := 0
				for start := 0; start < len(data); start += 32768 {
					if err := ctx.CancellationContext().Err(); err != nil {
						return ExitStatus(err)
					}
					end := min(start+32768, len(data))
					lines += bytes.Count(data[start:end], []byte{lineSep})
				}
				if len(data) > 0 && data[len(data)-1] != lineSep {
					lines++
				}
				if int(count) >= lines {
					result = data
				} else {
					skip := lines - int(count)
					cur := 0
					for j := 0; j < skip; j++ {
						if err := ctx.CancellationContext().Err(); err != nil {
							return ExitStatus(err)
						}
						nl := bytes.IndexByte(data[cur:], lineSep)
						if nl < 0 {
							cur = len(data)
							break
						}
						cur += nl + 1
					}
					result = data[cur:]
				}
			}
		}

		if _, err := ctx.Stdout.Write(result); err != nil {
			return 1
		}
	}

	if follow {
		type followState struct {
			name   string
			path   string
			offset int64
		}
		var fstates []followState
		for _, name := range files {
			if name == "-" {
				continue
			}
			path := name
			if !filepath.IsAbs(path) {
				path = filepath.Join(ctx.Dir, path)
			}
			off := int64(0)
			if st, err := os.Stat(path); err == nil {
				off = st.Size()
			}
			fstates = append(fstates, followState{name: name, path: path, offset: off})
		}

		if len(fstates) == 0 {
			return exitCode
		}

		lastHeaderIdx := 0
		if len(files) > 1 {
			lastHeaderIdx = len(files) - 1
		}

		for {
			select {
			case <-ctx.CancellationContext().Done():
				return ExitStatus(ctx.CancellationContext().Err())
			case <-time.After(50 * time.Millisecond):
			}
			for i := 0; i < len(fstates); i++ {
				if err := ctx.CancellationContext().Err(); err != nil {
					return ExitStatus(err)
				}
				idx := (lastHeaderIdx + i) % len(fstates)
				fs := &fstates[idx]
				st, err := os.Stat(fs.path)
				if err != nil {
					continue
				}
				for st.Size() > fs.offset {
					f, err := os.Open(fs.path)
					if err != nil {
						continue
					}
					f.Seek(fs.offset, io.SeekStart)
					buf := make([]byte, min(st.Size()-fs.offset, 32*1024))
					n, _ := ContextReader(ctx.CancellationContext(), f).Read(buf)
					f.Close()
					if n == 0 {
						break
					}
					if n > 0 {
						if len(fstates) > 1 && fstates[lastHeaderIdx].name != fs.name {
							hdr := fs.name
							if hdr == "-" {
								hdr = "standard input"
							} else {
								hdr = headQuoteString(hdr)
							}
							if _, err := fmt.Fprintf(ctx.Stdout, "\n==> %s <==\n", hdr); err != nil {
								return 1
							}
						}
						if _, err := ctx.Stdout.Write(buf[:n]); err != nil {
							return 1
						}
						fs.offset += int64(n)
						lastHeaderIdx = idx
					}
				}
			}
		}
	}
	return exitCode
}

func countSep(data []byte, sep byte) int {
	n := 0
	for _, b := range data {
		if b == sep {
			n++
		}
	}
	if len(data) > 0 && data[len(data)-1] != sep {
		n++
	}
	return n
}
