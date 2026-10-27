//go:build !nough

package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/ugh/ughtui"
)

func init() { startAgent = startUgh }

type ughAgent struct{ *ughtui.Model }

func (ughAgent) exited(msg tea.Msg) bool {
	_, ok := msg.(ughtui.ExitMsg)
	return ok
}

func (a ughAgent) status() agentStatus {
	st := a.Status()
	return agentStatus{Running: st.Running, Agents: st.Agents, Waiting: st.Asks > 0 || st.Prompting}
}

func startUgh(o agentOptions, host *agentHost) (agent, error) {
	m, err := ughtui.New(ughtui.Options{
		Model: o.Model, Thinking: o.Thinking, System: o.System, Cwd: o.Cwd,
		Mode: o.Mode, MaxTurns: o.MaxTurns, Resume: o.Resume, Host: host,
	})
	if err != nil {
		return nil, err
	}
	return ughAgent{m}, nil
}
