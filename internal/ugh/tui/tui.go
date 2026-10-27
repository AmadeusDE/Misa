// Package tui is ugh's Bubble Tea interface (PLAN.md §7, M3): a scrolling
// transcript, a growing editor, a footer with model/thinking/perm
// mode/tokens/cwd, a spinner with elapsed time, streamed assistant text and
// compact tool lines (ctrl+o expands), a permission modal fed by the
// channel prompt broker, a model picker, and slash commands. All provider
// and agent work runs in goroutines that send tea.Msgs; nothing blocks in
// Update. It never assumes it owns the program: both cmd/ugh and embedding
// hosts run it through the public ughtui package.
package tui

import "github.com/AmadeusDE/Misa/internal/ugh/agent"

// Options configures one TUI launch.
type Options struct {
	Model    string // provider/model ref
	Thinking string // off|low|medium|high
	System   string
	Cwd      string
	Mode     string // permission mode override (manual|classify|auto)
	MaxTurns int    // root turn budget override (0: config agent.max_turns_root)
	Resume   bool   // resume the cwd's latest session
}

const (
	defaultModel  = "openrouter/xiaomi/mimo-v2.6-pro"
	defaultSystem = agent.DefaultSystem
)
