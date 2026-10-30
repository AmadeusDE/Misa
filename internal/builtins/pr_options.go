// Option parsing and validation for `pr`, adapted from uutils coreutils pr
// (MIT): src/uu/pr/src/pr.rs (build_options, recreate_arguments) and
// src/uu/pr/locales/en-US.ftl at commit fee61ff5d4f87e0f912a52e639942b5f7ad58663.
// See docs/CREDITS.md.
package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const prSpec = "^" +
	"h(header):" +
	"D(date-format):" +
	"d(double-space)" +
	"n(number-lines):?" +
	"N(first-line-number):" +
	"t(omit-header)" +
	"T(omit-pagination)" +
	"l(length):" +
	"r(no-file-warnings)" +
	"F(form-feed)" +
	"f" +
	"b" +
	"w(width):" +
	"W(page-width):" +
	"a(across)" +
	"(columns):" +
	"s(separator):?" +
	"S(sep-string):?" +
	"m(merge)" +
	"o(indent):" +
	"J(join-lines)" +
	"(pages):" +
	"e(expand-tabs):?"

var prSpecParsed *Spec

func init() {
	var err error
	prSpecParsed, err = ParseSpec(prSpec)
	if err != nil {
		panic("builtins: bad pr spec: " + err.Error())
	}
}

type prNumbering struct {
	width int
	sep   string
	first int
}

type prExpandTabs struct {
	char  byte
	width int
}

type prColumnMode struct {
	columns int
	width   int
	sep     string
	across  bool
}

type prOptions struct {
	number       *prNumbering
	header       string
	doubleSpace  bool
	contentSep   string
	date         string
	startPage    int
	endPage      int // 0 means "no end"
	showHeader   bool
	contentLines int
	pageSep      string
	columns      *prColumnMode
	mergeFiles   int // 0 means "not merging"
	offset       int
	formFeed     bool
	colSep       string
	lineWidth    int // -1 means unlimited
	expand       *prExpandTabs
}

var prNumberValueRe = regexp.MustCompile(`^[^-][0-9]*$`)

// prPreprocessArgs handles the two argument forms that the option parser cannot
// express: the legacy `-COLUMN` and `+FIRST[:LAST]` operands (removed from the
// argument list before parsing), and a bare `-n` whose value is a separate
// token. Everything else goes to the normal parser.
func prPreprocessArgs(argv []string) (args []string, column, page string) {
	args = append([]string(nil), argv...)
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			break
		}
		if args[i] == "-n" {
			// `-n` takes an optional value: a following token that could be
			// [SEP][DIGITS] is its value, anything else is a file operand.
			if i+1 < len(args) && prNumberValueRe.MatchString(args[i+1]) {
				args[i] = "-n" + args[i+1]
				args = append(args[:i+1], args[i+2:]...)
			}
			break
		}
	}

	out := make([]string, 0, len(args))
	pastTerminator := false
	for _, a := range args {
		if pastTerminator {
			out = append(out, a)
			continue
		}
		if a == "--" {
			pastTerminator = true
			out = append(out, a)
			continue
		}
		if column == "" {
			if digits, ok := prAsColumnOperand(a); ok {
				column = digits
				continue
			}
		}
		if page == "" {
			if spec, ok := prAsPageOperand(a); ok {
				page = spec
				continue
			}
		}
		out = append(out, a)
	}
	return out, column, page
}

// prAsColumnOperand returns the digits of a whole-token `-COLUMN` operand.
func prAsColumnOperand(arg string) (string, bool) {
	if len(arg) < 2 || arg[0] != '-' {
		return "", false
	}
	digits := arg[1:]
	for i := 0; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			return "", false
		}
	}
	return digits, true
}

// prAsPageOperand returns the range of a whole-token `+FIRST[:LAST]` operand.
func prAsPageOperand(arg string) (string, bool) {
	if len(arg) < 2 || arg[0] != '+' {
		return "", false
	}
	spec := arg[1:]
	isDigits := func(s string) bool {
		if s == "" {
			return false
		}
		for i := 0; i < len(s); i++ {
			if s[i] < '0' || s[i] > '9' {
				return false
			}
		}
		return true
	}
	if first, last, ok := strings.Cut(spec, ":"); ok {
		if isDigits(first) && isDigits(last) {
			return spec, true
		}
		return "", false
	}
	if isDigits(spec) {
		return spec, true
	}
	return "", false
}

// prParseUsize parses an unsigned option value the way uutils' parse_usize does:
// values above INT_MAX are "too large", anything unparsable is invalid.
func prParseUsize(message, raw string) (int, error) {
	n, err := strconv.ParseUint(raw, 10, 64)
	switch {
	case err == nil && n <= prMaxInt:
		return int(n), nil
	case err == nil, isRangeError(err):
		return 0, fmt.Errorf("%s: '%s': Value too large for defined data type", message, raw)
	default:
		return 0, fmt.Errorf("invalid -%s argument '%s'", message, raw)
	}
}

func isRangeError(err error) bool {
	ne, ok := err.(*strconv.NumError)
	return ok && ne.Err == strconv.ErrRange
}

func prTryHelp(msg string) error {
	return fmt.Errorf("%s\nTry 'pr --help' for more information.", msg)
}

func prParseNumbering(raw string, first int) (*prNumbering, error) {
	invalid := func(arg string) error {
		return prTryHelp(fmt.Sprintf("'-n' extra characters or invalid number in the argument: ‘%s’", arg))
	}
	tooLarge := func(arg string) error {
		return prTryHelp(fmt.Sprintf("'-n' extra characters or invalid number in the argument: '%s': Value too large for defined data type", arg))
	}

	n, err := strconv.ParseUint(raw, 10, 64)
	switch {
	case err == nil && n <= prMaxInt:
		return &prNumbering{width: int(n), sep: "\t", first: first}, nil
	case err == nil, isRangeError(err):
		return nil, tooLarge(raw)
	}

	if raw == "" || raw[0] >= 0x80 {
		return nil, invalid(raw)
	}
	num := &prNumbering{width: 5, sep: string(raw[0]), first: first}
	digits := raw[1:]
	d, err := strconv.ParseUint(digits, 10, 64)
	switch {
	case err == nil && d <= prMaxInt:
		num.width = int(d)
	case err == nil, isRangeError(err):
		return nil, tooLarge(digits)
	}
	return num, nil
}

func prParseExpandTabs(raw string) (*prExpandTabs, error) {
	invalid := func(arg string) error {
		return prTryHelp(fmt.Sprintf("'-e' extra characters or invalid number in the argument: ‘%s’", arg))
	}
	if raw == "" {
		return &prExpandTabs{char: '\t', width: 8}, nil
	}
	c := raw[0]
	if c >= '0' && c <= '9' {
		w, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || w <= 0 {
			return nil, invalid(raw)
		}
		return &prExpandTabs{char: '\t', width: int(w)}, nil
	}
	if c >= 0x80 {
		return nil, invalid(raw)
	}
	if len(raw) > 1 {
		w, err := strconv.ParseInt(raw[1:], 10, 32)
		if err != nil || w <= 0 {
			return nil, invalid(raw[1:])
		}
		return &prExpandTabs{char: c, width: int(w)}, nil
	}
	return &prExpandTabs{char: c, width: 8}, nil
}

func buildPrOptions(ctx *Context, flags *ParseResult, files []string, columnOperand, pageOperand string) (*prOptions, error) {
	formFeedUsed := flags.Bool("F") || flags.Bool("f")
	isMerge := flags.Bool("m")
	_, columnsGiven := flags.ExplicitString("columns")

	if isMerge && columnsGiven {
		return nil, fmt.Errorf("cannot specify number of columns when printing in parallel")
	}
	if isMerge && flags.Bool("a") {
		return nil, fmt.Errorf("cannot specify both printing across and printing in parallel")
	}

	mergeFiles := 0
	if isMerge {
		mergeFiles = len(files)
	}

	header := ""
	if h, ok := flags.ExplicitString("h"); ok {
		header = h
	} else if !isMerge && files[0] != "-" {
		header = files[0]
	}

	firstNumber := 1
	if raw, ok := flags.ExplicitString("N"); ok {
		n, err := prParseUsize("'-N NUMBER' invalid starting line number", raw)
		if err != nil {
			return nil, err
		}
		firstNumber = n
	}

	var number *prNumbering
	if raw, ok := flags.ExplicitString("n"); ok {
		num, err := prParseNumbering(raw, firstNumber)
		if err != nil {
			return nil, err
		}
		number = num
	} else if flags.Bool("n") {
		number = &prNumbering{width: 5, sep: "\t", first: firstNumber}
	}

	var expand *prExpandTabs
	if raw, ok := flags.ExplicitString("e"); ok {
		ex, err := prParseExpandTabs(raw)
		if err != nil {
			return nil, err
		}
		expand = ex
	} else if flags.Bool("e") {
		expand = &prExpandTabs{char: '\t', width: 8}
	}

	doubleSpace := flags.Bool("d")
	contentSep := "\n"
	if doubleSpace {
		contentSep = "\n\n"
	}

	dateFormat := ""
	if d, ok := flags.ExplicitString("D"); ok {
		dateFormat = d
	}
	date := prLastModified(ctx, files, isMerge, dateFormat)

	startPage, endPage, err := prParsePageRange(flags, pageOperand)
	if err != nil {
		return nil, err
	}

	defaultLines := prLinesPerPage
	if formFeedUsed {
		defaultLines = prLinesPerPageFormFeed
	}
	pageLength := defaultLines
	if raw, ok := flags.ExplicitString("l"); ok {
		n, err := prParseUsize("'-l PAGE_LENGTH' invalid number of lines", raw)
		if err != nil {
			return nil, err
		}
		pageLength = n
	}
	if pageLength == 0 {
		return nil, fmt.Errorf("invalid --length argument '0'")
	}

	// A page length of 10 or less leaves no room for a header and trailer, so
	// --help states it implies -t.
	pageLengthLeHeaderAndTrailer := pageLength <= prHeaderLines+prTrailerLines
	showHeader := !pageLengthLeHeaderAndTrailer && !flags.Bool("t") && !flags.Bool("T")
	contentLines := pageLength
	if !pageLengthLeHeaderAndTrailer {
		contentLines = pageLength - (prHeaderLines + prTrailerLines)
	}

	pageSep := "\n"
	if formFeedUsed {
		pageSep = "\f"
	}

	across := flags.Bool("a")

	colSep := "\t"
	if s, ok := flags.ExplicitString("S"); ok {
		colSep = s
	} else if s, ok := flags.ExplicitString("s"); ok {
		colSep = s
	} else if flags.Bool("S") {
		colSep = " "
	}

	// With both -w and -s the default width is 512 columns rather than 72.
	defaultColumnWidth := prDefaultColumnWidth
	if _, ok := flags.ExplicitString("w"); ok {
		if _, ok := flags.ExplicitString("s"); ok || flags.Bool("s") {
			defaultColumnWidth = prWidthWithSepOption
		}
	}
	columnWidth := defaultColumnWidth
	if raw, ok := flags.ExplicitString("w"); ok {
		n, err := prParseUsize("'-w PAGE_WIDTH' invalid number of characters", raw)
		if err != nil {
			return nil, err
		}
		columnWidth = n
	}
	if columnWidth == 0 {
		return nil, fmt.Errorf("invalid --width argument '0'")
	}

	pageWidth := -1
	if !flags.Bool("J") {
		if raw, ok := flags.ExplicitString("W"); ok {
			n, err := prParseUsize("'-W PAGE_WIDTH' invalid number of characters", raw)
			if err != nil {
				return nil, err
			}
			pageWidth = n
		}
	}
	if pageWidth == 0 {
		return nil, fmt.Errorf("invalid --page-width argument '0'")
	}

	columnCount := 0
	if columnOperand != "" {
		n, err := prParseUsize("invalid number of columns", columnOperand)
		if err != nil {
			return nil, err
		}
		columnCount = n
	}
	if columnCount == 0 && columnOperand != "" {
		return nil, fmt.Errorf("invalid --columns argument '0'")
	}
	if raw, ok := flags.ExplicitString("columns"); ok {
		n, err := prParseUsize("invalid number of columns", raw)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return nil, fmt.Errorf("invalid --columns argument '0'")
		}
		columnCount = n
	}

	var columns *prColumnMode
	if columnCount > 0 {
		columns = &prColumnMode{columns: columnCount, width: columnWidth, sep: colSep, across: across}
	}

	offset := 0
	if raw, ok := flags.ExplicitString("o"); ok {
		n, err := strconv.ParseInt(raw, 10, 32)
		switch {
		case err != nil && isRangeError(err) && strings.HasPrefix(raw, "-"):
			return nil, fmt.Errorf("'-o MARGIN' invalid line offset: '%s'", raw)
		case err != nil && isRangeError(err):
			return nil, fmt.Errorf("'-o MARGIN' invalid line offset: '%s': Value too large for defined data type", raw)
		case err != nil:
			return nil, fmt.Errorf("'-o MARGIN' invalid line offset: '%s'", raw)
		case n < 0:
			return nil, fmt.Errorf("'-o MARGIN' invalid line offset: '%s'", raw)
		}
		offset = int(n)
	}

	joinLines := flags.Bool("J")
	printSep := ""
	switch {
	case columns != nil:
		printSep = columns.sep
	case mergeFiles > 0:
		printSep = "\t"
	}
	columnsToPrint := 1
	if mergeFiles > 0 {
		columnsToPrint = mergeFiles
	} else if columns != nil {
		columnsToPrint = columns.columns
	}

	lineWidth := pageWidth
	switch {
	case joinLines:
		lineWidth = -1
	case columnsToPrint > 1:
		lineWidth = prDefaultColumnWidth
		if columns != nil {
			lineWidth = columns.width
		}
	}

	return &prOptions{
		number:       number,
		header:       header,
		doubleSpace:  doubleSpace,
		contentSep:   contentSep,
		date:         date,
		startPage:    startPage,
		endPage:      endPage,
		showHeader:   showHeader,
		contentLines: contentLines,
		pageSep:      pageSep,
		columns:      columns,
		mergeFiles:   mergeFiles,
		offset:       offset,
		formFeed:     formFeedUsed,
		colSep:       printSep,
		lineWidth:    lineWidth,
		expand:       expand,
	}, nil
}

func prParsePageRange(flags *ParseResult, pageOperand string) (start, end int, err error) {
	start, end = 1, 0
	if pageOperand != "" {
		first, last, hasLast := strings.Cut(pageOperand, ":")
		n, err := strconv.ParseUint(first, 10, 32)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid + argument '%s'", pageOperand)
		}
		start = int(n)
		if hasLast {
			m, err := strconv.ParseUint(last, 10, 32)
			if err != nil {
				return 0, 0, fmt.Errorf("invalid + argument '%s'", pageOperand)
			}
			end = int(m)
		}
	}
	if raw, ok := flags.ExplicitString("pages"); ok {
		first, last, hasLast := strings.Cut(raw, ":")
		value := func(s string) (int, error) {
			n, err := strconv.ParseUint(s, 10, 32)
			if err != nil || n == 0 {
				return 0, fmt.Errorf("invalid --pages argument '%s'", raw)
			}
			return int(n), nil
		}
		n, err := value(first)
		if err != nil {
			return 0, 0, err
		}
		start = n
		end = 0
		if hasLast {
			m, err := value(last)
			if err != nil {
				return 0, 0, err
			}
			end = m
		}
	}
	if end > 0 && start > end {
		return 0, 0, fmt.Errorf("invalid --pages argument '%d:%d'", start, end)
	}
	return start, end, nil
}

// prLastModified is the date shown in the page header: the first file's mtime,
// or the current time for stdin and for -m.
func prLastModified(ctx *Context, files []string, isMerge bool, format string) string {
	t := time.Now()
	if !isMerge && files[0] != "-" {
		path := files[0]
		if !filepath.IsAbs(path) {
			path = filepath.Join(ctx.Dir, path)
		}
		fi, err := os.Stat(path)
		if err != nil {
			return ""
		}
		t = fi.ModTime()
	}
	if format == "" {
		format = "%Y-%m-%d %H:%M"
		if prPosixDateEnv(ctx) {
			format = "%b %e %H:%M %Y"
		}
	}
	return prStrftime(t, format)
}

// prPosixDateEnv reports whether POSIXLY_CORRECT is set together with a POSIX
// locale in LC_TIME or LC_ALL, which selects the "%b %e %H:%M %Y" header date.
func prPosixDateEnv(ctx *Context) bool {
	if _, ok := lookupEnv(ctx, "POSIXLY_CORRECT"); !ok {
		return false
	}
	lcTime, _ := lookupEnv(ctx, "LC_TIME")
	lcAll, _ := lookupEnv(ctx, "LC_ALL")
	return lcTime == "POSIX" || lcAll == "POSIX"
}
