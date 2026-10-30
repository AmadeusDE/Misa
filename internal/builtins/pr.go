// pr — paginate or columnate FILE(s) for printing.
//
// Behaviour, page layout, option semantics and error messages are adapted from
// uutils coreutils pr (MIT): src/uu/pr/src/pr.rs, src/uu/pr/locales/en-US.ftl and
// tests/by-util/test_pr.rs at commit fee61ff5d4f87e0f912a52e639942b5f7ad58663.
// See docs/CREDITS.md.
//
// Differences from uutils are noted in docs/plans/builtins.md: input is streamed
// a page at a time instead of being read whole, the header date is the file mtime
// in local time (uutils formats it in UTC), and -i/--output-tabs is not
// implemented.
package builtins

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func init() { Register(Command{Name: "pr", Run: runPr}) }

const (
	prLinesPerPage         = 66
	prLinesPerPageFormFeed = 63
	prHeaderLines          = 5
	prTrailerLines         = 5
	prDefaultColumnWidth   = 72
	prWidthWithSepOption   = 512
	prFormFeedChar         = '\f'
	prMaxInt               = 2147483647
)

// prLine is one line of input annotated with its file, page and line number.
type prLine struct {
	fileID  int
	number  int
	content []byte
}

func runPr(ctx *Context) int {
	args, columnOperand, pageOperand := prPreprocessArgs(ctx.Args)
	flags, err := prSpecParsed.ParseArgs(args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "pr: %v\nTry 'pr --help' for more information.\n", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, prSpecParsed) {
		return 0
	}

	files := flags.Operands
	if len(files) == 0 {
		files = []string{"-"}
	}

	quiet := flags.Bool("r")
	groups := make([][]string, 0, len(files))
	if flags.Bool("m") {
		groups = append(groups, files)
	} else {
		for _, f := range files {
			groups = append(groups, []string{f})
		}
	}

	for _, group := range groups {
		opts, err := buildPrOptions(ctx, flags, group, columnOperand, pageOperand)
		if err != nil {
			if !quiet {
				fmt.Fprintf(ctx.Stderr, "pr: %v\n", err)
			}
			return 1
		}
		if err := prPrintGroup(ctx, group, opts); err != nil {
			if !quiet {
				fmt.Fprintf(ctx.Stderr, "pr: %v\n", err)
			}
			return 1
		}
	}
	return 0
}

// prPrintGroup paginates one group of files: one file normally, all files
// side by side with -m.
func prPrintGroup(ctx *Context, files []string, opts *prOptions) error {
	if opts.mergeFiles > 0 {
		return prPrintMerged(ctx, files, opts)
	}
	src, err := prOpenSource(ctx, files[0], 0, opts)
	if err != nil {
		return err
	}
	defer src.close()
	return prPrintSingle(ctx, src, opts)
}

func prPrintSingle(ctx *Context, src *prSource, opts *prOptions) error {
	emitted := 0
	lastFinal := false
	for {
		page, num, ok := src.next()
		if !ok {
			break
		}
		emitted = num + 1
		lastFinal = src.lastFinal
		if !prPageWanted(opts, num+1) {
			continue
		}
		if err := prWritePage(ctx.Stdout, page, opts, num+1); err != nil {
			return err
		}
	}
	if err := src.err(); err != nil {
		return err
	}
	// uutils counts the page still under construction even when it ends up
	// empty, so a file whose last page was filled exactly reports one page more
	// than it printed. That count is only visible in the warning below.
	total := emitted
	if !lastFinal {
		total++
	}
	if opts.startPage > total {
		fmt.Fprintf(ctx.Stderr, "pr: starting page number %d exceeds page count %d\n", opts.startPage, total)
	}
	return nil
}

func prPrintMerged(ctx *Context, files []string, opts *prOptions) error {
	srcs := make([]*prSource, len(files))
	for i, name := range files {
		src, err := prOpenSource(ctx, name, i, opts)
		if err != nil {
			for _, s := range srcs[:i] {
				if s != nil {
					s.close()
				}
			}
			return err
		}
		srcs[i] = src
	}
	defer func() {
		for _, s := range srcs {
			s.close()
		}
	}()

	pages := make([][]prLine, len(srcs))
	alive := make([]bool, len(srcs))
	for i, src := range srcs {
		pages[i], _, alive[i] = src.next()
	}
	for pageIdx := 0; ; pageIdx++ {
		any := false
		for _, a := range alive {
			if a {
				any = true
				break
			}
		}
		if !any {
			break
		}
		if prPageWanted(opts, pageIdx+1) {
			var lines []prLine
			for i := range srcs {
				if alive[i] {
					lines = append(lines, pages[i]...)
				}
			}
			if err := prWritePage(ctx.Stdout, lines, opts, pageIdx+1); err != nil {
				return err
			}
		}
		for i, src := range srcs {
			pages[i], _, alive[i] = src.next()
		}
	}
	for _, src := range srcs {
		if err := src.err(); err != nil {
			return err
		}
	}
	return nil
}

func prPageWanted(opts *prOptions, page int) bool {
	if page < opts.startPage {
		return false
	}
	if opts.endPage > 0 && page > opts.endPage {
		return false
	}
	return true
}

// prSource streams one file a page at a time. A page ends at a form feed or
// after linesPer lines, exactly like uutils' get_pages.
type prSource struct {
	sc        *bufio.Scanner
	fileID    int
	opts      *prOptions
	linesPer  int
	lineNum   int
	pageNum   int
	page      []prLine
	prev      byte // terminator of the previous token: '\n', '\f' or 0
	done      bool
	lastFinal bool // the last page returned came from the end-of-input flush
	name      string
	readErr   error
	file      *os.File
}

func prOpenSource(ctx *Context, name string, fileID int, opts *prOptions) (*prSource, error) {
	src := &prSource{fileID: fileID, opts: opts, linesPer: prLinesForPage(opts), name: name}
	var r io.Reader
	if name == "-" {
		if ctx.Stdin == nil {
			r = strings.NewReader("")
		} else {
			r = ctx.Stdin
		}
	} else {
		path := name
		if !filepath.IsAbs(path) {
			path = filepath.Join(ctx.Dir, path)
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %s", name, fmtOsErr(err))
		}
		src.file = f
		r = f
	}
	if opts.number != nil {
		src.lineNum = opts.number.first
	} else {
		src.lineNum = 1
	}
	src.sc = bufio.NewScanner(r)
	src.sc.Buffer(make([]byte, 0, 64*1024), 256*1024*1024)
	src.sc.Split(prScanLines)
	return src, nil
}

func (s *prSource) close() {
	if s.file != nil {
		s.file.Close()
		s.file = nil
	}
}

// err reports a read failure the way pr reports a file it cannot read:
// "pr: NAME: reason", or just the reason for stdin.
func (s *prSource) err() error {
	if s.readErr != nil {
		return s.readErr
	}
	err := s.sc.Err()
	if err == nil {
		return nil
	}
	if s.file != nil {
		s.readErr = fmt.Errorf("%s: %s", s.name, fmtOsErr(err))
	} else {
		s.readErr = fmt.Errorf("%s", fmtOsErr(err))
	}
	return s.readErr
}

// prScanLines is a bufio.SplitFunc that ends a token at a newline or a form
// feed and keeps the terminator in the token, so the caller can tell them apart.
func prScanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' || data[i] == prFormFeedChar {
			return i + 1, data[:i+1], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func (s *prSource) newLine(content []byte) prLine {
	ln := prLine{fileID: s.fileID, number: s.lineNum}
	if s.opts.expand != nil {
		ln.content = prExpandLine(nil, content, s.opts.expand)
	} else {
		ln.content = append([]byte(nil), content...)
	}
	return ln
}

func (s *prSource) emit(final bool) ([]prLine, int, bool) {
	page := s.page
	s.page = nil
	num := s.pageNum
	s.pageNum++
	s.lastFinal = final
	return page, num, true
}

// next returns the next page, its zero-based index and whether there was one.
func (s *prSource) next() ([]prLine, int, bool) {
	if s.done {
		return nil, 0, false
	}
	for s.sc.Scan() {
		tok := s.sc.Bytes()
		term := byte(0)
		content := tok
		if n := len(tok); n > 0 && (tok[n-1] == '\n' || tok[n-1] == prFormFeedChar) {
			term = tok[n-1]
			content = tok[:n-1]
		}
		if term == prFormFeedChar {
			// `\n\f`: the form feed does not open a line of its own.
			if !(len(content) == 0 && s.prev == '\n') {
				s.page = append(s.page, s.newLine(content))
			}
			s.prev = prFormFeedChar
			return s.emit(false)
		}
		if term == '\n' {
			// `\f\n`: the newline does not open a line of its own.
			if !(len(content) == 0 && s.prev == prFormFeedChar) {
				s.page = append(s.page, s.newLine(content))
				s.lineNum++
			}
			s.prev = '\n'
			if len(s.page) >= s.linesPer {
				return s.emit(false)
			}
			continue
		}
		// Last line without a terminator.
		s.page = append(s.page, s.newLine(content))
		s.prev = 0
	}
	s.done = true
	if len(s.page) > 0 {
		return s.emit(true)
	}
	return nil, 0, false
}

// prExpandLine expands input tabs (or the -e CHAR) to spaces, like uutils'
// apply_expand_tab: a plain TAB always uses a width of 8, other CHARs use the
// width from -e.
func prExpandLine(dst, line []byte, ex *prExpandTabs) []byte {
	for _, b := range line {
		switch {
		case b == ex.char:
			need := ex.width - len(dst)%ex.width
			for i := 0; i < need; i++ {
				dst = append(dst, ' ')
			}
		case b == '\t':
			need := 8 - len(dst)%8
			for i := 0; i < need; i++ {
				dst = append(dst, ' ')
			}
		default:
			dst = append(dst, b)
		}
	}
	return dst
}

func prWritePage(w io.Writer, lines []prLine, opts *prOptions, page int) error {
	if opts.showHeader {
		for _, h := range prHeaderContent(opts, page) {
			if _, err := io.WriteString(w, h); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
	}
	if err := prWriteColumns(w, lines, opts); err != nil {
		return err
	}
	if opts.showHeader && !opts.formFeed {
		for i := 0; i < prTrailerLines; i++ {
			if i > 0 {
				if _, err := io.WriteString(w, "\n"); err != nil {
					return err
				}
			}
		}
	}
	if opts.showHeader || opts.formFeed {
		if _, err := io.WriteString(w, opts.pageSep); err != nil {
			return err
		}
	}
	return nil
}

// prHeaderContent is the five-line page header: blank, blank, the header line
// (date, centered file name, "Page N"), blank, blank.
func prHeaderContent(opts *prOptions, page int) []string {
	datePart := opts.date
	filename := opts.header
	pagePart := "Page " + strconv.Itoa(page)

	total := opts.lineWidth
	if total < 0 {
		total = prDefaultColumnWidth
	}
	d, f, p := len([]rune(datePart)), len([]rune(filename)), len([]rune(pagePart))
	var line string
	if d+f+p+2 < total {
		spaceForName := total - d - p
		before := (spaceForName - f) / 2
		after := spaceForName - f - before
		line = datePart + strings.Repeat(" ", before) + filename + strings.Repeat(" ", after) + pagePart
	} else {
		line = datePart + " " + filename + " " + pagePart
	}
	return []string{"", "", line, "", ""}
}

// prWriteColumns lays the page's lines out in columns and prints them.
func prWriteColumns(w io.Writer, lines []prLine, opts *prOptions) error {
	contentRows := opts.contentLines
	if opts.doubleSpace {
		contentRows /= 2
	}
	columns := 1
	across := false
	if opts.mergeFiles > 0 {
		columns = opts.mergeFiles
	} else if opts.columns != nil {
		columns = opts.columns.columns
		across = opts.columns.across
	}

	table := prBuildTable(lines, opts, contentRows, columns, across)
	notFoundBreak := false
	for _, row := range table {
		indexes := 0
		if opts.mergeFiles > 0 {
			indexes = len(row)
		} else {
			for _, cell := range row {
				if cell == nil {
					break
				}
				indexes++
			}
		}
		cellsWritten := 0
		for i, cell := range row {
			var ln *prLine
			switch {
			case cell != nil:
				ln = cell
			case opts.mergeFiles > 0:
				ln = &prLine{}
			default:
				notFoundBreak = true
			}
			if ln == nil {
				break
			}
			if err := prWriteOffset(w, opts.offset); err != nil {
				return err
			}
			if _, err := io.WriteString(w, prLineForPrinting(opts, ln, columns, i, indexes)); err != nil {
				return err
			}
			cellsWritten++
		}
		// A partly filled last row keeps its content; only a row with nothing
		// at all ends the page, and then only when no trailer follows.
		if cellsWritten == 0 && notFoundBreak && (opts.formFeed || !opts.showHeader) {
			break
		}
		if _, err := io.WriteString(w, opts.contentSep); err != nil {
			return err
		}
	}
	return nil
}

func prBuildTable(lines []prLine, opts *prOptions, contentRows, columns int, across bool) [][]*prLine {
	switch {
	case opts.mergeFiles > 0:
		return prTableMerged(lines, contentRows, columns)
	case across:
		return prTableAcross(lines, contentRows, columns)
	case len(lines) < contentRows*columns:
		return prTableShortFile(lines, contentRows, columns)
	default:
		return prTableDown(lines, contentRows, columns)
	}
}

func prTableAcross(lines []prLine, contentRows, columns int) [][]*prLine {
	table := make([][]*prLine, 0, contentRows)
	for i := 0; i < contentRows; i++ {
		row := make([]*prLine, columns)
		for j := 0; j < columns; j++ {
			if idx := i*columns + j; idx < len(lines) {
				row[j] = &lines[idx]
			}
		}
		table = append(table, row)
	}
	return table
}

func prTableDown(lines []prLine, contentRows, columns int) [][]*prLine {
	table := make([][]*prLine, 0, contentRows)
	for i := 0; i < contentRows; i++ {
		row := make([]*prLine, columns)
		for j := 0; j < columns; j++ {
			if idx := contentRows*j + i; idx < len(lines) {
				row[j] = &lines[idx]
			}
		}
		table = append(table, row)
	}
	return table
}

// prTableShortFile spreads a partly filled page over the columns the way pr
// does: every column takes the ceiling of what is still unplaced divided by the
// number of columns left, so the leftmost columns are the long ones.
func prTableShortFile(lines []prLine, contentRows, columns int) [][]*prLine {
	bounds := make([]int, columns+1)
	placed := 0
	for c := 0; c < columns; c++ {
		placed += (len(lines) - placed + (columns - c) - 1) / (columns - c)
		bounds[c+1] = placed
	}
	rows := 0
	for c := 0; c < columns; c++ {
		if n := bounds[c+1] - bounds[c]; n > rows {
			rows = n
		}
	}
	table := make([][]*prLine, 0, contentRows)
	for i := 0; i < rows; i++ {
		row := make([]*prLine, columns)
		for j := 0; j < columns; j++ {
			if idx := bounds[j] + i; idx < bounds[j+1] {
				row[j] = &lines[idx]
			}
		}
		table = append(table, row)
	}
	for i := rows; i < contentRows; i++ {
		table = append(table, make([]*prLine, columns))
	}
	return table
}

func prTableMerged(lines []prLine, contentRows, columns int) [][]*prLine {
	filled := make([]*prLine, 0, contentRows*columns)
	offset := 0
	for col := 0; col < columns; col++ {
		inserted := 0
		for idx := offset; idx < len(lines); idx++ {
			if lines[idx].fileID != col {
				break
			}
			filled = append(filled, &lines[idx])
			inserted++
		}
		offset += inserted
		for i := inserted; i < contentRows; i++ {
			filled = append(filled, nil)
		}
	}
	table := make([][]*prLine, 0, contentRows)
	for i := 0; i < contentRows; i++ {
		row := make([]*prLine, columns)
		for j := 0; j < columns; j++ {
			if idx := contentRows*j + i; idx < len(filled) {
				row[j] = filled[idx]
			}
		}
		table = append(table, row)
	}
	return table
}

// prLineForPrinting formats one table cell: optional line number, content,
// padding, truncation and column separator.
//
// Width: the padding is computed from the byte length plus 7 bytes per tab
// (uutils' display-length approximation) and the cell is truncated to a rune
// count, which is what uutils does.
func prLineForPrinting(opts *prOptions, ln *prLine, columns, index, indexes int) string {
	complete := prFormatNumber(opts, ln.number, index) + string(ln.content)
	sep := ""
	if index+1 != indexes {
		sep = opts.colSep
	}
	if opts.lineWidth < 0 {
		return complete + sep
	}
	displayLength := len(complete) + strings.Count(complete, "\t")*7
	minWidth := (opts.lineWidth - (columns - 1)) / columns
	if minWidth < 0 {
		minWidth = 0
	}
	if displayLength < minWidth {
		complete += strings.Repeat(" ", minWidth-displayLength)
	}
	if r := []rune(complete); len(r) > minWidth {
		complete = string(r[:minWidth])
	}
	return complete + sep
}

func prFormatNumber(opts *prOptions, number, index int) string {
	if opts.number == nil || number == 0 {
		return ""
	}
	if opts.mergeFiles > 0 && index != 0 {
		return ""
	}
	digits := strconv.Itoa(number)
	width := opts.number.width
	if len(digits) > width {
		digits = digits[len(digits)-width:]
	}
	pad := width - len(digits)
	if pad < 0 {
		pad = 0
	}
	return strings.Repeat(" ", pad) + digits + opts.number.sep
}

// prWriteOffset writes the -o indent in fixed-size chunks so a huge margin does
// not allocate a huge string.
func prWriteOffset(w io.Writer, n int) error {
	const spaces = "                                                                "
	for n > 0 {
		chunk := n
		if chunk > len(spaces) {
			chunk = len(spaces)
		}
		if _, err := io.WriteString(w, spaces[:chunk]); err != nil {
			return err
		}
		n -= chunk
	}
	return nil
}

// prLinesForPage is how many input lines make up one page: the content lines
// times the columns, halved for -d.
func prLinesForPage(opts *prOptions) int {
	lines := opts.contentLines
	if opts.doubleSpace {
		lines /= 2
	}
	lines *= prColumnsFor(opts)
	if lines < 1 {
		lines = 1
	}
	return lines
}

func prColumnsFor(opts *prOptions) int {
	if opts.columns != nil {
		return opts.columns.columns
	}
	return 1
}
