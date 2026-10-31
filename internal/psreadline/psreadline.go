// Package psreadline reads PowerShell's PSReadLine history file. It only reads: Misa never writes external
// shell history. Entries carry no timestamps.
package psreadline

import (
	"bytes"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

// Path is the history file under appData (%APPDATA%), or "" when appData is empty.
func Path(appData string) string {
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, "Microsoft", "Windows", "PowerShell", "PSReadLine", "ConsoleHost_history.txt")
}

// Parse returns the history entries oldest first. A line ending in an odd number of backticks continues onto the
// next line; the entry joins the lines with "\n" without the backtick. The text may be UTF-8 (with or without a
// BOM) or UTF-16 (BOM, or LE without one), and lines may end in CRLF or LF.
func Parse(data []byte) []string {
	text := decode(data)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	var entries []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			if e := strings.Join(cur, "\n"); strings.TrimSpace(e) != "" {
				entries = append(entries, e)
			}
			cur = nil
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if continues(line) {
			cur = append(cur, line[:len(line)-1])
			continue
		}
		cur = append(cur, line)
		flush()
	}
	flush()
	return entries
}

func continues(line string) bool {
	n := len(line) - len(strings.TrimRight(line, "`"))
	return n%2 == 1
}

func decode(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		data = data[3:]
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		return utf16Text(data[2:], false)
	case bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		return utf16Text(data[2:], true)
	case looksUTF16LE(data):
		return utf16Text(data, false)
	}
	return strings.ToValidUTF8(string(data), "�")
}

// looksUTF16LE spots BOM-less UTF-16LE: mostly ASCII text leaves a NUL in every high byte.
func looksUTF16LE(data []byte) bool {
	if len(data) < 2 || len(data)%2 != 0 {
		return false
	}
	nul := 0
	for i := 1; i < len(data); i += 2 {
		if data[i] == 0 {
			nul++
		}
	}
	return nul*2 > len(data)/2 && !hasEvenNUL(data)
}

func hasEvenNUL(data []byte) bool {
	for i := 0; i < len(data); i += 2 {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

func utf16Text(b []byte, bigEndian bool) string {
	u := make([]uint16, len(b)/2)
	for i := range u {
		if bigEndian {
			u[i] = uint16(b[2*i])<<8 | uint16(b[2*i+1])
		} else {
			u[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
		}
	}
	return string(utf16.Decode(u))
}
