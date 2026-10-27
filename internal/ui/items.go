package ui

import tea "charm.land/bubbletea/v2"

type menuItem struct {
	title   string
	desc    string
	action  func(m *Model) tea.Cmd
	preview string
}

func (i menuItem) Title() string       { return i.title }
func (i menuItem) Description() string { return i.desc }
func (i menuItem) FilterValue() string { return i.title }

type fileItem struct {
	name  string
	isDir bool
}

func (i fileItem) Title() string {
	if i.isDir {
		return i.name + "/"
	}
	return i.name
}

func (i fileItem) Description() string {
	if i.isDir {
		return "directory"
	}
	return "file"
}

func (i fileItem) FilterValue() string { return i.name }
