package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

type fieldKind int

const (
	fieldText fieldKind = iota
	fieldPrompt
	fieldFlag
	fieldChoice
	fieldSave
)

type formField struct {
	label   string
	kind    fieldKind
	in      textinput.Model
	area    textarea.Model
	on      bool
	choices []string
	choice  int
}

func newTextField(label, placeholder string) formField {
	in := textinput.New()
	in.Placeholder = placeholder
	in.Prompt = ""
	in.KeyMap.DeleteWordBackward = key.NewBinding(key.WithKeys("alt+backspace"))
	in.KeyMap.WordForward = key.NewBinding(key.WithKeys("ctrl+right", "alt+f"))
	in.KeyMap.WordBackward = key.NewBinding(key.WithKeys("ctrl+left", "alt+b"))
	in.KeyMap.NextSuggestion = key.NewBinding()
	in.KeyMap.PrevSuggestion = key.NewBinding()
	return formField{label: label, kind: fieldText, in: in}
}

// formCore is a column of labelled fields with one focused, shared by the TUI's forms.
type formCore struct {
	fields []formField
	focus  int
	fixed  int // a field shown but not editable (the name of a bot being edited); -1 for none
	err    string
	saving bool
	w, h   int
}

const formLabelWidth = 13

func (f *formCore) resize(w, h int) {
	f.w, f.h = w, h
	iw := max(w-formLabelWidth-2, 10)
	for i := range f.fields {
		switch f.fields[i].kind {
		case fieldText:
			f.fields[i].in.SetWidth(iw)
		case fieldPrompt:
			f.fields[i].area.SetWidth(iw)
			f.fields[i].area.SetHeight(min(max(h-len(f.fields)-4, 3), 20))
		}
	}
}

func (f *formCore) focusOn(i int) {
	f.fields[f.focus].blur()
	f.focus = (i + len(f.fields)) % len(f.fields)
	if f.focus == f.fixed {
		f.focus = (f.focus + 1) % len(f.fields)
	}
	f.fields[f.focus].focus()
}

func (ff *formField) focus() {
	switch ff.kind {
	case fieldText:
		ff.in.Focus()
	case fieldPrompt:
		ff.area.Focus()
	}
}

func (ff *formField) blur() {
	switch ff.kind {
	case fieldText:
		ff.in.Blur()
	case fieldPrompt:
		ff.area.Blur()
	}
}

// update handles one message; submit is true when the form should be saved, cancel when it was dismissed.
func (f *formCore) update(msg tea.Msg) (submit, cancel bool) {
	if f.saving {
		return false, false
	}
	cur := &f.fields[f.focus]
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch s := k.String(); s {
		case "esc":
			return false, true
		case "ctrl+s":
			return true, false
		case "tab":
			f.focusOn(f.focus + 1)
			return false, false
		case "shift+tab":
			f.focusOn(f.focus - 1)
			return false, false
		case "down", "up":
			if cur.kind != fieldPrompt {
				f.focusOn(f.focus + map[string]int{"down": 1, "up": -1}[s])
				return false, false
			}
		}
		switch cur.kind {
		case fieldFlag:
			if s := k.String(); s == "space" || s == "enter" || s == "x" {
				cur.on = !cur.on
			}
			return false, false
		case fieldChoice:
			switch k.String() {
			case "space", "enter", "right", "l":
				cur.choice = (cur.choice + 1) % len(cur.choices)
			case "left", "h":
				cur.choice = (cur.choice + len(cur.choices) - 1) % len(cur.choices)
			}
			return false, false
		case fieldSave:
			return k.String() == "enter", false
		case fieldText:
			if k.String() == "enter" {
				f.focusOn(f.focus + 1)
				return false, false
			}
		}
	}
	switch cur.kind {
	case fieldText:
		cur.in, _ = cur.in.Update(msg)
	case fieldPrompt:
		cur.area, _ = cur.area.Update(msg)
	}
	return false, false
}

// render draws the fields under title, scrolled to keep the focused one in view.
func (f *formCore) render(title string) string {
	var lines []string
	focusLine := 0
	for i, ff := range f.fields {
		if i == f.focus {
			focusLine = len(lines)
		}
		label := ff.label
		if i == f.focus {
			label = selectedStyle.Render(pad(ff.label, formLabelWidth))
		} else {
			label = dimStyle.Render(pad(ff.label, formLabelWidth))
		}
		switch ff.kind {
		case fieldText:
			if i == f.fixed {
				lines = append(lines, label+" "+ff.in.Value()+dimStyle.Render("  (fixed)"))
			} else {
				lines = append(lines, label+" "+ff.in.View())
			}
		case fieldFlag:
			box := "[ ]"
			if ff.on {
				box = "[x]"
			}
			lines = append(lines, label+" "+box)
		case fieldChoice:
			var opts []string
			for j, c := range ff.choices {
				if j == ff.choice {
					opts = append(opts, selectedStyle.Render("("+c+")"))
				} else {
					opts = append(opts, dimStyle.Render(" "+c+" "))
				}
			}
			lines = append(lines, label+" "+strings.Join(opts, " "))
		case fieldPrompt:
			for j, l := range strings.Split(ff.area.View(), "\n") {
				if j == 0 {
					lines = append(lines, label+" "+l)
				} else {
					lines = append(lines, strings.Repeat(" ", formLabelWidth+1)+l)
				}
			}
		case fieldSave:
			txt := "[ Save ]"
			if f.saving {
				txt = "saving..."
			}
			if i == f.focus {
				txt = selectedStyle.Render(txt)
			}
			lines = append(lines, txt)
		}
	}
	head := titleStyle.Render(title) + "\n"
	if f.err != "" {
		head += errStyle.Render(f.err)
	}
	room := max(f.h-3, 3)
	start := min(max(focusLine-room+1, 0), max(len(lines)-room, 0))
	if f.fields[f.focus].kind == fieldPrompt {
		start = min(start, focusLine)
	}
	end := min(start+room, len(lines))
	return head + "\n" + strings.Join(lines[start:end], "\n")
}

func pad(s string, n int) string {
	if r := []rune(s); len(r) < n {
		return s + strings.Repeat(" ", n-len(r))
	}
	return s
}
