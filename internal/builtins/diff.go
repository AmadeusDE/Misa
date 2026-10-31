// Behaviour adapted from uutils diffutils (MIT/Apache-2.0).
package builtins

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func init() { Register(Command{Name: "diff", Run: diffRun}) }

func diffRun(ctx *Context) (code int) {
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	if ctx.CancellationContext().Err() != nil {
		return 0
	}
	opts, flags, err := diffParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, err)
		return 2
	}
	if HandleHelpVersion(ctx, flags, diffSpec) {
		return 0
	}
	opts.commandFlags = diffCommandFlags(ctx.Args)
	locale := ""
	for _, key := range []string{"LANG", "LC_TIME", "LC_ALL"} {
		value := ctx.Getenv(key)
		if value != "" {
			locale = value
		}
	}
	opts.contextPOSIXHeaders = locale == "C" || locale == "POSIX" || locale == ""
	for _, name := range opts.excludeFiles {
		data, err := diffRead(ctx, name)
		if err != nil {
			diffReadError(ctx, name, err)
			return 2
		}
		for _, pattern := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
			opts.excludes = append(opts.excludes, pattern)
		}
	}
	a, b := flags.Operands[0], flags.Operands[1]
	ai, ae := os.Stat(resolvePath(ctx, a))
	bi, be := os.Stat(resolvePath(ctx, b))
	if a != "-" && b != "-" && ae == nil && be == nil {
		if ai.IsDir() && !bi.IsDir() {
			a = filepath.Join(a, filepath.Base(b))
		} else if !ai.IsDir() && bi.IsDir() {
			b = filepath.Join(b, filepath.Base(a))
		}
	}
	return diffCompare(ctx, a, b, opts, false, map[string]bool{})
}

func diffCompare(ctx *Context, a, b string, o diffOptions, nested bool, seen map[string]bool) int {
	if ctx.CancellationContext().Err() != nil {
		return 0
	}
	ai, ae := os.Stat(resolvePath(ctx, a))
	bi, be := os.Stat(resolvePath(ctx, b))
	if a == "-" && b == "-" || a != "-" && b != "-" && ae == nil && be == nil && os.SameFile(ai, bi) {
		return diffIdentical(ctx, a, b, o)
	}
	// GNU treats a right-hand directory as a file when the top-level left stat fails.
	if !nested && os.IsNotExist(ae) && (o.newFile || o.unidirectional) && bi != nil && bi.IsDir() {
		diffReadError(ctx, b, fmt.Errorf("Is a directory"))
		return 2
	}
	if a != "-" && ae == nil && ai.IsDir() || b != "-" && be == nil && bi.IsDir() {
		return diffDirectory(ctx, a, b, ai, bi, ae, be, o, seen)
	}
	ad, errA := diffRead(ctx, a)
	bd, errB := diffRead(ctx, b)
	bothMissing := os.IsNotExist(errA) && os.IsNotExist(errB)
	if !bothMissing && os.IsNotExist(errA) && (o.newFile || o.unidirectional) {
		ad, errA = nil, nil
	}
	if !bothMissing && os.IsNotExist(errB) && o.newFile {
		bd, errB = nil, nil
	}
	if errA != nil || errB != nil {
		if errA != nil {
			diffReadError(ctx, a, errA)
		}
		if errB != nil {
			diffReadError(ctx, b, errB)
		}
		return 2
	}
	al, bl := diffLines(ad), diffLines(bd)
	if o.stripCR {
		for _, lines := range [][]diffLine{al, bl} {
			for i := range lines {
				if lines[i].newline {
					lines[i].text = strings.TrimSuffix(lines[i].text, "\r")
				}
			}
		}
	}
	if o.style == "ed" && (len(al) > 0 && !al[len(al)-1].newline || len(bl) > 0 && !bl[len(bl)-1].newline) {
		fmt.Fprintln(ctx.Stderr, "No newline at end of file")
		return 2
	}
	if o.brief && !o.ignoreBlank && len(o.ignoreMatching) == 0 {
		if diffEqual(al, bl, o) {
			return diffIdentical(ctx, a, b, o)
		}
		fmt.Fprintf(ctx.Stdout, "Files %s and %s differ\n", a, b)
		return 1
	}
	edits := diffMyers(al, bl, o)
	changed := len(diffVisibleChanges(al, bl, edits, o)) != 0
	if o.brief {
		if !changed {
			return diffIdentical(ctx, a, b, o)
		}
		fmt.Fprintf(ctx.Stdout, "Files %s and %s differ\n", a, b)
		return 1
	}
	if !changed && o.style != "side" {
		return diffIdentical(ctx, a, b, o)
	}
	var output string
	headerBI := bi
	if ai == nil && a != "-" && !nested {
		headerBI = nil
	}
	headers := [2]string{diffOptionHeader(ctx, a, ai, o), diffOptionHeader(ctx, b, headerBI, o)}
	for i, label := range o.labels {
		headers[i] = label
	}
	switch o.style {
	case "unified":
		output = diffUnified(al, bl, edits, o, headers)
	case "context":
		output = diffContext(al, bl, edits, o, headers)
	case "rcs":
		output = diffRCS(al, bl, edits, o)
	case "ed":
		var filtered []diffEdit
		for i := 0; i < len(edits); {
			if edits[i].kind == ' ' {
				filtered = append(filtered, edits[i])
				i++
				continue
			}
			j := i + 1
			for j < len(edits) && edits[j].kind != ' ' {
				j++
			}
			if len(diffVisibleChanges(al, bl, edits[i:j], o)) > 0 {
				filtered = append(filtered, edits[i:j]...)
			}
			i = j
		}
		var err error
		output, err = diffEd(al, bl, filtered)
		if err != nil {
			fmt.Fprintln(ctx.Stderr, err)
			return 2
		}
	case "side":
		output = diffSide(al, bl, edits, o)
	default:
		output = diffNormal(al, bl, edits, o)
	}
	if nested && changed {
		names := [2]string{a, b}
		for i, label := range o.labels {
			names[i] = label
		}
		fmt.Fprintf(ctx.Stdout, "diff %s%s %s\n", diffDirectoryFlags(o), diffNameQuote(names[0]), diffNameQuote(names[1]))
	}
	output = diffColor(output, o)
	if _, err := io.WriteString(ctx.Stdout, output); err != nil {
		fmt.Fprintln(ctx.Stderr, "diff:", err)
		return 2
	}
	if changed {
		return 1
	}
	return diffIdentical(ctx, a, b, o)
}

func diffIdentical(ctx *Context, a, b string, o diffOptions) int {
	if o.identical {
		fmt.Fprintf(ctx.Stdout, "Files %s and %s are identical\n", a, b)
	}
	return 0
}

func diffRead(ctx *Context, name string) ([]byte, error) {
	if name == "-" {
		if ctx.Stdin == nil {
			return nil, nil
		}
		return io.ReadAll(ctx.Stdin)
	}
	return ReadFileContext(ctx.CancellationContext(), resolvePath(ctx, name))
}

func diffReadError(ctx *Context, name string, err error) {
	message := err.Error()
	switch {
	case os.IsNotExist(err):
		message = "No such file or directory"
	case os.IsPermission(err):
		message = "Permission denied"
	default:
		if e, ok := err.(*os.PathError); ok {
			message = e.Err.Error()
		}
		if message != "" {
			message = strings.ToUpper(message[:1]) + message[1:]
		}
	}
	fmt.Fprintf(ctx.Stderr, "diff: %s: %s\n", name, message)
}

func diffHeader(name string, info os.FileInfo) string {
	stamp := time.Now()
	if info != nil {
		stamp = info.ModTime()
	}
	return name + "\t" + stamp.Format("2006-01-02 15:04:05.000000000 -0700")
}

func diffOptionHeader(ctx *Context, name string, info os.FileInfo, o diffOptions) string {
	stamp := time.Unix(0, 0)
	if info != nil {
		stamp = info.ModTime()
	} else if name == "-" {
		stamp = time.Now()
	}

	stamp = stamp.In(ctx.Location())
	if o.style == "context" && o.contextPOSIXHeaders {
		return name + "\t" + stamp.Format("Mon Jan _2 15:04:05 2006")
	}
	return name + "\t" + stamp.Format("2006-01-02 15:04:05.000000000 -0700")
}
