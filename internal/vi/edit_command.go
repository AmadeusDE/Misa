package vi

import (
	"fmt"
	"path/filepath"
	"strings"
)

func parseEditCommand(command string) (arg string, force, ok bool) {
	i := 0
	for i < len(command) && command[i] >= 'a' && command[i] <= 'z' {
		i++
	}
	switch command[:i] {
	case "e", "ed", "edi", "edit":
	default:
		return "", false, false
	}
	if i < len(command) && command[i] == '!' {
		force = true
		i++
	}
	if i < len(command) && command[i] != ' ' && command[i] != '\t' {
		return "", false, false
	}
	return strings.TrimSpace(command[i:]), force, true
}

func (e *Editor) editCommand(command string, ranged bool) bool {
	arg, force, ok := parseEditCommand(command)
	if !ok {
		return false
	}
	if ranged {
		e.statusMsg = "E481: No range allowed"
		return true
	}
	if e.modified && !force {
		e.statusMsg = "E37: No write since last change (add ! to override)"
		return true
	}
	path := e.filename
	if arg != "" {
		var err error
		path, err = e.expandEditPath(arg)
		if err != nil {
			e.statusMsg = err.Error()
			return true
		}
	}
	if path == "" {
		e.statusMsg = "E32: No file name"
		return true
	}
	b, err := readBuffer(path)
	if err != nil {
		e.statusMsg = err.Error()
		return true
	}
	e.replaceBuffer(b)
	flags := ""
	if e.file.missing {
		flags += " [New]"
	}
	if e.file.dos {
		flags += " [dos]"
	}
	if !e.file.eol {
		flags += " [noeol]"
	}
	e.statusMsg = fmt.Sprintf("\"%s\"%s %d lines, %d bytes", filepath.Base(path), flags, len(e.lines), len(e.fileContent()))
	return true
}
