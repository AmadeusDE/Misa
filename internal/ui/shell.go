package ui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/term"
)

type shellOutput struct {
	output   string
	err      error
	exitCode int
}

type termData struct {
	t *term.Terminal
}

type termExited struct {
	t        *term.Terminal
	exitCode int
}

// termFrameInterval caps how often a terminal's output repaints the UI, so a
// flood of output (cmatrix -u 0, yes) can't keep the whole mux re-rendering.
const termFrameInterval = time.Second / 60

func waitTermUpdate(t *term.Terminal) tea.Cmd {
	return func() tea.Msg {
		select {
		case <-t.Dirty():
			time.Sleep(t.FrameDelay(termFrameInterval))
			select {
			case <-t.Exited():
				return termExited{t: t, exitCode: t.WaitExit()}
			default:
			}
			return termData{t: t}
		case <-t.Exited():
			return termExited{t: t, exitCode: t.WaitExit()}
		}
	}
}

// needsShell reports whether input uses syntax only sh can interpret (pipes,
// redirection, quoting, expansion, globs), which in-process builtins would
// otherwise receive as literal arguments.
func needsShell(input string) bool {
	return strings.ContainsAny(input, "|&;<>()$`\\\"'*?[~\n#{}")
}

// isCompound reports whether input is more than one simple command (lists,
// pipes, redirection, command substitution). Misa's own commands such as clear
// and cd only handle simple commands and leave these to sh.
func isCompound(input string) bool {
	return strings.ContainsAny(input, "|&;<>`\n") || strings.Contains(input, "$(")
}

// compoundAware returns name, or "" when input is compound so that callers'
// switches on Misa's own command names fall through to sh.
func compoundAware(name, input string) string {
	if isCompound(input) {
		return ""
	}
	return name
}

func stripNativePrefix(cmd string) (bool, string) {
	if strings.HasPrefix(cmd, "! ") {
		return true, strings.TrimSpace(cmd[2:])
	}
	if cmd == "!" {
		return false, ""
	}
	return false, ""
}
