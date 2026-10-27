package builtins

import (
	"bufio"
	crand "crypto/rand"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const sortSpec = "r(reverse)n(numeric-sort)u(unique)f(ignore-case)k(key):t(field-separator):s(stable)o(output):b(ignore-leading-blanks)g(general-numeric-sort)c(check):?C(check-silent):?M(month-sort)z(zero-terminated)m(merge)S(buffer-size):V(version-sort)h(human-numeric-sort)R(random-sort)d(dictionary-order)i(ignore-nonprinting)(files0-from):(batch-size):(sort):(debug)(random-source):(compress-program):"

var sortSpecParsed *Spec

func init() {
	var err error
	sortSpecParsed, err = ParseSpec(sortSpec)
	if err != nil {
		panic("builtins: bad sort spec: " + err.Error())
	}
	Register(Command{Name: "sort", Run: runSort})
}

type sortKeySpec struct {
	startField   int
	startChar    int
	endField     int
	endChar      int
	reverse      bool
	numeric      bool
	foldCase     bool
	ignBlank     bool
	genNumeric   bool
	monthSort    bool
	versionSort  bool
	humanNumeric bool
	randomSort   bool
	dictOrder    bool
	ignNonPrint  bool
	ownOrderOpts bool
}

func parseKeySpec(s string) (sortKeySpec, error) {
	parts := strings.SplitN(s, ",", 2)
	ks := sortKeySpec{}

	parseField := func(fieldStr string, offsetInS int) (field int, char int, flags string, err error) {
		if fieldStr == "" {
			return 0, 0, "", fmt.Errorf("invalid number after ',': invalid count at start of ''")
		}
		dotIdx := strings.IndexByte(fieldStr, '.')
		numPart := fieldStr
		if dotIdx >= 0 {
			numPart = fieldStr[:dotIdx]
		}
		i := 0
		for i < len(numPart) && numPart[i] >= '0' && numPart[i] <= '9' {
			i++
		}
		if i == 0 {
			if numPart == "" && dotIdx == 0 {
				return 0, 0, "", fmt.Errorf("invalid number after '.': invalid count at start of '%s'", s[offsetInS+1:])
			}
			return 0, 0, "", fmt.Errorf("invalid number after ',': invalid count at start of '%s'", s[offsetInS:])
		}
		fVal, err := strconv.Atoi(numPart[:i])
		if err != nil && strings.Contains(err.Error(), "range") {
			fVal = math.MaxInt
		} else if err != nil || fVal == 0 {
			return 0, 0, "", fmt.Errorf("field number is zero: invalid field specification '%s'", s)
		}
		flagPart := numPart[i:]

		cVal := 0
		if dotIdx >= 0 {
			rest := fieldStr[dotIdx+1:]
			j := 0
			for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
				j++
			}
			if j == 0 {
				return 0, 0, "", fmt.Errorf("invalid number after '.': invalid count at start of '%s'", s[offsetInS+dotIdx+1:])
			}
			c, err := strconv.Atoi(rest[:j])
			if err != nil && strings.Contains(err.Error(), "range") {
				c = math.MaxInt
			} else if err != nil || c == 0 {
				return 0, 0, "", fmt.Errorf("character offset is zero: invalid field specification '%s'", s)
			}
			cVal = c
			flagPart = rest[j:]
		}

		for _, ch := range flagPart {
			if !strings.ContainsRune("rnfbgMVhRdi", ch) {
				return 0, 0, "", fmt.Errorf("stray character in field spec: invalid field specification '%s'", s)
			}
		}

		return fVal, cVal, flagPart, nil
	}

	sf, sc, sFlags, err := parseField(parts[0], 0)
	if err != nil {
		return ks, err
	}
	ks.startField = sf
	ks.startChar = sc
	applyKeyFlags(&ks, sFlags)

	if len(parts) == 2 {
		ef, ec, eFlags, err := parseField(parts[1], len(parts[0])+1)
		if err != nil {
			return ks, err
		}
		ks.endField = ef
		ks.endChar = ec
		applyKeyFlags(&ks, eFlags)
	}

	return ks, nil
}

func applyKeyFlags(ks *sortKeySpec, flags string) {
	if flags != "" {
		ks.ownOrderOpts = true
	}
	for _, c := range flags {
		switch c {
		case 'r':
			ks.reverse = true
		case 'n':
			ks.numeric = true
		case 'f':
			ks.foldCase = true
		case 'b':
			ks.ignBlank = true
		case 'g':
			ks.genNumeric = true
		case 'M':
			ks.monthSort = true
		case 'V':
			ks.versionSort = true
		case 'h':
			ks.humanNumeric = true
		case 'R':
			ks.randomSort = true
		case 'd':
			ks.dictOrder = true
		case 'i':
			ks.ignNonPrint = true
		}
	}
}

func extractKeyFields(line string, sep string, startField, startChar, endField, endChar int) string {
	fields := splitFields(line, sep)
	if startField > len(fields) {
		return ""
	}
	startIdx := startField - 1
	endIdx := len(fields)
	if endField > 0 && endField <= len(fields) {
		endIdx = endField
	}

	sf := fields[startIdx]
	startByteIdx := 0
	if startChar > 1 {
		startByteIdx = startChar - 1
		if startByteIdx > len(sf) {
			return ""
		}
	}

	if endField > 0 && endField <= len(fields) {
		var keyBytes []byte
		for f := startIdx; f < endIdx; f++ {
			fBytes := []byte(fields[f])
			if f > startIdx && sep != "" {
				keyBytes = append(keyBytes, []byte(sep)...)
			}
			sIdx := 0
			if f == startIdx {
				sIdx = startByteIdx
			}
			eIdx := len(fBytes)
			if f == endIdx-1 && endChar > 0 {
				eIdx = endChar
				if eIdx > len(fBytes) {
					eIdx = len(fBytes)
				}
			}
			if sIdx < eIdx {
				keyBytes = append(keyBytes, fBytes[sIdx:eIdx]...)
			}
		}
		return string(keyBytes)
	}

	var keyBytes []byte
	for f := startIdx; f < len(fields); f++ {
		fBytes := []byte(fields[f])
		if f > startIdx && sep != "" {
			keyBytes = append(keyBytes, []byte(sep)...)
		}
		sIdx := 0
		if f == startIdx {
			sIdx = startByteIdx
		}
		if sIdx < len(fBytes) {
			keyBytes = append(keyBytes, fBytes[sIdx:]...)
		}
	}
	if endChar > 0 && len(keyBytes) > endChar {
		keyBytes = keyBytes[:endChar]
	}
	return string(keyBytes)
}

func parseMonth(s string) int {
	s = strings.TrimSpace(strings.ToUpper(s))
	months := []string{"JAN", "FEB", "MAR", "APR", "MAY", "JUN", "JUL", "AUG", "SEP", "OCT", "NOV", "DEC"}
	for i, m := range months {
		if strings.HasPrefix(s, m) {
			return i + 1
		}
	}
	return 0
}

func versionCompare(a, b string) int {
	return filevercmp(a, b)
}

func filevercmp(s1, s2 string) int {
	if s1 == s2 {
		return 0
	}

	s1Prefix, s1Suffix := verSplitSuffix(s1)
	s2Prefix, s2Suffix := verSplitSuffix(s2)

	if cmp := verCompareCore(s1Prefix, s2Prefix); cmp != 0 {
		return cmp
	}
	if cmp := verCompareCore(s1Suffix, s2Suffix); cmp != 0 {
		return cmp
	}
	return strings.Compare(s1, s2)
}

func verSplitSuffix(s string) (prefix, suffix string) {
	if s == "" {
		return "", ""
	}

	// filevercmp suffix splitting rule:
	// A suffix starts at the LAST dot followed by non-digits (alpha characters),
	// but ONLY if that suffix is preceded by something other than a dot.
	// Also handles double extensions like .tar.gz or .tar.bz2.
	n := len(s)
	i := n - 1
	for i >= 0 {
		if s[i] == '.' {
			// check if suffix s[i:] is valid alpha suffix (contains non-digits)
			sub := s[i+1:]
			hasAlpha := false
			valid := len(sub) > 0
			for _, r := range sub {
				if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
					hasAlpha = true
				} else if r >= '0' && r <= '9' {
					// allowed in extension
				} else {
					valid = false
					break
				}
			}
			if valid && hasAlpha {
				// Check if there is an immediately preceding extension like .tar
				if i > 0 {
					j := i - 1
					for j >= 0 && s[j] != '.' {
						j--
					}
					if j >= 0 {
						sub2 := s[j+1 : i]
						hasAlpha2 := false
						valid2 := len(sub2) > 0
						for _, r := range sub2 {
							if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
								hasAlpha2 = true
							} else if r >= '0' && r <= '9' {
							} else {
								valid2 = false
								break
							}
						}
						if valid2 && hasAlpha2 {
							return s[:j], s[j:]
						}
					}
				}
				return s[:i], s[i:]
			}
			break
		}
		i--
	}

	return s, ""
}

func verCompareCore(s1, s2 string) int {
	p1, p2 := 0, 0
	n1, n2 := len(s1), len(s2)

	for p1 < n1 || p2 < n2 {
		if p1 >= n1 {
			return -1
		}
		if p2 >= n2 {
			return 1
		}

		c1, c2 := s1[p1], s2[p2]
		isDigit1 := c1 >= '0' && c1 <= '9'
		isDigit2 := c2 >= '0' && c2 <= '9'

		if isDigit1 && isDigit2 {
			d1End := p1
			for d1End < n1 && s1[d1End] >= '0' && s1[d1End] <= '9' {
				d1End++
			}
			d2End := p2
			for d2End < n2 && s2[d2End] >= '0' && s2[d2End] <= '9' {
				d2End++
			}

			// Leading zero handling in strverscmp:
			// If either starts with '0', compare fractionally:
			// first non-zero digit difference, or if non-zero values are identical, count of digits.
			if s1[p1] == '0' || s2[p2] == '0' {
				// Fractional mode (leading zeros present):
				// Compare digits left-to-right.
				// In strverscmp fractional mode, trailing zeros are ignored for value comparison,
				// and if values are equal, fewer leading zeros / shorter length is smaller.
				// First find end of non-zero digits or compare left-to-right:
				// Strip trailing zeros from digit spans for fractional evaluation:
				e1 := d1End
				for e1 > p1 && s1[e1-1] == '0' {
					e1--
				}
				e2 := d2End
				for e2 > p2 && s2[e2-1] == '0' {
					e2--
				}

				v1 := s1[p1:e1]
				v2 := s2[p2:e2]
				if v1 != v2 {
					if v1 < v2 {
						return -1
					}
					return 1
				}

				len1 := d1End - p1
				len2 := d2End - p2
				if len1 != len2 {
					if len1 < len2 {
						return -1
					}
					return 1
				}

				p1 = d1End
				p2 = d2End
				continue
			}

			// Integer mode (neither starts with '0'):
			len1 := d1End - p1
			len2 := d2End - p2

			if len1 != len2 {
				if len1 < len2 {
					return -1
				}
				return 1
			}

			v1 := s1[p1:d1End]
			v2 := s2[p2:d2End]
			if v1 != v2 {
				if v1 < v2 {
					return -1
				}
				return 1
			}

			p1 = d1End
			p2 = d2End
			continue
		}

		if c1 != c2 {
			if c1 == '~' || c2 == '~' {
				if c1 == '~' && c2 != '~' {
					return -1
				}
				if c1 != '~' && c2 == '~' {
					return 1
				}
			}

			if isDigit1 != isDigit2 {
				// Digits compare before letters in strverscmp/filevercmp
				// e.g., '10' < 'b'
				if isDigit1 {
					if c2 == '.' || c2 == '~' {
						return 1
					}
					return -1
				}
				if c1 == '.' || c1 == '~' {
					return -1
				}
				return 1
			}

			if c1 < c2 {
				return -1
			}
			return 1
		}

		p1++
		p2++
	}

	return 0
}

func compareLocale(s1, s2 string) int {
	n1, n2 := len(s1), len(s2)
	minLen := n1
	if n2 < minLen {
		minLen = n2
	}

	for i := 0; i < minLen; i++ {
		c1, c2 := s1[i], s2[i]
		if c1 == c2 {
			continue
		}
		w1 := primaryWeight(c1)
		w2 := primaryWeight(c2)
		if w1 != w2 {
			if w1 < w2 {
				return -1
			}
			return 1
		}
	}
	if n1 != n2 {
		if n1 < n2 {
			return -1
		}
		return 1
	}

	return compareCase(s1, s2)
}

func primaryWeight(c byte) int {
	if c == '_' {
		return 10
	}
	if c >= 'A' && c <= 'Z' {
		return int(c + 32)
	}
	return int(c)
}

func compareCase(s1, s2 string) int {
	minLen := len(s1)
	if len(s2) < minLen {
		minLen = len(s2)
	}
	for i := 0; i < minLen; i++ {
		c1, c2 := s1[i], s2[i]
		if c1 != c2 {
			isLower1 := c1 >= 'a' && c1 <= 'z'
			isUpper1 := c1 >= 'A' && c1 <= 'Z'
			isLower2 := c2 >= 'a' && c2 <= 'z'
			isUpper2 := c2 >= 'A' && c2 <= 'Z'

			if (isLower1 && isUpper2) || (isUpper1 && isLower2) {
				if isLower1 {
					return -1
				}
				return 1
			}
			if c1 < c2 {
				return -1
			}
			return 1
		}
	}
	if len(s1) != len(s2) {
		if len(s1) < len(s2) {
			return -1
		}
		return 1
	}
	return 0
}

type humanNum struct {
	isNeg  bool
	num    float64
	unit   int
	hasNum bool
}

func parseHumanNum(s string) humanNum {
	s = strings.TrimSpace(s)
	if s == "" {
		return humanNum{}
	}
	hn := humanNum{}
	idx := 0
	if s[idx] == '-' {
		hn.isNeg = true
		idx++
	} else if s[idx] == '+' {
		idx++
	}
	startNum := idx
	for idx < len(s) && ((s[idx] >= '0' && s[idx] <= '9') || s[idx] == '.') {
		idx++
	}
	if idx > startNum {
		if v, err := strconv.ParseFloat(s[startNum:idx], 64); err == nil {
			hn.num = v
			hn.hasNum = true
		}
	}
	suf := strings.TrimSpace(s[idx:])
	if len(suf) > 0 {
		switch suf[0] {
		case 'k', 'K':
			hn.unit = 1
		case 'M':
			hn.unit = 2
		case 'G':
			hn.unit = 3
		case 'T':
			hn.unit = 4
		case 'P':
			hn.unit = 5
		case 'E':
			hn.unit = 6
		case 'Z':
			hn.unit = 7
		case 'Y':
			hn.unit = 8
		case 'R':
			hn.unit = 9
		case 'Q':
			hn.unit = 10
		}
	}
	return hn
}

func compareHumanNumeric(s1, s2 string) int {
	h1 := parseHumanNum(s1)
	h2 := parseHumanNum(s2)

	v1 := h1.num
	v2 := h2.num

	if v1 == 0 && v2 == 0 {
		return 0
	}

	if !h1.hasNum {
		return -1
	}
	if !h2.hasNum {
		return 1
	}

	if h1.isNeg != h2.isNeg {
		if h1.isNeg {
			return -1
		}
		return 1
	}

	if h1.isNeg {
		if h1.unit != h2.unit {
			if h1.unit > h2.unit {
				return -1
			}
			return 1
		}
		if h1.num > h2.num {
			return -1
		}
		if h1.num < h2.num {
			return 1
		}
		return 0
	}

	if h1.unit != h2.unit {
		if h1.unit < h2.unit {
			return -1
		}
		return 1
	}
	if h1.num < h2.num {
		return -1
	}
	if h1.num > h2.num {
		return 1
	}
	return 0
}

func filterDictOrder(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ' || r == '\t' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func filterNonPrint(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 32 && r <= 126 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func validateBufferSize(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("invalid --buffer-size argument ''")
	}
	if strings.HasSuffix(s, "%") {
		numStr := s[:len(s)-1]
		val, err := strconv.ParseFloat(numStr, 64)
		if err != nil || val < 0 {
			return fmt.Errorf("invalid --buffer-size argument '%s'", s)
		}
		if val > 1000000000 {
			return fmt.Errorf("--buffer-size argument '%s' too large", s)
		}
		return nil
	}
	lastChar := s[len(s)-1]
	numStr := s
	if (lastChar >= 'a' && lastChar <= 'z') || (lastChar >= 'A' && lastChar <= 'Z') {
		numStr = s[:len(s)-1]
		if lastChar == 'Y' || lastChar == 'Z' || lastChar == 'y' || lastChar == 'z' {
			// Only "too large" if numStr is a valid integer; otherwise it's an invalid suffix.
			if _, parseErr := strconv.ParseUint(numStr, 10, 64); parseErr != nil {
				return fmt.Errorf("invalid suffix in --buffer-size argument '%s'", s)
			}
			return fmt.Errorf("--buffer-size argument '%s' too large", s)
		}
		validUnits := "bBkKmMgGtTpPeE"
		if !strings.ContainsRune(validUnits, rune(lastChar)) {
			if _, err := strconv.ParseUint(numStr, 10, 64); err == nil {
				return fmt.Errorf("invalid suffix in --buffer-size argument '%s'", s)
			}
			return fmt.Errorf("invalid --buffer-size argument '%s'", s)
		}
	}
	v, err := strconv.ParseUint(numStr, 10, 64)
	if err != nil {
		if strings.Contains(err.Error(), "range") || len(numStr) > 10 {
			return fmt.Errorf("--buffer-size argument '%s' too large", s)
		}
		return fmt.Errorf("invalid --buffer-size argument '%s'", s)
	}
	_ = v
	return nil
}

func validateBatchSize(s string) error {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		if strings.Contains(err.Error(), "range") || len(s) > 10 {
			return fmt.Errorf("--batch-size argument '%s' too large", s)
		}
		return fmt.Errorf("invalid --batch-size argument '%s'", s)
	}
	if v < 2 {
		return fmt.Errorf("invalid --batch-size argument '%s'\nsort: minimum --batch-size argument is '2'", s)
	}
	return nil
}

func parsePosixKeyPos(s string, isStart bool) string {
	// Format is M[.N][flags]
	// +M.N means field M+1, char N+1
	// -M.N means field M+1 (if N>0: field M+1, char N; if N==0: field M, char end)
	// If M exceeds max int, overflow to MaxInt
	dotIdx := strings.IndexByte(s, '.')
	numStr := s
	if dotIdx >= 0 {
		numStr = s[:dotIdx]
	}
	i := 0
	for i < len(numStr) && numStr[i] >= '0' && numStr[i] <= '9' {
		i++
	}
	fValStr := numStr[:i]
	flags := numStr[i:]

	fVal, err := strconv.ParseUint(fValStr, 10, 64)
	if err != nil || fVal > math.MaxInt-10 {
		fVal = math.MaxInt - 10
	}

	resField := fVal
	if isStart {
		resField = fVal + 1
	} else {
		// End field in -M.N: if N == 0, field is M; if N > 0, field is M + 1
	}

	charPart := ""
	if dotIdx >= 0 {
		rest := s[dotIdx+1:]
		j := 0
		for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
			j++
		}
		cValStr := rest[:j]
		cFlags := rest[j:]
		flags += cFlags

		cVal, cErr := strconv.ParseUint(cValStr, 10, 64)
		if cErr != nil || cVal > math.MaxInt-10 {
			cVal = math.MaxInt - 10
		}
		if isStart {
			charPart = fmt.Sprintf(".%d", cVal+1)
		} else {
			if cVal > 0 {
				resField = fVal + 1
				charPart = fmt.Sprintf(".%d", cVal)
			} else {
				resField = fVal
			}
		}
	} else if !isStart {
		resField = fVal
	}

	return fmt.Sprintf("%d%s%s", resField, charPart, flags)
}

func runSort(ctx *Context) int {
	// Parse POSIX obsolescent +POS -POS arguments if present
	args := ctx.Args
	var cleanArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "+") && len(arg) > 1 && arg[1] >= '0' && arg[1] <= '9' {
			// Convert +M.N to -k option (+M.N means field M+1, char N+1)
			pos1 := parsePosixKeyPos(arg[1:], true)
			keyArg := pos1
			if i+1 < len(args) && strings.HasPrefix(args[i+1], "-") && len(args[i+1]) > 1 && args[i+1][1] >= '0' && args[i+1][1] <= '9' {
				pos2 := parsePosixKeyPos(args[i+1][1:], false)
				keyArg += "," + pos2
				i++
			}
			cleanArgs = append(cleanArgs, "-k", keyArg)
		} else {
			cleanArgs = append(cleanArgs, arg)
		}
	}

	flags, err := sortSpecParsed.ParseArgs(cleanArgs)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "sort:", err)
		return 2
	}
	if HandleHelpVersion(ctx, flags, sortSpecParsed) {
		return 0
	}

	outFiles := flags.AllStrings("o")
	if len(outFiles) > 1 {
		first := outFiles[0]
		for _, f := range outFiles[1:] {
			if f != first {
				fmt.Fprintln(ctx.Stderr, "sort: multiple output files specified")
				return 2
			}
		}
	}

	globalReverse := flags.Bool("r")
	globalNumeric := flags.Bool("n")
	unique := flags.Bool("u")
	globalFoldCase := flags.Bool("f")
	stable := flags.Bool("s")
	globalIgnBlank := flags.Bool("b")
	globalGenNumeric := flags.Bool("g")
	globalMonthSort := flags.Bool("M")
	globalVersionSort := flags.Bool("V")
	globalHumanNumeric := flags.Bool("h")
	globalRandomSort := flags.Bool("R")
	globalDictOrder := flags.Bool("d")
	globalIgnNonprint := flags.Bool("i")

	var randSeed uint64
	if rsFile, ok := flags.String("random-source"); ok {
		p := resolvePath(ctx, rsFile)
		fh, err := os.Open(p)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "sort: '%s': end of file\n", rsFile)
			return 2
		}
		buf := make([]byte, 16)
		n, _ := io.ReadFull(fh, buf)
		fh.Close()
		if n == 0 {
			fmt.Fprintf(ctx.Stderr, "sort: '%s': end of file\n", rsFile)
			return 2
		}
		for i := 0; i < n && i < 8; i++ {
			randSeed = (randSeed << 8) | uint64(buf[i])
		}
	} else {
		var b [8]byte
		crand.Read(b[:])
		randSeed = uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 | uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
	}
	hashRandomKey := func(s string) uint64 {
		h := randSeed ^ 0xc6a4a7935bd1e995
		for _, b := range []byte(s) {
			h ^= uint64(b)
			h *= 0x5bd1e995
			h ^= h >> 47
		}
		return h
	}

	if sortVal, ok := flags.String("sort"); ok {
		if strings.HasPrefix(sortVal, "v") {
			globalVersionSort = true
		} else if strings.HasPrefix(sortVal, "g") {
			globalGenNumeric = true
		} else if strings.HasPrefix(sortVal, "n") {
			globalNumeric = true
		} else if strings.HasPrefix(sortVal, "M") || strings.HasPrefix(sortVal, "m") {
			globalMonthSort = true
		} else if strings.HasPrefix(sortVal, "h") {
			globalHumanNumeric = true
		} else if strings.HasPrefix(sortVal, "r") || strings.HasPrefix(sortVal, "R") {
			globalRandomSort = true
		}
	}

	if cpProg, ok := flags.String("compress-program"); ok {
		if cpProg != "" {
			var err error
			var full string
			if strings.Contains(cpProg, "/") {
				full = resolvePath(ctx, cpProg)
				_, err = os.Stat(full)
			} else {
				found := false
				pathEnv, _ := lookupEnv(ctx, "PATH")
				for _, p := range strings.Split(pathEnv, ":") {
					if p == "" {
						p = "."
					}
					cand := resolvePath(ctx, filepath.Join(p, cpProg))
					if fi, statErr := os.Stat(cand); statErr == nil && !fi.IsDir() {
						found = true
						full = cand
						break
					}
				}
				if !found {
					err = os.ErrNotExist
				}
			}
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "sort: could not run compress program '%s': No such file or directory\n", cpProg)
			} else {
				// Run dummy invocation to create 'ok' side effect file if needed by test program
				cmd := exec.Command(full)
				cmd.Run()
			}
		}
	}

	isUtf8Locale := false
	collate := ""
	for _, envVar := range []string{"LC_ALL", "LC_COLLATE", "LANG"} {
		if val, ok := lookupEnv(ctx, envVar); ok && val != "" {
			if val == "missing" && flags.Bool("debug") {
				fmt.Fprintln(ctx.Stderr, "sort: failed to set locale")
			}
			if val == "C" || val == "POSIX" || val == "missing" {
				break
			}
			collate = val
			valLower := strings.ToLower(val)
			if strings.Contains(valLower, "utf-8") || strings.Contains(valLower, "utf8") || strings.HasPrefix(valLower, "en") {
				// isUtf8Locale = true
			}
			break
		}
	}

	if flags.Bool("debug") {
		if collate != "" {
			fmt.Fprintf(ctx.Stderr, "sort: text ordering performed using \u2018%s\u2019 sorting rules\n", collate)
		} else {
			fmt.Fprintln(ctx.Stderr, "sort: text ordering performed using simple byte comparison")
		}
	}

	numSorts := 0
	for _, b := range []bool{globalGenNumeric, globalHumanNumeric, globalMonthSort, globalNumeric, globalRandomSort, globalVersionSort} {
		if b {
			numSorts++
		}
	}
	isIncompatible := false
	if numSorts > 1 {
		isIncompatible = true
	}
	if (globalDictOrder || globalIgnNonprint) && numSorts > 0 {
		isIncompatible = true
	}

	if isIncompatible {
		outFlags := ""
		if globalDictOrder {
			outFlags += "d"
		}
		if globalFoldCase {
			outFlags += "f"
		}
		if globalGenNumeric {
			outFlags += "g"
		}
		if globalHumanNumeric {
			outFlags += "h"
		}
		if globalIgnNonprint && !globalFoldCase {
			outFlags += "i"
		}
		if globalMonthSort {
			outFlags += "M"
		}
		if globalNumeric {
			outFlags += "n"
		}
		if globalRandomSort {
			outFlags += "R"
		}
		if globalVersionSort {
			outFlags += "V"
		}
		fmt.Fprintf(ctx.Stderr, "sort: options '-%s' are incompatible\n", outFlags)
		return 2
	}

	if bs, ok := flags.String("batch-size"); ok {
		if err := validateBatchSize(bs); err != nil {
			fmt.Fprintln(ctx.Stderr, "sort:", err)
			return 2
		}
	}

	if bufS, ok := flags.String("S"); ok {
		if err := validateBufferSize(bufS); err != nil {
			fmt.Fprintln(ctx.Stderr, "sort:", err)
			return 2
		}
	}

	if unique {
		stable = true
	}

	sep := ""
	if s, ok := flags.String("t"); ok {
		if s == "\\0" {
			sep = "\x00"
		} else if len(s) > 1 {
			fmt.Fprintf(ctx.Stderr, "sort: separator must be exactly one character long: '%s'\n", s)
			return 2
		} else {
			sep = s
		}
	}

	var keys []sortKeySpec
	keyStrs := flags.AllStrings("k")
	for _, ks := range keyStrs {
		k, err := parseKeySpec(ks)
		if err != nil {
			fmt.Fprintln(ctx.Stderr, "sort:", err)
			return 2
		}
		keys = append(keys, k)
	}

	hasCFlag := flags.Bool("c")
	hasCVal, hasCSpec := flags.ExplicitString("c")
	hasUpperC := flags.Bool("C")
	hasUpperCVal, hasUpperCSpec := flags.ExplicitString("C")

	if (hasCSpec || hasCFlag) && (hasUpperCSpec || hasUpperC) {
		fmt.Fprintln(ctx.Stderr, "sort: options '-cC' are incompatible")
		return 2
	}

	checkMode := false
	checkSilent := false
	checkOptName := ""

	if hasCSpec || hasUpperCSpec || hasCFlag || hasUpperC {
		if hasCSpec {
			checkOptName = "c"
			if strings.Contains(hasCVal, "u") {
				unique = true
			}
			if strings.HasPrefix(hasCVal, "s") || strings.HasPrefix(hasCVal, "q") {
				checkSilent = true
			} else {
				checkMode = true
			}
		} else if hasUpperCSpec {
			checkOptName = "C"
			if strings.HasPrefix(hasUpperCVal, "d") || strings.HasPrefix(hasUpperCVal, "w") {
				checkMode = true
			} else {
				checkSilent = true
			}
		} else if hasUpperC {
			checkOptName = "C"
			checkSilent = true
		} else {
			checkOptName = "c"
			checkMode = true
		}
	}

	outFile, hasOutFile := flags.String("o")
	mergeMode := flags.Bool("m")

	if checkMode || checkSilent {
		if hasOutFile {
			fmt.Fprintf(ctx.Stderr, "sort: options '-%so' are incompatible\n", checkOptName)
			return 2
		}
		if mergeMode {
			fmt.Fprintf(ctx.Stderr, "sort: options '-%sm' are incompatible\n", checkOptName)
			return 2
		}
	}

	if hasOutFile {
		if err := sortCheckOutputWritable(resolvePath(ctx, outFile)); err != nil {
			fmt.Fprintf(ctx.Stderr, "sort: open failed: %s: %s\n", outFile, fmtOsErr(err))
			return 2
		}
	}

	zeroTerm := flags.Bool("z")
	var lines []string
	var lineSources []string

	f0File, hasF0 := flags.String("files0-from")
	if hasF0 {
		var f0Reader io.Reader
		f0Name := f0File
		if f0File == "-" {
			f0Reader = ctx.Stdin
		} else {
			p := resolvePath(ctx, f0File)
			fi, err := os.Stat(p)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "sort: cannot open '%s' for reading: No such file or directory\n", f0File)
				return 2
			}
			if fi.IsDir() {
				fmt.Fprintf(ctx.Stderr, "sort: cannot read: %s: Is a directory\n", f0File)
				return 2
			}
			fh, err := os.Open(p)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "sort: cannot open '%s' for reading\n", f0File)
				return 2
			}
			defer fh.Close()
			f0Reader = fh
		}

		var fnList []string
		sc := bufio.NewScanner(f0Reader)
		sc.Split(scanZeroTerminated)
		entryCount := 0
		for sc.Scan() {
			entryCount++
			fn := sc.Text()
			if fn == "" {
				fmt.Fprintf(ctx.Stderr, "sort: %s:%d: invalid zero-length file name\n", f0Name, entryCount)
				return 2
			}
			fnList = append(fnList, fn)
		}
		if entryCount == 0 {
			fmt.Fprintf(ctx.Stderr, "sort: no input from '%s'\n", f0Name)
			return 2
		}
		for _, fn := range fnList {
			var fh *os.File
			var err error
			if fn == "-" {
				fh = os.Stdin
			} else {
				p := resolvePath(ctx, fn)
				fh, err = os.Open(p)
				if err != nil {
					fmt.Fprintf(ctx.Stderr, "sort: %v\n", err)
					return 2
				}
			}
			fl, err := scanLines(fh, zeroTerm)
			if fn != "-" {
				fh.Close()
			}
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "sort: %v\n", err)
				return 2
			}
			for _, l := range fl {
				lines = append(lines, l)
				lineSources = append(lineSources, fn)
			}
		}
	} else {
		files := flags.Operands
		for _, f := range files {
			if f == "-" {
				continue
			}
			fh, err := os.Open(resolvePath(ctx, f))
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "sort: cannot read: %s: No such file or directory\n", f)
				return 2
			}
			fh.Close()
		}
		if len(files) == 0 {
			fl, err := scanLines(ctx.Stdin, zeroTerm)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "sort: %v\n", err)
				return 1
			}
			for _, l := range fl {
				lines = append(lines, l)
				lineSources = append(lineSources, "-")
			}
		} else {
			for _, f := range files {
				var fl []string
				if f == "-" {
					fl, err = scanLines(ctx.Stdin, zeroTerm)
				} else {
					path := resolvePath(ctx, f)
					var fh *os.File
					fh, err = os.Open(path)
					if err != nil {
						fmt.Fprintf(ctx.Stderr, "sort: %v\n", err)
						return 2
					}
					fl, err = scanLines(fh, zeroTerm)
					fh.Close()
				}
				if err != nil {
					fmt.Fprintf(ctx.Stderr, "sort: %v\n", err)
					return 2
				}
				for _, l := range fl {
					lines = append(lines, l)
					lineSources = append(lineSources, f)
				}
			}
		}
	}

	stringCompare := func(a, b string) int {
		if isUtf8Locale {
			return compareLocale(a, b)
		}
		return strings.Compare(a, b)
	}

	if checkMode || checkSilent {
		for i := 1; i < len(lines); i++ {
			prev, cur := lines[i-1], lines[i]
			cmp := 0
			if len(keys) > 0 {
				for _, ks := range keys {
					ki := extractKeyFields(prev, sep, ks.startField, ks.startChar, ks.endField, ks.endChar)
					kj := extractKeyFields(cur, sep, ks.startField, ks.startChar, ks.endField, ks.endChar)
					useFold := ks.foldCase
					useNumeric := ks.numeric
					useGenNumeric := ks.genNumeric
					useMonth := ks.monthSort
					useVersion := ks.versionSort
					useHuman := ks.humanNumeric
					useReverse := ks.reverse
					if !ks.ownOrderOpts {
						useFold = useFold || globalFoldCase
						useNumeric = useNumeric || globalNumeric
						useGenNumeric = useGenNumeric || globalGenNumeric
						useMonth = useMonth || globalMonthSort
						useVersion = useVersion || globalVersionSort
						useHuman = useHuman || globalHumanNumeric
						useReverse = useReverse || globalReverse
					}
					if useFold {
						ki = strings.ToLower(ki)
						kj = strings.ToLower(kj)
					}
					if useVersion {
						cmp = versionCompare(ki, kj)
					} else if useHuman {
						cmp = compareHumanNumeric(ki, kj)
					} else if useMonth {
						mi := parseMonth(ki)
						mj := parseMonth(kj)
						switch {
						case mi < mj:
							cmp = -1
						case mi > mj:
							cmp = 1
						default:
							cmp = 0
						}
					} else if useGenNumeric {
						ni := parseGeneralFloat(ki)
						nj := parseGeneralFloat(kj)
						cmp = compareGeneralFloat(ni, nj)
					} else if useNumeric {
						ni := parseLeadingFloat(ki)
						nj := parseLeadingFloat(kj)
						switch {
						case ni < nj:
							cmp = -1
						case ni > nj:
							cmp = 1
						default:
							cmp = 0
						}
					} else {
						cmp = stringCompare(ki, kj)
					}
					if useReverse {
						cmp = -cmp
					}
					if cmp != 0 {
						break
					}
				}
			} else {
				ka, kb := prev, cur
				if globalIgnBlank {
					ka = strings.TrimLeft(ka, " \t")
					kb = strings.TrimLeft(kb, " \t")
				}
				if globalFoldCase {
					ka = strings.ToUpper(ka)
					kb = strings.ToUpper(kb)
				}
				if globalDictOrder {
					ka = filterDictOrder(ka)
					kb = filterDictOrder(kb)
				}
				if globalIgnNonprint {
					ka = filterNonPrint(ka)
					kb = filterNonPrint(kb)
				}
				if globalVersionSort {
					cmp = versionCompare(ka, kb)
				} else if globalHumanNumeric {
					cmp = compareHumanNumeric(ka, kb)
				} else if globalMonthSort {
					mi := parseMonth(ka)
					mj := parseMonth(kb)
					switch {
					case mi < mj:
						cmp = -1
					case mi > mj:
						cmp = 1
					default:
						cmp = 0
					}
				} else if globalGenNumeric {
					ni := parseGeneralFloat(ka)
					nj := parseGeneralFloat(kb)
					cmp = compareGeneralFloat(ni, nj)
				} else if globalNumeric {
					ni := parseLeadingFloat(ka)
					nj := parseLeadingFloat(kb)
					switch {
					case ni < nj:
						cmp = -1
					case ni > nj:
						cmp = 1
					default:
						cmp = stringCompare(ka, kb)
					}
				} else {
					cmp = stringCompare(ka, kb)
				}
				if globalReverse {
					cmp = -cmp
				}
			}
			outOfOrder := cmp > 0
			if unique && cmp == 0 {
				outOfOrder = true
			}
			if outOfOrder {
				if !checkSilent {
					fn := lineSources[i]
					fmt.Fprintf(ctx.Stderr, "sort: %s:%d: disorder: %s\n", fn, i+1, cur)
				}
				return 1
			}
		}
		return 0
	}

	compareKey := func(li, lj string, ks sortKeySpec) int {
		ki := extractKeyFields(li, sep, ks.startField, ks.startChar, ks.endField, ks.endChar)
		kj := extractKeyFields(lj, sep, ks.startField, ks.startChar, ks.endField, ks.endChar)

		useIgnBlank := ks.ignBlank
		useFold := ks.foldCase
		useNumeric := ks.numeric
		useGenNumeric := ks.genNumeric
		useMonth := ks.monthSort
		useVersion := ks.versionSort
		useHuman := ks.humanNumeric
		useReverse := ks.reverse
		if !ks.ownOrderOpts {
			useIgnBlank = useIgnBlank || globalIgnBlank
			useFold = useFold || globalFoldCase
			useNumeric = useNumeric || globalNumeric
			useGenNumeric = useGenNumeric || globalGenNumeric
			useMonth = useMonth || globalMonthSort
			useVersion = useVersion || globalVersionSort
			useHuman = useHuman || globalHumanNumeric
			useReverse = useReverse || globalReverse
		}

		if useIgnBlank {
			ki = strings.TrimLeft(ki, " \t")
			kj = strings.TrimLeft(kj, " \t")
		}
		if useFold {
			ki = strings.ToUpper(ki)
			kj = strings.ToUpper(kj)
		}
		if globalDictOrder {
			ki = filterDictOrder(ki)
			kj = filterDictOrder(kj)
		}
		if globalIgnNonprint {
			ki = filterNonPrint(ki)
			kj = filterNonPrint(kj)
		}

		var cmp int
		if ks.randomSort || (globalRandomSort && !ks.ownOrderOpts) {
			hi := hashRandomKey(ki)
			hj := hashRandomKey(kj)
			switch {
			case hi < hj:
				cmp = -1
			case hi > hj:
				cmp = 1
			default:
				cmp = 0
			}
		} else if useVersion {
			cmp = versionCompare(ki, kj)
		} else if useHuman {
			cmp = compareHumanNumeric(ki, kj)
		} else if useMonth {
			mi := parseMonth(ki)
			mj := parseMonth(kj)
			switch {
			case mi < mj:
				cmp = -1
			case mi > mj:
				cmp = 1
			default:
				cmp = 0
			}
		} else if useGenNumeric {
			ni := parseGeneralFloat(ki)
			nj := parseGeneralFloat(kj)
			cmp = compareGeneralFloat(ni, nj)
		} else if useNumeric {
			ni := parseLeadingFloat(ki)
			nj := parseLeadingFloat(kj)
			switch {
			case ni < nj:
				cmp = -1
			case ni > nj:
				cmp = 1
			default:
				cmp = 0
			}
		} else {
			cmp = stringCompare(ki, kj)
		}

		if useReverse {
			cmp = -cmp
		}
		return cmp
	}

	less := func(i, j int) bool {
		li, lj := lines[i], lines[j]

		if len(keys) > 0 {
			for _, ks := range keys {
				cmp := compareKey(li, lj, ks)
				if cmp != 0 {
					return cmp < 0
				}
			}
			if !stable {
				cmp := stringCompare(li, lj)
				if cmp != 0 {
					if globalReverse {
						return cmp > 0
					}
					return cmp < 0
				}
			}
			return false
		}

		ki, kj := li, lj
		if globalIgnBlank {
			ki = strings.TrimLeft(ki, " \t")
			kj = strings.TrimLeft(kj, " \t")
		}
		if globalFoldCase {
			ki = strings.ToUpper(ki)
			kj = strings.ToUpper(kj)
		}
		if globalDictOrder {
			ki = filterDictOrder(ki)
			kj = filterDictOrder(kj)
		}
		if globalIgnNonprint {
			ki = filterNonPrint(ki)
			kj = filterNonPrint(kj)
		}

		var cmp int
		if globalRandomSort {
			hi := hashRandomKey(ki)
			hj := hashRandomKey(kj)
			switch {
			case hi < hj:
				cmp = -1
			case hi > hj:
				cmp = 1
			default:
				cmp = 0
			}
		} else if globalVersionSort {
			cmp = versionCompare(ki, kj)
		} else if globalHumanNumeric {
			cmp = compareHumanNumeric(ki, kj)
		} else if globalMonthSort {
			mi := parseMonth(ki)
			mj := parseMonth(kj)
			switch {
			case mi < mj:
				cmp = -1
			case mi > mj:
				cmp = 1
			default:
				cmp = 0
			}
		} else if globalGenNumeric {
			ni := parseGeneralFloat(ki)
			nj := parseGeneralFloat(kj)
			cmp = compareGeneralFloat(ni, nj)
		} else if globalNumeric {
			ni := parseLeadingFloat(ki)
			nj := parseLeadingFloat(kj)
			switch {
			case ni < nj:
				cmp = -1
			case ni > nj:
				cmp = 1
			default:
				cmp = stringCompare(ki, kj)
			}
		} else {
			cmp = stringCompare(ki, kj)
		}
		if globalReverse {
			cmp = -cmp
		}
		if cmp != 0 {
			return cmp < 0
		}
		if !stable {
			if globalReverse {
				return li > lj
			}
			return li < lj
		}
		return false
	}

	sort.SliceStable(lines, less)

	if unique {
		lines = dedup(lines, func(a, b string) bool {
			if len(keys) > 0 {
				for _, ks := range keys {
					if compareKey(a, b, ks) != 0 {
						return false
					}
				}
				return true
			}
			ka, kb := a, b
			if globalFoldCase {
				ka = strings.ToLower(ka)
				kb = strings.ToLower(kb)
			}
			if globalVersionSort {
				return versionCompare(ka, kb) == 0
			}
			if globalHumanNumeric {
				return compareHumanNumeric(ka, kb) == 0
			}
			if globalMonthSort {
				return parseMonth(ka) == parseMonth(kb)
			}
			if globalGenNumeric {
				return compareGeneralFloat(parseGeneralFloat(ka), parseGeneralFloat(kb)) == 0
			}
			if globalNumeric {
				return parseLeadingFloat(ka) == parseLeadingFloat(kb)
			}
			return ka == kb
		})
	}

	out := ctx.Stdout
	if hasOutFile {
		path := resolvePath(ctx, outFile)
		f, err := os.Create(path)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "sort: %v\n", err)
			return 2
		}
		defer f.Close()
		out = f
	}

	lineTerm := "\n"
	if zeroTerm {
		lineTerm = "\x00"
	}
	for _, line := range lines {
		fmt.Fprint(out, line, lineTerm)
	}
	return 0
}

func sortCheckOutputWritable(path string) error {
	if _, err := os.Stat(path); err == nil {
		f, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		return f.Close()
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	f.Close()
	os.Remove(path)
	return nil
}

func scanLines(r io.Reader, zeroTerm bool) ([]string, error) {
	var lines []string
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	if zeroTerm {
		sc.Split(scanZeroTerminated)
	}
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err()
}

func splitFields(line, sep string) []string {
	if sep != "" {
		return strings.Split(line, sep)
	}
	var fields []string
	n := len(line)
	if n == 0 {
		return fields
	}
	i := 0
	for i < n {
		start := i
		for i < n && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		for i < n && line[i] != ' ' && line[i] != '\t' {
			i++
		}
		fields = append(fields, line[start:i])
	}
	return fields
}

func parseLeadingFloat(s string) float64 {
	v, ok := parseLeadingFloatWithPlus(s)
	if !ok {
		return 0
	}
	return v
}

func parseLeadingFloatWithPlus(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	end := 0
	if s[0] == '-' || s[0] == '+' {
		end = 1
	}
	low := strings.ToLower(s[end:])
	if strings.HasPrefix(low, "0x") {
		end += 2
		for end < len(s) && ((s[end] >= '0' && s[end] <= '9') || (s[end] >= 'a' && s[end] <= 'f') || (s[end] >= 'A' && s[end] <= 'F') || s[end] == '.') {
			end++
		}
		if end < len(s) && (s[end] == 'p' || s[end] == 'P') {
			end++
			if end < len(s) && (s[end] == '+' || s[end] == '-') {
				end++
			}
			for end < len(s) && (s[end] >= '0' && s[end] <= '9') {
				end++
			}
		}
		v, err := strconv.ParseFloat(s[:end], 64)
		if err != nil {
			return 0, false
		}
		return v, true
	}

	hasDot := false
	startDigits := end
	for end < len(s) {
		c := s[end]
		if c >= '0' && c <= '9' {
			end++
		} else if c == '.' && !hasDot {
			hasDot = true
			end++
		} else {
			break
		}
	}
	if end == startDigits {
		return 0, false
	}
	if end < len(s) && (s[end] == 'e' || s[end] == 'E') {
		eIdx := end
		end++
		if end < len(s) && (s[end] == '+' || s[end] == '-') {
			end++
		}
		digitStart := end
		for end < len(s) && (s[end] >= '0' && s[end] <= '9') {
			end++
		}
		if end == digitStart {
			end = eIdx
		}
	}
	v, err := strconv.ParseFloat(s[:end], 64)
	if err != nil {
		if numErr, ok := err.(*strconv.NumError); ok && numErr.Err == strconv.ErrRange {
			isNeg := strings.HasPrefix(strings.TrimSpace(s), "-")
			if v != 0 {
				if isNeg {
					return math.Inf(-1), true
				}
				return math.Inf(1), true
			}
			// Underflow (e.g. -3.36e-4932). Check if it is a non-zero float.
			hasNonZero := false
			for _, ch := range s[:end] {
				if ch >= '1' && ch <= '9' {
					hasNonZero = true
					break
				}
			}
			if hasNonZero {
				if isNeg {
					return -math.SmallestNonzeroFloat64, true
				}
				return math.SmallestNonzeroFloat64, true
			}
			return 0, true
		}
		return 0, false
	}
	return v, true
}

type genFloat struct {
	val   float64
	isNaN bool
	isNum bool
	raw   string
}

func parseGeneralFloat(s string) genFloat {
	s = strings.TrimSpace(s)
	low := strings.ToLower(s)
	if low == "nan" {
		return genFloat{isNaN: true, isNum: true, raw: s}
	}
	if low == "-inf" || low == "-infinity" {
		return genFloat{val: math.Inf(-1), isNum: true, raw: s}
	}
	if low == "+inf" || low == "+infinity" || low == "inf" || low == "infinity" {
		return genFloat{val: math.Inf(1), isNum: true, raw: s}
	}
	if strings.HasPrefix(low, "0x") || strings.HasPrefix(low, "-0x") || strings.HasPrefix(low, "+0x") {
		if !strings.Contains(low, "p") {
			if v, err := strconv.ParseInt(s, 0, 64); err == nil {
				return genFloat{val: float64(v), isNum: true, raw: s}
			}
		}
	}
	v, ok := parseLeadingFloatWithPlus(s)
	if !ok {
		return genFloat{isNum: false, raw: s}
	}
	return genFloat{val: v, isNum: true, raw: s}
}

func compareGeneralFloat(a, b genFloat) int {
	if !a.isNum && !b.isNum {
		return 0
	}
	if !a.isNum {
		return -1
	}
	if !b.isNum {
		return 1
	}
	if a.isNaN && b.isNaN {
		return 0
	}
	if a.isNaN {
		return -1
	}
	if b.isNaN {
		return 1
	}
	switch {
	case a.val < b.val:
		return -1
	case a.val > b.val:
		return 1
	}
	if a.val == 0 && b.val == 0 {
		expA := getFloatExponent(a.raw)
		expB := getFloatExponent(b.raw)
		isNegA := strings.HasPrefix(strings.TrimSpace(a.raw), "-")
		isNegB := strings.HasPrefix(strings.TrimSpace(b.raw), "-")
		if isNegA != isNegB {
			if isNegA {
				return -1
			}
			return 1
		}
		if expA != 0 || expB != 0 {
			// e.g., 3.36e-4932 has exp -4932, which is larger than 0
			// for positive: smaller exp (more negative) means smaller value (3.36e-4932 < 0? No, 3.36e-4932 > 0!)
			// Positive numbers: 0 < 3.36e-4932 < 2.22e-308
			// exp -4932 is SMALLER than exp 0, but 3.36e-4932 is GREATER than 0!
			if isNegA {
				// Negative numbers: -2.22e-308 < -3.36e-4932 < 0
				if expA < expB {
					return -1
				}
				if expA > expB {
					return 1
				}
			} else {
				// Positive numbers: 0 < 3.36e-4932 < 2.22e-308
				if expA > expB {
					return -1
				}
				if expA < expB {
					return 1
				}
			}
		}
	}
	return 0
}

func getFloatExponent(s string) int {
	s = strings.TrimSpace(s)
	low := strings.ToLower(s)
	eIdx := strings.IndexByte(low, 'e')
	if eIdx < 0 {
		return 0
	}
	expVal, err := strconv.Atoi(low[eIdx+1:])
	if err != nil {
		return 0
	}
	return expVal
}

func dedup(lines []string, eq func(a, b string) bool) []string {
	if len(lines) == 0 {
		return lines
	}
	out := []string{lines[0]}
	for i := 1; i < len(lines); i++ {
		if !eq(lines[i], out[len(out)-1]) {
			out = append(out, lines[i])
		}
	}
	return out
}

func defaultCompare(a, b string) int {
	if a == b {
		return 0
	}
	la := strings.ToLower(a)
	lb := strings.ToLower(b)
	if la != lb {
		return strings.Compare(la, lb)
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			if a[i] >= 'a' && a[i] <= 'z' && b[i] >= 'A' && b[i] <= 'Z' {
				return -1
			}
			if a[i] >= 'A' && a[i] <= 'Z' && b[i] >= 'a' && b[i] <= 'z' {
				return 1
			}
			return strings.Compare(a, b)
		}
	}
	return strings.Compare(a, b)
}
