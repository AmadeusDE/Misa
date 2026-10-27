package companion

import (
	"strconv"
	"strings"
)

func PreviewPiece(id string, sel Selection) string {
	data := loadData()
	for _, m := range data.moods {
		if m.ID == id {
			return previewMood(m)
		}
	}
	for _, m := range data.modifiers {
		if m.ID == id {
			return previewModifier(m, sel.Modifiers)
		}
	}
	for _, k := range data.yourKemono {
		if k.ID == id {
			return previewKemono(k)
		}
	}
	for _, k := range data.compKemono {
		if k.ID == id {
			return previewKemono(k)
		}
	}
	for _, p := range append(append(YourTermPieces(), CompanionTermPieces()...), PronounPieces()...) {
		if p.ID == id {
			var b strings.Builder
			b.WriteString(p.Label + "\n")
			if p.Desc != "" {
				b.WriteString(p.Desc + "\n")
			}
			if len(p.Terms) > 0 {
				b.WriteString("\nValues\n  " + joinComma(p.Terms) + "\n")
			}
			return b.String()
		}
	}
	return "(no preview available)"
}

func previewMood(m moodFile) string {
	var b strings.Builder
	b.WriteString(m.Label + "\n" + m.Desc + "\n\n")
	if m.NSFW {
		b.WriteString("NSFW. Unlocks the part menu and uses original mommy parts.\n\n")
	}
	writeLineBlock(&b, "Lines", lineBlock{Positive: m.Positive, Negative: m.Negative})
	return b.String()
}

func previewModifier(m modifierFile, selected []string) string {
	var b strings.Builder
	b.WriteString(m.Label)
	if m.Group != "" {
		b.WriteString("  (" + m.Group + ")")
	}
	b.WriteString("\n" + m.Desc + "\n\n")
	if len(m.CompanionTerms) > 0 {
		b.WriteString("Companion terms: " + joinComma(m.CompanionTerms) + "\n")
	}
	if len(m.YourTerms) > 0 {
		b.WriteString("Your terms: " + joinComma(m.YourTerms) + "\n")
	}
	if len(m.Items) > 0 {
		b.WriteString("Items: " + joinComma(m.Items) + "\n")
	}
	b.WriteString("\n")
	for _, mood := range []string{"normal", "needy", "ominous", "thirsty"} {
		if block, ok := m.Lines[mood]; ok {
			writeLineBlock(&b, mood, block)
		}
	}
	if len(m.When) > 0 {
		b.WriteString("Combo lines (need extra modifiers)\n")
		have := toSet(selected)
		have[m.ID] = true
		for _, w := range m.When {
			b.WriteString("  with " + joinComma(w.With))
			if hasAll(have, w.With) {
				b.WriteString("  [active]\n")
			} else {
				b.WriteString("\n")
			}
			for mood, block := range w.Lines {
				writeLineBlock(&b, "  "+mood, block)
			}
		}
	}
	return b.String()
}

func previewKemono(k kemonoFile) string {
	var b strings.Builder
	b.WriteString(k.Label + "\n" + k.Desc + "\n\n")
	if k.Sfx != "" {
		b.WriteString("sfx:" + k.Sfx + "\n")
	}
	if len(k.YourTerms) > 0 {
		b.WriteString("Your terms: " + joinComma(k.YourTerms) + "\n")
	}
	if len(k.Emotes) > 0 {
		b.WriteString("Emotes: " + joinComma(k.Emotes) + "\n")
	}
	b.WriteString("\n")
	for mood, block := range k.Lines {
		writeLineBlock(&b, mood, block)
	}
	return b.String()
}

func writeLineBlock(b *strings.Builder, heading string, block lineBlock) {
	if len(block.Positive) == 0 && len(block.Negative) == 0 {
		return
	}
	b.WriteString(heading + "\n")
	if len(block.Positive) > 0 {
		b.WriteString("  success (" + strconv.Itoa(len(block.Positive)) + ")\n")
		for _, l := range block.Positive {
			b.WriteString("    + " + l + "\n")
		}
	}
	if len(block.Negative) > 0 {
		b.WriteString("  failure (" + strconv.Itoa(len(block.Negative)) + ")\n")
		for _, l := range block.Negative {
			b.WriteString("    - " + l + "\n")
		}
	}
	b.WriteString("\n")
}
