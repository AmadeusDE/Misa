package ui

import (
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/imgview"
)

// activateBrowserItem is shared by keyboard and mouse activation.
func (m *Model) activateBrowserItem() (tea.Model, tea.Cmd) {
	it, ok := m.fileList.SelectedItem().(fileItem)
	if !ok {
		return m, nil
	}
	if it.name == ".." {
		m.dir = filepath.Dir(m.dir)
		return m, m.reloadFileList()
	}
	path := filepath.Join(m.dir, it.name)
	if it.isDir {
		m.dir = path
		return m, m.reloadFileList()
	}
	cfg := config.LoadImageViewer()
	limits := imgview.Limits{MaxEncodedBytes: int64(cfg.MaxFileMiB) << 20, MaxPixels: int64(cfg.MaxPixels)}
	isImage, err := imgview.CheckImageFile(path, limits)
	if err != nil {
		m.status = "imgview: " + err.Error()
		return m, nil
	}
	if !isImage {
		return m, m.openInEditor(path)
	}
	var paths []string
	for _, item := range m.fileList.VisibleItems() {
		f, ok := item.(fileItem)
		if ok && !f.isDir {
			paths = append(paths, filepath.Join(m.dir, f.name))
		}
	}
	col, _, err := imgview.CollectBrowserPaths(paths, path, imgview.CollectOptions{MaxFiles: cfg.MaxImages, MaxEntriesScanned: cfg.MaxDirectoryEntries, MaxDepth: cfg.MaxDirectoryDepth}, limits)
	if err != nil {
		m.status = "imgview: " + err.Error()
		return m, nil
	}
	args := append([]string{"--start", path, "--"}, col.Paths...)
	return m, m.launchBrowserImageViewer(m.dir, args, it.name, col.Warning)
}

// reloadBrowserSelection retains the filename, filter and page whenever possible.
func (m *Model) reloadBrowserSelection(name string) tea.Cmd {
	cmd := m.reloadFileList()
	if cmd != nil {
		m.fileList.SetFilterText(m.fileList.FilterValue())
		m.fileList.SetSize(m.fileList.Width(), m.fileList.Height())
	}
	items := m.fileList.VisibleItems()
	if len(items) > 0 && m.fileList.Index() >= len(items) {
		m.fileList.Select(len(items) - 1)
	}
	for i, item := range items {
		if f, ok := item.(fileItem); ok && f.name == name {
			m.fileList.Select(i)
			break
		}
	}
	return nil
}
