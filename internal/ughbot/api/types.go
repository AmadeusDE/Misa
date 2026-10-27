package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/AmadeusDE/Misa/internal/ughbot/config"
)

var (
	ErrUnknownBot = errors.New("unknown bot")
	ErrBotExists  = errors.New("bot already exists")
	ErrReadOnly   = errors.New("bot is defined in ughbot.toml; edit that file")
	ErrInvalid    = errors.New("invalid bot")
	ErrNotPending = errors.New("approval is not pending")
	ErrNoApproval = errors.New("no such approval")
	ErrNoRoutine  = errors.New("no such routine")
	ErrBadRoutine = errors.New("invalid routine")
)

type BotInfo struct {
	Name  string `json:"name"`
	State string `json:"state"`
	Model string `json:"model,omitempty"`
	// Capabilities summarises what the bot may do (config.Capabilities.String).
	Capabilities string `json:"capabilities,omitempty"`
	// Editable is true for bots made in the UI; bots from ughbot.toml are changed in that file.
	Editable bool `json:"editable,omitempty"`
}

type Status struct {
	Version       string    `json:"version"`
	UptimeSeconds int64     `json:"uptime_seconds"`
	Bots          []BotInfo `json:"bots"`
	// VM describes the machines code tasks run in: whether a base is prepared, and what runs.
	VM string `json:"vm,omitempty"`
}

type CostQuery struct {
	Bot  string `json:"bot,omitempty"`
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

type CostRow struct {
	Bot string  `json:"bot"`
	Day string  `json:"day"`
	USD float64 `json:"usd"`
}

type CostReport struct {
	Rows []CostRow `json:"rows"`
}

type SendRequest struct {
	Text string `json:"text"`
}

type SendResponse struct {
	Ticket string `json:"ticket"`
}

type Event struct {
	Seq  uint64          `json:"seq"`
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

type ErrorBody struct {
	Error string `json:"error"`
}

type StatusError struct {
	Code    int
	Message string
}

func (e *StatusError) Error() string { return fmt.Sprintf("ughbot: %s (HTTP %d)", e.Message, e.Code) }

func DefaultSocketPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "ughbot.sock")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("ughbot-%d.sock", os.Getuid()))
}

// BotDef is a bot's full definition as the UI edits it.
type BotDef = config.Bot

// ApprovalInfo is one request in the approval queue. Args is the exact action that runs on approval.
type ApprovalInfo struct {
	ID        int64           `json:"id"`
	Bot       string          `json:"bot"`
	Action    string          `json:"action"`
	Summary   string          `json:"summary"`
	Args      json.RawMessage `json:"args"`
	State     string          `json:"state"`
	Created   time.Time       `json:"created"`
	Expires   time.Time       `json:"expires"`
	Decided   time.Time       `json:"decided,omitzero"`
	DecidedBy string          `json:"decided_by,omitempty"`
	Result    string          `json:"result,omitempty"`
}

// DecideRequest names who approved or denied, for the record ("cli", "web:<device>").
type DecideRequest struct {
	By string `json:"by"`
}

// RoutineRequest asks for a routine. At is text ("in 2h", "2006-01-02 15:04", RFC 3339) read
// against the daemon's clock; give it or Cron. Missed is "once" (default) or "skip".
type RoutineRequest struct {
	Bot    string `json:"bot"`
	Prompt string `json:"prompt"`
	Cron   string `json:"cron,omitempty"`
	At     string `json:"at,omitempty"`
	TZ     string `json:"tz,omitempty"`
	Missed string `json:"missed,omitempty"`
	// Owner names who asked, for the record ("cli", "tui", "web:<device>"); bots are named by the daemon.
	Owner string `json:"owner,omitempty"`
}

// RoutineInfo is one routine: it mails Prompt to Bot at each fire.
type RoutineInfo struct {
	ID      int64     `json:"id"`
	Owner   string    `json:"owner"`
	Bot     string    `json:"bot"`
	Prompt  string    `json:"prompt"`
	Cron    string    `json:"cron,omitempty"`
	At      time.Time `json:"at,omitzero"`
	TZ      string    `json:"tz,omitempty"`
	Missed  string    `json:"missed"`
	State   string    `json:"state"`
	Next    time.Time `json:"next,omitzero"`
	Last    time.Time `json:"last,omitzero"`
	Fired   int       `json:"fired"`
	Created time.Time `json:"created"`
}
