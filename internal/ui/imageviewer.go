package ui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/builtins"
)

// imageViewerWords handles shell quoting for a simple imgview invocation. It
// declines expansion, pipelines and redirection so the normal shell handles
// those rather than silently opening the wrong filename.
func imageViewerWords(input string) ([]string, bool) {
	var words []string
	var word strings.Builder
	var quote rune
	started, escaped := false, false
	for _, c := range input {
		if escaped {
			if quote == '"' && c != '$' && c != '`' && c != '"' && c != '\\' && c != '\n' {
				word.WriteRune('\\')
			}
			if c != '\n' {
				word.WriteRune(c)
			}
			started = true
			escaped = false
			continue
		}
		if quote == '\'' {
			if c == '\'' {
				quote = 0
			} else {
				word.WriteRune(c)
			}
			continue
		}
		if c == '\\' && quote != '"' {
			escaped = true
			started = true
			continue
		}
		if quote == '"' {
			if c == '\\' {
				escaped = true
				started = true
				continue
			}
			if c == '"' {
				quote = 0
			} else if c == '$' || c == '`' {
				return nil, false
			} else {
				word.WriteRune(c)
			}
			continue
		}
		switch {
		case c == '\'' || c == '"':
			quote = c
			started = true
		case unicode.IsSpace(c):
			if c == '\n' || c == '\r' {
				return nil, false
			}
			if started {
				words = append(words, word.String())
				word.Reset()
				started = false
			}
		case strings.ContainsRune("|&;<>()$`*?[~#{}", c):
			return nil, false
		default:
			word.WriteRune(c)
			started = true
		}
	}
	if escaped || quote != 0 {
		return nil, false
	}
	if started {
		words = append(words, word.String())
	}
	return words, true
}

func (m *Model) tryImageViewer(input string) (bool, tea.Cmd) {
	words, ok := imageViewerWords(input)
	if !ok || len(words) == 0 || words[0] != "imgview" {
		return false, nil
	}
	_, err := builtins.ImgviewViewer(words[1:], m.dir)
	if errors.Is(err, builtins.ErrImgviewInfo) {
		return false, nil // the builtin prints help and version
	}
	if err != nil {
		m.shellBuffer = append(m.shellBuffer, "imgview: "+err.Error())
		m.lastExitCode = 1
		m.cmdInput.SetValue("")
		m.updateShellView()
		return true, nil
	}
	m.cmdInput.SetValue("")
	return true, m.launchImageViewer(m.dir, words[1:])
}

// Injectable process boundary keeps tests from ever executing the test binary.
var imageViewerExecutable = os.Executable
var imageViewerExecProcess = tea.ExecProcess

type imageViewerFinished struct {
	owner       *Model
	dir         string
	args        []string
	err         error
	startFailed bool
}

// launchImageViewer hands the terminal to the standalone multicall viewer.
// The caller validates arguments first; the file browser can reuse this seam.
func (m *Model) launchImageViewer(dir string, args []string) tea.Cmd {
	args = append([]string(nil), args...)
	exe, err := imageViewerExecutable()
	if err == nil {
		var info os.FileInfo
		info, err = os.Stat(exe)
		if err == nil && (!info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0) {
			err = fmt.Errorf("%s is not an executable file", exe)
		}
	}
	if err != nil {
		return m.imageViewerFallback(dir, args, err)
	}
	cmd := exec.Command(exe, append([]string{"imgview"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "MISA_IMGVIEW_EMBEDDED=1")
	return imageViewerExecProcess(cmd, func(err error) tea.Msg {
		return imageViewerFinished{owner: m, dir: dir, args: args, err: err, startFailed: err != nil && cmd.Process == nil}
	})
}

func (m *Model) imageViewerFallback(dir string, args []string, cause error) tea.Cmd {
	viewer, err := builtins.ImgviewViewer(args, dir)
	if err != nil {
		m.shellBuffer = append(m.shellBuffer, "imgview: "+err.Error())
		m.lastExitCode = 1
		m.updateShellView()
		return nil
	}
	viewer.SetNotice("standalone imgview unavailable; using in-UI viewer: " + cause.Error())
	enter := m.enterProcMon(viewer)
	return tea.Batch(enter, viewer.Init())
}

func (m *Model) updateImageViewerFinished(msg imageViewerFinished) (tea.Model, tea.Cmd) {
	if msg.err == nil {
		m.lastExitCode = 0
		return m, nil
	}
	if msg.startFailed {
		return m, m.imageViewerFallback(msg.dir, msg.args, msg.err)
	}
	m.lastExitCode = 1
	m.shellBuffer = append(m.shellBuffer, "imgview: "+msg.err.Error()+" (check terminal and image files)")
	m.status = "imgview: " + msg.err.Error()
	if m.screen == screenShell {
		m.updateFullShellVP()
	} else {
		m.updateShellView()
	}
	return m, nil
}
