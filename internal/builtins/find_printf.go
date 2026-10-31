// Formatting inspired by Toybox find.c, copyright 2014 Rob Landley,
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb (0BSD).
package builtins

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

type findFormatPart struct {
	text      string
	directive byte
	stop      bool
}

func findPrintf(format string) (findNode, error) {
	var parts []findFormatPart
	var literal strings.Builder
	flush := func() {
		if literal.Len() > 0 {
			parts = append(parts, findFormatPart{text: literal.String()})
			literal.Reset()
		}
	}
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c == '%' {
			flush()
			i++
			if i >= len(format) {
				return nil, fmt.Errorf("incomplete printf directive")
			}
			c = format[i]
			if c == '%' {
				literal.WriteByte('%')
				continue
			}
			if c == 'T' {
				i++
				if i >= len(format) || format[i] != '@' {
					return nil, fmt.Errorf("unsupported printf directive %%T")
				}
				c = '@'
			} else if !strings.ContainsRune("pfhPsmMugtyd", rune(c)) {
				return nil, fmt.Errorf("unsupported printf directive %%%c", c)
			}
			parts = append(parts, findFormatPart{directive: c})
			continue
		}
		if c == '\\' {
			i++
			if i >= len(format) {
				return nil, fmt.Errorf("trailing backslash in printf format")
			}
			c = format[i]
			switch c {
			case 'a':
				c = '\a'
			case 'b':
				c = '\b'
			case 'f':
				c = '\f'
			case 'n':
				c = '\n'
			case 'r':
				c = '\r'
			case 't':
				c = '\t'
			case 'v':
				c = '\v'
			case '\\':
				c = '\\'
			case 'c':
				flush()
				parts = append(parts, findFormatPart{stop: true})
				continue
			default:
				if c < '0' || c > '7' {
					return nil, fmt.Errorf("unsupported printf escape \\%c", c)
				}
				n := int(c - '0')
				for j := 0; j < 2 && i+1 < len(format) && format[i+1] >= '0' && format[i+1] <= '7'; j++ {
					i++
					n = n*8 + int(format[i]-'0')
				}
				c = byte(n)
			}
		}
		literal.WriteByte(c)
	}
	flush()
	return func(e *findEntry) bool {
		var out strings.Builder
		for _, part := range parts {
			if part.stop {
				break
			}
			if part.directive == 0 {
				out.WriteString(part.text)
				continue
			}
			var s string
			switch part.directive {
			case 'p':
				s = e.path
			case 'f':
				s = filepath.Base(e.path)
			case 'h':
				s = filepath.Dir(e.path)
			case 'P':
				if e.path != e.root {
					s = strings.TrimPrefix(e.path, strings.TrimRight(e.root, "/")+"/")
				}
			case 's':
				s = strconv.FormatInt(e.info.Size(), 10)
			case 'm':
				s = strconv.FormatUint(uint64(findMode(e.info)), 8)
			case 'M':
				s = findModeString(e.info)
			case 'u', 'g':
				st := findStat(e.info)
				if st == nil {
					return e.fail(fmt.Errorf("metadata unavailable: %s", e.path))
				}
				if part.directive == 'u' {
					s = statLookupUser(st.Uid)
				} else {
					s = statLookupGroup(st.Gid)
				}
			case 't':
				s = e.info.ModTime().In(e.ctx.Location()).Format("Mon Jan _2 15:04:05.0000000000 2006")
			case '@':
				t := e.info.ModTime()
				sec, ns := t.Unix(), t.Nanosecond()
				if sec < 0 && ns != 0 {
					s = fmt.Sprintf("-%d.%09d0", -(sec + 1), 1000000000-ns)
				} else {
					s = fmt.Sprintf("%d.%09d0", sec, ns)
				}
			case 'y':
				s = string(findType(e.info))
			case 'd':
				s = strconv.Itoa(e.depth)
			}
			out.WriteString(s)
		}
		_, err := fmt.Fprint(e.ctx.Stdout, out.String())
		if err != nil {
			return e.fail(err)
		}
		return true
	}, nil
}
