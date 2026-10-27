package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/term"
)

// job is a program moved off the screen: stopped by ^Z, or resumed with bg.
// Its terminal keeps running (and keeps its update subscription), so fg can
// put it back exactly as it was.
type job struct {
	id  int
	cmd string
	t   *term.Terminal
}

func (j *job) state() string {
	if j.t.Stopped() {
		return "Stopped"
	}
	return "Running"
}

func (m *Model) jobFor(t *term.Terminal) *job {
	for _, j := range m.jobs {
		if j.t == t {
			return j
		}
	}
	return nil
}

func (m *Model) nextJobID() int {
	id := 1
	for _, j := range m.jobs {
		id = max(id, j.id+1)
	}
	return id
}

func (m *Model) jobLine(j *job, state string) string {
	mark := " "
	if len(m.jobs) > 0 && m.jobs[len(m.jobs)-1] == j {
		mark = "+"
	}
	return fmt.Sprintf("[%d]%s  %-8s %s", j.id, mark, state, j.cmd)
}

// suspendForeground turns the stopped foreground program into a job and
// gives the prompt back, as a shell does on ^Z.
func (m *Model) suspendForeground() {
	j := &job{id: m.nextJobID(), cmd: m.shell.lastCmd, t: m.terminal}
	m.jobs = append(m.jobs, j)
	m.terminal = nil
	m.sel.clear()
	m.shellBuffer = append(m.shellBuffer, "", m.jobLine(j, "Stopped"))
	if m.menuLaunch {
		m.menuLaunch = false
		m.exitFullShell()
		m.status = fmt.Sprintf("[%d] stopped — fg in the shell resumes it", j.id)
	}
	m.resize()
	m.updateFullShellVP()
}

// finishJob reports a job whose program ended while off screen.
func (m *Model) finishJob(j *job, exitCode int) {
	state := "Done"
	if exitCode != 0 {
		state = fmt.Sprintf("Exit %d", exitCode)
	}
	m.shellBuffer = append(m.shellBuffer, m.jobLine(j, state))
	m.removeJob(j)
	j.t.Close()
	m.updateFullShellVP()
}

func (m *Model) removeJob(j *job) {
	for i, k := range m.jobs {
		if k == j {
			m.jobs = append(m.jobs[:i], m.jobs[i+1:]...)
			return
		}
	}
}

// pickJob resolves a job argument (%n, n, or none for the latest).
func (m *Model) pickJob(args []string) (*job, error) {
	if len(m.jobs) == 0 {
		return nil, fmt.Errorf("no current job")
	}
	if len(args) == 0 {
		return m.jobs[len(m.jobs)-1], nil
	}
	id, err := strconv.Atoi(strings.TrimPrefix(args[0], "%"))
	if err == nil {
		for _, j := range m.jobs {
			if j.id == id {
				return j, nil
			}
		}
	}
	return nil, fmt.Errorf("%s: no such job", args[0])
}

// jobCommand runs the jobs, fg and bg builtins. They act on this Misa's own
// state, so they never run as separate processes.
func (m *Model) jobCommand(name string, args []string) tea.Cmd {
	if name == "jobs" {
		for _, j := range m.jobs {
			m.shellBuffer = append(m.shellBuffer, m.jobLine(j, j.state()))
		}
		return nil
	}
	j, err := m.pickJob(args)
	if err != nil {
		m.shellBuffer = append(m.shellBuffer, name+": "+err.Error())
		return nil
	}
	if name == "bg" {
		if j.t.Stopped() {
			j.t.Continue()
		}
		m.shellBuffer = append(m.shellBuffer, fmt.Sprintf("[%d]  %s &", j.id, j.cmd))
		return nil
	}
	m.removeJob(j)
	m.shellBuffer = append(m.shellBuffer, j.cmd)
	m.shell.lastCmd = j.cmd
	if m.screen != screenShell {
		m.enterFullShell()
	}
	m.terminal = j.t
	m.resize()
	if m.terminal.Stopped() {
		m.terminal.Continue()
	}
	return nil
}

func isJobCommand(name string) bool {
	return name == "jobs" || name == "fg" || name == "bg"
}

// closeTerminals kills this Misa's programs, jobs included: a stopped job
// would otherwise never act on the hangup that ends the others.
func (m *Model) closeTerminals() {
	m.stopAgent()
	if m.terminal != nil {
		m.terminal.Close()
		m.terminal = nil
	}
	for _, j := range m.jobs {
		j.t.Close()
	}
	m.jobs = nil
}

// Shutdown closes every program still running in any pane, for when Misa
// itself exits.
func (m *Model) Shutdown() {
	for _, p := range m.allMuxPanes() {
		if p.terminal != nil {
			p.terminal.Close()
		}
		if p.misa != nil {
			p.misa.closeTerminals()
		}
	}
	m.closeTerminals()
	m.closeDetached()
}
