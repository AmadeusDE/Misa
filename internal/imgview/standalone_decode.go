package imgview

import (
	"strconv"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

// standaloneDecoder bounds incomplete control strings as well as ordinary input.
type standaloneDecoder struct {
	buf                       []byte
	probeID                   uint32
	probing, probeDone, kitty bool
	discardKind               byte
	discardEsc                bool
}

func (d *standaloneDecoder) feed(p []byte) (out []tea.Msg) {
	// Bound retained input even if a caller supplies a very large read.
	for len(p) > 0 {
		n := min(len(p), 4096)
		out = append(out, d.feedChunk(p[:n])...)
		p = p[n:]
	}
	return out
}

func (d *standaloneDecoder) feedChunk(p []byte) (out []tea.Msg) {
	d.buf = append(d.buf, p...)
	for len(d.buf) > 0 {
		if d.discardKind != 0 {
			b := d.buf[0]
			d.buf = d.buf[1:]
			if (d.discardEsc && b == '\\') || (d.discardKind == ']' && b == '\a') {
				d.discardKind, d.discardEsc = 0, false
			} else {
				d.discardEsc = b == 27
			}
			continue
		}
		b := d.buf
		if b[0] != 27 {
			if !utf8.FullRune(b) {
				break
			}
			r, n := utf8.DecodeRune(b)
			d.buf = b[n:]
			k := tea.KeyPressMsg{Code: r}
			switch {
			case r == 13 || r == 10:
				k.Code = tea.KeyEnter
			case r == 9:
				k.Code = tea.KeyTab
			case r == 127 || r == 8:
				k.Code = tea.KeyBackspace
			case r >= 1 && r <= 26:
				k.Code = r + 'a' - 1
				k.Mod = tea.ModCtrl
			case r >= 32:
				k.Text = string(r)
			}
			out = append(out, k)
			continue
		}
		if len(b) < 2 {
			break
		}
		if b[1] == '_' || b[1] == ']' || b[1] == 'P' {
			i := strings.Index(string(b[2:]), "\x1b\\")
			end, terminatorLen := -1, 2
			if i >= 0 {
				end = i + 2
			}
			if b[1] == ']' {
				if bell := strings.IndexByte(string(b[2:]), '\a'); bell >= 0 && (end < 0 || bell+2 < end) {
					end, terminatorLen = bell+2, 1
				}
			}
			if (end < 0 && len(b) > 4096) || end > 4096 {
				// Never reinterpret an oversized string's tail as user input.
				d.discardKind = b[1]
				continue
			}
			if end < 0 {
				break
			}
			if b[1] == '_' && d.probing && !d.probeDone {
				body := string(b[2:end])
				parts := strings.SplitN(body, ";", 2)
				if len(parts) == 2 && strings.HasPrefix(parts[0], "G") && parts[1] == "OK" {
					matched, valid := false, true
					for _, field := range strings.Split(parts[0][1:], ",") {
						pair := strings.SplitN(field, "=", 2)
						if len(pair) != 2 || len(pair[0]) != 1 || !((pair[0][0] >= 'a' && pair[0][0] <= 'z') || (pair[0][0] >= 'A' && pair[0][0] <= 'Z')) || pair[1] == "" {
							valid = false
							break
						}
						if pair[0] == "i" {
							if matched {
								valid = false
								break
							}
							matched = true
							if pair[1] != strconv.FormatUint(uint64(d.probeID), 10) {
								valid = false
								break
							}
						}
					}
					if valid && matched {
						d.kitty = true
					}
				}
			}
			d.buf = b[end+terminatorLen:]
			continue
		}
		if b[1] == '[' || b[1] == 'O' {
			end := 2
			for end < len(b) && !(b[end] >= 0x40 && b[end] <= 0x7e) {
				end++
			}
			if end == len(b) {
				if len(b) > 4096 {
					d.buf = nil
				}
				break
			}
			s := string(b[2:end])
			final := b[end]
			d.buf = b[end+1:]
			if b[1] == '[' && final == 'c' && strings.HasPrefix(s, "?") {
				if d.probing {
					d.probeDone = true
				}
				continue
			}
			if b[1] == '[' && s == "" && final == 'I' {
				out = append(out, tea.FocusMsg{})
				continue
			}
			if b[1] == '[' && s == "" && final == 'O' {
				out = append(out, tea.BlurMsg{})
				continue
			}
			if strings.HasPrefix(s, "<") && (final == 'M' || final == 'm') {
				fields := strings.Split(s[1:], ";")
				if len(fields) == 3 {
					c, e1 := strconv.Atoi(fields[0])
					x, e2 := strconv.Atoi(fields[1])
					y, e3 := strconv.Atoi(fields[2])
					if e1 == nil && e2 == nil && e3 == nil && x > 0 && y > 0 {
						m := tea.Mouse{X: x - 1, Y: y - 1}
						if c&4 != 0 {
							m.Mod |= tea.ModShift
						}
						if c&8 != 0 {
							m.Mod |= tea.ModAlt
						}
						if c&16 != 0 {
							m.Mod |= tea.ModCtrl
						}
						switch {
						case c&64 != 0:
							if c&1 == 0 {
								m.Button = tea.MouseWheelUp
							} else {
								m.Button = tea.MouseWheelDown
							}
							out = append(out, tea.MouseWheelMsg(m))
						case final == 'm':
							m.Button = tea.MouseLeft
							out = append(out, tea.MouseReleaseMsg(m))
						case c&32 != 0:
							m.Button = tea.MouseLeft
							out = append(out, tea.MouseMotionMsg(m))
						default:
							if c&3 == 0 {
								m.Button = tea.MouseLeft
							} else if c&3 == 1 {
								m.Button = tea.MouseMiddle
							} else {
								m.Button = tea.MouseRight
							}
							out = append(out, tea.MouseClickMsg(m))
						}
					}
				}
				continue
			}
			keys := map[byte]rune{'A': tea.KeyUp, 'B': tea.KeyDown, 'C': tea.KeyRight, 'D': tea.KeyLeft, 'H': tea.KeyHome, 'F': tea.KeyEnd, 'P': tea.KeyF1}
			code := keys[final]
			fields := strings.Split(s, ";")
			mod := tea.KeyMod(0)
			if len(fields) > 1 {
				n, _ := strconv.Atoi(strings.Split(fields[1], ":")[0])
				n--
				if n&1 != 0 {
					mod |= tea.ModShift
				}
				if n&2 != 0 {
					mod |= tea.ModAlt
				}
				if n&4 != 0 {
					mod |= tea.ModCtrl
				}
			}
			if final == '~' {
				switch fields[0] {
				case "1", "7":
					code = tea.KeyHome
				case "4", "8":
					code = tea.KeyEnd
				case "5":
					code = tea.KeyPgUp
				case "6":
					code = tea.KeyPgDown
				case "11":
					code = tea.KeyF1
				}
			}
			if final == 'u' {
				n, e := strconv.Atoi(strings.Split(fields[0], ":")[0])
				if e == nil && n > 0 && n <= utf8.MaxRune {
					code = rune(n)
				}
				if len(fields) > 1 && strings.HasSuffix(fields[1], ":3") {
					continue
				}
			}
			if code != 0 {
				k := tea.KeyPressMsg{Code: code, Mod: mod}
				if final == 'u' && code >= 32 && !(code >= 0xe000 && code <= 0xf8ff) && mod&(tea.ModCtrl|tea.ModAlt) == 0 {
					k.Text = string(code)
					if shifted := strings.Split(fields[0], ":"); mod&tea.ModShift != 0 && len(shifted) > 1 {
						if n, e := strconv.Atoi(shifted[1]); e == nil && utf8.ValidRune(rune(n)) && n >= 32 {
							k.Text = string(rune(n))
						}
					}
				}
				out = append(out, k)
			}
			continue
		}
		d.buf = b[1:]
		out = append(out, tea.KeyPressMsg{Code: tea.KeyEsc})
	}
	return
}
func (d *standaloneDecoder) flushEscape() []tea.Msg {
	if len(d.buf) == 1 && d.buf[0] == 27 {
		d.buf = nil
		return []tea.Msg{tea.KeyPressMsg{Code: tea.KeyEsc}}
	}
	return nil
}
