package lscore

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"unicode/utf8"
)

type FormatOpts struct {
	Long            bool
	OnePerLine      bool
	Grid            bool
	Across          bool
	Tree            bool
	Comma           bool
	Table           bool
	Icons           bool
	Classify        bool
	IndicatorStyle  string
	HumanSize       bool
	NoColor         bool
	Inode           bool
	NumericUID      bool
	ShowBlocks      bool
	GitColumn       bool
	ShowTotal       bool
	TabGrid         bool
	Dereference     bool
	Quoting         string
	Zero            bool
	Width           int
	QuotedNames     bool   // -Q: quote filenames
	ReplaceNonPrint bool   // -q: replace non-printing chars with ?
	HideOwner       bool   // -o: omit group name (for long format)
	HideGroup       bool   // -g: omit owner name (for long format)
	TimeStyle       string // --time-style format
}

func Format(w io.Writer, files []*FileInfo, opts FormatOpts, width int) {
	resetColorState()

	if opts.ShowTotal && (opts.Long || opts.ShowBlocks) {
		printTotal(w, files)
	}

	if len(files) == 0 {
		return
	}

	switch {
	case opts.Tree:
		formatTree(w, files, opts, "")
	case opts.Table:
		formatTable(w, files, opts, width)
	case opts.Long:
		formatLong(w, files, opts)
	case opts.OnePerLine:
		formatByLine(w, files, opts)
	case opts.Comma:
		formatComma(w, files, opts, width)
	case opts.Across:
		formatGridAcross(w, files, opts, width)
	default:
		formatGrid(w, files, opts, width)
	}
}

func printTotal(w io.Writer, files []*FileInfo) {
	var total int64
	for _, fi := range files {
		total += (fi.Blocks + 1) / 2
	}
	fmt.Fprintf(w, "total %d\n", total)
}

// entryPrefix renders the leading "-i inode" / "-s blocks" columns shared
// by every non-long, non-table format. A dangling symlink whose target
// could not be resolved (only possible when dereferencing, i.e. -L) shows
// "?" for the field that would have required the target's stat info.
func entryPrefix(fi *FileInfo, opts FormatOpts) string {
	var b strings.Builder
	if opts.Inode {
		b.WriteString(inodeCell(fi, opts))
		b.WriteByte(' ')
	}
	if opts.ShowBlocks {
		b.WriteString(blocksCell(fi, opts))
		b.WriteByte(' ')
	}
	return b.String()
}

func quoted(name string, opts FormatOpts) string {
	if opts.Quoting == "" {
		return name
	}
	return QuoteName(name, opts.Quoting)
}

func entryName(fi *FileInfo, opts FormatOpts) string {
	var b strings.Builder

	if opts.Icons {
		b.WriteString(Icon(fi))
		b.WriteByte(' ')
	}

	b.WriteString(ColorName(fi, opts, quoted(fi.Name, opts)))

	b.WriteString(indicatorChar(fi, opts))

	if opts.Long && fi.IsSymlink && fi.LinkTarget != "" {
		b.WriteString(" -> ")
		b.WriteString(quoted(fi.LinkTarget, opts))
	}

	return b.String()
}

func entryNamePlain(fi *FileInfo, opts FormatOpts) string {
	var b strings.Builder

	if opts.Icons {
		b.WriteString(Icon(fi))
		b.WriteByte(' ')
	}

	b.WriteString(quoted(fi.Name, opts))

	b.WriteString(indicatorChar(fi, opts))

	if opts.Long && fi.IsSymlink && fi.LinkTarget != "" {
		b.WriteString(" -> ")
		b.WriteString(quoted(fi.LinkTarget, opts))
	}

	return b.String()
}

// indicatorChar returns the trailing type indicator for a file, according
// to opts.IndicatorStyle ("", "slash", "file-type", "classify") or the
// legacy opts.Classify boolean (treated as "classify").
func indicatorChar(fi *FileInfo, opts FormatOpts) string {
	style := opts.IndicatorStyle
	if style == "" && opts.Classify {
		style = "classify"
	}
	switch style {
	case "slash":
		if fi.IsDir {
			return "/"
		}
		return ""
	case "file-type":
		return classifyChar(fi, false)
	case "classify":
		return classifyChar(fi, true)
	default:
		return ""
	}
}

func classifyChar(fi *FileInfo, includeExec bool) string {
	if fi.IsDir {
		return "/"
	}
	if fi.IsSymlink {
		return "@"
	}
	if fi.Mode&os.ModeNamedPipe != 0 {
		return "|"
	}
	if fi.Mode&os.ModeSocket != 0 {
		return "="
	}
	if includeExec && fi.Mode&0111 != 0 {
		return "*"
	}
	return ""
}

func formatByLine(w io.Writer, files []*FileInfo, opts FormatOpts) {
	term := "\n"
	if opts.Zero {
		term = "\x00"
	}
	for _, fi := range files {
		fmt.Fprint(w, entryPrefix(fi, opts)+entryName(fi, opts)+term)
	}
}

// formatComma renders -m (comma-separated) output, wrapping to width like
// GNU ls: a trailing comma is reserved as part of every non-last item's
// placement decision, so a line breaks before an item that would push
// that item's own trailing comma past the width, not merely before an
// item whose own text doesn't fit. width<=0 means unbounded (no wrapping,
// and raw bytes such as embedded newlines are left untouched).
func formatComma(w io.Writer, files []*FileInfo, opts FormatOpts, width int) {
	unlimited := width <= 0

	if !unlimited && opts.Quoting == "" {
		files = sanitizeNames(files)
	}

	pos := 0
	for i, fi := range files {
		prefix := entryPrefix(fi, opts)
		plainW := displayWidth(prefix + entryNamePlain(fi, opts))
		display := prefix + entryName(fi, opts)
		isLast := i == len(files)-1

		if i == 0 {
			fmt.Fprint(w, display)
			pos = plainW
		} else {
			// For non-last items, reserve 1 char for the trailing comma
			reserve := 0
			if !isLast {
				reserve = 1 // for the comma after this item
			}
			need := pos + 1 + plainW + reserve // pos + space + item + comma
			if !unlimited && need > width {
				// Line wrap: newline before this item
				fmt.Fprint(w, "\n", display)
				pos = plainW
			} else {
				fmt.Fprint(w, " ", display)
				pos += 1 + plainW
			}
		}

		if !isLast {
			fmt.Fprint(w, ",")
			pos++
		}
	}
	fmt.Fprintln(w)
}

func sanitizeNames(files []*FileInfo) []*FileInfo {
	out := make([]*FileInfo, len(files))
	for i, fi := range files {
		if strings.ContainsAny(fi.Name, "\n\r\t") {
			clone := *fi
			clone.Name = sanitizeControl(fi.Name)
			out[i] = &clone
		} else {
			out[i] = fi
		}
	}
	return out
}

func displayWidth(s string) int {
	w := 0
	inEscape := false
	for i := 0; i < len(s); {
		if s[i] == '\033' {
			inEscape = true
			i++
			continue
		}
		if inEscape {
			if s[i] == 'm' {
				inEscape = false
			}
			i++
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		w++
		i += size
	}
	return w
}

func truncateAnsi(s string, maxWidth int) string {
	var b strings.Builder
	w := 0
	inEscape := false
	for i := 0; i < len(s); {
		if s[i] == '\033' {
			inEscape = true
			b.WriteByte(s[i])
			i++
			continue
		}
		if inEscape {
			b.WriteByte(s[i])
			if s[i] == 'm' {
				inEscape = false
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if w+1 > maxWidth {
			break
		}
		b.WriteRune(r)
		w++
		i += size
	}
	return b.String()
}

func formatGrid(w io.Writer, files []*FileInfo, opts FormatOpts, termWidth int) {
	if termWidth <= 0 {
		termWidth = 80
	}

	names := make([]string, len(files))
	widths := make([]int, len(files))
	for i, fi := range files {
		prefix := entryPrefix(fi, opts)
		names[i] = prefix + entryName(fi, opts)
		widths[i] = displayWidth(prefix + entryNamePlain(fi, opts))
	}

	n := len(files)
	bestCols := 1

	for cols := n; cols > 1; cols-- {
		rows := (n + cols - 1) / cols
		colWidths := make([]int, cols)

		for i := 0; i < n; i++ {
			col := i / rows
			if widths[i] > colWidths[col] {
				colWidths[col] = widths[i]
			}
		}

		total := 0
		for _, cw := range colWidths {
			total += cw + 2
		}
		total -= 2

		if total <= termWidth {
			bestCols = cols

			rows = (n + bestCols - 1) / bestCols
			for r := 0; r < rows; r++ {
				for c := 0; c < bestCols; c++ {
					idx := c*rows + r
					if idx >= n {
						continue
					}
					if c > 0 {
						if opts.TabGrid {
							fmt.Fprint(w, "\t")
						} else {
							fmt.Fprint(w, "  ")
						}
					}
					fmt.Fprint(w, names[idx])
					if !opts.TabGrid && c < bestCols-1 {
						nextIdx := (c+1)*rows + r
						if nextIdx < n {
							pad := colWidths[c] - widths[idx]
							fmt.Fprint(w, strings.Repeat(" ", pad))
						}
					}
				}
				fmt.Fprintln(w)
			}
			return
		}
	}

	for _, name := range names {
		fmt.Fprintln(w, name)
	}
}

func formatGridAcross(w io.Writer, files []*FileInfo, opts FormatOpts, termWidth int) {
	if termWidth <= 0 {
		termWidth = 80
	}

	names := make([]string, len(files))
	widths := make([]int, len(files))
	for i, fi := range files {
		prefix := entryPrefix(fi, opts)
		names[i] = prefix + entryName(fi, opts)
		widths[i] = displayWidth(prefix + entryNamePlain(fi, opts))
	}

	n := len(files)

	maxW := 0
	for _, w := range widths {
		if w > maxW {
			maxW = w
		}
	}
	cols := termWidth / (maxW + 2)
	if cols < 1 {
		cols = 1
	}

	colW := maxW
	for i := 0; i < n; i++ {
		if i > 0 && i%cols == 0 {
			fmt.Fprintln(w)
		} else if i > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprint(w, names[i])
		if i%cols < cols-1 && i < n-1 {
			pad := colW - widths[i]
			fmt.Fprint(w, strings.Repeat(" ", pad))
		}
	}
	fmt.Fprintln(w)
}

type columnDef struct {
	header string
	align  int // -1 left, 1 right
}

func longColumns(opts FormatOpts) []columnDef {
	var cols []columnDef
	if opts.Inode {
		cols = append(cols, columnDef{"Inode", 1})
	}
	if opts.ShowBlocks {
		cols = append(cols, columnDef{"Blocks", 1})
	}
	cols = append(cols, columnDef{"Permissions", -1})
	cols = append(cols, columnDef{"#", 1})
	if opts.NumericUID {
		cols = append(cols, columnDef{"UID", 1})
		cols = append(cols, columnDef{"GID", 1})
	} else {
		// -o: omit group, show owner
		// -g: omit owner, show group
		// -og: omit both
		if !opts.HideGroup {
			cols = append(cols, columnDef{"Owner", -1})
		}
		if !opts.HideOwner {
			cols = append(cols, columnDef{"Group", -1})
		}
	}
	cols = append(cols, columnDef{"Size", 1})
	cols = append(cols, columnDef{"Modified", -1})
	if opts.GitColumn {
		cols = append(cols, columnDef{"Git", -1})
	}
	cols = append(cols, columnDef{"Name", -1})
	return cols
}

func FormatModeString(mode os.FileMode) string {
	buf := []byte("----------")
	switch {
	case mode.IsDir():
		buf[0] = 'd'
	case mode&os.ModeSymlink != 0:
		buf[0] = 'l'
	case mode&os.ModeDevice != 0:
		if mode&os.ModeCharDevice != 0 {
			buf[0] = 'c'
		} else {
			buf[0] = 'b'
		}
	case mode&os.ModeNamedPipe != 0:
		buf[0] = 'p'
	case mode&os.ModeSocket != 0:
		buf[0] = 's'
	}

	perm := mode.Perm()
	if perm&0400 != 0 {
		buf[1] = 'r'
	}
	if perm&0200 != 0 {
		buf[2] = 'w'
	}
	if mode&os.ModeSetuid != 0 {
		if perm&0100 != 0 {
			buf[3] = 's'
		} else {
			buf[3] = 'S'
		}
	} else if perm&0100 != 0 {
		buf[3] = 'x'
	}

	if perm&0040 != 0 {
		buf[4] = 'r'
	}
	if perm&0020 != 0 {
		buf[5] = 'w'
	}
	if mode&os.ModeSetgid != 0 {
		if perm&0010 != 0 {
			buf[6] = 's'
		} else {
			buf[6] = 'S'
		}
	} else if perm&0010 != 0 {
		buf[6] = 'x'
	}

	if perm&0004 != 0 {
		buf[7] = 'r'
	}
	if perm&0002 != 0 {
		buf[8] = 'w'
	}
	if mode&os.ModeSticky != 0 {
		if perm&0001 != 0 {
			buf[9] = 't'
		} else {
			buf[9] = 'T'
		}
	} else if perm&0001 != 0 {
		buf[9] = 'x'
	}

	return string(buf)
}

func inodeCell(fi *FileInfo, opts FormatOpts) string {
	if opts.Dereference && fi.LinkBroken || fi.lacks(FieldInode) {
		return unavailable
	}
	return strconv.FormatUint(fi.Inode, 10)
}

func blocksCell(fi *FileInfo, opts FormatOpts) string {
	if opts.Dereference && fi.LinkBroken || fi.lacks(FieldBlocks) {
		return unavailable
	}
	return strconv.FormatInt(fi.Blocks/2, 10)
}

func fileRow(fi *FileInfo, opts FormatOpts) []string {
	var cols []string
	if opts.Inode {
		cols = append(cols, inodeCell(fi, opts))
	}
	if opts.ShowBlocks {
		cols = append(cols, blocksCell(fi, opts))
	}
	cols = append(cols, FormatModeString(fi.Mode))
	cols = append(cols, nlinkCell(fi))
	if opts.NumericUID {
		cols = append(cols, uidCell(fi), gidCell(fi))
	} else {
		// -o: omit group, show owner
		// -g: omit owner, show group
		// -og: omit both
		if !opts.HideGroup {
			cols = append(cols, ownerCell(fi))
		}
		if !opts.HideOwner {
			cols = append(cols, groupCell(fi))
		}
	}
	if opts.HumanSize {
		cols = append(cols, HumanSize(fi.Size))
	} else {
		cols = append(cols, strconv.FormatInt(fi.Size, 10))
	}
	cols = append(cols, formatTime(fi.ModTime, opts))
	if opts.GitColumn {
		sym := GitStatusSymbol(fi.GitStatus)
		code := GitStatusColor(fi.GitStatus)
		if !opts.NoColor && code != "" {
			sym = "\033[" + code + "m" + sym + "\033[39m"
		}
		cols = append(cols, sym)
	}
	cols = append(cols, entryName(fi, opts))
	return cols
}

func fileRowPlain(fi *FileInfo, opts FormatOpts) []string {
	var cols []string
	if opts.Inode {
		cols = append(cols, inodeCell(fi, opts))
	}
	if opts.ShowBlocks {
		cols = append(cols, blocksCell(fi, opts))
	}
	cols = append(cols, FormatModeString(fi.Mode))
	cols = append(cols, nlinkCell(fi))
	if opts.NumericUID {
		cols = append(cols, uidCell(fi), gidCell(fi))
	} else {
		// -o: omit group, show owner
		// -g: omit owner, show group
		// -og: omit both
		if !opts.HideGroup {
			cols = append(cols, ownerCell(fi))
		}
		if !opts.HideOwner {
			cols = append(cols, groupCell(fi))
		}
	}
	if opts.HumanSize {
		cols = append(cols, HumanSize(fi.Size))
	} else {
		cols = append(cols, strconv.FormatInt(fi.Size, 10))
	}
	cols = append(cols, formatTime(fi.ModTime, opts))
	if opts.GitColumn {
		cols = append(cols, GitStatusSymbol(fi.GitStatus))
	}
	cols = append(cols, entryNamePlain(fi, opts))
	return cols
}

func formatLong(w io.Writer, files []*FileInfo, opts FormatOpts) {
	if !opts.Zero {
		tw := tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)
		for _, fi := range files {
			fmt.Fprintln(tw, strings.Join(fileRow(fi, opts), "\t"))
		}
		tw.Flush()
		return
	}

	var buf strings.Builder
	tw := tabwriter.NewWriter(&buf, 0, 0, 1, ' ', 0)
	for _, fi := range files {
		fmt.Fprintln(tw, strings.Join(fileRow(fi, opts), "\t"))
	}
	tw.Flush()
	fmt.Fprint(w, strings.ReplaceAll(buf.String(), "\n", "\x00"))
}

func formatTable(w io.Writer, files []*FileInfo, opts FormatOpts, termWidth int) {
	colDefs := longColumns(opts)
	numCols := len(colDefs)

	rows := make([][]string, len(files))
	plainRows := make([][]string, len(files))
	for i, fi := range files {
		rows[i] = fileRow(fi, opts)
		plainRows[i] = fileRowPlain(fi, opts)
	}

	widths := make([]int, numCols)
	for i, cd := range colDefs {
		if displayWidth(cd.header) > widths[i] {
			widths[i] = displayWidth(cd.header)
		}
	}
	for _, row := range plainRows {
		for i, cell := range row {
			if i < numCols {
				cw := displayWidth(cell)
				if cw > widths[i] {
					widths[i] = cw
				}
			}
		}
	}

	if termWidth > 0 && numCols > 0 {
		overhead := numCols + 1 + numCols*2
		total := overhead
		for _, cw := range widths {
			total += cw
		}
		if total > termWidth {
			nameCol := numCols - 1
			excess := total - termWidth
			newNameW := widths[nameCol] - excess
			if newNameW < 10 {
				formatLong(w, files, opts)
				return
			}
			widths[nameCol] = newNameW
		}
	}

	hline := func(left, mid, right, fill string) {
		fmt.Fprint(w, left)
		for i, cw := range widths {
			fmt.Fprint(w, strings.Repeat(fill, cw+2))
			if i < numCols-1 {
				fmt.Fprint(w, mid)
			}
		}
		fmt.Fprintln(w, right)
	}

	padCell := func(cell string, plainCell string, col int) string {
		cw := displayWidth(plainCell)
		if cw > widths[col] {
			cell = truncateAnsi(cell, widths[col])
			cw = widths[col]
		}
		pad := widths[col] - cw
		if colDefs[col].align > 0 {
			return strings.Repeat(" ", pad) + cell
		}
		return cell + strings.Repeat(" ", pad)
	}

	hline("┌", "┬", "┐", "─")

	fmt.Fprint(w, "│")
	for i, cd := range colDefs {
		fmt.Fprintf(w, " %s ", padCell(cd.header, cd.header, i))
		fmt.Fprint(w, "│")
		_ = i
	}
	fmt.Fprintln(w)

	hline("├", "┼", "┤", "─")

	for ri, row := range rows {
		fmt.Fprint(w, "│")
		for i, cell := range row {
			if i < numCols {
				fmt.Fprintf(w, " %s ", padCell(cell, plainRows[ri][i], i))
				fmt.Fprint(w, "│")
			}
		}
		fmt.Fprintln(w)
	}

	hline("└", "┴", "┘", "─")
}

func formatTime(t time.Time, opts FormatOpts) string {
	if strings.HasPrefix(opts.TimeStyle, "+") {
		return t.Format(convertStrftimeToGo(opts.TimeStyle[1:]))
	}
	switch opts.TimeStyle {
	case "full-iso":
		return t.Format("2006-01-02 15:04:05.000000000 -0700")
	case "long-iso":
		return t.Format("2006-01-02 15:04")
	case "iso":
		return t.Format("01-02 15:04")
	case "locale":
		return t.Format("Mon Jan _2 15:04:05 2006")
	}

	now := time.Now()
	sixMonths := 182 * 24 * time.Hour
	if now.Sub(t) > sixMonths || t.After(now) {
		return t.Format("Jan _2  2006")
	}
	return t.Format("Jan _2 15:04")
}

func convertStrftimeToGo(fmtStr string) string {
	var b strings.Builder
	for i := 0; i < len(fmtStr); i++ {
		if fmtStr[i] == '%' && i+1 < len(fmtStr) {
			i++
			switch fmtStr[i] {
			case 'Y':
				b.WriteString("2006")
			case 'y':
				b.WriteString("06")
			case 'm':
				b.WriteString("01")
			case 'd':
				b.WriteString("02")
			case 'e':
				b.WriteString("_2")
			case 'H':
				b.WriteString("15")
			case 'M':
				b.WriteString("04")
			case 'S':
				b.WriteString("05")
			case 'b', 'h':
				b.WriteString("Jan")
			case 'B':
				b.WriteString("January")
			case 'a':
				b.WriteString("Mon")
			case 'A':
				b.WriteString("Monday")
			case 'z':
				b.WriteString("-0700")
			case 'Z':
				b.WriteString("MST")
			case '%':
				b.WriteByte('%')
			default:
				b.WriteByte('%')
				b.WriteByte(fmtStr[i])
			}
		} else {
			b.WriteByte(fmtStr[i])
		}
	}
	return b.String()
}

func formatTree(w io.Writer, files []*FileInfo, opts FormatOpts, prefix string) {
	for i, fi := range files {
		isLast := i == len(files)-1

		connector := "├── "
		if isLast {
			connector = "└── "
		}

		var line strings.Builder
		line.WriteString(prefix)
		line.WriteString(connector)

		if opts.GitColumn && fi.GitStatus != "" {
			sym := GitStatusSymbol(fi.GitStatus)
			code := GitStatusColor(fi.GitStatus)
			if !opts.NoColor && code != "" {
				line.WriteString("\033[" + code + "m" + sym + "\033[39m ")
			} else {
				line.WriteString(sym + " ")
			}
		}

		line.WriteString(entryName(fi, opts))
		fmt.Fprintln(w, line.String())

		if len(fi.Children) > 0 {
			childPrefix := prefix
			if isLast {
				childPrefix += "    "
			} else {
				childPrefix += "│   "
			}
			formatTree(w, fi.Children, opts, childPrefix)
		}
	}
}
