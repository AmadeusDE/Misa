package tui

import (
	_ "embed"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/ughbot/config"
)

//go:embed presets/bot-manager.md
var botManagerPrompt string

// botManagerPreset fills the new-bot form for a first bot that creates and organizes the others.
// It only prefills the form: nothing exists until the user picks a name and model and saves.
func botManagerPreset() api.BotDef {
	return api.BotDef{
		PromptText: botManagerPrompt,
		Spend:      config.Spend{Daily: 1, Monthly: 20},
		Capabilities: config.Capabilities{
			Message:    []string{"*"},
			Manage:     []string{"*"},
			CreateBots: true,
		},
	}
}
