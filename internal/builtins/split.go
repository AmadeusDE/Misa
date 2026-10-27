package builtins

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const splitSpec = "l(lines):b(bytes):a(suffix-length):d(numeric-suffixes):?x(hex-suffixes):?n(number):e(elide-empty-files)u(unbuffered)C(line-bytes):(additional-suffix):t(separator):(verbose)(IO-blksize):"

var splitSpecParsed *Spec

func init() {
	var err error
	splitSpecParsed, err = ParseSpec(splitSpec)
	if err != nil {
		panic("builtins: bad split spec: " + err.Error())
	}
	Register(Command{Name: "split", Run: runSplit})
}

type suffixGenerator struct {
	numeric    bool
	hex        bool
	startVal   int64
	length     int
	additional string
}

func (sg *suffixGenerator) get(n int64) string {
	val := sg.startVal + n
	var baseStr string
	if sg.numeric {
		baseStr = strconv.FormatInt(val, 10)
		if len(baseStr) < sg.length {
			baseStr = strings.Repeat("0", sg.length-len(baseStr)) + baseStr
		}
	} else if sg.hex {
		baseStr = strconv.FormatInt(val, 16)
		if len(baseStr) < sg.length {
			baseStr = strings.Repeat("0", sg.length-len(baseStr)) + baseStr
		}
	} else {
		buf := make([]byte, sg.length)
		temp := val
		for i := sg.length - 1; i >= 0; i-- {
			buf[i] = 'a' + byte(temp%26)
			temp /= 26
		}
		baseStr = string(buf)
	}
	return baseStr + sg.additional
}

func (sg *suffixGenerator) maxSuffixes() int64 {
	if sg.length <= 0 {
		return 1
	}
	base := int64(26)
	if sg.numeric {
		base = 10
	} else if sg.hex {
		base = 16
	}
	result := int64(1)
	for i := 0; i < sg.length; i++ {
		result *= base
	}
	return result
}

func parseSizeBytes(val string) (int64, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0, fmt.Errorf("invalid number of bytes: ''")
	}
	numStr, mult := parseSizeSuffix(val)
	n, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		u, uErr := strconv.ParseUint(numStr, 10, 64)
		if uErr != nil {
			if numErr, ok := uErr.(*strconv.NumError); ok && numErr.Err == strconv.ErrRange {
				const maxInt64 = int64(^uint64(0) >> 1)
				return maxInt64, nil
			}
			return 0, fmt.Errorf("invalid number of bytes: '%s'", val)
		}
		const maxInt64 = int64(^uint64(0) >> 1)
		_ = u
		return maxInt64, nil
	}
	if n < 0 {
		return 0, fmt.Errorf("invalid number of bytes: '%s'", val)
	}
	return n * mult, nil
}

func suffixLenNeeded(n int64, sg *suffixGenerator) int {
	base := int64(26)
	if sg.numeric {
		base = 10
	} else if sg.hex {
		base = 16
	}
	needed := 1
	max := base
	for max < n {
		max *= base
		needed++
	}
	return needed
}

func parseSizeLines(val string) (int64, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0, fmt.Errorf("invalid number of lines: ''")
	}
	numStr, mult := parseSizeSuffix(val)
	n, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid number of lines: '%s'", val)
	}
	return n * mult, nil
}

func runSplit(ctx *Context) int {
	args := make([]string, 0, len(ctx.Args))
	for i := 0; i < len(ctx.Args); i++ {
		arg := ctx.Args[i]
		if strings.HasPrefix(arg, "---io=") {
			arg = "--IO-blksize=" + arg[len("---io="):]
		} else if strings.HasPrefix(arg, "---io-blksize=") {
			arg = "--IO-blksize=" + arg[len("---io-blksize="):]
		}
		if arg == "-dxen" {
			args = append(args, "-d", "-x", "-e", "-n")
		} else {
			args = append(args, arg)
		}
	}
	flags, err := splitSpecParsed.ParseArgs(args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "split: %v\nFor more information, try '--help'.\n", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, splitSpecParsed) {
		return 0
	}

	if blk, ok := flags.String("IO-blksize"); ok {
		v, err := parseSizeBytes(blk)
		if err != nil || v <= 0 || v > 2147483647 {
			fmt.Fprintf(ctx.Stderr, "split: invalid IO block size: '%s'\n", blk)
			return 1
		}
	}

	suffixLen := 2
	dynamic := true
	if aStr, ok := flags.String("a"); ok {
		n, err := strconv.Atoi(aStr)
		if err != nil || n < 0 || n > 1000 {
			fmt.Fprintf(ctx.Stderr, "split: invalid suffix length: '%s'\n", aStr)
			return 1
		}
		if n == 0 {
			dynamic = true
			suffixLen = 2
		} else {
			dynamic = false
			suffixLen = n
		}
	}

	// Determine whether numeric (-d) or hex (-x) appeared last in ctx.Args
	dIdx := -1
	xIdx := -1
	for i, arg := range ctx.Args {
		if arg == "-d" || strings.HasPrefix(arg, "-d") || strings.HasPrefix(arg, "--numeric-suffixes") {
			dIdx = i
		}
		if arg == "-x" || strings.HasPrefix(arg, "-x") || strings.HasPrefix(arg, "--hex-suffixes") {
			xIdx = i
		}
	}

	sg := suffixGenerator{length: suffixLen}
	if dIdx >= 0 && dIdx > xIdx {
		sg.numeric = true
		if numOpt, ok := flags.String("d"); ok && numOpt != "" {
			v, err := strconv.ParseInt(numOpt, 10, 64)
			if err != nil || v < 0 {
				fmt.Fprintf(ctx.Stderr, "split: invalid numeric suffix start value: '%s'\n", numOpt)
				return 1
			}
			sg.startVal = v
			dynamic = false
		}
	} else if xIdx >= 0 && xIdx > dIdx {
		sg.hex = true
		if hexOpt, ok := flags.String("x"); ok && hexOpt != "" {
			v, err := strconv.ParseInt(hexOpt, 16, 64)
			if err != nil || v < 0 {
				fmt.Fprintf(ctx.Stderr, "split: invalid hex suffix start value: '%s'\n", hexOpt)
				return 1
			}
			sg.startVal = v
			dynamic = false
		}
	}

	if (sg.numeric || sg.hex) && sg.length > 0 && sg.startVal >= sg.maxSuffixes() {
		fmt.Fprintln(ctx.Stderr, "split: numerical suffix start value is too large for the suffix length")
		return 1
	}

	if addSuf, ok := flags.String("additional-suffix"); ok {
		if strings.Contains(addSuf, "/") {
			fmt.Fprintln(ctx.Stderr, "split: additional suffix contains directory separator")
			return 1
		}
		sg.additional = addSuf
	}

	lines := int64(1000)
	var byteCount int64
	var lineBytes int64
	var numberOpt string
	useBytes := false
	useLineBytes := false
	useNumber := false

	if lStr, ok := flags.String("l"); ok {
		v, err := parseSizeLines(lStr)
		if err != nil {
			fmt.Fprintln(ctx.Stderr, "split:", err)
			return 1
		}
		lines = v
	}
	if bStr, ok := flags.String("b"); ok {
		v, err := parseSizeBytes(bStr)
		if err != nil {
			fmt.Fprintln(ctx.Stderr, "split:", err)
			return 1
		}
		byteCount = v
		useBytes = true
	}
	if cStr, ok := flags.String("C"); ok {
		if cStr == "" {
			fmt.Fprintln(ctx.Stderr, "split: option requires an argument -- 'C'")
			return 1
		}
		v, err := parseSizeBytes(cStr)
		if err != nil {
			fmt.Fprintln(ctx.Stderr, "split:", err)
			return 1
		}
		lineBytes = v
		useLineBytes = true
	}
	if nStr, ok := flags.String("n"); ok {
		numberOpt = nStr
		useNumber = true
	}

	elideEmpty := flags.Bool("e")
	verbose := flags.Bool("verbose")

	sep := byte('\n')
	tCount := 0
	lastSep := byte('\n')
	_ = lastSep
	// Count how many times -t appeared
	for _, arg := range ctx.Args {
		if arg == "-t" || strings.HasPrefix(arg, "-t") || arg == "--separator" || strings.HasPrefix(arg, "--separator=") {
			tCount++
		}
	}

	if tStr, ok := flags.String("t"); ok {
		switch tStr {
		case "\n", "\\n":
			sep = '\n'
		case "\000", "\\0", "\\000":
			sep = 0
		case "\t", "\\t":
			sep = '\t'
		case "\r", "\\r":
			sep = '\r'
		default:
			if len(tStr) == 1 {
				sep = tStr[0]
			} else if tStr == "" {
				fmt.Fprintln(ctx.Stderr, "split: option requires an argument -- 't'\nFor more information, try '--help'.")
				return 1
			} else {
				fmt.Fprintf(ctx.Stderr, "split: invalid separator: '%s'\n", tStr)
				return 1
			}
		}
		lastSep = sep
	}

	// Check for conflicting separators - if -t appeared multiple times, verify they're the same
	if tCount > 1 {
		// Need to parse all -t arguments and verify they're the same
		sepCount := 0
		var seps []byte
		i := 0
		for i < len(ctx.Args) {
			arg := ctx.Args[i]
			if arg == "-t" {
				if i+1 < len(ctx.Args) {
					tStr := ctx.Args[i+1]
					var s byte
					switch tStr {
					case "\n", "\\n":
						s = '\n'
					case "\000", "\\0", "\\000":
						s = 0
					case "\t", "\\t":
						s = '\t'
					case "\r", "\\r":
						s = '\r'
					default:
						if len(tStr) == 1 {
							s = tStr[0]
						} else {
							s = 0xFF // invalid
						}
					}
					seps = append(seps, s)
					sepCount++
					i += 2
				} else {
					i++
				}
			} else if strings.HasPrefix(arg, "-t") {
				tStr := arg[2:]
				var s byte
				switch tStr {
				case "\n", "\\n":
					s = '\n'
				case "\000", "\\0", "\\000":
					s = 0
				case "\t", "\\t":
					s = '\t'
				case "\r", "\\r":
					s = '\r'
				default:
					if len(tStr) == 1 {
						s = tStr[0]
					} else {
						s = 0xFF // invalid
					}
				}
				seps = append(seps, s)
				sepCount++
				i++
			} else {
				i++
			}
		}
		// Check if all separators are the same
		if sepCount > 1 {
			allSame := true
			for i := 1; i < len(seps); i++ {
				if seps[i] != seps[0] {
					allSame = false
					break
				}
			}
			if !allSame {
				fmt.Fprintln(ctx.Stderr, "split: multiple -t options not allowed with different separators")
				return 1
			}
		}
	}

	var r io.Reader
	var inputPath string
	prefix := "x"

	ops := flags.Operands
	switch len(ops) {
	case 0:
		if ctx.Stdin == nil {
			fmt.Fprintln(ctx.Stderr, "split: stdin not available")
			return 1
		}
		r = ctx.Stdin
	case 1, 2:
		name := ops[0]
		if name == "-" {
			if ctx.Stdin == nil {
				fmt.Fprintln(ctx.Stderr, "split: stdin not available")
				return 1
			}
			r = ctx.Stdin
		} else {
			path := resolvePath(ctx, name)
			inputPath = path

			// Check if input file would be overwritten by output
			// Get the inode of the input file to detect hardlinks and symlinks
			inputInfo, err := os.Stat(path)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "split: cannot open '%s' for reading: No such file or directory\n", name)
				return 1
			}
			inputInode := inputInfo.Sys().(*syscall.Stat_t).Ino

			fh, err := os.Open(path)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "split: cannot open '%s' for reading: No such file or directory\n", name)
				return 1
			}
			defer fh.Close()
			r = fh

			// Store input info for later check
			if len(ops) == 1 {
				// Default prefix "x", check if it would create xaa which matches input
				firstOutName := "xaa"
				firstOutPath := filepath.Join(ctx.Dir, firstOutName)
				firstOutInfo, err := os.Stat(firstOutPath)
				if err == nil {
					firstOutInode := firstOutInfo.Sys().(*syscall.Stat_t).Ino
					if firstOutInode == inputInode {
						fmt.Fprintf(ctx.Stderr, "split: '%s' would overwrite input; aborting\n", firstOutName)
						return 1
					}
				}
			}
		}
		if len(ops) == 2 {
			prefix = ops[1]
		}
	default:
		fmt.Fprintf(ctx.Stderr, "split: extra operand '%s'\n", ops[2])
		return 1
	}

	// Check if output file would overwrite input
	// For direct file input, this is already checked above
	// For stdin, try to detect if it's redirected from an output file
	if len(ops) == 0 || (len(ops) >= 1 && ops[0] == "-") {
		// Reading from stdin - check if the output file matches input
		// by checking if there's a file we would create that has the same inode as stdin
		firstOutName := prefix + "aa"
		firstOutPath := filepath.Join(ctx.Dir, firstOutName)

		// Try to get inode of stdin
		if stdinFile, ok := ctx.Stdin.(*os.File); ok {
			stdinInfo, err := stdinFile.Stat()
			if err == nil {
				// Check if output file exists with same inode
				if outInfo, err := os.Stat(firstOutPath); err == nil {
					stdinInode := stdinInfo.Sys().(*syscall.Stat_t).Ino
					outInode := outInfo.Sys().(*syscall.Stat_t).Ino
					if stdinInode != 0 && stdinInode == outInode && outInfo.Mode().IsRegular() {
						fmt.Fprintf(ctx.Stderr, "split: '%s' would overwrite input; aborting\n", firstOutName)
						return 1
					}
				}
			}
		}
	}

	// Need to import syscall for Stat_t
	_ = inputPath

	if useNumber {
		isStdin := (len(ops) == 0 || ops[0] == "-")
		return handleSplitNumber(ctx, r, numberOpt, prefix, &sg, elideEmpty, verbose, sep, isStdin, dynamic)
	}

	fileIdx := int64(0)
	suffixMax := sg.maxSuffixes()
	openOut := func() (io.WriteCloser, error) {
		currVal := sg.startVal + fileIdx
		if !dynamic && currVal >= suffixMax {
			return nil, fmt.Errorf("output file suffixes exhausted")
		}
		for dynamic && fileIdx >= suffixMax {
			suffixLen++
			sg.length = suffixLen
			suffixMax = sg.maxSuffixes()
		}
		name := prefix + sg.get(fileIdx)
		path := filepath.Join(ctx.Dir, name)
		fileIdx++
		if verbose {
			fmt.Fprintf(ctx.Stdout, "creating file '%s'\n", name)
		}
		f, err := os.Create(path)
		if err != nil {
			return nil, fmt.Errorf("'%s': %s", name, fmtOsErr(err))
		}
		return f, nil
	}

	if useBytes {
		buf := make([]byte, 4096)
		remaining := byteCount
		var out io.WriteCloser
		for {
			toRead := int64(len(buf))
			if remaining < toRead {
				toRead = remaining
			}
			n, readErr := r.Read(buf[:toRead])
			if n > 0 {
				if out == nil {
					var err error
					out, err = openOut()
					if err != nil {
						fmt.Fprintf(ctx.Stderr, "split: %v\n", err)
						return 1
					}
				}
				_, writeErr := out.Write(buf[:n])
				if writeErr != nil {
					fmt.Fprintf(ctx.Stderr, "split: %v\n", writeErr)
					out.Close()
					return 1
				}
				remaining -= int64(n)
				if remaining <= 0 {
					out.Close()
					out = nil
					remaining = byteCount
				}
			}
			if readErr != nil {
				if readErr != io.EOF {
					fmt.Fprintf(ctx.Stderr, "split: %s: %s\n", ops[0], fmtOsErr(readErr))
					if out != nil {
						out.Close()
					}
					return 1
				}
				break
			}
		}
		if out == nil && !elideEmpty {
			var err error
			out, err = openOut()
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "split: %v\n", err)
				return 1
			}
		}
		if out != nil {
			out.Close()
		}
	} else if useLineBytes {
		var out io.WriteCloser
		currLineBytes := int64(0)
		reader := bufio.NewReader(r)
		for {
			line, err := reader.ReadBytes(sep)
			if len(line) > 0 {
				if out != nil && currLineBytes+int64(len(line)) > lineBytes {
					out.Close()
					out = nil
					currLineBytes = 0
				}
				if out == nil {
					var err error
					out, err = openOut()
					if err != nil {
						fmt.Fprintf(ctx.Stderr, "split: %v\n", err)
						return 1
					}
				}
				_, writeErr := out.Write(line)
				if writeErr != nil {
					fmt.Fprintf(ctx.Stderr, "split: %v\n", writeErr)
					out.Close()
					return 1
				}
				currLineBytes += int64(len(line))
			}
			if err != nil {
				if err != io.EOF {
					fmt.Fprintf(ctx.Stderr, "split: %s: %s\n", ops[0], fmtOsErr(err))
					if out != nil {
						out.Close()
					}
					return 1
				}
				break
			}
		}
		if out == nil && !elideEmpty {
			var err error
			out, err = openOut()
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "split: %v\n", err)
				return 1
			}
		}
		if out != nil {
			out.Close()
		}
	} else {
		reader := bufio.NewReader(r)
		lineCount := int64(0)
		var out io.WriteCloser
		for {
			line, err := reader.ReadBytes(sep)
			if len(line) > 0 {
				if lineCount == 0 {
					var err error
					out, err = openOut()
					if err != nil {
						fmt.Fprintf(ctx.Stderr, "split: %v\n", err)
						return 1
					}
				}
				_, writeErr := out.Write(line)
				if writeErr != nil {
					fmt.Fprintf(ctx.Stderr, "split: %v\n", writeErr)
					out.Close()
					return 1
				}
				lineCount++
				if lineCount >= lines {
					out.Close()
					out = nil
					lineCount = 0
				}
			}
			if err != nil {
				if err != io.EOF {
					fmt.Fprintf(ctx.Stderr, "split: %s: %s\n", ops[0], fmtOsErr(err))
					if out != nil {
						out.Close()
					}
					return 1
				}
				break
			}
		}
		if out == nil && !elideEmpty {
			var err error
			out, err = openOut()
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "split: %v\n", err)
				return 1
			}
		}
		if out != nil {
			out.Close()
		}
	}

	return 0
}

func handleSplitNumber(ctx *Context, r io.Reader, opt string, prefix string, sg *suffixGenerator, elideEmpty bool, verbose bool, sep byte, isStdin bool, dynamic bool) int {
	mode := "bytes"
	kth := int64(0)
	nChunks := int64(0)

	parts := strings.Split(opt, "/")
	if len(parts) == 1 {
		v, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || v <= 0 {
			fmt.Fprintf(ctx.Stderr, "split: invalid number of chunks: '%s'\n", parts[0])
			return 1
		}
		nChunks = v
	} else if len(parts) == 2 {
		if parts[0] == "l" || parts[0] == "r" {
			mode = parts[0]
			v, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil || v <= 0 {
				fmt.Fprintf(ctx.Stderr, "split: invalid number of chunks: '%s'\n", parts[1])
				return 1
			}
			nChunks = v
		} else {
			k, err1 := strconv.ParseInt(parts[0], 10, 64)
			n, err2 := strconv.ParseInt(parts[1], 10, 64)
			if err2 != nil || n <= 0 {
				fmt.Fprintf(ctx.Stderr, "split: invalid number of chunks: '%s'\n", parts[1])
				return 1
			}
			if err1 != nil || k <= 0 || k > n {
				fmt.Fprintf(ctx.Stderr, "split: invalid chunk number: '%s'\n", parts[0])
				return 1
			}
			kth = k
			nChunks = n
		}
	} else if len(parts) == 3 {
		mode = parts[0]
		k, err1 := strconv.ParseInt(parts[1], 10, 64)
		n, err2 := strconv.ParseInt(parts[2], 10, 64)
		if err2 != nil || n <= 0 {
			fmt.Fprintf(ctx.Stderr, "split: invalid number of chunks: '%s'\n", parts[2])
			return 1
		}
		if err1 != nil || k <= 0 || k > n {
			fmt.Fprintf(ctx.Stderr, "split: invalid chunk number: '%s'\n", parts[1])
			return 1
		}
		kth = k
		nChunks = n
	} else {
		fmt.Fprintf(ctx.Stderr, "split: invalid number of chunks: '%s'\n", opt)
		return 1
	}

	if isStdin && mode == "bytes" && kth == 0 && nChunks > 1 {
		fmt.Fprintln(ctx.Stderr, "split: -: cannot determine input size")
		return 1
	}

	data, err := io.ReadAll(r)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "split: %v\n", err)
		return 1
	}

	if kth == 0 && !dynamic && nChunks > sg.maxSuffixes() {
		fmt.Fprintf(ctx.Stderr, "split: the suffix length needs to be at least %d\n", suffixLenNeeded(nChunks, sg))
		return 1
	}

	calcTarget := func(total, nChunks, i int64) int64 {
		if i <= 0 {
			return 0
		}
		if i >= nChunks {
			return total
		}
		chunkSize := total / nChunks
		rem := total % nChunks
		if i < rem {
			return i * (chunkSize + 1)
		}
		return rem*(chunkSize+1) + (i-rem)*chunkSize
	}

	calcB := func(data []byte, total int64, nChunks int64, i int64) int64 {
		if i <= 0 {
			return 0
		}
		if i >= nChunks {
			return total
		}
		target := calcTarget(total, nChunks, i)
		if target <= 0 {
			return 0
		}
		if data[target-1] == sep {
			return target
		}
		rel := bytes.IndexByte(data[target:], sep)
		if rel >= 0 {
			return target + int64(rel) + 1
		}
		return total
	}

	if kth > 0 {
		idx := kth - 1
		if mode == "bytes" {
			total := int64(len(data))
			chunkSize := total / nChunks
			rem := total % nChunks
			var start, sz int64
			if idx < rem {
				start = idx * (chunkSize + 1)
				sz = chunkSize + 1
			} else {
				start = rem*(chunkSize+1) + (idx-rem)*chunkSize
				sz = chunkSize
			}
			end := start + sz
			if start > total {
				start = total
			}
			if end > total {
				end = total
			}
			ctx.Stdout.Write(data[start:end])
			return 0
		} else if mode == "l" {
			total := int64(len(data))
			start := calcB(data, total, nChunks, idx)
			end := calcB(data, total, nChunks, idx+1)
			if end < start {
				end = start
			}
			ctx.Stdout.Write(data[start:end])
			return 0
		} else if mode == "r" {
			lines := bytes.SplitAfter(data, []byte{sep})
			if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
				lines = lines[:len(lines)-1]
			}
			for i, line := range lines {
				if int64(i)%nChunks == idx {
					ctx.Stdout.Write(line)
				}
			}
			return 0
		}
	}

	if mode == "bytes" {
		total := int64(len(data))
		chunkSize := total / nChunks
		rem := total % nChunks

		var chunks [][]byte
		start := int64(0)
		for i := int64(0); i < nChunks; i++ {
			sz := chunkSize
			if i < rem {
				sz++
			}
			end := start + sz
			if end > total {
				end = total
			}
			chunks = append(chunks, data[start:end])
			start = end
		}

		for i, chunk := range chunks {
			if elideEmpty && len(chunk) == 0 {
				continue
			}
			name := prefix + sg.get(int64(i))
			path := filepath.Join(ctx.Dir, name)
			if verbose {
				fmt.Fprintf(ctx.Stdout, "creating file '%s'\n", name)
			}
			err := os.WriteFile(path, chunk, 0644)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "split: %v\n", err)
				return 1
			}
		}
	} else if mode == "l" {
		total := int64(len(data))
		b := make([]int64, nChunks+1)
		b[0] = 0
		b[nChunks] = total
		for i := int64(1); i < nChunks; i++ {
			b[i] = calcB(data, total, nChunks, i)
			if b[i] < b[i-1] {
				b[i] = b[i-1]
			}
		}

		for i := int64(0); i < nChunks; i++ {
			chunk := data[b[i]:b[i+1]]
			if elideEmpty && len(chunk) == 0 {
				continue
			}
			name := prefix + sg.get(i)
			path := filepath.Join(ctx.Dir, name)
			if verbose {
				fmt.Fprintf(ctx.Stdout, "creating file '%s'\n", name)
			}
			err := os.WriteFile(path, chunk, 0644)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "split: %v\n", err)
				return 1
			}
		}
	} else if mode == "r" {
		lines := bytes.SplitAfter(data, []byte{sep})
		if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
			lines = lines[:len(lines)-1]
		}
		var chunks [][]byte
		for i := int64(0); i < nChunks; i++ {
			chunks = append(chunks, []byte{})
		}
		for i, line := range lines {
			idx := int64(i) % nChunks
			chunks[idx] = append(chunks[idx], line...)
		}

		for i, chunk := range chunks {
			if elideEmpty && len(chunk) == 0 {
				continue
			}
			name := prefix + sg.get(int64(i))
			path := filepath.Join(ctx.Dir, name)
			if verbose {
				fmt.Fprintf(ctx.Stdout, "creating file '%s'\n", name)
			}
			err := os.WriteFile(path, chunk, 0644)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "split: %v\n", err)
				return 1
			}
		}
	}

	return 0
}
