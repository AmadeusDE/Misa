package ui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/term"
)

func newRoot(t *testing.T, w, h int) *Model {
	t.Helper()
	m := New(t.TempDir())
	m.nestedMode = false
	m.width, m.height = w, h
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

func TestKeyEncoding(t *testing.T) {
	tests := []struct {
		name string
		msg  tea.KeyPressMsg
		want string
	}{
		{"shift+tab", tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}, "\x1b[Z"},
		{"ctrl+left", tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModCtrl}, "\x1b[1;5D"},
		{"ctrl+shift+right", tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModCtrl | tea.ModShift}, "\x1b[1;6C"},
		{"shift+home", tea.KeyPressMsg{Code: tea.KeyHome, Mod: tea.ModShift}, "\x1b[1;2H"},
		{"ctrl+pgup", tea.KeyPressMsg{Code: tea.KeyPgUp, Mod: tea.ModCtrl}, "\x1b[5;5~"},
		{"ctrl+backslash", tea.KeyPressMsg{Code: '\\', Mod: tea.ModCtrl}, "\x1c"},
		{"ctrl+c", tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}, "\x03"},
		{"ctrl+space", tea.KeyPressMsg{Code: tea.KeySpace, Mod: tea.ModCtrl}, "\x00"},
		{"ctrl+alt+a", tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl | tea.ModAlt}, "\x1b\x01"},
		{"enter", tea.KeyPressMsg{Code: tea.KeyEnter}, "\r"},
		{"tab", tea.KeyPressMsg{Code: tea.KeyTab}, "\t"},
		{"escape", tea.KeyPressMsg{Code: tea.KeyEscape}, "\x1b"},
		{"alt+enter", tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt}, "\x1b\r"},
		{"alt+backspace", tea.KeyPressMsg{Code: tea.KeyBackspace, Mod: tea.ModAlt}, "\x1b\x7f"},
		{"alt+x", tea.KeyPressMsg{Code: 'x', Mod: tea.ModAlt}, "\x1bx"},
		{"alt+shift+x", tea.KeyPressMsg{Code: 'x', Mod: tea.ModAlt | tea.ModShift}, "\x1bX"},
		{"shifted text", tea.KeyPressMsg{Code: 'x', Text: "X", Mod: tea.ModShift}, "X"},
		{"multibyte text", tea.KeyPressMsg{Code: tea.KeyExtended, Text: "é界"}, "é界"},
		{"space", tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}, " "},
		{"f5", tea.KeyPressMsg{Code: tea.KeyF5}, "\x1b[15~"},
		{"shift+f1", tea.KeyPressMsg{Code: tea.KeyF1, Mod: tea.ModShift}, "\x1b[1;2P"},
		{"delete", tea.KeyPressMsg{Code: tea.KeyDelete}, "\x1b[3~"},
	}
	for _, tt := range tests {
		if got := string(keyToBytesMode(tt.msg, false)); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestUnknownKeyIsNotSentAsItsName(t *testing.T) {
	if b := keyToBytesMode(tea.KeyPressMsg{Code: tea.KeyF20}, false); b != nil {
		t.Errorf("f20 encoded as %q, want nil", b)
	}
}

func TestPasteBracketedOnlyWhenRequested(t *testing.T) {
	tm := term.New(2, 10)
	if got := string(pasteInput("hi", tm)); got != "hi" {
		t.Errorf("plain paste = %q, want hi", got)
	}
	tm.ProcessOutput([]byte("\x1b[?2004h"))
	if got := string(pasteInput("hi", tm)); got != "\x1b[200~hi\x1b[201~" {
		t.Errorf("bracketed paste = %q", got)
	}
}

func TestInsertAtCursorMultibyte(t *testing.T) {
	ti := textinput.New()
	ti.SetValue("éx")
	ti.SetCursor(1)
	insertAtCursor(&ti, "Z")
	if ti.Value() != "éZx" || ti.Position() != 2 {
		t.Errorf("value %q pos %d, want éZx pos 2", ti.Value(), ti.Position())
	}
}

func TestExtractRangeColumnsAndBounds(t *testing.T) {
	lines := []string{"─界ab", "second"}
	if got := extractRange(lines, 0, 1, 0, 4); got != "界a" {
		t.Errorf("got %q, want 界a", got)
	}
	if got := extractRange(lines, -1, -3, 0, 2); got != "─" {
		t.Errorf("negative bounds: got %q, want ─", got)
	}
	if got := extractRange(lines, 0, 99, 5, 99); got != "\nsecond" {
		t.Errorf("overflow bounds: got %q", got)
	}
}

func TestNeedsShell(t *testing.T) {
	for _, in := range []string{"echo hi > f", "echo a | wc", `echo "a b"`, "ls *.go", "echo $HOME", "ls ~/x", "echo a # comment", "{ ls; }"} {
		if !needsShell(in) {
			t.Errorf("needsShell(%q) = false", in)
		}
	}
	for _, in := range []string{"ls -la", "echo hello world", "cat file.txt"} {
		if needsShell(in) {
			t.Errorf("needsShell(%q) = true", in)
		}
	}
}

func TestEmptyAliasDoesNotCrash(t *testing.T) {
	m := New(t.TempDir())
	m.nestedMode = true
	m.width, m.height = 80, 24
	m.enterFullShell()
	m.aliases = map[string]string{"empty": ""}
	m.cmdInput.SetValue("empty")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
}

func TestCtrlCInterruptsCommandInsteadOfClosingPane(t *testing.T) {
	m := newRoot(t, 80, 24)
	m.addMuxPane()
	p := m.activePane()
	p.misa.enterFullShell()
	p.misa.execShellCmd("sleep 30")
	defer func() {
		if p.misa.terminal != nil {
			p.misa.terminal.Close()
		}
	}()
	m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if len(m.mux.panes) != 1 || p.misa.wantsClose {
		t.Fatalf("ctrl+c closed the pane (panes=%d)", len(m.mux.panes))
	}
	select {
	case <-p.misa.terminal.Exited():
	case <-time.After(5 * time.Second):
		t.Fatal("ctrl+c did not interrupt the command")
	}
}

func TestTinyWindowDoesNotPanic(t *testing.T) {
	m := newRoot(t, 80, 24)
	for i := 0; i < 3; i++ {
		m.addMuxPane()
	}
	m.mux.layout = layoutMaster
	m.rebuildMuxTree()
	for _, size := range [][2]int{{20, 2}, {3, 1}, {1, 1}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m.View()
	}
}

func TestTagRoundTripKeepsSplitRatio(t *testing.T) {
	m := newRoot(t, 100, 30)
	m.addMuxPane()
	m.addMuxPane()
	m.mux.tree.ratio = 0.6
	m.resize()
	m.switchTag(1)
	m.switchTag(0)
	if m.mux.tree.ratio != 0.6 {
		t.Errorf("ratio = %v, want 0.6", m.mux.tree.ratio)
	}
}

func TestTopBarClickHitsBar(t *testing.T) {
	m := newRoot(t, 100, 30)
	m.muxBarPosition = "top"
	m.muxBarAutoHide = false
	m.addMuxPane()
	m.switchTag(1)
	m.addMuxPane()
	m.switchTag(0)
	_, frameV := docStyle.GetFrameSize()
	dx, _ := m.muxContentOffset()
	m.Update(tea.MouseClickMsg{X: dx + 4, Y: frameV / 2, Button: tea.MouseLeft})
	if m.mux.activeTag != 1 {
		t.Errorf("activeTag = %d, want 1 after clicking tag 2 in the top bar", m.mux.activeTag)
	}
}

func pumpRootUntilExit(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for cmd != nil {
		done := make(chan tea.Msg, 1)
		go func(c tea.Cmd) { done <- c() }(cmd)
		var msg tea.Msg
		select {
		case msg = <-done:
		case <-deadline:
			t.Fatal("terminal never exited")
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			cmd = batch[0]
			continue
		}
		_, cmd = m.Update(msg)
		if pm, ok := msg.(paneMsg); ok {
			msg = pm.msg
		}
		if _, ok := msg.(termExited); ok {
			return
		}
	}
}

func TestMenuLaunchReturnsToMenuThroughRoot(t *testing.T) {
	m := newRoot(t, 80, 24)
	m.addMuxPane()
	p := m.activePane()
	p.misa.screen = screenMenu
	cmd := routeTo(p.misa, p.misa.launch(helperArgv(t, "status", "", "3")...))
	pumpRootUntilExit(t, m, cmd)
	if p.misa.screen != screenMenu || p.misa.menuLaunch {
		t.Errorf("screen=%v menuLaunch=%v, want back at menu", p.misa.screen, p.misa.menuLaunch)
	}
}

func TestViewportShrinksForPromptAfterCommand(t *testing.T) {
	m := New(t.TempDir())
	m.nestedMode = true
	m.width, m.height = 80, 24
	m.enterFullShell()
	cmd := m.execShellCmd("true")
	if m.fullShellVP.Height() != m.mux.primaryH {
		t.Errorf("running viewport height = %d, want %d", m.fullShellVP.Height(), m.mux.primaryH)
	}
	m.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	pumpUntilExit(t, m, cmd)
	if m.fullShellVP.Height() != m.mux.primaryH-2 {
		t.Errorf("idle viewport height = %d, want %d", m.fullShellVP.Height(), m.mux.primaryH-2)
	}
}

func TestHoverDuringSelectionRedrawsFrame(t *testing.T) {
	m := newRoot(t, 80, 24)
	m.addMuxPane()
	p := m.activePane()
	p.misa.enterFullShell()
	p.misa.shellBuffer = []string{"hello world"}
	p.misa.updateFullShellVP()
	dx, dy := m.muxContentOffset()
	m.Update(tea.MouseClickMsg{X: dx + 1, Y: dy, Button: tea.MouseLeft})
	m.View()
	if !p.misa.sel.active {
		t.Fatal("mouse press did not start selection")
	}
	m.Update(tea.MouseMotionMsg{X: dx + 4, Y: dy, Button: tea.MouseNone})
	if !p.misa.sel.hasRange {
		t.Fatal("hover did not extend selection")
	}
	if m.frame.reuse {
		t.Error("frame reused after selection changed")
	}
}

func TestAutorunOutputDoesNotClearPendingInput(t *testing.T) {
	m := newRoot(t, 80, 24)
	m.addMuxPane()
	p := m.activePane()
	p.misa.enterFullShell()
	p.misa.cmdInput.SetValue("unfinished")
	m.Update(paneMsg{owner: p.misa, msg: shellOutput{output: "autorun result"}})
	if got := p.misa.cmdInput.Value(); got != "unfinished" {
		t.Errorf("input after autorun = %q, want unfinished", got)
	}
}

func TestMuxBarInfoRefreshesOnTickNotEveryRender(t *testing.T) {
	m := newRoot(t, 100, 30)
	m.muxBarInfo = []string{"cwd"}
	m.muxBarView()
	first := m.muxBarRight
	m.dir = "/new-working-directory"
	m.muxBarView()
	if m.muxBarRight != first {
		t.Error("mux bar refreshed between ticks")
	}
	m.Update(tickMsg(time.Now()))
	m.muxBarView()
	if !strings.Contains(m.muxBarRight, "/new-working-directory") {
		t.Errorf("mux bar did not refresh on tick: %q", m.muxBarRight)
	}
}

func TestMuxBarConfigChangeInvalidatesCachedInfo(t *testing.T) {
	m := newRoot(t, 100, 30)
	m.applyMuxConfig(config.MuxConfig{BarInfo: "version"})
	m.muxBarView()
	if !strings.Contains(m.muxBarRight, "misa 0.1.0") {
		t.Fatal("initial bar info missing version")
	}
	m.Update(muxConfigChanged{cfg: config.MuxConfig{BarInfo: "cwd"}})
	m.muxBarView()
	if !strings.Contains(m.muxBarRight, m.dir) {
		t.Errorf("bar info still cached after config change: %q", m.muxBarRight)
	}
}

func TestPaneOutputReturnsToOwnerPane(t *testing.T) {
	m := newRoot(t, 100, 30)
	m.addMuxPane()
	a := m.activePane()
	m.addMuxPane()
	b := m.activePane()
	b.misa.cmdInput.SetValue("unfinished command")
	m.Update(paneMsg{owner: a.misa, msg: shellOutput{output: "owner-A"}})
	if !slices.Contains(a.misa.shellBuffer, "owner-A") {
		t.Errorf("owner pane missing output: %q", a.misa.shellBuffer)
	}
	if slices.Contains(b.misa.shellBuffer, "owner-A") || b.misa.cmdInput.Value() != "unfinished command" {
		t.Errorf("focused pane was disturbed: %q / %q", b.misa.shellBuffer, b.misa.cmdInput.Value())
	}
}

func TestRouteToKeepsRuntimeMessages(t *testing.T) {
	owner := &Model{}
	if _, ok := routeTo(owner, tea.Quit)().(tea.QuitMsg); !ok {
		t.Error("quit was wrapped")
	}
	batch, ok := routeTo(owner, tea.Batch(func() tea.Msg { return shellOutput{} }, tea.Quit))().(tea.BatchMsg)
	if !ok {
		t.Fatal("batch was wrapped")
	}
	if _, ok := batch[0]().(paneMsg); !ok {
		t.Error("batched pane message not routed")
	}
	for name, cmd := range map[string]tea.Cmd{
		"clipboard":        tea.SetClipboard("x"),
		"background query": tea.RequestBackgroundColor,
	} {
		if _, wrapped := routeTo(owner, cmd)().(paneMsg); wrapped {
			t.Errorf("%s request was wrapped; only the program can carry it out", name)
		}
	}
}

func TestMouseLaunchedCommandReturnsToMenu(t *testing.T) {
	m := newRoot(t, 80, 24)
	m.addMuxPane()
	p := m.activePane()
	p.misa.screen = screenMenu
	p.misa.inputMode = false
	p.misa.resize()
	p.misa.mainList.SetItems([]list.Item{menuItem{title: "run", action: func(n *Model) tea.Cmd {
		return n.launch(helperArgv(t, "status", "", "0")...)
	}}})
	p.misa.mainList.Select(0)
	dx, dy := m.muxContentOffset()
	click := tea.MouseClickMsg{X: dx + 2, Y: dy + p.misa.menuListStartY(), Button: tea.MouseLeft}
	_, cmd := m.Update(click)
	if cmd == nil || p.misa.terminal == nil {
		t.Fatalf("click did not launch with a subscription (cmd=%v terminal=%v)", cmd != nil, p.misa.terminal != nil)
	}
	pumpRootUntilExit(t, m, cmd)
	if p.misa.terminal != nil || p.misa.screen != screenMenu {
		t.Errorf("terminal=%v screen=%v, want back at menu", p.misa.terminal != nil, p.misa.screen)
	}
}

func TestViewportSelectionIsHighlighted(t *testing.T) {
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000"))
	view := "first\n" + red.Render("ab界cd") + "\nthird"
	sel := textSel{hasRange: true, startRow: 11, startCol: 1, endRow: 11, endCol: 4}

	got := strings.Split(sel.highlightRows(view, 10), "\n")
	if ansi.Strip(strings.Join(got, "\n")) != ansi.Strip(view) {
		t.Fatalf("highlight changed the text: %q", got)
	}
	reversed := lipgloss.NewStyle().Reverse(true).Render("b界")
	if !strings.Contains(got[1], reversed) {
		t.Errorf("row 11 = %q, want it to contain %q", got[1], reversed)
	}
	if !strings.HasSuffix(got[1], red.Render("cd")) {
		t.Errorf("row 11 = %q lost the style after the selection", got[1])
	}
	if got[0] != "first" || got[2] != "third" {
		t.Errorf("unselected rows changed: %q", got)
	}
}

func TestTypingClearsViewportSelection(t *testing.T) {
	m := New(t.TempDir())
	m.nestedMode = true
	m.width, m.height = 80, 24
	m.enterFullShell()
	m.sel = textSel{hasRange: true, endRow: 1}
	m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if m.sel.hasRange {
		t.Error("selection survived typing")
	}
}

func TestViewportStaysAtBottomAfterCommand(t *testing.T) {
	m := New(t.TempDir())
	m.nestedMode = true
	m.width, m.height = 80, 24
	m.enterFullShell()
	for i := 0; i < 60; i++ {
		m.shellBuffer = append(m.shellBuffer, "line")
	}
	m.updateFullShellVP()
	pumpUntilExit(t, m, m.execShellCmd("true"))
	if !m.fullShellVP.AtBottom() {
		t.Error("prompt pushed the last output below the view")
	}
}
