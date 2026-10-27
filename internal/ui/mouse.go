package ui

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

type mouseAction int

const (
	mousePress mouseAction = iota
	mouseRelease
	mouseMotion
)

// mouseMsg is a mouse event in one shape, whatever its kind, so handlers and
// pane coordinate translation deal with a single type.
type mouseMsg struct {
	tea.Mouse
	Action mouseAction
}

func fromTea(msg tea.MouseMsg) mouseMsg {
	m := mouseMsg{Mouse: msg.Mouse()}
	switch msg.(type) {
	case tea.MouseReleaseMsg:
		m.Action = mouseRelease
	case tea.MouseMotionMsg:
		m.Action = mouseMotion
	}
	return m
}

// toTea turns m back into Bubble Tea's message for its kind, for embedded
// programs that take Bubble Tea input.
func (m mouseMsg) toTea() tea.Msg {
	switch {
	case m.Action == mouseRelease:
		return tea.MouseReleaseMsg(m.Mouse)
	case m.Action == mouseMotion:
		return tea.MouseMotionMsg(m.Mouse)
	case m.Button >= tea.MouseWheelUp && m.Button <= tea.MouseWheelRight:
		return tea.MouseWheelMsg(m.Mouse)
	}
	return tea.MouseClickMsg(m.Mouse)
}

func (m mouseMsg) shift() bool { return m.Mod.Contains(tea.ModShift) }
func (m mouseMsg) alt() bool   { return m.Mod.Contains(tea.ModAlt) }
func (m mouseMsg) ctrl() bool  { return m.Mod.Contains(tea.ModCtrl) }

func isLeftClick(msg mouseMsg) bool {
	return msg.Button == tea.MouseLeft && msg.Action == mousePress
}

func isHover(msg mouseMsg) bool {
	return msg.Action == mouseMotion && msg.Button == tea.MouseNone
}

func isWheelUp(msg mouseMsg) bool {
	return msg.Button == tea.MouseWheelUp
}

func isWheelDown(msg mouseMsg) bool {
	return msg.Button == tea.MouseWheelDown
}

func (m *Model) muxContentOffset() (dx, dy int) {
	h, v := docStyle.GetFrameSize()
	dx = h / 2
	dy = v / 2
	if m.showMuxBar() && m.muxBarPosition == "top" {
		dy++
	}
	return dx, dy
}

func (m *Model) findLeafAt(x, y int) *muxNode {
	if m.mux.tree == nil {
		return nil
	}
	return leafAt(m.mux.tree, x, y)
}

func leafAt(n *muxNode, x, y int) *muxNode {
	if n == nil {
		return nil
	}
	if n.isLeaf() {
		if x >= n.x && x < n.x+n.w && y >= n.y && y < n.y+n.h {
			return n
		}
		return nil
	}
	if l := leafAt(n.first, x, y); l != nil {
		return l
	}
	return leafAt(n.second, x, y)
}

func listClickIndex(l list.Model, clickY int, contentStartY int) int {
	listY := clickY - contentStartY
	if listY < 0 {
		return -1
	}

	pg := l.Paginator
	itemsOnPage := pg.ItemsOnPage(len(l.Items()))
	startIdx := pg.Page * pg.PerPage

	if listY >= itemsOnPage {
		return -1
	}
	return startIdx + listY
}

func clickSelectList(l *list.Model, clickY, contentStartY int) bool {
	idx := listClickIndex(*l, clickY, contentStartY)
	if idx < 0 || idx >= len(l.Items()) {
		return false
	}
	l.Select(idx)
	return true
}

// muxRelMouse translates to pane coordinates, clamped to the pane so drags
// that leave it keep selecting along its edge.
func muxRelMouse(msg mouseMsg, dx, dy int, leaf *muxNode) mouseMsg {
	msg.X = max(0, min(msg.X-dx-leaf.x, leaf.w-1))
	msg.Y = max(0, min(msg.Y-dy-leaf.y, leaf.h-1))
	return msg
}

func (m *Model) handleMuxMouse(msg mouseMsg) (tea.Model, tea.Cmd) {
	dx, dy := m.muxContentOffset()
	mx, my := msg.X-dx, msg.Y-dy

	if isLeftClick(msg) {
		_, frameV := docStyle.GetFrameSize()
		if barY := m.muxBarY(); barY >= 0 && msg.Y-frameV/2 == barY {
			return m.handleMuxBarClick(mx)
		}
	}

	leaf := m.mouseLeaf(msg, mx, my)
	if leaf == nil || leaf.pane == nil {
		return m, nil
	}
	clicked := msg.Action == mousePress && !isWheelUp(msg) && !isWheelDown(msg)
	if clicked || isHover(msg) && m.muxFocusFollowsMouse {
		m.focusPane(leaf.pane)
	}
	return m.routePaneMouse(leaf.pane, muxRelMouse(msg, dx, dy, leaf))
}

// mouseLeaf picks the pane an event belongs to: presses, wheel and hover go
// to the pane under the pointer; drags and releases stay with the pane the
// press focused.
func (m *Model) mouseLeaf(msg mouseMsg, mx, my int) *muxNode {
	if msg.Action == mousePress || isHover(msg) {
		return m.findLeafAt(mx, my)
	}
	return m.focusedLeaf()
}

func (m *Model) focusPane(p *muxPane) {
	for i, q := range m.mux.panes {
		if q == p {
			m.mux.activeIdx = i
		}
	}
}

func (m *Model) routePaneMouse(p *muxPane, msg mouseMsg) (tea.Model, tea.Cmd) {
	switch {
	case p.kind == paneMisa && p.misa != nil:
		_, cmd := p.misa.Update(msg)
		return m, routeTo(p.misa, cmd)
	case p.terminal != nil:
		if !forwardMouse(p.terminal, msg, msg.X, msg.Y) && !scrollAltScreen(p.terminal, msg) {
			return m, m.handleMuxPaneSelMouse(p, msg)
		}
	case p.kind == paneShell && isWheelUp(msg):
		p.vp.ScrollUp(3)
	case p.kind == paneShell && isWheelDown(msg):
		p.vp.ScrollDown(3)
	case p.kind == panePager && isWheelUp(msg):
		p.pagerVP.ScrollUp(3)
	case p.kind == panePager && isWheelDown(msg):
		p.pagerVP.ScrollDown(3)
	}
	return m, nil
}

func (m *Model) handleMuxPaneSelMouse(p *muxPane, msg mouseMsg) tea.Cmd {
	switch {
	case msg.Button == tea.MouseLeft && msg.Action == mousePress:
		m.sel.begin(msg.Y, msg.X)
	case msg.Action == mouseMotion && m.sel.active:
		m.sel.extend(msg.Y, msg.X)
	case msg.Action == mouseRelease && m.sel.active:
		m.sel.extend(msg.Y, msg.X)
		m.sel.active = false
		if m.sel.hasRange && p.terminal != nil {
			return m.sel.copyToClipboard(p.terminal)
		}
	}
	return nil
}

// muxBarY is the bar's row below the frame's top margin, or -1 when hidden.
func (m *Model) muxBarY() int {
	if !m.showMuxBar() {
		return -1
	}
	_, totalH := m.muxDimensions()
	if m.muxBarPosition == "top" {
		return 0
	}
	return totalH
}

func (m *Model) handleMuxBarClick(x int) (tea.Model, tea.Cmd) {
	if x >= 0 && x < 30 {
		tag := x / 3
		if tag < 10 {
			m.switchTag(tag)
		}
	}
	return m, nil
}
