package ui

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func browserFixture(t *testing.T) (*Model, string, **exec.Cmd, *tea.ExecCallback) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	for _, name := range []string{"a.png", "z ;$.png", ".hidden.png"} {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err = png.Encode(f, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	exe := filepath.Join(t.TempDir(), "misa")
	if err := os.WriteFile(exe, nil, 0755); err != nil {
		t.Fatal(err)
	}
	oldExe, oldExec := imageViewerExecutable, imageViewerExecProcess
	t.Cleanup(func() { imageViewerExecutable, imageViewerExecProcess = oldExe, oldExec })
	imageViewerExecutable = func() (string, error) { return exe, nil }
	var cmd *exec.Cmd
	var cb tea.ExecCallback
	imageViewerExecProcess = func(c *exec.Cmd, callback tea.ExecCallback) tea.Cmd {
		cmd, cb = c, callback
		return func() tea.Msg { return nil }
	}
	m := New(dir)
	m.width, m.height = 80, 24
	m.enterFileBrowser()
	m.fileList.SetSize(80, 24)
	return m, exe, &cmd, &cb
}
func selectBrowserName(t *testing.T, m *Model, name string) {
	t.Helper()
	for i, it := range m.fileList.Items() {
		if it.(fileItem).name == name {
			m.fileList.Select(i)
			return
		}
	}
	t.Fatalf("missing %s", name)
}
func TestBrowserImageLaunchAndReturn(t *testing.T) {
	m, exe, cmd, cb := browserFixture(t)
	selectBrowserName(t, m, "z ;$.png")
	_, launch := m.updateFileBrowser(tea.KeyPressMsg{Code: tea.KeyEnter})
	if launch == nil || *cmd == nil {
		t.Fatal("image did not launch")
	}
	dir := m.dir
	selected := filepath.Join(dir, "z ;$.png")
	want := []string{exe, "imgview", "--start", selected, "--", filepath.Join(dir, "a.png"), selected}
	if (*cmd).Dir != dir || !reflect.DeepEqual((*cmd).Args, want) {
		t.Fatalf("command: %q dir %q", (*cmd).Args, (*cmd).Dir)
	}
	os.WriteFile(filepath.Join(dir, "b.txt"), nil, 0644)
	(*cmd).Process = &os.Process{Pid: 42}
	m.updateImageViewerFinished((*cb)(errors.New("exit status 2")).(imageViewerFinished))
	if m.screen != screenFileBrowser || m.fileList.SelectedItem().(fileItem).name != "z ;$.png" || !strings.Contains(m.status, "exit status 2") {
		t.Fatalf("return: screen %v status %q item %v", m.screen, m.status, m.fileList.SelectedItem())
	}
	selectBrowserName(t, m, "b.txt")
}
func TestBrowserImageFallbackReturn(t *testing.T) {
	m, _, _, _ := browserFixture(t)
	imageViewerExecutable = func() (string, error) { return "", errors.New("unavailable") }
	selectBrowserName(t, m, "a.png")
	m.updateFileBrowser(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.screen != screenProcMon {
		t.Fatal("missing fallback")
	}
	os.WriteFile(filepath.Join(m.dir, "new.txt"), nil, 0644)
	m.updateProcMon(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if m.screen != screenFileBrowser || m.fileList.SelectedItem().(fileItem).name != "a.png" {
		t.Fatal("fallback did not return to selection")
	}
	selectBrowserName(t, m, "new.txt")
}

func TestBrowserMouseAndOrdinaryFiles(t *testing.T) {
	m, _, cmd, _ := browserFixture(t)
	// First click selects; the repeated click opens.
	i := 0
	for n, item := range m.fileList.VisibleItems() {
		if item.(fileItem).name == "a.png" {
			i = n
		}
	}
	m.fileList.Select(i - 1)
	y := 4 + i - m.fileList.Paginator.Page*m.fileList.Paginator.PerPage
	m.updateFileBrowser(mouseMsg{Mouse: tea.Mouse{Button: tea.MouseLeft, Y: y}, Action: mousePress})
	if *cmd != nil {
		t.Fatal("first click launched")
	}
	m.updateFileBrowser(mouseMsg{Mouse: tea.Mouse{Button: tea.MouseLeft, Y: y}, Action: mousePress})
	if *cmd == nil {
		t.Fatalf("mouse did not launch image i=%d y=%d page=%+v index=%d screen=%v", i, y, m.fileList.Paginator, m.fileList.Index(), m.screen)
	}
	m, _, cmd, _ = browserFixture(t)
	os.WriteFile(filepath.Join(m.dir, "notes.txt"), []byte("hello"), 0644)
	os.Mkdir(filepath.Join(m.dir, "sub"), 0755)
	m.reloadFileList()
	selectBrowserName(t, m, "notes.txt")
	m.updateFileBrowser(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.screen != screenEditor || *cmd != nil {
		t.Fatalf("text did not open editor: %v", m.screen)
	}
	m.screen = screenFileBrowser
	selectBrowserName(t, m, "sub")
	m.updateFileBrowser(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.dir != filepath.Join(m.fileList.Title, ".") || filepath.Base(m.dir) != "sub" || *cmd != nil {
		t.Fatalf("directory did not open: %s", m.dir)
	}
}

func TestBrowserFilterEnterAndReturn(t *testing.T) {
	m, _, cmd, cb := browserFixture(t)
	m.updateFileBrowser(tea.KeyPressMsg{Code: '/', Text: "/"})
	_, filter := m.updateFileBrowser(tea.KeyPressMsg{Code: 'a', Text: "a"})
	if filter != nil {
		m.updateFileBrowser(filter())
	}
	m.updateFileBrowser(tea.KeyPressMsg{Code: tea.KeyEnter})
	if *cmd != nil || m.fileList.SettingFilter() {
		t.Fatal("enter should finish filtering, not open")
	}
	for i, it := range m.fileList.VisibleItems() {
		if it.(fileItem).name == "a.png" {
			m.fileList.Select(i)
		}
	}
	m.updateFileBrowser(tea.KeyPressMsg{Code: tea.KeyEnter})
	if *cmd == nil {
		t.Fatal("filtered image not opened")
	}
	m.updateImageViewerFinished((*cb)(nil).(imageViewerFinished))
	if m.fileList.FilterValue() != "a" || m.fileList.SelectedItem().(fileItem).name != "a.png" {
		t.Fatal("filter or selection lost")
	}
}

func TestBrowserImageStartFailureStaysInBrowser(t *testing.T) {
	m, _, cmd, cb := browserFixture(t)
	selectBrowserName(t, m, "z ;$.png")
	m.updateFileBrowser(tea.KeyPressMsg{Code: tea.KeyEnter})
	if *cmd == nil {
		t.Fatal("missing launch")
	}
	m.updateImageViewerFinished((*cb)(errors.New("cannot start")).(imageViewerFinished))
	if m.screen != screenFileBrowser || !strings.Contains(m.status, "cannot start") || m.fileList.SelectedItem().(fileItem).name != "z ;$.png" {
		t.Fatalf("start failure did not return to browser: %s", m.status)
	}
}

func TestBrowserFilteredReturnUpdatesPagination(t *testing.T) {
	m, _, cmd, cb := browserFixture(t)
	for i := 0; i < 40; i++ {
		name := fmt.Sprintf("item%02d.png", i)
		if err := os.WriteFile(filepath.Join(m.dir, name), []byte("\x89PNG\r\n\x1a\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m.reloadFileList()
	m.fileList.SetSize(80, 16)
	m.fileList.SetFilterText("item")
	selectVisible := func(name string) {
		t.Helper()
		for i, item := range m.fileList.VisibleItems() {
			if item.(fileItem).name == name {
				m.fileList.Select(i)
				return
			}
		}
		t.Fatalf("missing visible %s", name)
	}
	selectVisible("item24.png")
	beforePage, beforeIndex := m.fileList.Paginator.Page, m.fileList.Index()
	m.updateFileBrowser(tea.KeyPressMsg{Code: tea.KeyEnter})
	if *cmd == nil {
		t.Fatal("filtered image did not launch")
	}
	for i := 30; i < 40; i++ {
		if err := os.Remove(filepath.Join(m.dir, fmt.Sprintf("item%02d.png", i))); err != nil {
			t.Fatal(err)
		}
	}
	m.updateImageViewerFinished((*cb)(nil).(imageViewerFinished))
	p := m.fileList.Paginator
	wantPages := (len(m.fileList.VisibleItems()) + p.PerPage - 1) / p.PerPage
	if p.TotalPages != wantPages || p.Page != beforePage || m.fileList.Index() != beforeIndex || m.fileList.SelectedItem().(fileItem).name != "item24.png" {
		t.Fatalf("filtered return: paginator=%+v want pages=%d page=%d index=%d selected=%v", p, wantPages, beforePage, beforeIndex, m.fileList.SelectedItem())
	}
	if m.fileList.FilterValue() != "item" {
		t.Fatal("filter changed")
	}
}

func TestBrowserCollectionFollowsFilteredDisplayOrder(t *testing.T) {
	m, _, cmd, cb := browserFixture(t)
	for _, name := range []string{"a000000b.png", "ab.png", "other.png"} {
		if err := os.WriteFile(filepath.Join(m.dir, name), []byte("\x89PNG\r\n\x1a\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m.reloadFileList()
	m.fileList.SetFilterText("ab")
	visible := m.fileList.VisibleItems()
	if len(visible) != 2 || visible[0].(fileItem).name != "ab.png" {
		t.Fatalf("expected ranked, non-lexical matches: %v", visible)
	}
	m.fileList.Select(1)
	m.updateFileBrowser(tea.KeyPressMsg{Code: tea.KeyEnter})
	want := []string{filepath.Join(m.dir, "ab.png"), filepath.Join(m.dir, "a000000b.png")}
	if *cmd == nil || !reflect.DeepEqual((*cmd).Args[5:], want) {
		t.Fatalf("collection did not follow visible matches: %v", *cmd)
	}
	m.updateImageViewerFinished((*cb)(nil).(imageViewerFinished))
	if m.fileList.FilterValue() != "ab" || m.fileList.SelectedItem().(fileItem).name != "a000000b.png" {
		t.Fatal("filtered selection lost on return")
	}
}
