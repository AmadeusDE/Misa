package ui

import (
	"fmt"
	"path/filepath"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/fileops"
)

func (m *Model) enterFileBrowser() tea.Cmd {
	m.screen = screenFileBrowser
	m.resize()
	return m.reloadFileList()
}

func (m *Model) reloadFileList() tea.Cmd {
	entries, err := fileops.List(m.dir)
	if err != nil {
		m.status = "error: " + err.Error()
		return nil
	}

	items := make([]list.Item, 0, len(entries)+1)
	if filepath.Dir(m.dir) != m.dir {
		items = append(items, fileItem{name: "..", isDir: true})
	}
	for _, e := range entries {
		items = append(items, fileItem{name: e.Name, isDir: e.IsDir})
	}

	m.fileList.Title = m.dir
	m.status = ""
	return m.fileList.SetItems(items)
}

func (m *Model) updateFileBrowser(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case mouseMsg:
		return m.handleFileBrowserMouse(msg)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "backspace", "q":
			m.screen = screenMenu
			m.refreshRootMenu()
			m.resize()
			return m, nil

		case "enter":
			it, ok := m.fileList.SelectedItem().(fileItem)
			if !ok {
				return m, nil
			}
			if it.name == ".." {
				m.dir = filepath.Dir(m.dir)
				return m, m.reloadFileList()
			}
			if it.isDir {
				m.dir = filepath.Join(m.dir, it.name)
				return m, m.reloadFileList()
			}
			m.openInEditor(filepath.Join(m.dir, it.name))
			return m, nil

		case "n":
			m.screen = screenNewFile
			m.fileInput.SetValue("")
			m.fileInput.Placeholder = "newfile.txt"
			m.fileInput.Focus()
			return m, nil

		case "d":
			it, ok := m.fileList.SelectedItem().(fileItem)
			if !ok || it.name == ".." {
				return m, nil
			}
			m.screen = screenConfirmDelete
			m.pendingDelete = filepath.Join(m.dir, it.name)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.fileList, cmd = m.fileList.Update(msg)
	return m, cmd
}

func (m *Model) updateNewFile(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			m.screen = screenFileBrowser
			return m, nil
		case "enter":
			name := m.fileInput.Value()
			m.screen = screenFileBrowser
			if name == "" {
				return m, nil
			}
			if _, err := fileops.CreateFile(m.dir, name); err != nil {
				m.status = "error: " + err.Error()
				return m, nil
			}
			return m, m.reloadFileList()
		}
	}

	var cmd tea.Cmd
	m.fileInput, cmd = m.fileInput.Update(msg)
	return m, cmd
}

func (m *Model) updateConfirmDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "y":
		err := fileops.Delete(m.pendingDelete)
		m.pendingDelete = ""
		m.screen = screenFileBrowser
		if err != nil {
			m.status = "error: " + err.Error()
			return m, nil
		}
		return m, m.reloadFileList()
	case "n", "esc":
		m.pendingDelete = ""
		m.screen = screenFileBrowser
		return m, nil
	}
	return m, nil
}

func (m *Model) handleFileBrowserMouse(msg mouseMsg) (tea.Model, tea.Cmd) {
	if isLeftClick(msg) {
		startY := 2
		if m.fileList.FilteringEnabled() {
			startY = 4
		}
		idx := listClickIndex(m.fileList, msg.Y, startY)
		if idx >= 0 && idx < len(m.fileList.Items()) {
			if idx == m.fileList.Index() {
				it, ok := m.fileList.SelectedItem().(fileItem)
				if !ok {
					return m, nil
				}
				if it.name == ".." {
					m.dir = filepath.Dir(m.dir)
					return m, m.reloadFileList()
				}
				if it.isDir {
					m.dir = filepath.Join(m.dir, it.name)
					return m, m.reloadFileList()
				}
				m.openInEditor(filepath.Join(m.dir, it.name))
				return m, nil
			}
			m.fileList.Select(idx)
		}
		return m, nil
	}
	if isWheelUp(msg) || isWheelDown(msg) {
		idx := m.fileList.Index()
		if isWheelUp(msg) {
			if idx > 0 {
				m.fileList.Select(idx - 3)
			}
		} else {
			total := len(m.fileList.Items())
			if idx < total-1 {
				next := idx + 3
				if next >= total {
					next = total - 1
				}
				m.fileList.Select(next)
			}
		}
		return m, nil
	}
	return m, nil
}

func fileHelpKeys() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open/cd")),
		key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new file")),
		key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

func (m *Model) fileBrowserView() string {
	return m.fileList.View()
}

func (m *Model) newFileView() string {
	return fmt.Sprintf("New file name in %s:\n\n%s\n\n(enter to create, esc to cancel)", m.dir, renderInput(m.fileInput))
}

func (m *Model) confirmDeleteView() string {
	return fmt.Sprintf("Delete %s? (y/n)", m.pendingDelete)
}
