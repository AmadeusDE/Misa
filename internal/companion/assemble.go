package companion

import "strings"

func Assemble(sel Selection) *config {
	data := loadData()
	c := &config{
		Vars:  map[string]varConfig{},
		Moods: map[string]moodConfig{},
	}

	moods := uniqueNonEmpty(sel.Moods)
	if len(moods) == 0 {
		moods = []string{"normal"}
	}
	for i, m := range moods {
		moods[i] = normalizeMood(m)
	}
	moods = uniqueNonEmpty(moods)
	thirsty := HasThirsty(moods)

	modSet := toSet(sel.Modifiers)
	yourKemSet := toSet(sel.YourKemonos)
	compKemSet := toSet(sel.CompanionKemonos)

	yourTerms := append([]string(nil), sel.YourTerms...)
	compTerms := append([]string(nil), sel.CompanionTerms...)
	items := append([]string(nil), sel.Items...)
	yourParts := append([]string(nil), sel.YourParts...)
	compParts := append([]string(nil), sel.CompanionParts...)
	emotes := append([]string(nil), sel.Emotes...)
	love := append([]string(nil), sel.Love...)
	pos := append([]string(nil), sel.PrefixPositive...)
	neg := append([]string(nil), sel.PrefixNegative...)

	for _, m := range data.modifiers {
		if !modSet[m.ID] {
			continue
		}
		yourTerms = append(yourTerms, m.YourTerms...)
		compTerms = append(compTerms, m.CompanionTerms...)
		items = append(items, m.Items...)
		emotes = append(emotes, m.Emotes...)
		pos = append(pos, m.PrefixPositive...)
		neg = append(neg, m.PrefixNegative...)
		if thirsty {
			yourTerms = append(yourTerms, m.ThirstyYourTerms...)
		}
	}
	for _, k := range data.yourKemono {
		if !yourKemSet[k.ID] {
			continue
		}
		yourTerms = append(yourTerms, k.YourTerms...)
		emotes = append(emotes, k.Emotes...)
		pos = append(pos, k.PrefixPositive...)
	}
	for _, k := range data.compKemono {
		if !compKemSet[k.ID] {
			continue
		}
		compTerms = append(compTerms, k.CompanionTerms...)
		emotes = append(emotes, k.Emotes...)
		pos = append(pos, k.PrefixPositive...)
	}

	if len(yourTerms) == 0 {
		yourTerms = []string{"girl", "puppy"}
	}
	if len(compTerms) == 0 {
		compTerms = []string{"mommy", "owner"}
	}
	if len(pos) == 0 {
		pos = append([]string(nil), data.catalog.PrefixPositive...)
	}
	if len(neg) == 0 {
		neg = append([]string(nil), data.catalog.PrefixNegative...)
	}
	if len(items) == 0 {
		items = append([]string(nil), data.catalog.Items...)
	}
	if len(yourParts) == 0 {
		yourParts = PartDefaults()
	}
	if len(compParts) == 0 {
		compParts = PartDefaults()
	}
	if len(emotes) == 0 {
		emotes = append([]string(nil), data.catalog.Emotes...)
	}
	if len(love) == 0 {
		love = append([]string(nil), data.catalog.Love...)
	}

	yourTerms = uniqueNonEmpty(yourTerms)
	compTerms = uniqueNonEmpty(compTerms)
	items = uniqueNonEmpty(items)
	yourParts = uniqueNonEmpty(yourParts)
	compParts = uniqueNonEmpty(compParts)
	emotes = uniqueNonEmpty(emotes)
	love = uniqueNonEmpty(love)
	pos = uniqueNonEmpty(pos)
	neg = uniqueNonEmpty(neg)

	pronouns := sel.CompanionPronouns
	if len(pronouns) == 0 {
		pronouns = []string{"she her her hers herself"}
	}
	youPronouns := sel.YourPronouns
	if len(youPronouns) == 0 {
		youPronouns = []string{"they them their theirs themself"}
	}

	c.Vars["mood"] = varConfig{Defaults: moods}
	c.Vars["role"] = varConfig{Defaults: compTerms}
	c.Vars["affectionate_term"] = varConfig{Defaults: yourTerms}
	c.Vars["denigrating_term"] = varConfig{Defaults: prefixed(neg, yourTerms)}
	c.Vars["good"] = varConfig{Defaults: pos}
	c.Vars["prefix_positive"] = varConfig{Defaults: pos}
	c.Vars["prefix_negative"] = varConfig{Defaults: neg}
	c.Vars["diminutive"] = varConfig{Defaults: pos}
	c.Vars["your_part"] = varConfig{Defaults: yourParts}
	c.Vars["companion_part"] = varConfig{Defaults: compParts}
	c.Vars["part"] = varConfig{Defaults: compParts}
	c.Vars["item"] = varConfig{Defaults: items}
	c.Vars["emote"] = varConfig{Defaults: emotes}
	c.Vars["love"] = varConfig{Defaults: love}
	c.Vars["pronoun"] = varConfig{Defaults: pronouns}
	c.Vars["you_pronoun"] = varConfig{Defaults: youPronouns}

	selected := toSet(moods)
	for _, mf := range data.moods {
		id := normalizeMood(mf.ID)
		if !selected[id] {
			continue
		}
		block := moodConfig{
			Positive: append([]string(nil), mf.Positive...),
			Negative: append([]string(nil), mf.Negative...),
		}
		for _, m := range data.modifiers {
			if !modSet[m.ID] {
				continue
			}
			block = addLines(block, m.Lines[id], "")
			for _, w := range m.When {
				if hasAll(modSet, w.With) {
					block = addLines(block, w.Lines[id], "")
				}
			}
		}
		for _, k := range data.compKemono {
			if !compKemSet[k.ID] {
				continue
			}
			block = addKemonoLines(block, k.Lines, id, k.Sfx)
		}
		for _, k := range data.yourKemono {
			if !yourKemSet[k.ID] {
				continue
			}
			block = addKemonoLines(block, k.Lines, id, k.Sfx)
		}
		c.Moods[id] = block
	}
	return c
}

func addKemonoLines(dst moodConfig, lines map[string]lineBlock, mood, sfx string) moodConfig {
	src, ok := lines[mood]
	if !ok || (len(src.Positive) == 0 && len(src.Negative) == 0) {
		src = lines["normal"]
	}
	return addLines(dst, src, sfx)
}

func addLines(dst moodConfig, src lineBlock, sfx string) moodConfig {
	for _, l := range src.Positive {
		l = strings.ReplaceAll(l, "{sfx}", sfx)
		dst.Positive = appendUniqueKeep(dst.Positive, l)
	}
	for _, l := range src.Negative {
		l = strings.ReplaceAll(l, "{sfx}", sfx)
		dst.Negative = appendUniqueKeep(dst.Negative, l)
	}
	return dst
}

func appendUniqueKeep(dst []string, v string) []string {
	if v == "" {
		return dst
	}
	for _, x := range dst {
		if x == v {
			return dst
		}
	}
	return append(dst, v)
}

func uniqueNonEmpty(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func toSet(vals []string) map[string]bool {
	m := map[string]bool{}
	for _, v := range vals {
		if v != "" {
			m[v] = true
		}
	}
	return m
}

func hasAll(set map[string]bool, need []string) bool {
	for _, n := range need {
		if !set[n] {
			return false
		}
	}
	return len(need) > 0
}

func prefixed(prefixes, terms []string) []string {
	if len(terms) == 0 {
		return nil
	}
	if len(prefixes) == 0 {
		return append([]string(nil), terms...)
	}
	var out []string
	for _, p := range prefixes {
		for _, t := range terms {
			out = append(out, p+" "+t)
		}
	}
	return out
}
