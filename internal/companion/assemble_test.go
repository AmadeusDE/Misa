package companion

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssembleIncludesOriginalMommyLines(t *testing.T) {
	base := Assemble(Selection{Moods: []string{"normal", "ominous", "thirsty"}})
	blob := moodBlob(base)
	for _, line := range []string{
		"*pets your head*",
		"you're such a smart cookie~",
		"What you have set in motion today will be remembered for aeons to come!",
		"give {role} a kiss~",
	} {
		if !strings.Contains(blob, line) {
			t.Errorf("missing base line %q", line)
		}
	}
	for _, line := range []string{
		"*tugs your leash*",
		"keep it up and {role} might let you cum",
		"no more {part} for you {denigrating_term}",
		"on your knees",
	} {
		if strings.Contains(blob, line) {
			t.Errorf("dominant line should not be in base moods: %q", line)
		}
	}
	withMommy := Assemble(Selection{Moods: []string{"thirsty"}, Modifiers: []string{"mommy"}})
	mom := moodBlob(withMommy)
	for _, line := range []string{
		"*tugs your leash*",
		"keep it up and {role} might let you cum",
		"no more {companion_part} for you {denigrating_term}",
	} {
		if !strings.Contains(mom, line) {
			t.Errorf("mommy modifier missing original dominant line %q", line)
		}
	}
}

func TestAssembleThirstyUsesOriginalParts(t *testing.T) {
	cfg := Assemble(Selection{Moods: []string{"thirsty"}})
	for _, key := range []string{"your_part", "companion_part"} {
		parts := cfg.Vars[key].Defaults
		for _, want := range []string{"milk", "cock", "cum", "mouth"} {
			if !containsStr(parts, want) {
				t.Errorf("%s missing %q (got %v)", key, want, parts)
			}
		}
	}
}

func TestGenderedPartsDisabledByDefault(t *testing.T) {
	d := PartDefaults()
	for _, extra := range []string{"pussy", "clit", "tits", "cunt", "balls"} {
		if containsStr(d, extra) {
			t.Errorf("gendered part %q should be off by default, got %v", extra, d)
		}
	}
	cfg := Assemble(Selection{Moods: []string{"thirsty"}})
	if containsStr(cfg.Vars["your_part"].Defaults, "pussy") || containsStr(cfg.Vars["companion_part"].Defaults, "pussy") {
		t.Errorf("assemble should not enable extra gendered parts by default: your=%v companion=%v", cfg.Vars["your_part"].Defaults, cfg.Vars["companion_part"].Defaults)
	}
	have := false
	for _, p := range PartPieces() {
		if p.ID == "pussy" {
			have = true
		}
	}
	if !have {
		t.Error("pussy should still be listed as a selectable part")
	}
}

func TestFamilyModifiersHaveExplicitThirsty(t *testing.T) {
	for _, id := range []string{"imouto", "daughter", "otouto", "onii"} {
		cfg := Assemble(Selection{Moods: []string{"thirsty"}, Modifiers: []string{id}})
		blob := strings.Join(cfg.Moods["thirsty"].Positive, "\n")
		explicit := strings.Contains(blob, "{your_part}") || strings.Contains(blob, "{companion_part}") || strings.Contains(blob, "fuck") || strings.Contains(blob, "cum") || strings.Contains(blob, "inside")
		if !explicit {
			t.Errorf("%s thirsty lines should be explicit, got:\n%s", id, blob)
		}
	}
}

func TestAssembleItemAndPartAreSeparate(t *testing.T) {
	cfg := Assemble(Selection{
		Moods:          []string{"normal"},
		Items:          []string{"hugs", "snacks"},
		YourParts:      []string{"milk", "cock"},
		CompanionParts: []string{"milk", "cock"},
	})
	if containsStr(cfg.Vars["item"].Defaults, "cock") {
		t.Errorf("item should stay SFW, got %v", cfg.Vars["item"].Defaults)
	}
	if containsStr(cfg.Vars["your_part"].Defaults, "hugs") {
		t.Errorf("your_part should stay NSFW, got %v", cfg.Vars["your_part"].Defaults)
	}
	if !containsStr(cfg.Vars["item"].Defaults, "hugs") {
		t.Errorf("item missing hugs: %v", cfg.Vars["item"].Defaults)
	}
	if !containsStr(cfg.Vars["your_part"].Defaults, "cock") {
		t.Errorf("your_part missing cock: %v", cfg.Vars["your_part"].Defaults)
	}
	if !containsStr(cfg.Vars["companion_part"].Defaults, "cock") {
		t.Errorf("companion_part missing cock: %v", cfg.Vars["companion_part"].Defaults)
	}
	for _, line := range append(cfg.Moods["normal"].Positive, cfg.Moods["normal"].Negative...) {
		if strings.Contains(line, "{part}") {
			t.Errorf("SFW mood line should not use {part}: %q", line)
		}
	}
}

func TestAssembleDoesNotDedupeAcrossMoods(t *testing.T) {
	cfg := Assemble(Selection{Moods: []string{"normal", "needy"}})
	n := cfg.Moods["normal"]
	k := cfg.Moods["needy"]
	shared := "you did it~!"
	if !containsStr(n.Positive, shared) {
		t.Fatalf("normal should keep %q", shared)
	}
	if !containsStr(k.Positive, shared) {
		t.Fatalf("needy should also keep %q (no cross-mood dedup)", shared)
	}
}

func TestAssembleComboModifierLines(t *testing.T) {
	alone := Assemble(Selection{Moods: []string{"normal"}, Modifiers: []string{"tsundere"}})
	both := Assemble(Selection{Moods: []string{"normal"}, Modifiers: []string{"tsundere", "onee"}})
	combo := "it's not like onee-chan was watching the whole time"
	if strings.Contains(moodBlob(alone), combo) {
		t.Error("tsundere alone should not include sister combo lines")
	}
	if !strings.Contains(moodBlob(both), combo) {
		t.Error("tsundere+onee should include combo lines")
	}
}

func TestAssembleWriteIns(t *testing.T) {
	cfg := Assemble(Selection{
		Moods:             []string{"normal"},
		YourTerms:         []string{"starlight"},
		CompanionTerms:    []string{"captain"},
		YourPronouns:      []string{"they them their theirs themself"},
		CompanionPronouns: []string{"he him his his himself"},
	})
	if !containsStr(cfg.Vars["affectionate_term"].Defaults, "starlight") {
		t.Errorf("write-in your term missing: %v", cfg.Vars["affectionate_term"].Defaults)
	}
	if !containsStr(cfg.Vars["role"].Defaults, "captain") {
		t.Errorf("write-in companion term missing: %v", cfg.Vars["role"].Defaults)
	}
}

func TestAssembleKemonoAddsYourTerm(t *testing.T) {
	cfg := Assemble(Selection{Moods: []string{"normal"}, YourKemonos: []string{"puppy"}})
	if !containsStr(cfg.Vars["affectionate_term"].Defaults, "puppy") {
		t.Errorf("puppy kemono should add your term, got %v", cfg.Vars["affectionate_term"].Defaults)
	}
}

func TestOriginalEmotesAndLove(t *testing.T) {
	cfg := Assemble(Selection{Moods: []string{"normal"}})
	if !containsStr(cfg.Vars["emote"].Defaults, "❤️") {
		t.Errorf("missing original emote, got %v", cfg.Vars["emote"].Defaults)
	}
	if !containsStr(cfg.Vars["love"].Defaults, "adore") {
		t.Errorf("missing original love words, got %v", cfg.Vars["love"].Defaults)
	}
}

func TestWriteSetupWritesResponses(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("AppData", cfg) // os.UserConfigDir on Windows
	dir := filepath.Join(cfg, "misa")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "companion.yaml"), []byte("enabled: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := WriteSetup(CompanionConfig{Moods: []string{"normal"}})
	if err != nil {
		t.Fatal(err)
	}
	resp := filepath.Join(dir, "responses.yaml")
	data, err := os.ReadFile(resp)
	if err != nil {
		t.Fatalf("responses.yaml not written: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "*pets your head*") {
		t.Errorf("responses.yaml missing assembled lines:\n%s", body)
	}
	if !strings.Contains(body, "item:") {
		t.Errorf("responses.yaml missing item var:\n%s", body)
	}
}

func TestHasThirsty(t *testing.T) {
	if HasThirsty(nil) || HasThirsty([]string{"normal"}) {
		t.Error("expected no thirsty")
	}
	if !HasThirsty([]string{"normal", "thirsty"}) {
		t.Error("expected thirsty")
	}
}

func moodBlob(c *config) string {
	var b strings.Builder
	for _, mc := range c.Moods {
		for _, l := range mc.Positive {
			b.WriteString(l)
			b.WriteByte('\n')
		}
		for _, l := range mc.Negative {
			b.WriteString(l)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func containsStr(vals []string, want string) bool {
	for _, v := range vals {
		if v == want {
			return true
		}
	}
	return false
}
