package companion

type Selection struct {
	Moods             []string
	YourPronouns      []string
	CompanionPronouns []string
	YourTerms         []string
	CompanionTerms    []string
	PrefixPositive    []string
	PrefixNegative    []string
	Items             []string
	YourParts         []string
	CompanionParts    []string
	Emotes            []string
	Love              []string
	YourKemonos       []string
	CompanionKemonos  []string
	Modifiers         []string
}

type Piece struct {
	ID    string
	Label string
	Desc  string
	Group string
	Terms []string
}

type lineBlock struct {
	Positive []string `yaml:"positive"`
	Negative []string `yaml:"negative"`
}

type moodFile struct {
	ID       string   `yaml:"id"`
	Label    string   `yaml:"label"`
	Desc     string   `yaml:"desc"`
	NSFW     bool     `yaml:"nsfw"`
	Positive []string `yaml:"positive"`
	Negative []string `yaml:"negative"`
}

type whenBlock struct {
	With  []string             `yaml:"with"`
	Lines map[string]lineBlock `yaml:"lines"`
}

type modifierFile struct {
	ID               string               `yaml:"id"`
	Label            string               `yaml:"label"`
	Desc             string               `yaml:"desc"`
	Group            string               `yaml:"group"`
	CompanionTerms   []string             `yaml:"companion_terms"`
	YourTerms        []string             `yaml:"your_terms"`
	Items            []string             `yaml:"items"`
	PrefixPositive   []string             `yaml:"prefix_positive"`
	PrefixNegative   []string             `yaml:"prefix_negative"`
	ThirstyYourTerms []string             `yaml:"thirsty_your_terms"`
	Emotes           []string             `yaml:"emotes"`
	Lines            map[string]lineBlock `yaml:"lines"`
	When             []whenBlock          `yaml:"when"`
}

type kemonoFile struct {
	ID             string               `yaml:"id"`
	Label          string               `yaml:"label"`
	Desc           string               `yaml:"desc"`
	Kind           string               `yaml:"kind"`
	YourTerms      []string             `yaml:"your_terms"`
	CompanionTerms []string             `yaml:"companion_terms"`
	Emotes         []string             `yaml:"emotes"`
	PrefixPositive []string             `yaml:"prefix_positive"`
	Sfx            string               `yaml:"sfx"`
	Lines          map[string]lineBlock `yaml:"lines"`
}

type termEntry struct {
	ID    string   `yaml:"id"`
	Label string   `yaml:"label"`
	Desc  string   `yaml:"desc"`
	Group string   `yaml:"group"`
	Terms []string `yaml:"terms"`
	Forms string   `yaml:"forms"`
}

type partEntry struct {
	ID      string `yaml:"id"`
	Label   string `yaml:"label"`
	Group   string `yaml:"group"`
	Desc    string `yaml:"desc"`
	Default bool   `yaml:"default"`
}

type catalogFile struct {
	Pronouns       []termEntry `yaml:"pronouns"`
	YourTerms      []termEntry `yaml:"your_terms"`
	CompanionTerms []termEntry `yaml:"companion_terms"`
	PrefixPositive []string    `yaml:"prefix_positive"`
	PrefixNegative []string    `yaml:"prefix_negative"`
	ThirstyTerms   []string    `yaml:"thirsty_terms"`
	Items          []string    `yaml:"items"`
	Parts          []partEntry `yaml:"parts"`
	Emotes         []string    `yaml:"emotes"`
	Love           []string    `yaml:"love"`
}
