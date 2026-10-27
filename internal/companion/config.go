package companion

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type CompanionConfig struct {
	Enabled           *bool    `yaml:"enabled,omitempty"`
	Moods             []string `yaml:"moods,omitempty"`
	YourPronouns      []string `yaml:"your_pronouns,omitempty"`
	CompanionPronouns []string `yaml:"companion_pronouns,omitempty"`
	YourTerms         []string `yaml:"your_terms,omitempty"`
	CompanionTerms    []string `yaml:"companion_terms,omitempty"`
	PrefixPositive    []string `yaml:"prefix_positive,omitempty"`
	PrefixNegative    []string `yaml:"prefix_negative,omitempty"`
	Items             []string `yaml:"items,omitempty"`
	YourParts         []string `yaml:"your_parts,omitempty"`
	CompanionParts    []string `yaml:"companion_parts,omitempty"`
	Parts             []string `yaml:"parts,omitempty"`
	Emotes            []string `yaml:"emotes,omitempty"`
	Love              []string `yaml:"love,omitempty"`
	YourKemonos       []string `yaml:"your_kemonos,omitempty"`
	CompanionKemonos  []string `yaml:"companion_kemonos,omitempty"`
	Modifiers         []string `yaml:"modifiers,omitempty"`

	Preset           string   `yaml:"preset,omitempty"`
	Mood             []string `yaml:"mood,omitempty"`
	Role             []string `yaml:"role,omitempty"`
	Pronoun          []string `yaml:"pronoun,omitempty"`
	AffectionateTerm []string `yaml:"affectionate_term,omitempty"`
	DenigratingTerm  []string `yaml:"denigrating_term,omitempty"`
	Part             []string `yaml:"part,omitempty"`
	Emote            []string `yaml:"emote,omitempty"`
	Good             []string `yaml:"good,omitempty"`
	Diminutive       []string `yaml:"diminutive,omitempty"`
	ResponsesFile    string   `yaml:"responses_file,omitempty"`
}

func (mc CompanionConfig) Selection() Selection {
	moods := mc.Moods
	if len(moods) == 0 {
		moods = mc.Mood
	}
	compPronouns := mc.CompanionPronouns
	if len(compPronouns) == 0 {
		compPronouns = mc.Pronoun
	}
	yourTerms := mc.YourTerms
	if len(yourTerms) == 0 {
		yourTerms = mc.AffectionateTerm
	}
	compTerms := mc.CompanionTerms
	if len(compTerms) == 0 {
		compTerms = mc.Role
	}
	pos := mc.PrefixPositive
	if len(pos) == 0 {
		pos = append(append([]string{}, mc.Good...), mc.Diminutive...)
	}
	emotes := mc.Emotes
	if len(emotes) == 0 {
		emotes = mc.Emote
	}
	yourParts := mc.YourParts
	compParts := mc.CompanionParts
	if len(yourParts) == 0 {
		yourParts = mc.Parts
	}
	if len(compParts) == 0 {
		compParts = mc.Parts
	}
	if len(yourParts) == 0 {
		yourParts = mc.Part
	}
	if len(compParts) == 0 {
		compParts = mc.Part
	}
	return Selection{
		Moods:             moods,
		YourPronouns:      mc.YourPronouns,
		CompanionPronouns: compPronouns,
		YourTerms:         yourTerms,
		CompanionTerms:    compTerms,
		PrefixPositive:    pos,
		PrefixNegative:    mc.PrefixNegative,
		Items:             mc.Items,
		YourParts:         yourParts,
		CompanionParts:    compParts,
		Emotes:            emotes,
		Love:              mc.Love,
		YourKemonos:       mc.YourKemonos,
		CompanionKemonos:  mc.CompanionKemonos,
		Modifiers:         mc.Modifiers,
	}
}

func companionConfigPath() string { return findFile("companion.yaml") }

// findFile looks for name next to the executable, then in ~/.config/misa,
// never in the working directory.
func findFile(name string) string {
	if execPath, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(execPath), name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if dir, err := os.UserConfigDir(); err == nil {
		p := filepath.Join(dir, "misa", name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func companionConfigSavePath() string {
	p := companionConfigPath()
	if p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "misa", "companion.yaml")
}

func LoadCompanionConfig() CompanionConfig {
	path := companionConfigPath()
	if path == "" {
		return CompanionConfig{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return CompanionConfig{}
	}
	var mc CompanionConfig
	yaml.Unmarshal(data, &mc)
	return mc
}

func SaveCompanionConfig(mc CompanionConfig) error {
	path := companionConfigSavePath()
	if path == "" {
		return os.ErrNotExist
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(&mc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func Enabled() bool {
	mc := LoadCompanionConfig()
	return mc.Enabled == nil || *mc.Enabled
}

func WriteSetup(mc CompanionConfig) error {
	savePath := companionConfigSavePath()
	if savePath == "" {
		return os.ErrNotExist
	}
	respPath := filepath.Join(filepath.Dir(savePath), "responses.yaml")
	mc.ResponsesFile = respPath
	if err := SaveCompanionConfig(mc); err != nil {
		return err
	}
	assembled := Assemble(mc.Selection())
	data, err := yaml.Marshal(assembled)
	if err != nil {
		return err
	}
	return os.WriteFile(respPath, data, 0o644)
}

func findResponsesOverride() string { return findFile("responses.yaml") }

func (mc CompanionConfig) applyOverrides(c *config) {
	overrideVar := func(key string, vals []string) {
		if len(vals) > 0 {
			c.Vars[key] = varConfig{Defaults: vals}
		}
	}
	overrideVar("mood", mc.Moods)
	if len(mc.Moods) == 0 {
		overrideVar("mood", mc.Mood)
	}
	overrideVar("role", mc.CompanionTerms)
	if len(mc.CompanionTerms) == 0 {
		overrideVar("role", mc.Role)
	}
	overrideVar("pronoun", mc.CompanionPronouns)
	if len(mc.CompanionPronouns) == 0 {
		overrideVar("pronoun", mc.Pronoun)
	}
	overrideVar("you_pronoun", mc.YourPronouns)
	overrideVar("affectionate_term", mc.YourTerms)
	if len(mc.YourTerms) == 0 {
		overrideVar("affectionate_term", mc.AffectionateTerm)
	}
	overrideVar("denigrating_term", mc.DenigratingTerm)
	overrideVar("your_part", mc.YourParts)
	overrideVar("companion_part", mc.CompanionParts)
	if len(mc.CompanionParts) == 0 {
		overrideVar("companion_part", mc.Parts)
		if len(mc.Parts) == 0 {
			overrideVar("companion_part", mc.Part)
		}
	}
	overrideVar("item", mc.Items)
	overrideVar("emote", mc.Emotes)
	if len(mc.Emotes) == 0 {
		overrideVar("emote", mc.Emote)
	}
	overrideVar("good", mc.PrefixPositive)
	if len(mc.PrefixPositive) == 0 {
		overrideVar("good", mc.Good)
	}
	overrideVar("prefix_negative", mc.PrefixNegative)
	overrideVar("love", mc.Love)
	overrideVar("diminutive", mc.Diminutive)
}
