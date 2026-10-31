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
	dos, eol            bool
	newEmpty            bool
}

func New(filename, dir string) *Editor {
	e := &Editor{lines: []string{""}, searchDir: 1, regs: make(map[byte]*regContent), marks: make(map[byte]pos), abbrevs: make(map[string]string), file: fileState{eol: true, newEmpty: true}}
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

func (e *Editor) save() error { return e.saveForce(false) }

func (e *Editor) saveForce(force bool) error {
	if e.filename == "" {
		return fmt.Errorf("no filename")
	}
	same := e.file.path == e.filename
	if same && e.file.readFailed && !force {
		return fmt.Errorf("file was not read; use :w! to overwrite")
	}
	info, err := os.Stat(e.filename)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("%s: not a regular file", e.filename)
	}
	if info != nil && info.Mode().Perm()&0222 == 0 && !force {
		return fmt.Errorf("file is read-only; use :w! to overwrite")
	}
	if same && !force && !e.file.readFailed {
		if e.file.missing != (info == nil) || (info != nil && (e.file.info == nil || !os.SameFile(e.file.info, info) || info.Size() != e.file.info.Size() || !info.ModTime().Equal(e.file.info.ModTime()))) {
			return fmt.Errorf("file changed externally; use :w! to overwrite")
		}
	}
	if err := os.MkdirAll(filepath.Dir(e.filename), 0755); err != nil {
		return err
	}
	content := e.fileContent()
	if err := writeFilePreservingLinks(e.filename, []byte(content), info); err != nil {
		return err
	}
	info, err = os.Stat(e.filename)
	if err != nil {
		return err
	}
	e.file.path, e.file.info, e.file.missing, e.file.readFailed = e.filename, info, false, false
	if content != "" {
		e.file.newEmpty = false
	}
	e.savedEdit()
	return nil
}

func (e *Editor) writeCommand(force, quit bool) {
	if err := e.saveForce(force); err != nil {
		e.statusMsg = err.Error()
		return
	}
	if quit {
		e.Quit = true
		return
	}
	flags := ""
	if e.file.dos {
		flags += " [dos]"
	}
	if !e.file.eol || e.file.newEmpty && e.fileContent() == "" {
		flags += " [noeol]"
	}
	e.statusMsg = fmt.Sprintf("\"%s\"%s %d lines %d bytes written", filepath.Base(e.filename), flags, len(e.lines), len(e.fileContent()))
}
