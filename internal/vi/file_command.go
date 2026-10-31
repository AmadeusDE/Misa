package vi

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type writeRequest struct {
	path                          string
	from, to                      int
	force, ranged, append, rename bool
}

func (e *Editor) resolveFile(name string) string {
	if !filepath.IsAbs(name) {
		name = filepath.Join(e.dir, name)
	}
	return filepath.Clean(name)
}

func (e *Editor) fileCommand(rest string, from, to int, ranged bool) bool {
	i := 0
	for i < len(rest) && rest[i] >= 'a' && rest[i] <= 'z' {
		i++
	}
	verb := rest[:i]
	switch verb {
	case "w", "write", "wq", "x", "xit", "sav", "save", "savea", "saveas", "f", "fi", "fil", "file":
	default:
		return false
	}
	force := i < len(rest) && rest[i] == '!'
	if force {
		i++
	}
	if i < len(rest) && rest[i] != ' ' && rest[i] != '\t' {
		return false
	}
	arg := strings.TrimSpace(rest[i:])
	if strings.HasPrefix(verb, "f") {
		if ranged {
			e.statusMsg = "E481: No range allowed"
			return true
		}
		e.renameFile(arg)
		return true
	}
	if ranged && !strings.HasPrefix(verb, "sav") && (from < 0 || to < 0 || from >= len(e.lines) || to >= len(e.lines) || from > to) {
		e.statusMsg = "E16: Invalid range"
		return true
	}
	quit := verb == "wq" || verb == "x" || verb == "xit"
	if (verb == "x" || verb == "xit") && !e.modified {
		e.Quit = true
		return true
	}
	if !ranged {
		from, to = 0, len(e.lines)-1
	}
	r := writeRequest{from: from, to: to, force: force, ranged: ranged, rename: strings.HasPrefix(verb, "sav")}
	if r.rename && (ranged || arg == "") {
		e.statusMsg = "E471: Argument required (no range allowed)"
		return true
	}
	if strings.HasPrefix(arg, "!") && !r.rename {
		if e.writeShell(strings.TrimSpace(arg[1:]), from, to) && quit {
			e.Quit = true
		}
		return true
	}
	if strings.HasPrefix(arg, ">>") && !r.rename {
		r.append = true
		arg = strings.TrimSpace(arg[2:])
	}
	r.path = e.filename
	if arg != "" {
		r.path = e.resolveFile(arg)
	}
	if err := e.writeFile(r); err != nil {
		e.statusMsg = err.Error()
	} else if quit {
		e.Quit = true
	}
	return true
}

func (e *Editor) renameFile(name string) {
	if name != "" {
		path := e.resolveFile(name)
		if path != e.filename {
			e.alternateFilename, e.filename = e.filename, path
			e.file.path, e.file.info = path, nil
			e.file.missing, e.file.readFailed, e.file.notEdited = false, false, true
		}
	}
	name = filepath.Base(e.filename)
	if e.filename == "" {
		name = "[No Name]"
	}
	flags := ""
	if e.file.notEdited {
		flags += " [Not edited]"
	}
	if e.modified {
		flags += " [Modified]"
	}
	e.statusMsg = fmt.Sprintf("\"%s\"%s %d lines", name, flags, len(e.lines))
}

func (e *Editor) save() error { return e.saveForce(false) }

func (e *Editor) saveForce(force bool) error {
	return e.writeFile(writeRequest{path: e.filename, from: 0, to: len(e.lines) - 1, force: force})
}

func (e *Editor) rangeContent(from, to int) string {
	if from == 0 && to == len(e.lines)-1 {
		return e.fileContent()
	}
	sep := "\n"
	if e.file.dos {
		sep = "\r\n"
	}
	content := strings.Join(e.lines[from:to+1], sep)
	if to < len(e.lines)-1 || e.file.eol {
		content += sep
	}
	return content
}

func (e *Editor) writeStatus(r writeRequest, content string, newFile bool) {
	flags := ""
	if newFile {
		flags += " [New]"
	}
	if e.file.dos {
		flags += " [dos]"
	}
	if !strings.HasSuffix(content, "\n") {
		flags += " [noeol]"
	}
	action := "written"
	if r.append {
		action = "appended"
	}
	e.statusMsg = fmt.Sprintf("\"%s\"%s %d lines, %d bytes %s", filepath.Base(r.path), flags, r.to-r.from+1, len(content), action)
}

func (e *Editor) writeFile(r writeRequest) error {
	if r.path == "" {
		return fmt.Errorf("no filename")
	}
	info, err := os.Stat(r.path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if info != nil && !info.Mode().IsRegular() {
		return fmt.Errorf("%s: not a regular file", r.path)
	}
	same := r.path == e.filename || info != nil && e.file.info != nil && os.SameFile(info, e.file.info)
	if same && r.ranged && !r.append && (r.from != 0 || r.to != len(e.lines)-1) && !r.force {
		return fmt.Errorf("E140: Use ! to write partial buffer")
	}
	if !r.force && !r.append && info != nil && (!same || e.file.notEdited) {
		return fmt.Errorf("E13: File exists (add ! to override)")
	}
	if r.append && info == nil && !r.force {
		return fmt.Errorf("E212: Can't open file for writing (add ! to create)")
	}
	if info != nil && info.Mode().Perm()&0222 == 0 && !r.force {
		return fmt.Errorf("file is read-only; use :w! to overwrite")
	}
	if same && e.file.readFailed && !r.force {
		return fmt.Errorf("file was not read; use :w! to overwrite")
	}
	if same && !r.force && !e.file.readFailed && !e.file.notEdited {
		if e.file.missing != (info == nil) || info != nil && (e.file.info == nil || !os.SameFile(e.file.info, info) || info.Size() != e.file.info.Size() || !info.ModTime().Equal(e.file.info.ModTime())) {
			return fmt.Errorf("file changed externally; use :w! to overwrite")
		}
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0755); err != nil {
		return err
	}
	content := e.rangeContent(r.from, r.to)
	if r.append {
		err = appendFile(r.path, []byte(content))
	} else {
		err = writeFilePreservingLinks(r.path, []byte(content), info)
	}
	if err != nil {
		return err
	}
	updated, err := os.Stat(r.path)
	if err != nil {
		return err
	}
	adopt := r.rename || e.filename == "" && !r.append && !r.ranged
	if adopt {
		if e.filename != r.path {
			e.alternateFilename = e.filename
		}
		e.filename = r.path
	} else if !same {
		e.alternateFilename = r.path
	}
	if same || adopt {
		e.file.path, e.file.info = r.path, updated
		e.file.missing, e.file.readFailed, e.file.notEdited = false, false, false
		if !r.ranged && !r.append {
			e.savedEdit()
		}
		if content != "" {
			e.file.newEmpty = false
		}
	}
	e.writeStatus(r, content, info == nil)
	return nil
}
