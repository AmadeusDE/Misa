package ui

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/AmadeusDE/Misa/internal/companion"
	"github.com/AmadeusDE/Misa/internal/sysinfo"
	"github.com/AmadeusDE/Misa/internal/term"
	"github.com/AmadeusDE/Misa/internal/vi"
)

type paneKind int

const (
	paneShell paneKind = iota
	paneEditor
	panePager
	paneMisa
)

type muxSplit int

const (
	splitHorizontal muxSplit = iota
	splitVertical
)

type muxLayout int

const (
	layoutDwindle muxLayout = iota
	layoutMaster
	layoutFibonacci
	layoutCount
)

func (l muxLayout) String() string {
	switch l {
	case layoutDwindle:
		return "dwindle"
	case layoutMaster:
		return "master"
	case layoutFibonacci:
		return "fibonacci"
	}
	return "unknown"
}

type muxPane struct {
	kind paneKind
	w, h int

	shellBuf []string
	input    textinput.Model
	vp       viewport.Model
	shell    shellState
	exitCode int

	terminal *term.Terminal

	editor *vi.Editor

	pagerVP  viewport.Model
	pagerBuf string

	misa *Model
}

type muxNode struct {
	pane *muxPane

	dir    muxSplit
	ratio  float64
	first  *muxNode
	second *muxNode

	x, y, w, h int
}

func (n *muxNode) isLeaf() bool { return n.first == nil }

func allLeaves(n *muxNode) []*muxNode {
	if n == nil {
		return nil
	}
	if n.isLeaf() {
		return []*muxNode{n}
	}
	return append(allLeaves(n.first), allLeaves(n.second)...)
}

type muxTag struct {
	panes     []*muxPane
	activeIdx int
	layout    muxLayout
	tree      *muxNode
}

type muxState struct {
	panes        []*muxPane
	activeIdx    int // -1 = no panes, 0+ = index into panes
	layout       muxLayout
	ctrlWPending bool
	resizeMode   bool
	tree         *muxNode
	primaryW     int
	primaryH     int

	tags      [10]*muxTag
	activeTag int
}

func (m *Model) hasSplit() bool {
	return len(m.mux.panes) > 0
}

func (m *Model) isAppActive() bool {
	return m.mux.activeIdx < 0
}

func (m *Model) activePane() *muxPane {
	if m.mux.activeIdx < 0 || m.mux.activeIdx >= len(m.mux.panes) {
		return nil
	}
	return m.mux.panes[m.mux.activeIdx]
}

func (m *Model) focusedLeaf() *muxNode {
	if m.mux.tree == nil || m.mux.activeIdx < 0 {
		return nil
	}
	leaves := allLeaves(m.mux.tree)
	if m.mux.activeIdx >= len(leaves) {
		return nil
	}
	return leaves[m.mux.activeIdx]
}

func (m *Model) initMux() {
	m.mux.activeIdx = -1
	m.mux.layout = layoutDwindle
	m.mux.activeTag = 0
}

func (m *Model) newShellPane() *muxPane {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "enter a command..."
	ti.CharLimit = 256
	ti.Focus()

	vp := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))

	p := &muxPane{
		kind:  paneShell,
		input: ti,
		vp:    vp,
	}
	p.shell.loadFromDisk()
	p.shell.searchHistory = m.historySearch
	return p
}

func (m *Model) newMisaPane() (*muxPane, tea.Cmd) {
	p := m.newShellPane()
	nested := New(m.dir)
	nested.nestedMode = true
	p.kind = paneMisa
	p.misa = nested
	return p, routeTo(nested, nested.startNested())
}

// --- Layout algorithms ---

func buildDwindle(leaves []*muxNode, x, y, w, h int) *muxNode {
	if len(leaves) == 1 {
		setLeafRect(leaves[0], x, y, w, h)
		return leaves[0]
	}

	dir := splitVertical
	if w <= h*2 {
		dir = splitHorizontal
	}

	ratio := 0.5
	node := &muxNode{dir: dir, ratio: ratio, x: x, y: y, w: w, h: h}
	if dir == splitVertical {
		firstW := int(float64(w) * ratio)
		node.first = buildDwindle(leaves[:1], x, y, firstW-1, h)
		node.second = buildDwindle(leaves[1:], x+firstW, y, w-firstW, h)
	} else {
		firstH := int(float64(h) * ratio)
		node.first = buildDwindle(leaves[:1], x, y, w, firstH-1)
		node.second = buildDwindle(leaves[1:], x, y+firstH, w, h-firstH)
	}
	return node
}

func buildMaster(leaves []*muxNode, x, y, w, h int) *muxNode {
	if len(leaves) <= 1 {
		setLeafRect(leaves[0], x, y, w, h)
		return leaves[0]
	}

	ratio := 0.55
	masterW := int(float64(w) * ratio)
	node := &muxNode{dir: splitVertical, ratio: ratio, x: x, y: y, w: w, h: h}
	setLeafRect(leaves[0], x, y, masterW-1, h)
	node.first = leaves[0]
	node.second = buildEvenStack(leaves[1:], x+masterW, y, w-masterW, h)
	return node
}

func buildEvenStack(leaves []*muxNode, x, y, w, h int) *muxNode {
	if len(leaves) == 1 {
		setLeafRect(leaves[0], x, y, w, h)
		return leaves[0]
	}

	n := len(leaves)
	ratio := 1.0 / float64(n)
	firstH := int(float64(h) * ratio)
	node := &muxNode{dir: splitHorizontal, ratio: ratio, x: x, y: y, w: w, h: h}
	setLeafRect(leaves[0], x, y, w, firstH-1)
	node.first = leaves[0]
	node.second = buildEvenStack(leaves[1:], x, y+firstH, w, h-firstH)
	return node
}

func buildFibonacci(leaves []*muxNode, x, y, w, h int, vertical bool) *muxNode {
	if len(leaves) == 1 {
		setLeafRect(leaves[0], x, y, w, h)
		return leaves[0]
	}

	dir := splitVertical
	if !vertical {
		dir = splitHorizontal
	}

	ratio := 0.5
	node := &muxNode{dir: dir, ratio: ratio, x: x, y: y, w: w, h: h}
	if dir == splitVertical {
		firstW := int(float64(w) * ratio)
		setLeafRect(leaves[0], x, y, firstW-1, h)
		node.first = leaves[0]
		node.second = buildFibonacci(leaves[1:], x+firstW, y, w-firstW, h, !vertical)
	} else {
		firstH := int(float64(h) * ratio)
		setLeafRect(leaves[0], x, y, w, firstH-1)
		node.first = leaves[0]
		node.second = buildFibonacci(leaves[1:], x, y+firstH, w, h-firstH, !vertical)
	}
	return node
}

func setLeafRect(n *muxNode, x, y, w, h int) {
	w, h = max(w, 1), max(h, 1)
	n.x, n.y, n.w, n.h = x, y, w, h
	if n.pane != nil {
		n.pane.w, n.pane.h = w, h
	}
}

func applyLayout(n *muxNode, x, y, w, h int) {
	if n.isLeaf() {
		setLeafRect(n, x, y, w, h)
		return
	}
	n.x, n.y, n.w, n.h = x, y, w, h
	ratio := n.ratio
	if ratio <= 0 {
		ratio = 0.5
	}
	if n.dir == splitVertical {
		firstW := int(float64(w) * ratio)
		if firstW < 2 {
			firstW = 2
		}
		applyLayout(n.first, x, y, firstW-1, h)
		applyLayout(n.second, x+firstW, y, w-firstW, h)
	} else {
		firstH := int(float64(h) * ratio)
		if firstH < 2 {
			firstH = 2
		}
		applyLayout(n.first, x, y, w, firstH-1)
		applyLayout(n.second, x, y+firstH, w, h-firstH)
	}
}

// --- Tree management ---

func (m *Model) muxDimensions() (int, int) {
	totalW, totalH := m.width, m.height
	if !m.nestedMode {
		h, v := docStyle.GetFrameSize()
		totalW -= h + 1
		totalH -= v
		if m.showMuxBar() {
			totalH--
		}
	}
	if totalW < 1 {
		totalW = 1
	}
	if totalH < 1 {
		totalH = 1
	}
	return totalW, totalH
}

func (m *Model) showMuxBar() bool {
	if m.muxBarAutoHide {
		return len(m.mux.panes) > 1
	}
	return true
}

func (m *Model) rebuildMuxTree() {
	if m.nestedMode {
		return
	}
	totalW, totalH := m.muxDimensions()

	n := len(m.mux.panes)
	if n == 0 {
		m.mux.tree = nil
		m.mux.primaryW = totalW
		m.mux.primaryH = totalH
		return
	}

	leaves := make([]*muxNode, n)
	for i, p := range m.mux.panes {
		leaves[i] = &muxNode{pane: p}
	}

	if n == 1 {
		setLeafRect(leaves[0], 0, 0, totalW, totalH)
		m.mux.tree = leaves[0]
	} else {
		switch m.mux.layout {
		case layoutDwindle:
			m.mux.tree = buildDwindle(leaves, 0, 0, totalW, totalH)
		case layoutMaster:
			m.mux.tree = buildMaster(leaves, 0, 0, totalW, totalH)
		case layoutFibonacci:
			m.mux.tree = buildFibonacci(leaves, 0, 0, totalW, totalH, true)
		}
	}

	m.mux.primaryW = totalW
	m.mux.primaryH = totalH
}

func (m *Model) applyMuxDimensions() {
	totalW, totalH := m.muxDimensions()
	if m.mux.tree != nil {
		applyLayout(m.mux.tree, 0, 0, totalW, totalH)
	}
	m.mux.primaryW = totalW
	m.mux.primaryH = totalH
}

func (m *Model) resizeMux() {
	if m.nestedMode {
		m.mux.primaryW = m.width
		m.mux.primaryH = m.height
		return
	}
	nLeaves := len(m.mux.panes)
	if m.mux.tree != nil && nLeaves > 0 && len(allLeaves(m.mux.tree)) == nLeaves {
		m.applyMuxDimensions()
	} else {
		m.rebuildMuxTree()
	}
	for _, p := range m.mux.panes {
		m.resizeSecondaryPane(p)
	}
}

func (m *Model) resizeSecondaryPane(p *muxPane) {
	if p.terminal != nil {
		p.terminal.Resize(p.h, p.w)
	}
	switch p.kind {
	case paneShell:
		p.vp.SetWidth(p.w)
		p.vp.SetHeight(p.h - 2)
		if p.vp.Height() < 1 {
			p.vp.SetHeight(1)
		}
	case paneEditor:
		if p.editor != nil {
			p.editor.Resize(p.w, p.h)
		}
	case panePager:
		p.pagerVP.SetWidth(p.w)
		p.pagerVP.SetHeight(p.h - 1)
		if p.pagerVP.Height() < 1 {
			p.pagerVP.SetHeight(1)
		}
	case paneMisa:
		if p.misa != nil {
			p.misa.width = p.w
			p.misa.height = p.h
			p.misa.resize()
		}
	}
}

// --- Pane management ---

func (m *Model) addMuxPane() tea.Cmd {
	p, cmd := m.newMisaPane()
	m.mux.panes = append(m.mux.panes, p)
	m.mux.activeIdx = len(m.mux.panes) - 1
	m.rebuildMuxTree()
	m.resize()
	return cmd
}

func (m *Model) anyTagHasPanes() bool {
	for i := 0; i < 10; i++ {
		if m.tagHasPanes(i) {
			return true
		}
	}
	return false
}

func (m *Model) closeMuxPane() tea.Cmd {
	if len(m.mux.panes) == 0 {
		if !m.anyTagHasPanes() {
			return tea.Quit
		}
		return nil
	}
	idx := m.mux.activeIdx
	if idx < 0 || idx >= len(m.mux.panes) {
		return nil
	}
	closing := m.mux.panes[idx]
	if closing.terminal != nil {
		closing.terminal.Close()
		closing.terminal = nil
	}
	if closing.kind == paneMisa && closing.misa != nil {
		m.detachSession(closing.misa)
		closing.misa.closeTerminals()
	}
	m.mux.panes = append(m.mux.panes[:idx], m.mux.panes[idx+1:]...)
	if len(m.mux.panes) == 0 {
		m.mux.activeIdx = -1
	} else if idx >= len(m.mux.panes) {
		m.mux.activeIdx = len(m.mux.panes) - 1
	}
	m.mux.resizeMode = false
	m.rebuildMuxTree()
	m.resize()

	if len(m.mux.panes) == 0 && !m.anyTagHasPanes() {
		return tea.Quit
	}
	return nil
}

// --- Tag/workspace management ---

func (m *Model) switchTag(tag int) {
	if tag == m.mux.activeTag {
		return
	}
	m.mux.tags[m.mux.activeTag] = &muxTag{
		panes:     m.mux.panes,
		activeIdx: m.mux.activeIdx,
		layout:    m.mux.layout,
		tree:      m.mux.tree,
	}
	m.mux.activeTag = tag
	t := m.mux.tags[tag]
	if t != nil {
		m.mux.panes = t.panes
		m.mux.activeIdx = t.activeIdx
		m.mux.layout = t.layout
		m.mux.tree = t.tree
	} else {
		m.mux.panes = nil
		m.mux.activeIdx = -1
		m.mux.layout = layoutDwindle
		m.mux.tree = nil
	}
	m.mux.resizeMode = false
	for _, p := range m.mux.panes {
		if p.kind == paneMisa && p.misa != nil && p.misa.terminal != nil {
			if p.misa.screen == screenShell && !p.misa.terminal.InAltScreen() {
				p.misa.updateFullShellVPWithTerm()
			}
		}
	}
	m.resize()
}

func (m *Model) tagHasPanes(idx int) bool {
	if idx == m.mux.activeTag {
		return len(m.mux.panes) > 0
	}
	t := m.mux.tags[idx]
	return t != nil && len(t.panes) > 0
}

func (m *Model) allMuxPanes() []*muxPane {
	panes := make([]*muxPane, 0, len(m.mux.panes)+10)
	panes = append(panes, m.mux.panes...)
	for i := 0; i < 10; i++ {
		if i == m.mux.activeTag {
			continue
		}
		if tag := m.mux.tags[i]; tag != nil {
			panes = append(panes, tag.panes...)
		}
	}
	return panes
}

// --- Resize mode ---

func (m *Model) handleResizeMode(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "enter", "q", "ctrl+[":
		m.mux.resizeMode = false
	case "ctrl+w":
		m.mux.resizeMode = false
		m.mux.ctrlWPending = true
	case "h":
		m.resizeActive(splitVertical, true, -0.05)
	case "l":
		m.resizeActive(splitVertical, false, 0.05)
	case "k":
		m.resizeActive(splitHorizontal, true, -0.05)
	case "j":
		m.resizeActive(splitHorizontal, false, 0.05)
	}
	return m, nil
}

func (m *Model) resizeActive(dir muxSplit, activeInSecond bool, delta float64) {
	focused := m.focusedLeaf()
	if focused == nil {
		return
	}
	node := findResizeAncestor(m.mux.tree, focused, dir, activeInSecond)
	if node == nil {
		return
	}
	node.ratio += delta
	if node.ratio < 0.1 {
		node.ratio = 0.1
	}
	if node.ratio > 0.9 {
		node.ratio = 0.9
	}
	m.applyMuxDimensions()
	for _, p := range m.mux.panes {
		m.resizeSecondaryPane(p)
	}
	m.resize()
}

func pathTo(root, target *muxNode) []*muxNode {
	if root == nil {
		return nil
	}
	if root == target {
		return []*muxNode{root}
	}
	if root.isLeaf() {
		return nil
	}
	if p := pathTo(root.first, target); p != nil {
		return append([]*muxNode{root}, p...)
	}
	if p := pathTo(root.second, target); p != nil {
		return append([]*muxNode{root}, p...)
	}
	return nil
}

func findResizeAncestor(root, target *muxNode, dir muxSplit, activeInSecond bool) *muxNode {
	path := pathTo(root, target)
	for i := len(path) - 2; i >= 0; i-- {
		node := path[i]
		if node.isLeaf() || node.dir != dir {
			continue
		}
		child := path[i+1]
		if activeInSecond && child == node.second {
			return node
		}
		if !activeInSecond && child == node.first {
			return node
		}
	}
	return nil
}

// --- Global keybinds (no ctrl+w prefix needed) ---

func (m *Model) handleGlobalMuxKey(key string) (bool, tea.Model, tea.Cmd) {
	switch key {
	case "alt+1":
		m.switchTag(0)
		return true, m, nil
	case "alt+2":
		m.switchTag(1)
		return true, m, nil
	case "alt+3":
		m.switchTag(2)
		return true, m, nil
	case "alt+4":
		m.switchTag(3)
		return true, m, nil
	case "alt+5":
		m.switchTag(4)
		return true, m, nil
	case "alt+6":
		m.switchTag(5)
		return true, m, nil
	case "alt+7":
		m.switchTag(6)
		return true, m, nil
	case "alt+8":
		m.switchTag(7)
		return true, m, nil
	case "alt+9":
		m.switchTag(8)
		return true, m, nil
	case "alt+0":
		m.switchTag(9)
		return true, m, nil

	case "alt+h", "alt+left":
		return m.globalPaneNav("h")
	case "alt+j", "alt+down":
		return m.globalPaneNav("j")
	case "alt+k", "alt+up":
		return m.globalPaneNav("k")
	case "alt+l", "alt+right":
		return m.globalPaneNav("l")
	}
	return false, m, nil
}

func (m *Model) globalPaneNav(dir string) (bool, tea.Model, tea.Cmd) {
	if len(m.mux.panes) <= 1 {
		return false, m, nil
	}
	focused := m.focusedLeaf()
	if focused == nil {
		return false, m, nil
	}
	target := m.findPaneInDirection(focused, dir)
	if target != nil {
		m.setActiveFromLeaf(target)
		return true, m, nil
	}
	return false, m, nil
}

// --- Input handling ---

func (m *Model) handleMuxCommand(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "ctrl+[":
		// cancel

	case "w", "ctrl+w":
		total := len(m.mux.panes)
		if total <= 1 {
			break
		}
		m.mux.activeIdx = (m.mux.activeIdx + 1) % total

	case "h", "left", "l", "right", "j", "down", "k", "up":
		focused := m.focusedLeaf()
		if focused == nil {
			break
		}
		target := m.findPaneInDirection(focused, key)
		if target != nil {
			m.setActiveFromLeaf(target)
		}

	case "s", "v":
		return m, m.addMuxPane()

	case "a":
		return m, m.addAgentPane(m.focusedDir())

	case "A":
		return m, m.reattachAgent()

	case "q":
		if cmd := m.closeMuxPane(); cmd != nil {
			return m, cmd
		}

	case "space":
		m.mux.layout = (m.mux.layout + 1) % layoutCount
		m.rebuildMuxTree()
		m.resize()

	case "r":
		if len(m.mux.panes) > 1 {
			m.mux.resizeMode = true
		}

	case "1":
		m.switchTag(0)
	case "2":
		m.switchTag(1)
	case "3":
		m.switchTag(2)
	case "4":
		m.switchTag(3)
	case "5":
		m.switchTag(4)
	case "6":
		m.switchTag(5)
	case "7":
		m.switchTag(6)
	case "8":
		m.switchTag(7)
	case "9":
		m.switchTag(8)
	case "0":
		m.switchTag(9)
	}

	return m, nil
}

func (m *Model) setActiveFromLeaf(n *muxNode) {
	for i, p := range m.mux.panes {
		if p == n.pane {
			m.mux.activeIdx = i
			return
		}
	}
}

func (m *Model) findPaneInDirection(cur *muxNode, key string) *muxNode {
	leaves := allLeaves(m.mux.tree)

	var best *muxNode
	bestDist := -1

	for _, l := range leaves {
		if l == cur {
			continue
		}
		var match bool
		switch key {
		case "h", "left":
			match = l.x+l.w <= cur.x
		case "l", "right":
			match = l.x >= cur.x+cur.w+1
		case "k", "up":
			match = l.y+l.h <= cur.y
		case "j", "down":
			match = l.y >= cur.y+cur.h+1
		}
		if !match {
			continue
		}
		dx := (l.x + l.w/2) - (cur.x + cur.w/2)
		dy := (l.y + l.h/2) - (cur.y + cur.h/2)
		if dx < 0 {
			dx = -dx
		}
		if dy < 0 {
			dy = -dy
		}
		dist := dx + dy
		if best == nil || dist < bestDist {
			best = l
			bestDist = dist
		}
	}
	return best
}

func (m *Model) focusMuxPanes() {}

func (m *Model) updateSecondaryPane(msg tea.Msg) (tea.Model, tea.Cmd) {
	p := m.activePane()
	if p == nil {
		return m, nil
	}

	if p.kind == paneMisa {
		return m.updateMuxMisa(p, msg)
	}

	switch msg := msg.(type) {
	case shellOutput:
		if p.kind == paneShell {
			p.exitCode = msg.exitCode
			output := msg.output
			if msg.err != nil && output == "" {
				output = msg.err.Error()
			}
			if output != "" {
				lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
				p.shellBuf = append(p.shellBuf, lines...)
			}
			if resp := companion.Respond(msg.exitCode); resp != "" {
				ps := lipgloss.NewStyle().Foreground(m.uiColors.prompt).Bold(true)
				for _, line := range strings.Split(resp, "\n") {
					p.shellBuf = append(p.shellBuf, ps.Render(line))
				}
			}
			m.updateMuxShellVP(p)
			p.input.SetValue("")
		}
		return m, nil

	case tea.PasteMsg:
		if p.terminal != nil {
			sendPaste(p.terminal, msg.Content)
			return m, nil
		}
		if p.kind == paneShell {
			var cmd tea.Cmd
			p.input, cmd = p.input.Update(msg)
			return m, cmd
		}

	case tea.KeyPressMsg:
		if m.copyMode.active {
			return m.updateCopyMode(msg)
		}

		if p.terminal != nil {
			if msg.String() == "ctrl+y" {
				rows := p.h
				if rows < 1 {
					rows = m.height - 2
				}
				m.copyMode = newCopyState(p.terminal, nil, rows, p.w)
				return m, nil
			}
			if msg.String() == "ctrl+v" {
				pasteToTerm(p.terminal)
				return m, nil
			}
			if m.selKey(msg, p.terminal) {
				return m, nil
			}
			sendKey(p.terminal, msg)
			return m, nil
		}

		key := msg.String()
		if key == "ctrl+c" && p.kind == paneShell {
			p.input.SetValue("")
			return m, nil
		}

		switch p.kind {
		case paneShell:
			return m.updateMuxShell(p, msg)
		case paneEditor:
			return m.updateMuxEditor(p, msg)
		case panePager:
			return m.updateMuxPager(p, msg)
		}
	}

	return m, nil
}

// --- Shell pane ---

func (m *Model) muxRenderPrompt(exitCode int) string {
	codeStr := fmt.Sprintf("E[%d]", exitCode)
	codeColor := m.uiColors.promptOK
	if exitCode != 0 {
		codeColor = m.uiColors.promptErr
	}
	return lipgloss.NewStyle().Foreground(codeColor).Render(codeStr) + " " +
		lipgloss.NewStyle().Foreground(m.uiColors.promptArrow).Render("➜") + " "
}

func (m *Model) updateMuxShellVP(p *muxPane) {
	wasAtBottom := p.vp.AtBottom()
	content := strings.Join(p.shellBuf, "\n")
	p.vp.SetContent(content)
	if wasAtBottom {
		p.vp.GotoBottom()
	}
}

func (m *Model) updateMuxShell(p *muxPane, msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(msg)
		return m, cmd
	}

	switch keyMsg.String() {
	case "ctrl+y":
		vpLines := strings.Split(strings.Join(p.shellBuf, "\n"), "\n")
		rows := p.h - 2
		if rows < 1 {
			rows = p.h
		}
		m.copyMode = newCopyState(p.terminal, vpLines, rows, p.w)
		return m, nil
	case "shift+up", "pgup":
		p.vp.ScrollUp(3)
		return m, nil
	case "shift+down", "pgdown":
		p.vp.ScrollDown(3)
		return m, nil
	case "up":
		val := p.shell.historyUp(p.input.Value())
		p.input.SetValue(val)
		p.input.SetCursor(len(val))
		return m, nil
	case "down":
		val := p.shell.historyDown()
		p.input.SetValue(val)
		p.input.SetCursor(len(val))
		return m, nil
	case "right":
		input := p.input.Value()
		if p.input.Position() >= utf8.RuneCountInString(input) && p.shell.suggestion != "" {
			accepted := p.shell.acceptSuggestion(input)
			if accepted != input {
				p.input.SetValue(accepted)
				p.input.SetCursor(len(accepted))
				return m, nil
			}
		}
	case "tab":
		input := p.input.Value()
		if strings.TrimSpace(input) == "" {
			return m, nil
		}
		completed := p.shell.tabComplete(input, m.dir)
		if completed != input {
			p.input.SetValue(completed)
			p.input.SetCursor(len(completed))
		}
		return m, nil
	case "enter":
		val := strings.TrimSpace(p.input.Value())
		if val == "" {
			return m, nil
		}
		p.shell.addHistory(val)
		p.shell.lastCmd = val
		p.shell.resetHistory()
		p.shell.suggestion = ""
		p.shellBuf = append(p.shellBuf, m.muxRenderPrompt(p.exitCode)+val)

		if native, nativeCmd := stripNativePrefix(val); native {
			p.input.SetValue("")
			p.shellBuf = append(p.shellBuf, m.muxRenderPrompt(p.exitCode)+"! "+nativeCmd)
			m.updateMuxShellVP(p)
			t, cmd, err := m.startPlan(shellPlan(nativeCmd), p.h, p.w)
			if err != nil {
				p.shellBuf = append(p.shellBuf, "error: "+err.Error())
				m.updateMuxShellVP(p)
				return m, nil
			}
			p.terminal = t
			return m, cmd
		}

		fields := strings.Fields(val)
		switch compoundAware(fields[0], val) {
		case "clear":
			p.shellBuf = nil
			m.updateMuxShellVP(p)
			p.input.SetValue("")
			return m, nil
		case "exit":
			if cmd := m.closeMuxPane(); cmd != nil {
				return m, cmd
			}
			return m, nil
		case "misa":
			nested := New(m.dir)
			nested.nestedMode = true
			nested.width = p.w
			nested.height = p.h
			if nested.startInShell {
				nested.enterFullShell()
			}
			nested.resize()
			p.kind = paneMisa
			p.misa = nested
			p.input.SetValue("")
			return m, nil
		case "vi", "edit":
			filename := ""
			if len(fields) > 1 {
				filename = strings.Join(fields[1:], " ")
			}
			p.kind = paneEditor
			p.editor = vi.New(filename, m.dir)
			m.resizeSecondaryPane(p)
			p.input.SetValue("")
			return m, nil
		case "less":
			if len(fields) > 1 {
				fname := strings.Join(fields[1:], " ")
				data, err := readFileForPager(fname, m.dir)
				if err != nil {
					p.shellBuf = append(p.shellBuf, "less: "+err.Error())
					m.updateMuxShellVP(p)
					p.input.SetValue("")
					return m, nil
				}
				p.kind = panePager
				p.pagerBuf = fname
				p.pagerVP = viewport.New(viewport.WithWidth(p.w), viewport.WithHeight(p.h-1))
				p.pagerVP.SetContent(data)
				p.input.SetValue("")
				return m, nil
			}
		}

		if fields[0] == "cd" && !isCompound(val) {
			if handled, cmd := m.handleBuiltinCmd(val); handled {
				p.input.SetValue("")
				m.updateMuxShellVP(p)
				return m, cmd
			}
		}

		p.input.SetValue("")
		m.updateMuxShellVP(p)
		t, cmd, err := m.startPlan(m.planCommand(val), p.h, p.w)
		if err != nil {
			p.shellBuf = append(p.shellBuf, "error: "+err.Error())
			m.updateMuxShellVP(p)
			return m, nil
		}
		p.terminal = t
		return m, cmd
	}

	prev := p.input.Value()
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	if p.input.Value() != prev {
		p.shell.complete(p.input.Value(), m.dir)
		p.shell.resetHistory()
	}
	return m, cmd
}

// --- Editor pane ---

func (m *Model) updateMuxEditor(p *muxPane, msg tea.Msg) (tea.Model, tea.Cmd) {
	if p.editor == nil {
		return m, nil
	}
	cmd := p.editor.Update(msg)
	if p.editor.Quit {
		reg, regLine := p.editor.Register()
		m.viRegister = reg
		m.viRegLine = regLine
		p.kind = paneShell
		p.editor = nil
		p.input.Focus()
		m.resizeSecondaryPane(p)
	}
	return m, cmd
}

// --- Pager pane ---

func (m *Model) updateMuxPager(p *muxPane, msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "q", "esc":
		p.kind = paneShell
		p.pagerBuf = ""
		p.input.Focus()
		m.resizeSecondaryPane(p)
		return m, nil
	case "j", "down":
		p.pagerVP.ScrollDown(1)
	case "k", "up":
		p.pagerVP.ScrollUp(1)
	case "space", "ctrl+f", "pgdown":
		p.pagerVP.ScrollDown(p.pagerVP.Height())
	case "b", "ctrl+b", "pgup":
		p.pagerVP.ScrollUp(p.pagerVP.Height())
	case "g", "home":
		p.pagerVP.GotoTop()
	case "G", "end":
		p.pagerVP.GotoBottom()
	case "ctrl+d":
		p.pagerVP.ScrollDown(p.pagerVP.Height() / 2)
	case "ctrl+u":
		p.pagerVP.ScrollUp(p.pagerVP.Height() / 2)
	}
	return m, nil
}

// --- Misa pane ---

func (m *Model) updateMuxMisa(p *muxPane, msg tea.Msg) (tea.Model, tea.Cmd) {
	if p.misa == nil {
		return m, nil
	}
	if _, ok := msg.(tea.WindowSizeMsg); ok {
		return m, nil
	}
	_, cmd := p.misa.Update(msg)
	if p.misa.wantsClose {
		return m, m.closeMuxPane()
	}
	return m, routeTo(p.misa, cmd)
}

// --- View ---

func (m *Model) muxView() string {
	totalW, totalH := m.muxDimensions()
	showBar := m.showMuxBar()

	var bar string
	if showBar {
		bar = ansi.Truncate(m.muxBarView(), totalW, "")
	}

	var panes string
	if len(m.mux.panes) == 0 {
		hint := lipgloss.NewStyle().Foreground(m.uiColors.ghost).Render(
			"ctrl+w v: new pane  alt+1-9/0: switch tag")
		panes = lipgloss.Place(totalW, totalH, lipgloss.Center, lipgloss.Center, hint)
	} else {
		panes = m.renderNode(m.mux.tree)
	}

	panes = forceHeight(panes, totalH)

	var full string
	if showBar {
		if m.muxBarPosition == "top" {
			full = bar + "\n" + panes
		} else {
			full = panes + "\n" + bar
		}
		full = cropToBox(full, totalW, totalH+1)
	} else {
		full = cropToBox(panes, totalW, totalH)
	}
	return forceFrame(full, m.width, m.height)
}

func (m *Model) renderNode(n *muxNode) string {
	if n == nil {
		return ""
	}
	if n.isLeaf() {
		return m.renderLeaf(n)
	}

	first := m.renderNode(n.first)
	second := m.renderNode(n.second)

	var result string
	if n.dir == splitVertical {
		divH := n.h
		if divH < 1 {
			divH = 1
		}
		divLines := make([]string, divH)
		for i := range divLines {
			divLines[i] = "│"
		}
		divider := lipgloss.NewStyle().Foreground(m.uiColors.divider).Render(strings.Join(divLines, "\n"))
		result = lipgloss.JoinHorizontal(lipgloss.Top, first, divider, second)
	} else {
		divW := n.w
		if divW < 1 {
			divW = 1
		}
		divider := lipgloss.NewStyle().Foreground(m.uiColors.divider).Render(strings.Repeat("─", divW))
		result = first + "\n" + divider + "\n" + second
	}
	return cropToBox(result, n.w, n.h)
}

func cropToBox(s string, maxW, maxH int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > maxH {
		lines = lines[:maxH]
	}
	for i, line := range lines {
		if lipgloss.Width(line) > maxW {
			lines[i] = ansi.Truncate(line, maxW, "")
		}
	}
	return strings.Join(lines, "\n")
}

func forceHeight(s string, h int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func forceFrame(content string, w, h int) string {
	marginH, marginV := docStyle.GetFrameSize()
	left := marginH / 2
	right := marginH - left
	top := marginV / 2
	bottom := marginV - top

	lines := strings.Split(content, "\n")
	contentH := h - top - bottom
	if contentH < 1 {
		contentH = 1
	}
	if len(lines) > contentH {
		lines = lines[:contentH]
	}
	for len(lines) < contentH {
		lines = append(lines, "")
	}

	lineW := w - 1
	if lineW < 1 {
		lineW = 1
	}
	pad := strings.Repeat(" ", lineW)
	out := make([]string, 0, h)
	for i := 0; i < top; i++ {
		out = append(out, pad)
	}
	lpad := strings.Repeat(" ", left)
	rpad := strings.Repeat(" ", right)
	contentW := lineW - left - right
	if contentW < 0 {
		contentW = 0
	}
	for _, line := range lines {
		lw := lipgloss.Width(line)
		if lw > contentW {
			line = ansi.Truncate(line, contentW, "")
			lw = lipgloss.Width(line)
		}
		gap := contentW - lw
		if gap < 0 {
			gap = 0
		}
		out = append(out, lpad+line+strings.Repeat(" ", gap)+rpad)
	}
	for i := 0; i < bottom; i++ {
		out = append(out, pad)
	}
	if len(out) > h {
		out = out[:h]
	}
	for len(out) < h {
		out = append(out, pad)
	}
	return strings.Join(out, "\n")
}

func (m *Model) renderLeaf(n *muxNode) string {
	if n.pane == nil {
		return forceBox("", n.w, n.h)
	}
	active := n.pane == m.activePane()
	content := m.renderPaneContent(n.pane, active)
	return forceBox(content, n.w, n.h)
}

func forceBox(s string, w, h int) string {
	w, h = max(w, 0), max(h, 0)
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, line := range lines {
		lw := lipgloss.Width(line)
		if lw > w {
			lines[i] = ansi.Truncate(line, w, "")
		} else if lw < w {
			lines[i] = line + strings.Repeat(" ", w-lw)
		}
	}
	for len(lines) < h {
		lines = append(lines, strings.Repeat(" ", w))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderPaneContent(p *muxPane, active bool) string {
	switch p.kind {
	case paneShell:
		return m.renderMuxShell(p, active)
	case paneEditor:
		if p.editor != nil {
			return p.editor.View()
		}
		return ""
	case panePager:
		return m.renderMuxPagerView(p)
	case paneMisa:
		if p.misa != nil {
			return p.misa.render()
		}
		return ""
	}
	return ""
}

func (m *Model) renderMuxShell(p *muxPane, active bool) string {
	if m.copyMode.active && active {
		return m.copyMode.render(m.uiColors.ghost)
	}

	if p.terminal != nil {
		showCur := active && m.cursorBlink && p.terminal.CursorVisible()
		var sel *term.Highlight
		if active && m.sel.hasRange {
			sel = &term.Highlight{
				StartRow: m.sel.startRow, StartCol: m.sel.startCol,
				EndRow: m.sel.endRow, EndCol: m.sel.endCol,
			}
		}
		return p.terminal.RenderFull(showCur, sel)
	}

	prompt := m.muxRenderPrompt(p.exitCode)
	inputView := renderInput(p.input)
	ghost := ""
	if active && p.shell.suggestion != "" {
		input := strings.TrimSpace(p.input.Value())
		if input != "" && strings.HasPrefix(p.shell.suggestion, input) {
			ghost = lipgloss.NewStyle().Foreground(m.uiColors.ghost).Render(p.shell.suggestion[len(input):])
		}
	}

	hint := lipgloss.NewStyle().Foreground(m.uiColors.ghost).Render("ctrl+w: pane cmds  alt+hjkl: navigate  alt+N: tag")

	return p.vp.View() +
		"\n" + fmt.Sprintf("%s%s%s", prompt, inputView, ghost) +
		"\n" + hint
}

func (m *Model) renderMuxPagerView(p *muxPane) string {
	hint := lipgloss.NewStyle().Foreground(m.uiColors.ghost).Render(
		fmt.Sprintf(" %s  q: quit  j/k: scroll", p.pagerBuf))
	return p.pagerVP.View() + "\n" + hint
}

// --- Status bar ---

func (m *Model) muxBarView() string {
	w, _ := m.muxDimensions()
	if w < 1 {
		w = 1
	}

	barBg := lipgloss.NewStyle().Background(lipgloss.Color("#3c3836"))
	dimStyle := barBg.Foreground(m.uiColors.ghost)
	activeStyle := barBg.Foreground(m.uiColors.prompt).Bold(true)
	occupiedStyle := barBg.Foreground(m.uiColors.promptArrow)
	sepStyle := barBg.Foreground(m.uiColors.divider)
	layoutStyle := barBg.Foreground(m.uiColors.promptArrow)
	resizeStyle := barBg.Foreground(lipgloss.Color("#fb4934")).Bold(true)

	tagLabels := [10]string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"}
	var left strings.Builder
	for i := 0; i < 10; i++ {
		tag := " " + tagLabels[i] + " "
		if i == m.mux.activeTag {
			left.WriteString(activeStyle.Render(tag))
		} else if m.tagWaiting(i) {
			left.WriteString(resizeStyle.Render(tag))
		} else if m.tagHasPanes(i) {
			left.WriteString(occupiedStyle.Render(tag))
		} else {
			left.WriteString(dimStyle.Render(tag))
		}
	}
	left.WriteString(sepStyle.Render(" │ "))
	left.WriteString(layoutStyle.Render(m.mux.layout.String()))

	if m.mux.resizeMode {
		left.WriteString(sepStyle.Render(" │ "))
		left.WriteString(resizeStyle.Render("RESIZE"))
	}
	if summary := m.agentSummary(); summary != "" {
		left.WriteString(sepStyle.Render(" │ "))
		left.WriteString(layoutStyle.Render(summary))
	}

	leftStr := left.String()

	if !m.muxBarReady {
		info := sysinfo.Fetch(m.dir)
		var rightParts []string
		for _, key := range m.muxBarInfo {
			if val := m.resolveInfoItem(key, info); val != "" {
				rightParts = append(rightParts, val)
			}
		}
		m.muxBarRight = strings.Join(rightParts, "  ")
		m.muxBarReady = true
	}
	rightStr := m.muxBarRight

	leftW := lipgloss.Width(leftStr)
	rightW := lipgloss.Width(rightStr)
	gap := w - leftW - rightW
	if gap < 1 {
		gap = 1
	}

	bar := leftStr + barBg.Render(strings.Repeat(" ", gap)) + rightStr
	barW := lipgloss.Width(bar)
	if barW < w {
		bar += barBg.Render(strings.Repeat(" ", w-barW))
	}
	return bar
}

func readFileForPager(name, dir string) (string, error) {
	path := name
	if !strings.HasPrefix(path, "/") {
		path = dir + "/" + path
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
