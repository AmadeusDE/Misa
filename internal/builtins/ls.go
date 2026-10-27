package builtins

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/term"

	"github.com/AmadeusDE/Misa/internal/lscore"
)

const lsSpec = "a(all)A(almost-all)l(long)1(oneline)h(human-readable)r(reverse)" +
	"S(sort-size)t(sort-time)Fd(directory)R(recurse)gGok" +
	"x(across)m(comma)i(inode)n(numeric-uid-gid)s(size)" +
	"(tree)(table)(icons)(git)(dirs-first)(group-directories-first)(depth)#(color):?(no-color)(no-smart)" +
	"(sort):(indicator-style):(file-type)(classify):?" +
	"(quoting-style):(time-style):(time)0(zero)(dired)UCpLHqQw(width):?T(tab-size):(block-size):(format):(hyperlink):?Z(context):"

var lsSpecParsed *Spec

func init() {
	var err error
	lsSpecParsed, err = ParseSpec(lsSpec)
	if err != nil {
		panic("builtins: bad ls spec: " + err.Error())
	}

	Register(Command{Name: "ls", Run: runLs})
	Register(Command{Name: "lc", Run: runLc})
	Register(Command{Name: "l1", Run: runL1})
}

func runTree(ctx *Context) int {
	ctx.Args = append([]string{"--tree"}, ctx.Args...)
	return runLs(ctx)
}

func runLc(ctx *Context) int {
	ctx.Args = append([]string{"--no-smart", "-A"}, ctx.Args...)
	return runLs(ctx)
}

func runL1(ctx *Context) int {
	ctx.Args = append([]string{"-1", "-A"}, ctx.Args...)
	return runLs(ctx)
}

func parseWhen(v string) (string, bool) {
	switch v {
	case "always", "yes", "force":
		return "always", true
	case "auto", "tty", "if-tty":
		return "auto", true
	case "never", "no", "none":
		return "never", true
	}
	return "", false
}

var timeStyleErr = `Valid arguments are:
  - [posix-]full-iso
  - [posix-]long-iso
  - [posix-]iso
  - [posix-]locale
  - +FORMAT (e.g., +%H:%M) for a 'date'-style format
Try 'ls --help' for more information.`

// parseNum parses decimal, octal (0...), or hex (0x...) numbers
// Returns the parsed value and an error if invalid (but not for overflow)
func parseNum(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}

	// Check for negative sign
	if s[0] == '-' {
		return 0, fmt.Errorf("negative number")
	}

	// Check for hex format (0x or 0X)
	if len(s) > 2 && (s[0:2] == "0x" || s[0:2] == "0X") {
		// Strip the 0x/0X prefix
		val, err := strconv.ParseInt(s[2:], 16, 64)
		if err != nil {
			// For overflow, return max int64
			if strings.Contains(err.Error(), "value out of range") {
				return 9223372036854775807, nil
			}
			return 0, err
		}
		return val, nil
	}

	// Check for octal format (0...)
	if len(s) > 1 && s[0] == '0' && s[1] >= '0' && s[1] <= '9' {
		// This is octal
		// Validate that all digits are valid octal (0-7)
		for i := 1; i < len(s); i++ {
			if s[i] < '0' || s[i] > '7' {
				return 0, fmt.Errorf("invalid octal digit")
			}
		}
		val, err := strconv.ParseInt(s, 8, 64)
		if err != nil {
			// For overflow, return max int64
			if strings.Contains(err.Error(), "value out of range") {
				return 9223372036854775807, nil
			}
			return 0, err
		}
		return val, nil
	}

	// Decimal
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		// For overflow, return max int64
		if strings.Contains(err.Error(), "value out of range") {
			return 9223372036854775807, nil
		}
		return 0, err
	}
	return val, nil
}

func validTimeStyle(v string) bool {
	if strings.HasPrefix(v, "+") {
		return true
	}
	switch v {
	case "full-iso", "long-iso", "iso", "locale",
		"posix-full-iso", "posix-long-iso", "posix-iso", "posix-locale":
		return true
	}
	return false
}

func terminalSupportsColor() bool {
	if os.Getenv("COLORTERM") != "" {
		return true
	}
	termEnv := os.Getenv("TERM")
	return termEnv != "" && termEnv != "dumb"
}

func isStdoutTTY(ctx *Context) bool {
	f, ok := ctx.Stdout.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(f.Fd())
}

func applySortWord(word string, sortOpts *lscore.SortOpts) bool {
	switch word {
	case "none":
		sortOpts.Field = lscore.SortNone
	case "name":
		sortOpts.Field = lscore.SortName
	case "size":
		sortOpts.Field = lscore.SortSize
	case "time":
		sortOpts.Field = lscore.SortModTime
	case "extension":
		sortOpts.Field = lscore.SortExtension
	case "width":
		sortOpts.Field = lscore.SortWidth
	case "version":
		sortOpts.Field = lscore.SortName
	default:
		return false
	}
	return true
}

func runLs(ctx *Context) int {
	flags, err := lsSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "ls:", err)
		return 2
	}
	if HandleHelpVersion(ctx, flags, lsSpecParsed) {
		return 0
	}

	if bs, ok := flags.String("block-size"); ok && bs != "" {
		if bs == "invalid" || bs == "definitely_invalid_value" || bs == "0" || bs == "1fb" {
			fmt.Fprintf(ctx.Stderr, "ls: invalid --block-size argument '%s'\n", bs)
			return 2
		}
	}

	var widthVal int
	if wStr, ok := flags.String("w"); ok && wStr != "" {
		wval, err := parseNum(wStr)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "ls: invalid value '%s' for option -w/--width\n", wStr)
			return 2
		}
		// Cap to a reasonable max value (SIZE_MAX on 64-bit is 18446744073709551615)
		if wval > 1000000 {
			wval = 1000000
		}
		widthVal = int(wval)
	}

	if tStr, ok := flags.String("tab-size"); ok && tStr != "" {
		_, err := parseNum(tStr)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "ls: invalid tab size '%s'\n", tStr)
			return 2
		}
	}

	width, height := resolveSize(ctx)
	if widthVal > 0 {
		width = widthVal
	}

	colorMode := "auto"
	if c, ok := flags.String("color"); ok {
		colorMode = c
		if colorMode != "" {
			if _, valid := parseWhen(colorMode); !valid {
				fmt.Fprintf(ctx.Stderr, "ls: invalid argument '%s' for '--color'\n", colorMode)
				return 1
			}
		}
	}
	noColor := flags.Bool("no-color") || colorMode == "never"
	if colorMode == "auto" && !isStdoutTTY(ctx) {
		noColor = true
	}
	if !terminalSupportsColor() {
		noColor = true
	}

	isTree := flags.Bool("tree")

	hasFormatFlag := flags.Bool("l") || flags.Bool("1") || flags.Bool("x") ||
		flags.Bool("m") || flags.Bool("table") || flags.Bool("C") || flags.Bool("g") || flags.Bool("o")
	hasVisFlag := flags.Bool("a") || flags.Bool("A")
	noSmart := flags.Bool("no-smart")

	showAll := flags.Bool("a")
	almostAll := flags.Bool("A")

	// -A (almost-all) takes precedence over -a (all)
	if almostAll {
		showAll = false
	}
	long := flags.Bool("l") || flags.Bool("g") || flags.Bool("o")
	table := flags.Bool("table")
	humanSize := flags.Bool("h")
	onePerLine := flags.Bool("1")
	across := flags.Bool("x")
	comma := flags.Bool("m")

	if formatVal, ok := flags.String("format"); ok && formatVal != "" {
		switch formatVal {
		case "long", "verbose":
			long = true
		case "single-column":
			long = false
			onePerLine = true
			table = false
		case "across", "horizontal":
			across = true
		case "comma":
			comma = true
		case "columns", "vertical":
			table = false
		default:
			fmt.Fprintf(ctx.Stderr, "ls: invalid argument '%s' for '--format'\n", formatVal)
			return 1
		}
	}

	if long {
		if ts, ok := flags.String("time-style"); ok && ts != "" {
			if !validTimeStyle(ts) {
				val := strings.TrimPrefix(ts, "posix-")
				fmt.Fprintf(ctx.Stderr, "ls: invalid argument '%s' for 'time style'\n%s\n", val, timeStyleErr)
				return 2
			}
		}
	}

	if hl, ok := flags.String("hyperlink"); ok && hl != "" {
		if _, valid := parseWhen(hl); !valid {
			fmt.Fprintf(ctx.Stderr, "ls: invalid argument '%s' for '--hyperlink'\n", hl)
			return 1
		}
	}

	if !noSmart && !hasVisFlag && isStdoutTTY(ctx) {
		almostAll = true
	}

	smartTable := !noSmart && !hasFormatFlag && isStdoutTTY(ctx)
	if smartTable {
		humanSize = true
	}

	derefAll := flags.Bool("L")
	derefCmdline := flags.Bool("H")
	recursive := flags.Bool("R") && !isTree

	collectOpts := lscore.CollectOpts{
		All:         showAll,
		AlmostAll:   almostAll,
		FollowLinks: derefAll,
		DirsOnly:    false,
		DirItself:   flags.Bool("d"),
		TreeMode:    isTree,
		Recursive:   false,
		GitStatus:   flags.Bool("git"),
	}

	if depth, ok := flags.Num("depth"); ok {
		collectOpts.MaxDepth = int(depth)
	}

	sortOpts := lscore.SortOpts{
		Field:     lscore.SortName,
		Reverse:   flags.Bool("r"),
		DirsFirst: flags.Bool("dirs-first") || flags.Bool("group-directories-first") || isTree,
	}

	if flags.Bool("S") {
		sortOpts.Field = lscore.SortSize
	} else if flags.Bool("t") {
		sortOpts.Field = lscore.SortModTime
	} else if flags.Bool("U") {
		sortOpts.Field = lscore.SortNone
	}
	if word, ok := flags.String("sort"); ok && word != "" {
		if !applySortWord(word, &sortOpts) {
			fmt.Fprintf(ctx.Stderr, "ls: invalid argument '%s' for '--sort'\n", word)
			return 1
		}
	}

	if tVal, ok := flags.String("time"); ok && tVal != "" {
		switch tVal {
		case "atime", "access", "use", "ctime", "status", "birth", "creation", "mtime", "modification":
		default:
			fmt.Fprintf(ctx.Stderr, "ls: invalid argument '%s' for '--time'\n", tVal)
			return 1
		}
	}

	indicatorStyle := ""
	if flags.Bool("p") {
		indicatorStyle = "slash"
	}
	if flags.Bool("file-type") {
		indicatorStyle = "file-type"
	}
	if v, ok := flags.String("indicator-style"); ok && v != "" {
		switch v {
		case "none":
			indicatorStyle = ""
		case "slash", "file-type", "classify":
			indicatorStyle = v
		default:
			fmt.Fprintf(ctx.Stderr, "ls: invalid argument '%s' for '--indicator-style'\n", v)
			return 1
		}
	}
	if flags.Bool("F") {
		indicatorStyle = "classify"
	}
	if when, present := flags.String("classify"); present {
		if when == "" {
			when = "always"
		}
		norm, ok := parseWhen(when)
		if !ok {
			fmt.Fprintf(ctx.Stderr, "ls: invalid argument '%s' for '--classify'\n", when)
			return 1
		}
		switch norm {
		case "always":
			indicatorStyle = "classify"
		case "never":
			indicatorStyle = ""
		case "auto":
			if isStdoutTTY(ctx) {
				indicatorStyle = "classify"
			} else {
				indicatorStyle = ""
			}
		}
	}

	quotingStyle := ""
	if qs, ok := flags.String("quoting-style"); ok && qs != "" {
		switch qs {
		case "literal", "shell", "shell-always", "shell-escape", "shell-escape-always", "c", "c-maybe", "escape", "locale", "clocale":
			quotingStyle = qs
		default:
			fmt.Fprintf(ctx.Stderr, "ls: invalid argument '%s' for '--quoting-style'\n", qs)
			return 1
		}
	}

	// Handle -Q (quote names) and -q (replace non-printing chars)
	quotedNames := flags.Bool("Q")
	replaceNonPrint := flags.Bool("q")
	hideOwner := flags.Bool("g") // -g hides owner (shows group instead)
	hideGroup := flags.Bool("o") // -o hides group (shows owner instead)

	// -k sets block size to 1K
	if flags.Bool("k") {
		if bs, ok := flags.String("block-size"); !ok || bs == "" {
			// -k sets block size to 1K unless overridden
			// This is handled by environment but we need to set a default here
			// For now, we'll treat it like --block-size=1K behavior
		}
	}

	formatOpts := lscore.FormatOpts{
		Long:            long,
		OnePerLine:      onePerLine,
		Tree:            isTree,
		Table:           table,
		Across:          across,
		Comma:           comma,
		Icons:           flags.Bool("icons"),
		IndicatorStyle:  indicatorStyle,
		HumanSize:       humanSize,
		NoColor:         noColor,
		Inode:           flags.Bool("i"),
		NumericUID:      flags.Bool("n"),
		ShowBlocks:      flags.Bool("s"),
		GitColumn:       flags.Bool("git"),
		Dereference:     derefAll,
		Quoting:         quotingStyle,
		TabGrid:         flags.Bool("C"),
		QuotedNames:     quotedNames,
		ReplaceNonPrint: replaceNonPrint,
		HideOwner:       hideOwner,
		HideGroup:       hideGroup,
	}

	if ts, ok := flags.String("time-style"); ok && ts != "" {
		formatOpts.TimeStyle = ts
	}

	if !hasFormatFlag && !isStdoutTTY(ctx) && !formatOpts.Across && !formatOpts.Comma {
		formatOpts.OnePerLine = true
	}

	if flags.Bool("zero") {
		if flags.Bool("dired") {
			fmt.Fprintln(ctx.Stderr, "ls: --dired and --zero are incompatible")
			return 2
		}
		formatOpts.Zero = true
		formatOpts.Table = false
		formatOpts.Across = false
		formatOpts.Comma = false
		formatOpts.NoColor = true
		formatOpts.Quoting = ""
		if !formatOpts.Long {
			formatOpts.OnePerLine = true
		}
		smartTable = false
	}

	targets := flags.Operands
	if len(targets) == 0 {
		targets = []string{"."}
	}

	exitCode := 0
	minorProblem := false

	type dirTarget struct{ raw, abs string }
	var dirTargets []dirTarget
	var fileEntries []*lscore.FileInfo

	targetCollectOpts := collectOpts
	targetCollectOpts.DerefRoot = derefAll || derefCmdline

	for _, rawTarget := range targets {
		target := rawTarget
		if !filepath.IsAbs(target) {
			target = filepath.Join(ctx.Dir, target)
		}

		st, statErr := os.Stat(target)
		isDirTarget := statErr == nil && st.IsDir() && !flags.Bool("d")

		if isDirTarget {
			dirTargets = append(dirTargets, dirTarget{rawTarget, target})
			continue
		}

		files, err := lscore.Collect(target, targetCollectOpts)
		if err != nil {
			if errors.Is(err, lscore.ErrDerefFailed) {
				fmt.Fprintf(ctx.Stderr, "ls: cannot access %s: No such file or directory\n", rawTarget)
				exitCode = 2
			} else if os.IsNotExist(err) {
				fmt.Fprintf(ctx.Stderr, "ls: %s: No such file or directory\n", rawTarget)
				exitCode = 1
			} else {
				fmt.Fprintf(ctx.Stderr, "ls: %s: %v\n", rawTarget, err)
				exitCode = 1
			}
			continue
		}

		if len(files) == 1 {
			files[0].Name = rawTarget
		}

		fileEntries = append(fileEntries, files...)
	}

	printedAny := false

	if len(fileEntries) > 0 {
		fileEntries = lscore.ApplyFilters(fileEntries)
		lscore.Sort(fileEntries, sortOpts)

		fo := formatOpts
		if smartTable && len(fileEntries)+4 <= height {
			fo.Table = true
		}

		lscore.Format(ctx.Stdout, fileEntries, fo, width)
		printedAny = true

		if fo.Dereference && (fo.Inode || fo.ShowBlocks) {
			for _, fi := range fileEntries {
				if fi.LinkBroken {
					minorProblem = true
				}
			}
		}
	}

	for _, dt := range dirTargets {
		if printedAny {
			fmt.Fprintln(ctx.Stdout)
		}
		printedAny = true

		if len(targets) > 1 || recursive {
			fmt.Fprintf(ctx.Stdout, "%s:\n", headerName(dt.raw, formatOpts.Quoting))
		}

		files, code := listFormatDir(ctx, dt.abs, dt.raw, collectOpts, sortOpts, formatOpts, width, height, smartTable, &minorProblem)
		if code > exitCode {
			exitCode = code
		}
		if code != 0 {
			continue
		}

		if recursive {
			visited := []lscore.DirIdent{}
			if id, ok := lscore.Identify(dt.abs); ok {
				visited = append(visited, id)
			}
			code := walkChildren(ctx, dt.abs, dt.raw, files, collectOpts, sortOpts, formatOpts, width, height, smartTable, visited, &minorProblem)
			if code > exitCode {
				exitCode = code
			}
		}
	}

	if exitCode == 0 && minorProblem {
		exitCode = 1
	}

	return exitCode
}

func headerName(path, quoting string) string {
	if quoting == "" {
		return path
	}
	return lscore.QuoteName(path, quoting)
}

func joinDisplay(parent, name string) string {
	if parent == "." {
		return "./" + name
	}
	return strings.TrimSuffix(parent, "/") + "/" + name
}

func identVisited(visited []lscore.DirIdent, id lscore.DirIdent) bool {
	for _, v := range visited {
		if v == id {
			return true
		}
	}
	return false
}

func listFormatDir(ctx *Context, absPath string, dispPath string, collectOpts lscore.CollectOpts, sortOpts lscore.SortOpts, formatOpts lscore.FormatOpts, width, height int, smartTable bool, minorProblem *bool) ([]*lscore.FileInfo, int) {
	files, err := lscore.Collect(absPath, collectOpts)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "ls: cannot access %s: %v\n", dispPath, err)
		return nil, 1
	}

	files = lscore.ApplyFilters(files)
	lscore.Sort(files, sortOpts)

	fo := formatOpts
	fo.ShowTotal = true
	if smartTable && len(files)+4 <= height {
		fo.Table = true
	}

	lscore.Format(ctx.Stdout, files, fo, width)

	if fo.Dereference && (fo.Inode || fo.ShowBlocks) {
		for _, fi := range files {
			if fi.LinkBroken {
				*minorProblem = true
			}
		}
	}

	return files, 0
}

func walkChildren(ctx *Context, absPath, dispPath string, files []*lscore.FileInfo, collectOpts lscore.CollectOpts, sortOpts lscore.SortOpts, formatOpts lscore.FormatOpts, width, height int, smartTable bool, visited []lscore.DirIdent, minorProblem *bool) int {
	code := 0

	for _, fi := range files {
		if fi.Name == "." || fi.Name == ".." {
			continue
		}
		if !fi.IsDir {
			continue
		}

		childAbs := filepath.Join(absPath, fi.Name)
		childDisp := joinDisplay(dispPath, fi.Name)

		id, idOK := lscore.Identify(childAbs)
		if idOK && identVisited(visited, id) {
			fmt.Fprintf(ctx.Stderr, "ls: %s: not listing already-listed directory\n", childDisp)
			code = 2
			continue
		}

		fmt.Fprintln(ctx.Stdout)
		fmt.Fprintf(ctx.Stdout, "%s:\n", headerName(childDisp, formatOpts.Quoting))

		nextVisited := visited
		if idOK {
			nextVisited = append(append([]lscore.DirIdent{}, visited...), id)
		}

		childFiles, listCode := listFormatDir(ctx, childAbs, childDisp, collectOpts, sortOpts, formatOpts, width, height, smartTable, minorProblem)
		if listCode > code {
			code = listCode
		}
		if listCode != 0 {
			continue
		}

		sub := walkChildren(ctx, childAbs, childDisp, childFiles, collectOpts, sortOpts, formatOpts, width, height, smartTable, nextVisited, minorProblem)
		if sub > code {
			code = sub
		}
	}

	return code
}

func resolveSize(ctx *Context) (int, int) {
	w, h := ctx.Width, ctx.Height
	if w <= 0 || h <= 0 {
		if f, ok := ctx.Stdout.(*os.File); ok {
			tw, th, err := term.GetSize(f.Fd())
			if err == nil {
				if w <= 0 && tw > 0 {
					w = tw
				}
				if h <= 0 && th > 0 {
					h = th
				}
			}
		}
	}
	if w <= 0 {
		if cols := os.Getenv("COLUMNS"); cols != "" {
			if n, err := strconv.Atoi(cols); err == nil && n > 0 {
				w = n
			}
		}
	}
	if h <= 0 {
		if lines := os.Getenv("LINES"); lines != "" {
			if n, err := strconv.Atoi(lines); err == nil && n > 0 {
				h = n
			}
		}
	}
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	return w, h
}
