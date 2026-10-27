package ui

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/pkgmanager"
)

func buildManagerItems(mgr pkgmanager.Manager) []list.Item {
	var items []list.Item
	for _, act := range mgr.Actions {
		act := act
		switch {
		case len(act.Run) > 0:
			items = append(items, menuItem{
				title: act.Title,
				desc:  act.Desc,
				action: func(m *Model) tea.Cmd {
					return m.launch(act.Run...)
				},
			})
		case act.ShellPrefill != "":
			items = append(items, menuItem{
				title: act.Title,
				desc:  act.Desc,
				action: func(m *Model) tea.Cmd {
					m.enterShellWith(act.ShellPrefill)
					return nil
				},
			})
		case act.List != nil:
			items = append(items, menuItem{
				title: act.Title,
				desc:  act.Desc,
				action: func(m *Model) tea.Cmd {
					return m.showListAction(mgr.Name, act)
				},
			})
		}
	}
	return items
}

func (m *Model) showListAction(mgrName string, act pkgmanager.Action) tea.Cmd {
	la := act.List
	if la == nil {
		return nil
	}
	results, err := la.FetchItems()
	if err != nil {
		m.status = mgrName + ": " + err.Error()
		return nil
	}
	if len(results) == 0 {
		m.status = mgrName + ": no items found"
		return nil
	}
	var items []list.Item
	for _, item := range results {
		item := item
		cmd := la.ItemCmd(item)
		items = append(items, menuItem{
			title: item,
			desc:  la.ItemDesc,
			action: func(m *Model) tea.Cmd {
				if len(cmd) == 0 {
					return nil
				}
				return m.launch(cmd...)
			},
		})
	}
	m.pushMenu(act.Title, items)
	return nil
}

func (m *Model) enterShellWith(prefill string) {
	m.menuStack = nil
	m.refreshRootMenu()
	m.resize()
	m.updateShellView()
	m.inputMode = true
	m.cmdInput.Focus()
	m.cmdInput.SetValue(prefill)
	m.cmdInput.SetCursor(len(prefill))
}
