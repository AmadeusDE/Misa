package ui

import (
	"unicode/utf8"

	"charm.land/bubbles/v2/textinput"
)

// renderInput renders ti with its whole placeholder. Without a width, Bubbles
// draws only the placeholder's first character, and a width would pad the
// input and push the ghost suggestion away from the text.
func renderInput(ti textinput.Model) string {
	v := ti.View()
	if ti.Value() != "" || ti.Width() > 0 {
		return v
	}
	_, first := utf8.DecodeRuneInString(ti.Placeholder)
	style := ti.Styles().Blurred.Placeholder
	if ti.Focused() {
		style = ti.Styles().Focused.Placeholder
	}
	return v + style.Render(ti.Placeholder[first:])
}
