package vi

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type fileState struct {
	path                string
	info                os.FileInfo
	missing, readFailed bool
	notEdited           bool
	dos, eol            bool
	newEmpty            bool
}

func New(filename, dir string) *Editor {
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	e := &Editor{dir: dir, lines: []string{""}, searchDir: 1, regs: make(map[byte]*regContent), marks: make(map[byte]pos), abbrevs: make(map[string]string), file: fileState{eol: true, newEmpty: true}}
	if filename != "" {
		if !filepath.IsAbs(filename) {
			filename = filepath.Join(dir, filename)
		}
		e.filename = filename
		e.loadFile()
	}
	return e
}

func (e *Editor) loadFile() {
	e.file.path = e.filename
	info, err := os.Stat(e.filename)
	if err != nil {
		if os.IsNotExist(err) {
			e.file.missing = true
			e.statusMsg = fmt.Sprintf("\"%s\" [New]", filepath.Base(e.filename))
			return
		}
		e.file.readFailed = true
		e.statusMsg = err.Error()
		return
	}
	e.file.info = info
	e.file.newEmpty = false
	if !info.Mode().IsRegular() {
		e.file.readFailed = true
		e.statusMsg = fmt.Sprintf("%s: not a regular file", e.filename)
		return
	}
	data, err := os.ReadFile(e.filename)
	if err != nil {
		e.file.readFailed = true
		e.statusMsg = err.Error()
		return
	}
	text := string(data)
	e.file.eol = strings.HasSuffix(text, "\n")
	e.file.dos = strings.Contains(text, "\n") && !strings.Contains(strings.ReplaceAll(text, "\r\n", ""), "\n")
	if e.file.dos {
		text = strings.ReplaceAll(text, "\r\n", "\n")
	}
	e.lines = strings.Split(text, "\n")
	if e.file.eol {
		e.lines = e.lines[:len(e.lines)-1]
	}
	if len(e.lines) == 0 {
		e.lines = []string{""}
	}
}

func (e *Editor) fileContent() string {
	if e.file.newEmpty && len(e.lines) == 1 && e.lines[0] == "" {
		return ""
	}
	sep := "\n"
	if e.file.dos {
		sep = "\r\n"
	}
	content := strings.Join(e.lines, sep)
	if e.file.eol {
		content += sep
	}
	return content
}
