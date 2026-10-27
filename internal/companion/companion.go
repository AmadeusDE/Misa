// Package companion provides encouraging/motivational messages after command execution.
// Response templates derived from cargo-mommy (MIT/Apache-2.0) and fwdekker/mommy (Unlicense).
// See NOTICE file for full attribution.
package companion

import (
	"math/rand"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type varConfig struct {
	Defaults []string `yaml:"defaults"`
}

type moodConfig struct {
	Positive []string `yaml:"positive"`
	Negative []string `yaml:"negative"`
}

type config struct {
	Vars  map[string]varConfig  `yaml:"vars"`
	Moods map[string]moodConfig `yaml:"moods"`
}

func loadConfig() *config {
	mc := LoadCompanionConfig()
	c := Assemble(mc.Selection())

	if path := findResponsesOverride(); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			var user config
			if yaml.Unmarshal(data, &user) == nil {
				if user.Vars != nil {
					if c.Vars == nil {
						c.Vars = map[string]varConfig{}
					}
					for k, v := range user.Vars {
						c.Vars[k] = v
					}
				}
				if user.Moods != nil {
					if c.Moods == nil {
						c.Moods = map[string]moodConfig{}
					}
					for k, v := range user.Moods {
						c.Moods[k] = v
					}
				}
			}
		}
	}

	mc.applyOverrides(c)
	return c
}

func Respond(exitCode int) string {
	if !Enabled() {
		return ""
	}

	cfg := loadConfig()
	negative := exitCode != 0

	mood := getVar(cfg, "mood")
	if mood == "" {
		mood = "normal"
	}
	mood = normalizeMood(mood)
	mc, ok := cfg.Moods[mood]
	if !ok {
		mc = cfg.Moods["normal"]
	}

	var templates []string
	if !negative {
		templates = mc.Positive
	} else {
		templates = mc.Negative
	}
	if len(templates) == 0 {
		return ""
	}

	tmpl := templates[rand.Intn(len(templates))]
	result := resolve(cfg, tmpl, negative)

	if rand.Float32() < 0.5 {
		if emote := getVar(cfg, "emote"); emote != "" {
			choices := splitChoices(emote)
			if len(choices) > 0 {
				result += " " + choices[rand.Intn(len(choices))]
			}
		}
	}

	return result
}

func getVar(cfg *config, name string) string {
	if vc, ok := cfg.Vars[name]; ok && len(vc.Defaults) > 0 {
		return vc.Defaults[rand.Intn(len(vc.Defaults))]
	}
	return ""
}

func getVarList(cfg *config, name string) []string {
	if vc, ok := cfg.Vars[name]; ok {
		return vc.Defaults
	}
	return nil
}

func splitChoices(s string) []string {
	var out []string
	for _, c := range strings.Split(s, "/") {
		t := strings.TrimSpace(c)
		if t != "" && !strings.HasPrefix(t, "#") {
			out = append(out, t)
		}
	}
	return out
}

func choose(s string) string {
	choices := splitChoices(s)
	if len(choices) == 0 {
		return s
	}
	return choices[rand.Intn(len(choices))]
}

func pickList(vals []string, fallback string) string {
	if len(vals) == 0 {
		return fallback
	}
	return vals[rand.Intn(len(vals))]
}

func parsePronouns(raw, fallback string) (they, them, their, theirs, themself string) {
	they, them, their, theirs, themself = "they", "them", "their", "theirs", "themself"
	if fallback != "" {
		w := strings.Fields(fallback)
		if len(w) >= 5 {
			they, them, their, theirs, themself = w[0], w[1], w[2], w[3], w[4]
		}
	}
	if raw == "" {
		raw = fallback
	}
	choices := splitChoices(raw)
	chosen := fallback
	if len(choices) > 0 {
		chosen = choices[rand.Intn(len(choices))]
	}
	words := strings.Fields(chosen)
	if len(words) >= 5 {
		return words[0], words[1], words[2], words[3], words[4]
	}
	if len(words) >= 3 {
		they, them, their = words[0], words[1], words[2]
		theirs = their + "s"
		themself = them + "self"
		return they, them, their, theirs, themself
	}
	return they, them, their, theirs, themself
}

func resolve(cfg *config, tmpl string, negative bool) string {
	role := choose(getVar(cfg, "role"))
	if role == "" {
		role = "mommy"
	}

	pronounStr := getVar(cfg, "pronoun")
	they, them, their, theirs, themself := parsePronouns(pronounStr, "she her her hers herself")

	youRaw := getVar(cfg, "you_pronoun")
	youThey, youThem, youTheir, youTheirs, youThemself := parsePronouns(youRaw, "they them their theirs themself")

	sweetie := choose(getVar(cfg, "affectionate_term"))
	if sweetie == "" {
		sweetie = "girl"
	}
	denigrating := choose(getVar(cfg, "denigrating_term"))
	if denigrating == "" {
		denigrating = sweetie
	}

	item := choose(getVar(cfg, "item"))
	if item == "" {
		item = "hugs"
	}
	yourPart := choose(getVar(cfg, "your_part"))
	if yourPart == "" {
		yourPart = "cock"
	}
	compPart := choose(getVar(cfg, "companion_part"))
	if compPart == "" {
		compPart = choose(getVar(cfg, "part"))
	}
	if compPart == "" {
		compPart = "milk"
	}

	pos := getVarList(cfg, "prefix_positive")
	if len(pos) == 0 {
		pos = getVarList(cfg, "good")
	}
	neg := getVarList(cfg, "prefix_negative")
	good := pickList(pos, "good")
	prefix := good
	if negative {
		prefix = pickList(neg, "silly")
	}

	love := choose(getVar(cfg, "love"))
	if love == "" {
		love = "love"
	}

	dimList := getVarList(cfg, "diminutive")
	if len(dimList) == 0 {
		dimList = pos
	}
	if len(dimList) == 0 {
		dimList = []string{"cute", "little", "tiny", "precious"}
	}
	shuffled := append([]string(nil), dimList...)
	rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	dim1 := shuffled[0]
	dim2 := ""
	if len(shuffled) > 1 {
		dim2 = shuffled[1]
	}

	r := strings.NewReplacer(
		"{role}", role, "{caregiver}", role,
		"{affectionate_term}", sweetie, "{sweetie}", sweetie,
		"{denigrating_term}", denigrating,
		"{your_part}", yourPart, "{companion_part}", compPart, "{part}", compPart, "{item}", item,
		"{pronoun}", them,
		"{they}", they, "{them}", them, "{their}", their, "{theirs}", theirs, "{themself}", themself,
		"{you_they}", youThey, "{you_them}", youThem, "{you_their}", youTheir,
		"{you_theirs}", youTheirs, "{you_themself}", youThemself,
		"{good}", good, "{prefix}", prefix, "{love}", love,
		"{diminutive}", dim1, "{diminutive1}", dim1, "{diminutive2}", dim2,
	)
	res := r.Replace(tmpl)
	res = strings.ReplaceAll(res, "  ", " ")
	return res
}
