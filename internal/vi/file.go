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
	e := &Editor{
		shellEnv: os.Environ(), dir: dir, searchDir: 1,
		regs: make(map[byte]*regContent), marks: make(map[byte]pos), abbrevs: make(map[string]string),
	}
	if filename != "" && !filepath.IsAbs(filename) {
		filename = filepath.Join(dir, filename)
	}
	var err error
	e.bufferIdentity, err = readBuffer(filename)
	if err != nil {
		e.statusMsg = err.Error()
	} else if e.file.missing {
		e.statusMsg = fmt.Sprintf("\"%s\" [New]", filepath.Base(filename))
	}
	return e
}

func readBuffer(path string) (bufferIdentity, error) {
	b := bufferIdentity{filename: path, lines: []string{""}, file: fileState{path: path, eol: true, newEmpty: true}}
	if path == "" {
		return b, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			b.file.missing = true
			return b, nil
		}
		b.file.readFailed = true
		return b, err
	}
	b.file.info = info
	b.file.newEmpty = false
	if !info.Mode().IsRegular() {
		b.file.readFailed = true
		return b, fmt.Errorf("%s: not a regular file", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		b.file.readFailed = true
		return b, err
	}
	text := string(data)
	b.file.eol = strings.HasSuffix(text, "\n")
	b.file.dos = strings.Contains(text, "\n") && !strings.Contains(strings.ReplaceAll(text, "\r\n", ""), "\n")
	if b.file.dos {
		text = strings.ReplaceAll(text, "\r\n", "\n")
	}
	b.lines = strings.Split(text, "\n")
	if b.file.eol {
		b.lines = b.lines[:len(b.lines)-1]
	}
	if len(b.lines) == 0 {
		b.lines = []string{""}
	}
	return b, nil
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
