package builtins

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func resolvePath(ctx *Context, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if rooted, ok := rootedOnVolume(ctx.Dir, path, runtime.GOOS == "windows"); ok {
		return rooted
	}
	resolved := filepath.Join(ctx.Dir, path)
	if path != "" && strings.ContainsRune(pathSeps, rune(path[len(path)-1])) && !strings.HasSuffix(resolved, string(os.PathSeparator)) {
		resolved += string(os.PathSeparator)
	}
	return resolved
}

// rootedOnVolume places a Windows path that has a root but no volume (\x or
// /x) on the volume of dir, as Windows itself does. It reports false for
// every other path and always off Windows.
func rootedOnVolume(dir, path string, windows bool) (string, bool) {
	if !windows {
		return "", false
	}
	vol, rest := splitVolume(path, true)
	if vol != "" || !strings.HasPrefix(rest, "/") {
		return "", false
	}
	dvol, _ := splitVolume(dir, true)
	if dvol == "" {
		return "", false
	}
	return filepath.Clean(dvol + path), true
}

func parseOctalMode(s string) (os.FileMode, error) {
	n, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid mode %q", s)
	}
	return os.FileMode(n), nil
}

func expandEscapes(s string) (string, bool) {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] != '\\' {
			out.WriteByte(s[i])
			i++
			continue
		}
		i++
		if i >= len(s) {
			out.WriteByte('\\')
			break
		}
		switch s[i] {
		case '\\':
			out.WriteByte('\\')
			i++
		case 'a':
			out.WriteByte('\a')
			i++
		case 'b':
			out.WriteByte('\b')
			i++
		case 'c':
			return out.String(), true
		case 'e':
			out.WriteByte(0x1b)
			i++
		case 'f':
			out.WriteByte('\f')
			i++
		case 'n':
			out.WriteByte('\n')
			i++
		case 'r':
			out.WriteByte('\r')
			i++
		case 't':
			out.WriteByte('\t')
			i++
		case 'v':
			out.WriteByte('\v')
			i++
		case '0':
			i++
			val := 0
			count := 0
			for count < 3 && i < len(s) && s[i] >= '0' && s[i] <= '7' {
				val = val*8 + int(s[i]-'0')
				i++
				count++
			}
			out.WriteByte(byte(val))
		case '1', '2', '3', '4', '5', '6', '7':
			val := int(s[i] - '0')
			i++
			for j := 0; j < 2 && i < len(s) && s[i] >= '0' && s[i] <= '7'; j++ {
				val = val*8 + int(s[i]-'0')
				i++
			}
			out.WriteByte(byte(val))
		case 'x':
			i++
			if i < len(s) && isHexDigit(s[i]) {
				val := hexVal(s[i])
				i++
				if i < len(s) && isHexDigit(s[i]) {
					val = val*16 + hexVal(s[i])
					i++
				}
				out.WriteByte(byte(val))
			} else {
				out.WriteByte('\\')
				out.WriteByte('x')
			}
		default:
			out.WriteByte('\\')
			out.WriteByte(s[i])
			i++
		}
	}
	return out.String(), false
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	default:
		return int(c-'A') + 10
	}
}

func isDirNotEmpty(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "not empty") || strings.Contains(msg, "directory not empty")
}

func dirIsNonEmpty(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	return err == nil
}

func fmtOsErr(err error) string {
	if text, ok := platformErrText(err); ok {
		return text
	}
	if os.IsNotExist(err) {
		msg := err.Error()
		if strings.Contains(msg, "not a directory") {
			return "Not a directory"
		}
		return "No such file or directory"
	}
	if os.IsPermission(err) {
		return "Permission denied"
	}
	msg := err.Error()
	if strings.Contains(msg, "not a directory") {
		return "Not a directory"
	}
	if idx := strings.LastIndex(msg, ": "); idx >= 0 {
		tail := msg[idx+2:]
		if len(tail) > 0 && tail[0] >= 'a' && tail[0] <= 'z' {
			tail = string(tail[0]-32) + tail[1:]
		}
		return tail
	}
	return msg
}

func parseSizeSuffix(s string) (string, int64) {
	if len(s) == 0 {
		return s, 1
	}
	lastChar := s[len(s)-1]
	if lastChar == 'g' || lastChar == 't' || lastChar == 'p' || lastChar == 'e' || lastChar == 'z' || lastChar == 'y' || lastChar == 'r' || lastChar == 'q' {
		return s, 1
	}
	upper := strings.ToUpper(s)
	type suffixEntry struct {
		suffix string
		mult   int64
	}
	suffixes := []suffixEntry{
		{"QB", math.MaxInt64},
		{"QIB", math.MaxInt64},
		{"QD", math.MaxInt64},
		{"Q", math.MaxInt64},
		{"RB", math.MaxInt64},
		{"RIB", math.MaxInt64},
		{"RD", math.MaxInt64},
		{"R", math.MaxInt64},
		{"YB", math.MaxInt64},
		{"YIB", math.MaxInt64},
		{"YD", math.MaxInt64},
		{"Y", math.MaxInt64},
		{"ZB", math.MaxInt64},
		{"ZIB", math.MaxInt64},
		{"ZD", math.MaxInt64},
		{"Z", math.MaxInt64},
		{"EB", 1000 * 1000 * 1000 * 1000 * 1000 * 1000},
		{"EIB", 1 << 60},
		{"ED", 1000 * 1000 * 1000 * 1000 * 1000 * 1000},
		{"E", 1 << 60},
		{"PB", 1000 * 1000 * 1000 * 1000 * 1000},
		{"PIB", 1 << 50},
		{"PD", 1000 * 1000 * 1000 * 1000 * 1000},
		{"P", 1 << 50},
		{"TB", 1000 * 1000 * 1000 * 1000},
		{"TIB", 1 << 40},
		{"TD", 1000 * 1000 * 1000 * 1000},
		{"T", 1 << 40},
		{"GB", 1000 * 1000 * 1000},
		{"GIB", 1 << 30},
		{"GD", 1000 * 1000 * 1000},
		{"G", 1 << 30},
		{"MB", 1000 * 1000},
		{"MIB", 1 << 20},
		{"MD", 1000 * 1000},
		{"M", 1 << 20},
		{"KB", 1000},
		{"KIB", 1 << 10},
		{"KD", 1000},
		{"K", 1 << 10},
		{"B", 512},
	}
	for _, e := range suffixes {
		if strings.HasSuffix(upper, e.suffix) {
			return s[:len(s)-len(e.suffix)], e.mult
		}
	}
	return s, 1
}

func scanZeroTerminated(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, 0); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func isTUIMode(ctx *Context) bool {
	_, ok := ctx.Stdout.(*bytes.Buffer)
	return ok
}
