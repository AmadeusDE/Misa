package ui

import (
	"errors"
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
	viewer, err := builtins.ImgviewViewer(words[1:], m.dir)
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
	enter := m.enterProcMon(viewer)
	m.cmdInput.SetValue("")
	return true, tea.Batch(enter, m.procViewer.Init())
}
