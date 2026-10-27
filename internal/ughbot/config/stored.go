package config

import (
	"encoding/json"
	"slices"
)

// legacyBot is how bots made in the UI were stored before capabilities: Go field names,
// a tools allowlist and grants.
type legacyBot struct {
	Model, Prompt, PromptText, Workdir string
	Tools                              []string
	CompactAt                          int
	Spend                              struct{ Daily, Monthly float64 }
	Grants                             struct{ Message, Manage []string }
}

// DecodeStored reads a bot definition from the database, converting the older stored shape.
func DecodeStored(raw []byte) (Bot, error) {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		return Bot{}, err
	}
	if _, legacy := keys["Model"]; !legacy {
		var b Bot
		err := json.Unmarshal(raw, &b)
		return b, err
	}
	var l legacyBot
	if err := json.Unmarshal(raw, &l); err != nil {
		return Bot{}, err
	}
	b := Bot{Model: l.Model, Prompt: l.Prompt, PromptText: l.PromptText, Workdir: l.Workdir, CompactAt: l.CompactAt,
		Spend: Spend{Daily: l.Spend.Daily, Monthly: l.Spend.Monthly}}
	b.Capabilities = Capabilities{Message: l.Grants.Message, Manage: l.Grants.Manage,
		Shell: slices.Contains(l.Tools, "bash")}
	switch {
	case slices.ContainsFunc(l.Tools, func(t string) bool { return t == "write" || t == "edit" }):
		b.Capabilities.Files = FilesWrite
	case slices.ContainsFunc(l.Tools, func(t string) bool { return t == "read" || t == "ls" || t == "grep" || t == "find" }):
		b.Capabilities.Files = FilesRead
	}
	if slices.Contains(l.Tools, "start_task") {
		b.Capabilities.Tasks = 3
	}
	return b, nil
}
