package procmon

import tea "charm.land/bubbletea/v2"

// typedKey is the text a key types, or its name for keys that type nothing,
// so text inputs keep spaces that Bubble Tea names "space".
func typedKey(msg tea.KeyPressMsg) string {
	if msg.Text != "" && !msg.Mod.Contains(tea.ModCtrl) && !msg.Mod.Contains(tea.ModAlt) {
		return msg.Text
	}
	return msg.String()
}
