package aiconfig

var helpText = map[mode][]string{
	modeProviders: {
		"Providers",
		"  up/down, pgup/pgdn, home/end  move        a  add a provider from a preset or a custom one",
		"  enter or e  edit a custom provider        k  set the key (masked; pasting works)",
		"  K  remove the stored key                  d  delete a custom provider (asks first)",
		"  t  test: a tiny completion                m  list models, pick the default, chat or help model",
		"  D  defaults: models and web search        r  reload from disk      q or esc  leave",
		"",
		"Keys come from the provider's environment variable first, then ugh's credential store.",
		"Everything here is shared with ugh: providers in config.toml, keys in its store.",
		"codex is a ChatGPT login: run `ugh login codex` in a terminal.",
	},
	modeModels: {
		"Models",
		"  enter  make it ugh's default model       c  use it for chat       h  use it for command help",
		"  /  filter (enter keeps it, esc clears)   r  fetch the list again  esc  back",
	},
	modeDefaults: {
		"Defaults",
		"  enter  edit the value; on the search engine, switch between duckduckgo, searxng and off",
		"  x  clear the value                       esc  back",
		"Models are written provider/model. Chat and help fall back to the default model.",
	},
}

func helpView(from mode) ([]string, string) {
	text := helpText[from]
	if len(text) == 0 {
		return nil, "any key closes help"
	}
	lines := []string{titleSt.Render(text[0] + ": keys")}
	for _, l := range text[1:] {
		lines = append(lines, dimSt.Render(l))
	}
	return lines, "any key closes help"
}
