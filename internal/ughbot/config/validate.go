package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var botName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// Validate returns every problem found, joined, so a config can be fixed in one pass.
func (c *Config) Validate() error {
	var errs []error
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	c.checkWeb(add)
	c.VM.check(add)
	for _, name := range c.BotNames() {
		c.checkBot(name, c.Bots[name], add)
	}
	return errors.Join(errs...)
}

func (c *Config) checkBot(name string, b Bot, add func(string, ...any)) {
	if !botName.MatchString(name) {
		add("bot %q: name must be lowercase letters, digits, - or _", name)
	}
	if b.Model == "" {
		add("bot %q: model is required", name)
	}
	if b.Spend.Daily < 0 || b.Spend.Monthly < 0 {
		add("bot %q: spend must not be negative", name)
	}
	if b.Spend.Daily > 0 && b.Spend.Monthly > 0 && b.Spend.Daily > b.Spend.Monthly {
		add("bot %q: daily spend %.2f exceeds monthly %.2f", name, b.Spend.Daily, b.Spend.Monthly)
	}
	if b.CompactAt < 0 {
		add("bot %q: compact_at must not be negative", name)
	}
	caps := b.Capabilities
	if caps.Files != "" && caps.Files != FilesRead && caps.Files != FilesWrite {
		add("bot %q: files must be %q, %q or empty, not %q", name, FilesRead, FilesWrite, caps.Files)
	}
	if caps.Tasks < 0 {
		add("bot %q: tasks must not be negative", name)
	}
	checkBotList(name, "message", caps.Message, add)
	checkBotList(name, "manage", caps.Manage, add)
	c.checkCodeDirs(name, caps.Code, add)
}

func checkBotList(bot, kind string, list []string, add func(string, ...any)) {
	for _, d := range duplicates(list) {
		add("bot %q: duplicate %s entry %q", bot, kind, d)
	}
	for _, g := range list {
		if g != "*" && !botName.MatchString(g) {
			add("bot %q: %s entry %q is not a bot name or *", bot, kind, g)
		}
	}
}

// CheckReferences reports the bots named in the given bots' message and manage lists that do not
// exist. It runs when a definition is written, not on load: deleting a bot must not stop others loading.
func (c *Config) CheckReferences(names ...string) error {
	var errs []error
	for _, name := range names {
		caps := c.Bots[name].Capabilities
		for kind, list := range map[string][]string{"message": caps.Message, "manage": caps.Manage} {
			for _, g := range list {
				if _, ok := c.Bots[g]; g != "*" && !ok {
					errs = append(errs, fmt.Errorf("bot %q: %s names unknown bot %q", name, kind, g))
				}
			}
		}
	}
	return errors.Join(errs...)
}

func duplicates(list []string) []string {
	seen := map[string]bool{}
	var dup []string
	for _, s := range list {
		if seen[s] {
			dup = append(dup, s)
		}
		seen[s] = true
	}
	return dup
}

func (c *Config) checkWeb(add func(string, ...any)) {
	w := c.Web
	if w.Port < 0 || w.Port > 65535 {
		add("web: port %d out of range", w.Port)
	}
	if w.Command != nil && (len(w.Command) == 0 || strings.TrimSpace(w.Command[0]) == "") {
		add("web: command must be a non-empty argv array whose first element names a program")
	}
}
