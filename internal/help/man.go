package help

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func FindManPage(command string) (string, bool) {
	if !validPageName(command) {
		return "", false
	}
	manDir := "/usr/share/man"
	sections := []string{"1", "8", "6", "2", "3", "4", "5", "7", "1p", "3p"}
	for _, sec := range sections {
		for _, ext := range []string{".gz", ""} {
			path := filepath.Join(manDir, "man"+sec, command+"."+sec+ext)
			if _, err := os.Stat(path); err == nil {
				return path, true
			}
		}
	}
	matches, _ := filepath.Glob(filepath.Join(manDir, "man*", command+".[1-8]*"))
	if len(matches) > 0 {
		return matches[0], true
	}
	return "", false
}

func ParseManPage(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var reader io.Reader = f
	if filepath.Ext(path) == ".gz" {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return "", err
		}
		defer gz.Close()
		reader = gz
	}
	return parseRoff(reader)
}

func ListManPages() []string {
	manDir := "/usr/share/man"
	seen := map[string]bool{}
	var names []string
	for _, sec := range []string{"1", "8", "6"} {
		entries, err := os.ReadDir(filepath.Join(manDir, "man"+sec))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			name = strings.TrimSuffix(name, ".gz")
			if dot := strings.LastIndex(name, "."); dot > 0 {
				name = name[:dot]
			}
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	return names
}

func parseRoff(r io.Reader) (string, error) {
	var out strings.Builder
	scanner := bufio.NewScanner(r)
	inPre := false
	pendingTag := false
	indent := 0

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, `.\"`) || strings.HasPrefix(line, `.'`) {
			continue
		}
		if strings.HasPrefix(line, ".") {
			parts := strings.Fields(line)
			if len(parts) == 0 {
				continue
			}
			macro := parts[0]
			args := parseMacroArgs(line)

			switch macro {
			case ".TH":
				if len(args) >= 2 {
					out.WriteString(fmt.Sprintf("# %s(%s)\n\n", args[0], args[1]))
				}
			case ".SH":
				pendingTag = false
				out.WriteString(fmt.Sprintf("\n\n## %s\n\n", strings.ToUpper(strings.Join(args, " "))))
			case ".SS":
				pendingTag = false
				out.WriteString(fmt.Sprintf("\n\n### %s\n\n", strings.Join(args, " ")))
			case ".PP", ".LP", ".P":
				pendingTag = false
				out.WriteString("\n\n")
			case ".sp":
				out.WriteString("\n\n")
			case ".nf":
				inPre = true
				out.WriteString("\n```\n")
			case ".fi":
				inPre = false
				out.WriteString("\n```\n")
			case ".RS":
				indent += 2
			case ".RE":
				indent -= 2
				if indent < 0 {
					indent = 0
				}
			case ".TP":
				pendingTag = true
			case ".IP":
				pendingTag = false
				tag := ""
				if len(args) > 0 {
					tag = cleanInline(args[0])
				}
				curr := out.String()
				if len(curr) > 0 && !strings.HasSuffix(curr, "\n\n") {
					if strings.HasSuffix(curr, "\n") {
						out.WriteString("\n")
					} else {
						out.WriteString("\n\n")
					}
				}
				if tag != "" && tag != "•" && tag != "*" {
					out.WriteString(fmt.Sprintf("* **%s** ", tag))
				} else {
					out.WriteString("* ")
				}
			case ".B":
				pendingTag = false
				out.WriteString(fmt.Sprintf("**%s** ", cleanInline(strings.Join(args, " "))))
			case ".I":
				pendingTag = false
				out.WriteString(fmt.Sprintf("*%s* ", cleanInline(strings.Join(args, " "))))
			case ".BR", ".BI", ".IR", ".IB", ".RB", ".RI":
				pendingTag = false
				out.WriteString(formatAlternating(macro, args) + " ")
			}
			continue
		}

		cleaned := cleanInline(stripTtyLinks(line))
		if inPre {
			out.WriteString(cleaned + "\n")
		} else if pendingTag {
			tag := cleaned
			if strings.HasPrefix(tag, "**") && strings.HasSuffix(tag, "**") && len(tag) >= 4 {
				tag = tag[2 : len(tag)-2]
			}
			curr := out.String()
			if len(curr) > 0 && !strings.HasSuffix(curr, "\n\n") {
				if strings.HasSuffix(curr, "\n") {
					out.WriteString("\n")
				} else {
					out.WriteString("\n\n")
				}
			}
			out.WriteString(fmt.Sprintf("* **%s**\n  ", tag))
			pendingTag = false
		} else if indent > 0 {
			out.WriteString(strings.Repeat(" ", indent) + cleaned + "\n")
		} else {
			out.WriteString(cleaned + " ")
		}
	}
	if inPre {
		out.WriteString("\n```\n")
	}
	result := regexp.MustCompile(`\n{3,}`).ReplaceAllString(out.String(), "\n\n")
	return strings.TrimSpace(result), scanner.Err()
}

func parseMacroArgs(line string) []string {
	var args []string
	var cur strings.Builder
	inQ := false
	runes := []rune(line)
	i := 0
	for i < len(runes) && runes[i] != ' ' && runes[i] != '\t' {
		i++
	}
	for i < len(runes) {
		r := runes[i]
		if (r == ' ' || r == '\t') && !inQ {
			if cur.Len() > 0 {
				args = append(args, cur.String())
				cur.Reset()
			}
		} else if r == '"' {
			inQ = !inQ
		} else {
			cur.WriteRune(r)
		}
		i++
	}
	if cur.Len() > 0 {
		args = append(args, cur.String())
	}
	return args
}

func cleanInline(s string) string {
	s = strings.ReplaceAll(s, `\-`, "-")
	s = strings.ReplaceAll(s, `\&`, "")
	s = strings.ReplaceAll(s, `\e`, `\`)
	s = strings.ReplaceAll(s, `\*(lq`, `"`)
	s = strings.ReplaceAll(s, `\*(rq`, `"`)
	s = strings.ReplaceAll(s, `\(aq`, `'`)
	s = strings.ReplaceAll(s, `\(dq`, `"`)
	s = strings.ReplaceAll(s, `\(em`, " — ")
	s = strings.ReplaceAll(s, `\(en`, "-")
	s = strings.ReplaceAll(s, `\(bu`, "• ")
	s = strings.ReplaceAll(s, `\,`, "")
	s = strings.ReplaceAll(s, `\/`, "")
	s = strings.ReplaceAll(s, `\^`, "")
	s = strings.ReplaceAll(s, `\|`, "")

	var out strings.Builder
	runes := []rune(s)
	bOpen, iOpen, cOpen := false, false, false
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' && i+2 < len(runes) && runes[i+1] == 'f' {
			ft := runes[i+2]
			i += 2
			switch ft {
			case 'B':
				if !bOpen {
					out.WriteString("**")
					bOpen = true
				}
			case 'I':
				if !iOpen {
					out.WriteString("*")
					iOpen = true
				}
			case 'C':
				if i+1 < len(runes) && runes[i+1] == '(' {
					i += 3
				}
				if !cOpen {
					out.WriteString("`")
					cOpen = true
				}
			case 'R', 'P':
				if bOpen {
					out.WriteString("**")
					bOpen = false
				}
				if iOpen {
					out.WriteString("*")
					iOpen = false
				}
				if cOpen {
					out.WriteString("`")
					cOpen = false
				}
			}
		} else {
			out.WriteRune(runes[i])
		}
	}
	if bOpen {
		out.WriteString("**")
	}
	if iOpen {
		out.WriteString("*")
	}
	if cOpen {
		out.WriteString("`")
	}
	return out.String()
}

func formatAlternating(macro string, args []string) string {
	var out strings.Builder
	odd := true
	for _, arg := range args {
		c := cleanInline(arg)
		if c == "" {
			continue
		}
		switch macro {
		case ".BR":
			if odd {
				fmt.Fprintf(&out, "**%s**", c)
			} else {
				out.WriteString(c)
			}
		case ".BI":
			if odd {
				fmt.Fprintf(&out, "**%s**", c)
			} else {
				fmt.Fprintf(&out, "*%s*", c)
			}
		case ".IR":
			if odd {
				fmt.Fprintf(&out, "*%s*", c)
			} else {
				out.WriteString(c)
			}
		case ".IB":
			if odd {
				fmt.Fprintf(&out, "*%s*", c)
			} else {
				fmt.Fprintf(&out, "**%s**", c)
			}
		case ".RB":
			if odd {
				out.WriteString(c)
			} else {
				fmt.Fprintf(&out, "**%s**", c)
			}
		case ".RI":
			if odd {
				out.WriteString(c)
			} else {
				fmt.Fprintf(&out, "*%s*", c)
			}
		}
		odd = !odd
	}
	return out.String()
}

func stripTtyLinks(s string) string {
	for {
		idx := strings.Index(s, "\\X'tty: link")
		if idx == -1 {
			break
		}
		end := idx + len("\\X'tty: link")
		for end < len(s) && s[end] != '\'' {
			end++
		}
		if end >= len(s) {
			break
		}
		s = s[:idx] + s[end+1:]
	}
	return strings.ReplaceAll(s, "\\X'tty: link'", "")
}
