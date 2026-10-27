package ui

import (
	"fmt"
	"slices"
	"strings"
)

// agentStatus is what the mux bar shows about a session.
type agentStatus struct {
	Running bool
	Agents  int  // subagents still working
	Waiting bool // a question or permission prompt waits for the user
}

func (m *Model) tagPanes(idx int) []*muxPane {
	if idx == m.mux.activeTag {
		return m.mux.panes
	}
	if t := m.mux.tags[idx]; t != nil {
		return t.panes
	}
	return nil
}

func paneSession(p *muxPane) *agentSession {
	if p.misa == nil {
		return nil
	}
	return p.misa.session
}

// tagWaiting reports whether a session on tag idx waits for the user.
func (m *Model) tagWaiting(idx int) bool {
	for _, p := range m.tagPanes(idx) {
		if s := paneSession(p); s != nil && s.a.status().Waiting {
			return true
		}
	}
	return false
}

// agentSummary describes every session in the mux, or is empty with none.
func (m *Model) agentSummary() string {
	all := slices.Clone(m.detached)
	for _, p := range m.allMuxPanes() {
		if s := paneSession(p); s != nil {
			all = append(all, s)
		}
	}
	sessions, busy, sub, waiting := 0, 0, 0, 0
	for _, s := range all {
		st := s.a.status()
		sessions++
		sub += st.Agents
		if st.Running {
			busy++
		}
		if st.Waiting {
			waiting++
		}
	}
	if sessions == 0 {
		return ""
	}
	var parts []string
	for _, c := range []struct {
		n     int
		label string
	}{{busy, "busy"}, {sub, "sub"}, {waiting, "waiting"}, {len(m.detached), "detached"}} {
		if c.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c.n, c.label))
		}
	}
	if len(parts) == 0 {
		return "ugh idle"
	}
	return "ugh " + strings.Join(parts, " · ")
}
