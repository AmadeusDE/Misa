package companion

func MoodPieces() []Piece {
	data := loadData()
	out := make([]Piece, 0, len(data.moods))
	order := []string{"normal", "needy", "ominous", "thirsty"}
	idx := map[string]moodFile{}
	for _, m := range data.moods {
		idx[normalizeMood(m.ID)] = m
	}
	for _, id := range order {
		m, ok := idx[id]
		if !ok {
			continue
		}
		out = append(out, Piece{ID: id, Label: m.Label, Desc: m.Desc, Group: "Mood"})
	}
	return out
}

func ListMoods() []string {
	var out []string
	for _, p := range MoodPieces() {
		out = append(out, p.ID)
	}
	return out
}

func DefaultMoods() []string {
	return []string{"normal"}
}

func PronounPieces() []Piece {
	data := loadData()
	out := make([]Piece, 0, len(data.catalog.Pronouns))
	for _, e := range data.catalog.Pronouns {
		out = append(out, Piece{ID: e.ID, Label: e.Label, Desc: e.Desc, Group: "Pronouns", Terms: []string{e.Forms}})
	}
	return out
}

func YourTermPieces() []Piece {
	return termPieces(loadData().catalog.YourTerms)
}

func CompanionTermPieces() []Piece {
	return termPieces(loadData().catalog.CompanionTerms)
}

func termPieces(entries []termEntry) []Piece {
	out := make([]Piece, 0, len(entries))
	for _, e := range entries {
		desc := e.Desc
		if desc == "" && len(e.Terms) > 0 {
			desc = joinComma(e.Terms)
		}
		out = append(out, Piece{ID: e.ID, Label: e.Label, Desc: desc, Group: e.Group, Terms: e.Terms})
	}
	return out
}

func PrefixPositiveDefaults() []string {
	return append([]string(nil), loadData().catalog.PrefixPositive...)
}

func PrefixNegativeDefaults() []string {
	return append([]string(nil), loadData().catalog.PrefixNegative...)
}

func ItemDefaults() []string {
	return append([]string(nil), loadData().catalog.Items...)
}

func PartDefaults() []string {
	var out []string
	for _, p := range loadData().catalog.Parts {
		if p.Default {
			out = append(out, p.ID)
		}
	}
	return out
}

func EmoteDefaults() []string {
	return append([]string(nil), loadData().catalog.Emotes...)
}

func stringPieces(vals []string, group string) []Piece {
	out := make([]Piece, 0, len(vals))
	for _, v := range vals {
		out = append(out, Piece{ID: v, Label: v, Desc: group, Group: group, Terms: []string{v}})
	}
	return out
}

func PrefixPositivePieces() []Piece {
	return stringPieces(PrefixPositiveDefaults(), "Positive prefix")
}

func PrefixNegativePieces() []Piece {
	return stringPieces(PrefixNegativeDefaults(), "Negative prefix")
}

func ItemPieces() []Piece {
	return stringPieces(ItemDefaults(), "Item")
}

func PartPieces() []Piece {
	var out []Piece
	for _, p := range loadData().catalog.Parts {
		label := p.Label
		if label == "" {
			label = p.ID
		}
		desc := p.Desc
		if desc == "" {
			desc = p.Group
		}
		if !p.Default {
			if desc != "" {
				desc += " — off by default"
			} else {
				desc = "off by default"
			}
		}
		out = append(out, Piece{ID: p.ID, Label: label, Desc: desc, Group: p.Group, Terms: []string{p.ID}})
	}
	return out
}

func EmotePieces() []Piece {
	return stringPieces(EmoteDefaults(), "Emote")
}

func ModifierPieces() []Piece {
	data := loadData()
	out := make([]Piece, 0, len(data.modifiers))
	for _, m := range data.modifiers {
		out = append(out, Piece{ID: m.ID, Label: m.Label, Desc: m.Desc, Group: m.Group})
	}
	return out
}

func YourKemonoPieces() []Piece {
	data := loadData()
	out := make([]Piece, 0, len(data.yourKemono))
	for _, k := range data.yourKemono {
		out = append(out, Piece{ID: k.ID, Label: k.Label, Desc: k.Desc, Group: "Your kemono", Terms: k.YourTerms})
	}
	return out
}

func CompanionKemonoPieces() []Piece {
	data := loadData()
	out := make([]Piece, 0, len(data.compKemono))
	for _, k := range data.compKemono {
		out = append(out, Piece{ID: k.ID, Label: k.Label, Desc: k.Desc, Group: "Companion kemono"})
	}
	return out
}

func joinComma(vals []string) string {
	out := ""
	for i, v := range vals {
		if i > 0 {
			out += ", "
		}
		out += v
	}
	return out
}
