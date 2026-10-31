package vi

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
)

type shellKind uint8

const (
	shellDisplay shellKind = iota
	shellRead
	shellFilter
	shellWrite
)

type shellJob struct {
	id       uint64
	cancel   context.CancelFunc
	pending  tea.Cmd
	kind     shellKind
	from, to int
	quit     bool
}
type shellResultMsg struct {
	id     uint64
	result shellRunResult
}

func (e *Editor) startShell(command string, stdin []byte, kind shellKind, from, to int, quit bool) {
	if e.macroDepth > 0 || e.replayingChange {
		e.statusMsg = "Shell commands in macros or dot-repeat are unsupported"
		return
	}
	if command == "" {
		e.statusMsg = "Shell error: command required"
		return
	}
	if e.shell != nil {
		e.statusMsg = "Shell command already running"
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.shellNext++
	id := e.shellNext
	dir, env := e.dir, append([]string(nil), e.shellEnv...)
	input := append([]byte(nil), stdin...)
	job := &shellJob{id: id, cancel: cancel, kind: kind, from: from, to: to, quit: quit}
	job.pending = func() tea.Msg { return shellResultMsg{id: id, result: runShell(ctx, command, input, dir, env)} }
	e.shell = job
	e.statusMsg = "Running shell command (Ctrl-C to cancel)"
}
func (e *Editor) takeShellCommand() tea.Cmd {
	if e.shell == nil {
		return nil
	}
	cmd := e.shell.pending
	e.shell.pending = nil
	return cmd
}
func (e *Editor) finishShell(msg shellResultMsg) {
	job := e.shell
	if job == nil || job.id != msg.id {
		return
	}
	job.cancel()
	e.shell = nil
	r := msg.result
	if r.err != nil {
		text := shellStreams(r.stdout, r.stderr)
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		if r.exitCode >= 0 {
			text += fmt.Sprintf("shell returned %d (%v)", r.exitCode, r.err)
		} else {
			text += fmt.Sprintf("Shell error: %v", r.err)
		}
		e.statusMsg = text
		e.shellOutput = &shellOutput{text: text}
		return
	}
	switch job.kind {
	case shellDisplay, shellWrite:
		text := shellStreams(r.stdout, r.stderr)
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += "[exit 0]"
		e.shellOutput = &shellOutput{text: text}
		e.statusMsg = "Shell command completed"
		if job.quit {
			e.Quit = true
		}
	case shellRead:
		e.insertRead([]byte(r.stdout), job.to, "shell")
		if r.stderr != "" {
			e.shellOutput = &shellOutput{text: r.stderr}
		}
	case shellFilter:
		e.replaceFilter([]byte(r.stdout), job.from, job.to)
		if r.stderr != "" {
			e.shellOutput = &shellOutput{text: r.stderr}
		}
	}
}

func shellStreams(stdout, stderr string) string {
	if stdout != "" && stderr != "" && !strings.HasSuffix(stdout, "\n") {
		stdout += "\n"
	}
	return stdout + stderr
}

// decodeLines removes only the final line terminator, retaining every blank line.
func (e *Editor) decodeLines(data []byte) ([]string, bool) {
	if len(data) == 0 {
		return nil, false
	}
	text := string(data)
	if e.file.dos {
		text = strings.ReplaceAll(text, "\r\n", "\n")
	}
	eol := strings.HasSuffix(text, "\n")
	if eol {
		text = text[:len(text)-1]
	}
	return strings.Split(text, "\n"), eol
}
func (e *Editor) insertRead(data []byte, after int, source string) {
	lines, eol := e.decodeLines(data)
	if len(lines) == 0 {
		e.statusMsg = fmt.Sprintf("%s: 0 lines read", source)
		return
	}
	e.beginEdit()
	defer e.commitEdit()
	at := after + 1
	empty := e.fileContent() == ""
	if empty {
		e.lines = nil
		at = 0
	}
	result := make([]string, 0, len(e.lines)+len(lines))
	result = append(result, e.lines[:at]...)
	result = append(result, lines...)
	result = append(result, e.lines[at:]...)
	e.lines = result
	if at == len(result)-len(lines) {
		e.file.eol = eol
	}
	e.file.newEmpty = false
	e.noteEdit(at, 0)
	e.row, e.col = at+len(lines)-1, 0
	e.clampCursor()
	e.statusMsg = fmt.Sprintf("%s: %d lines read", source, len(lines))
}
func (e *Editor) replaceFilter(data []byte, from, to int) {
	lines, eol := e.decodeLines(data)
	result := make([]string, 0, len(e.lines)-(to-from+1)+len(lines))
	result = append(result, e.lines[:from]...)
	result = append(result, lines...)
	result = append(result, e.lines[to+1:]...)
	if len(result) == 0 {
		result = []string{""}
	}
	e.beginEdit()
	defer e.commitEdit()
	atEOF := to == len(e.lines)-1
	e.lines = result
	if atEOF {
		e.file.eol = eol || len(lines) == 0 && from > 0
		e.file.newEmpty = len(data) == 0 && from == 0
	}
	e.noteEdit(from, 0)
	e.row, e.col = from, 0
	e.clampCursor()
	e.statusMsg = fmt.Sprintf("%d lines filtered", len(lines))
}
func (e *Editor) readCmd(arg string, after int) {
	if arg == "" {
		e.statusMsg = "E471: Argument required"
		return
	}
	if strings.HasPrefix(arg, "!") {
		e.startShell(strings.TrimSpace(arg[1:]), nil, shellRead, after, after, false)
		return
	}
	path := e.resolveFile(arg)
	info, err := os.Stat(path)
	if err == nil && !info.Mode().IsRegular() {
		err = fmt.Errorf("%s: not a regular file", path)
	}
	if err != nil {
		e.statusMsg = err.Error()
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		e.statusMsg = err.Error()
		return
	}
	e.insertRead(data, after, filepath.Base(path))
}
