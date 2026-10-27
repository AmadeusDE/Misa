package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/config"
)

// muxConfigChanged tells the root, which draws the mux, about mux settings
// saved from a pane's settings or onboarding screen.
type muxConfigChanged struct{ cfg config.MuxConfig }

func muxConfigSaved(cfg config.MuxConfig) tea.Cmd {
	return func() tea.Msg { return muxConfigChanged{cfg} }
}

func (m *Model) applyMuxConfig(cfg config.MuxConfig) {
	m.muxBarReady = false
	m.muxAutoPane = config.BoolDefault(cfg.AutoPane, true)
	m.muxBarAutoHide = config.BoolDefault(cfg.BarAutoHide, true)
	m.muxBarPosition = cfg.BarPosition
	m.muxBarInfo = parseMuxBarInfo(cfg.BarInfo)
	m.muxFocusFollowsMouse = config.BoolDefault(cfg.FocusFollowsMouse, true)
}
