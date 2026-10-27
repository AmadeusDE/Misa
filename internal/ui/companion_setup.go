package ui

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/companion"
)

func isCompanionListStep(s onboardingStep) bool {
	switch s {
	case obCompanionMoods, obCompanionYourPronouns, obCompanionPronouns, obCompanionYourTerms, obCompanionTerms, obCompanionPrefixes, obCompanionItems, obCompanionYourParts, obCompanionParts, obCompanionEmotes, obCompanionYourKemonos, obCompanionKemonos, obCompanionModifiers:
		return true
	}
	return false
}

func checkedSet(vals []string) map[string]bool {
	m := map[string]bool{}
	for _, v := range vals {
		m[v] = true
	}
	return m
}

func newCompanionOnboarding() *onboardingState {
	moods := map[string]bool{"normal": true}
	return &onboardingState{
		companionMoods:        moods,
		companionYourPronouns: map[string]bool{},
		companionPronouns:     map[string]bool{},
		companionYourTerms:    map[string]bool{},
		companionTerms:        map[string]bool{},
		companionPrefixPos:    checkedSet(companion.PrefixPositiveDefaults()),
		companionPrefixNeg:    checkedSet(companion.PrefixNegativeDefaults()),
		companionItems:        checkedSet(companion.ItemDefaults()),
		companionYourParts:    checkedSet(companion.PartDefaults()),
		companionParts:        checkedSet(companion.PartDefaults()),
		companionEmotes:       checkedSet(companion.EmoteDefaults()),
		companionYourKemonos:  map[string]bool{},
		companionKemonos:      map[string]bool{},
		companionModifiers:    map[string]bool{},
	}
}

func (m *Model) enterCompanionSetup() {
	ob := newCompanionOnboarding()
	ob.step = obCompanionMoods
	ob.companionOnly = true
	m.onboarding = ob
	m.screen = screenOnboarding
	m.showCompanionStep()
}

func (m *Model) finishCompanionOnly() {
	m.applyCompanionSelection()
	m.onboarding = nil
	if m.settings != nil {
		m.screen = screenSettings
		m.refreshSettingsItems()
		m.resize()
		return
	}
	m.enterSettings()
}

func (m *Model) skipRemainingCompanion() {
	if m.onboarding != nil && m.onboarding.companionOnly {
		m.finishCompanionOnly()
		return
	}
	m.onboarding.step = obTheme - 1
	m.obAdvance()
}

func (m *Model) abandonCompanionSetup() {
	if m.onboarding != nil && m.onboarding.companionOnly {
		m.onboarding = nil
		if m.settings != nil {
			m.screen = screenSettings
			m.resize()
			return
		}
		m.enterSettings()
		return
	}
	m.onboarding.companionOff = false
	m.onboarding.companionMoods = map[string]bool{}
	m.onboarding.step = obTheme - 1
	m.obAdvance()
}

func (m *Model) companionSetupStarted() bool {
	ob := m.onboarding
	if ob == nil {
		return false
	}
	return ob.companionOff || len(selectedIDs(ob.companionMoods)) > 0
}

func selectedIDs(sel map[string]bool) []string {
	if sel == nil {
		return nil
	}
	var out []string
	for k, on := range sel {
		if on {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func selectedPieceIDs(sel map[string]bool, pieces []companion.Piece) []string {
	if sel == nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, p := range pieces {
		if sel[p.ID] {
			if len(p.Terms) > 0 {
				for _, t := range p.Terms {
					if !seen[t] {
						seen[t] = true
						out = append(out, t)
					}
				}
			} else if !seen[p.ID] {
				seen[p.ID] = true
				out = append(out, p.ID)
			}
		}
	}
	for k, on := range sel {
		if !on || seen[k] {
			continue
		}
		catalog := false
		for _, p := range pieces {
			if p.ID == k {
				catalog = true
				break
			}
		}
		if !catalog {
			out = append(out, k)
		}
	}
	return out
}

func selectedCatalogIDs(sel map[string]bool, pieces []companion.Piece) []string {
	if sel == nil {
		return nil
	}
	var out []string
	for _, p := range pieces {
		if sel[p.ID] {
			out = append(out, p.ID)
		}
	}
	return out
}

func (m *Model) applyCompanionSelection() {
	ob := m.onboarding
	if ob == nil {
		return
	}
	if ob.companionOff {
		b := false
		companion.SaveCompanionConfig(companion.CompanionConfig{Enabled: &b})
		return
	}
	if len(selectedIDs(ob.companionMoods)) == 0 {
		return
	}
	companion.WriteSetup(companion.CompanionConfig{
		Moods:             selectedCatalogIDs(ob.companionMoods, companion.MoodPieces()),
		YourPronouns:      selectedPieceIDs(ob.companionYourPronouns, companion.PronounPieces()),
		CompanionPronouns: selectedPieceIDs(ob.companionPronouns, companion.PronounPieces()),
		YourTerms:         selectedPieceIDs(ob.companionYourTerms, companion.YourTermPieces()),
		CompanionTerms:    selectedPieceIDs(ob.companionTerms, companion.CompanionTermPieces()),
		PrefixPositive:    selectedIDs(ob.companionPrefixPos),
		PrefixNegative:    selectedIDs(ob.companionPrefixNeg),
		Items:             selectedIDs(ob.companionItems),
		YourParts:         selectedIDs(ob.companionYourParts),
		CompanionParts:    selectedIDs(ob.companionParts),
		Emotes:            selectedIDs(ob.companionEmotes),
		YourKemonos:       selectedCatalogIDs(ob.companionYourKemonos, companion.YourKemonoPieces()),
		CompanionKemonos:  selectedCatalogIDs(ob.companionKemonos, companion.CompanionKemonoPieces()),
		Modifiers:         selectedCatalogIDs(ob.companionModifiers, companion.ModifierPieces()),
	})
}

func (m *Model) showCompanionStep() {
	switch m.onboarding.step {
	case obCompanionMoods:
		m.rebuildCompanionToggleList("Moods", companion.MoodPieces(), m.onboarding.companionMoods, func(s map[string]bool) { m.onboarding.companionMoods = s }, true, false)
	case obCompanionYourPronouns:
		m.rebuildCompanionToggleList("Your pronouns", companion.PronounPieces(), m.onboarding.companionYourPronouns, func(s map[string]bool) { m.onboarding.companionYourPronouns = s }, false, true)
	case obCompanionPronouns:
		m.rebuildCompanionToggleList("Companion pronouns", companion.PronounPieces(), m.onboarding.companionPronouns, func(s map[string]bool) { m.onboarding.companionPronouns = s }, false, true)
	case obCompanionYourTerms:
		m.rebuildCompanionToggleList("What they call you", companion.YourTermPieces(), m.onboarding.companionYourTerms, func(s map[string]bool) { m.onboarding.companionYourTerms = s }, false, true)
	case obCompanionTerms:
		m.rebuildCompanionToggleList("Companion names", companion.CompanionTermPieces(), m.onboarding.companionTerms, func(s map[string]bool) { m.onboarding.companionTerms = s }, false, true)
	case obCompanionPrefixes:
		m.showPrefixPicker()
	case obCompanionItems:
		m.rebuildCompanionToggleList("Items (SFW {item})", companion.ItemPieces(), m.onboarding.companionItems, func(s map[string]bool) { m.onboarding.companionItems = s }, false, true)
	case obCompanionYourParts:
		m.rebuildCompanionToggleList("Your parts (NSFW {your_part})", companion.PartPieces(), m.onboarding.companionYourParts, func(s map[string]bool) { m.onboarding.companionYourParts = s }, false, true)
	case obCompanionParts:
		m.rebuildCompanionToggleList("Companion parts (NSFW {companion_part})", companion.PartPieces(), m.onboarding.companionParts, func(s map[string]bool) { m.onboarding.companionParts = s }, false, true)
	case obCompanionEmotes:
		m.rebuildCompanionToggleList("Emotes", companion.EmotePieces(), m.onboarding.companionEmotes, func(s map[string]bool) { m.onboarding.companionEmotes = s }, false, true)
	case obCompanionYourKemonos:
		m.rebuildCompanionToggleList("Your kemonos", companion.YourKemonoPieces(), m.onboarding.companionYourKemonos, func(s map[string]bool) { m.onboarding.companionYourKemonos = s }, false, false)
	case obCompanionKemonos:
		m.rebuildCompanionToggleList("Companion kemonos", companion.CompanionKemonoPieces(), m.onboarding.companionKemonos, func(s map[string]bool) { m.onboarding.companionKemonos = s }, false, false)
	case obCompanionModifiers:
		m.rebuildCompanionToggleList("Modifiers", companion.ModifierPieces(), m.onboarding.companionModifiers, func(s map[string]bool) { m.onboarding.companionModifiers = s }, false, false)
	}
}

func (m *Model) showPrefixPicker() {
	var pieces []companion.Piece
	pieces = append(pieces, companion.PrefixPositivePieces()...)
	pieces = append(pieces, companion.PrefixNegativePieces()...)
	sel := map[string]bool{}
	for k, on := range m.onboarding.companionPrefixPos {
		sel[k] = on
	}
	for k, on := range m.onboarding.companionPrefixNeg {
		if on {
			sel[k] = true
		}
	}
	m.rebuildCompanionToggleList("Prefixes", pieces, sel, func(s map[string]bool) {
		pos := map[string]bool{}
		neg := map[string]bool{}
		for _, p := range companion.PrefixPositivePieces() {
			if s[p.ID] {
				pos[p.ID] = true
			}
		}
		for _, p := range companion.PrefixNegativePieces() {
			if s[p.ID] {
				neg[p.ID] = true
			}
		}
		for k, on := range s {
			if !on {
				continue
			}
			if pos[k] || neg[k] {
				continue
			}
			pos[k] = true
		}
		m.onboarding.companionPrefixPos = pos
		m.onboarding.companionPrefixNeg = neg
	}, false, true)
}

func (m *Model) rebuildCompanionToggleList(title string, pieces []companion.Piece, selected map[string]bool, set func(map[string]bool), disable, writeIn bool) {
	if selected == nil {
		selected = map[string]bool{}
		set(selected)
	}
	var labels []string
	for _, p := range pieces {
		if selected[p.ID] {
			labels = append(labels, p.Label)
		}
	}
	for k, on := range selected {
		inCat := false
		for _, p := range pieces {
			if p.ID == k {
				inCat = true
				break
			}
		}
		if on && !inCat {
			labels = append(labels, k)
		}
	}
	doneDesc := "Continue with none selected"
	if len(labels) > 0 {
		doneDesc = "Continue with: " + strings.Join(labels, ", ")
	}
	items := []list.Item{menuItem{
		title: "(done)",
		desc:  doneDesc,
		action: func(m *Model) tea.Cmd {
			m.obAdvance()
			return nil
		},
	}}
	if disable {
		items = append(items, menuItem{
			title: "(disable)",
			desc:  "Turn off companion messages after commands",
			action: func(m *Model) tea.Cmd {
				m.onboarding.companionOff = true
				m.skipRemainingCompanion()
				return nil
			},
		})
	}
	if writeIn {
		items = append(items, menuItem{
			title: "(custom)",
			desc:  "Type a write-in value",
			action: func(m *Model) tea.Cmd {
				m.startCompanionWriteIn()
				return nil
			},
		})
	}
	var lastGroup string
	for _, p := range pieces {
		p := p
		mark := "[ ] "
		if selected[p.ID] {
			mark = "[x] "
		}
		desc := p.Desc
		if p.Group != "" && p.Group != lastGroup {
			desc = p.Group + " — " + p.Desc
			lastGroup = p.Group
		}
		items = append(items, menuItem{
			title:   mark + p.Label,
			desc:    desc,
			preview: p.ID,
			action: func(m *Model) tea.Cmd {
				selected[p.ID] = !selected[p.ID]
				set(selected)
				if m.onboarding.step == obCompanionMoods {
					m.onboarding.companionOff = false
				}
				m.showCompanionStep()
				return nil
			},
		})
	}
	for k, on := range selected {
		k, on := k, on
		inCat := false
		for _, p := range pieces {
			if p.ID == k {
				inCat = true
				break
			}
		}
		if !on || inCat {
			continue
		}
		items = append(items, menuItem{
			title: "[x] " + k,
			desc:  "Write-in",
			action: func(m *Model) tea.Cmd {
				delete(selected, k)
				set(selected)
				m.showCompanionStep()
				return nil
			},
		})
	}
	idx := m.mainList.Index()
	m.mainList.SetItems(items)
	if idx >= len(items) {
		idx = 0
	}
	m.mainList.Select(idx)
	m.mainList.Title = title
	m.screen = screenOnboarding
	m.resize()
}

func (m *Model) startCompanionWriteIn() {
	m.onboarding.writeTarget = "current"
	m.onboarding.editing = true
	m.onboarding.input.SetValue("")
	m.onboarding.input.Placeholder = "write-in"
	m.onboarding.input.Focus()
}

func (m *Model) commitCompanionWriteIn(val string) {
	m.onboarding.writeTarget = ""
	if val == "" {
		m.showCompanionStep()
		return
	}
	ob := m.onboarding
	add := func(sel map[string]bool, set func(map[string]bool)) {
		if sel == nil {
			sel = map[string]bool{}
		}
		sel[val] = true
		set(sel)
	}
	switch ob.step {
	case obCompanionYourPronouns:
		add(ob.companionYourPronouns, func(s map[string]bool) { ob.companionYourPronouns = s })
	case obCompanionPronouns:
		add(ob.companionPronouns, func(s map[string]bool) { ob.companionPronouns = s })
	case obCompanionYourTerms:
		add(ob.companionYourTerms, func(s map[string]bool) { ob.companionYourTerms = s })
	case obCompanionTerms:
		add(ob.companionTerms, func(s map[string]bool) { ob.companionTerms = s })
	case obCompanionPrefixes:
		add(ob.companionPrefixPos, func(s map[string]bool) { ob.companionPrefixPos = s })
	case obCompanionItems:
		add(ob.companionItems, func(s map[string]bool) { ob.companionItems = s })
	case obCompanionYourParts:
		add(ob.companionYourParts, func(s map[string]bool) { ob.companionYourParts = s })
	case obCompanionParts:
		add(ob.companionParts, func(s map[string]bool) { ob.companionParts = s })
	case obCompanionEmotes:
		add(ob.companionEmotes, func(s map[string]bool) { ob.companionEmotes = s })
	}
	m.showCompanionStep()
}

func (m *Model) showCompanionPreview() {
	it, ok := m.mainList.SelectedItem().(menuItem)
	if !ok || it.preview == "" || m.onboarding == nil {
		return
	}
	sel := companion.Selection{Modifiers: selectedCatalogIDs(m.onboarding.companionModifiers, companion.ModifierPieces())}
	body := companion.PreviewPiece(it.preview, sel)
	w, h := m.width-4, m.height-8
	if w < 20 {
		w = 20
	}
	if h < 8 {
		h = 8
	}
	header := settingKeyStyle.Render("Preview: "+it.title) + "\n" +
		settingDimStyle.Render(strings.Repeat("─", 40)) + "\n\n"
	footer := "\n" + settingHelpStyle.Render("  esc: back to list")
	vp := viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))
	vp.SetContent(header + body + footer)
	m.onboarding.previewVP = vp
	m.onboarding.previewing = true
}

func (m *Model) updateCompanionPreview(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case mouseMsg:
		if isWheelUp(msg) || isWheelDown(msg) {
			var cmd tea.Cmd
			m.onboarding.previewVP, cmd = m.onboarding.previewVP.Update(msg)
			return m, cmd
		}
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "backspace", "q", "p":
			m.onboarding.previewing = false
			m.resize()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.onboarding.previewVP, cmd = m.onboarding.previewVP.Update(msg)
	return m, cmd
}

func (m *Model) companionPreviewView() string {
	var b strings.Builder
	b.WriteString(m.obProgress() + "\n\n")
	b.WriteString(m.onboarding.previewVP.View())
	return b.String()
}

func companionSummary(mc companion.CompanionConfig) string {
	if mc.Enabled != nil && !*mc.Enabled {
		return "off"
	}
	if len(mc.Modifiers) > 0 {
		return mc.Modifiers[0]
	}
	if len(mc.CompanionTerms) > 0 {
		return mc.CompanionTerms[0]
	}
	if len(mc.Moods) > 0 {
		return strings.Join(mc.Moods, ",")
	}
	if companion.Enabled() {
		return "on"
	}
	return "off"
}

func companionStepTitle(s onboardingStep) string {
	return stepName(s)
}

func companionStepDesc(s onboardingStep) string {
	switch s {
	case obCompanionMoods:
		return "Moods are separate line pools — the same line can exist in more than one.\nThirsty is the NSFW toggle and unlocks your-parts and companion-parts:"
	case obCompanionYourPronouns:
		return "Your pronouns. Separate from nicknames. Write-ins allowed (e.g. she her her hers herself):"
	case obCompanionPronouns:
		return "Companion pronouns. Feeds {they}/{them}/{their}. Write-ins allowed:"
	case obCompanionYourTerms:
		return "What the companion calls you. {affectionate_term}. Write-ins allowed:"
	case obCompanionTerms:
		return "What the companion calls itself. {role}. Write-ins allowed:"
	case obCompanionPrefixes:
		return "Prefixes wrap call terms: good girl, silly wife.\nPositive on success, negative on failure. Write-ins allowed:"
	case obCompanionItems:
		return "SFW {item} values (hugs, snacks, tea…):"
	case obCompanionYourParts:
		return "Your body in thirsty lines: {your_part}.\nOriginal mommy defaults start on. Extra gendered parts start off:"
	case obCompanionParts:
		return "Companion body in thirsty lines: {companion_part}.\nOriginal mommy defaults start on. Extra gendered parts start off:"
	case obCompanionEmotes:
		return "Optional emotes appended to some lines:"
	case obCompanionYourKemonos:
		return "If you want to be called puppy, kitty, bun… these add those nicknames:"
	case obCompanionKemonos:
		return "Companion animal flavor. Extra lines and emotes stacked on moods:"
	case obCompanionModifiers:
		return "Mommy, sister, kouhai, tsundere, and the rest.\nModifiers can add extra lines when combined (preview shows combos):"
	}
	return ""
}
