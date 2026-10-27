// Package tui is the ughbot terminal UI: bot picker, chat, cost and status
// screens over the daemon API. It is an ordinary Bubble Tea program, so it runs
// in a terminal, a Misa pane or the web terminal unchanged.
package tui

import (
	"context"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
)

// Backend is the slice of api.Backend the UI uses; *client.Client satisfies it.
type Backend interface {
	ListBots(ctx context.Context) ([]api.BotInfo, error)
	Status(ctx context.Context) (api.Status, error)
	Cost(ctx context.Context, q api.CostQuery) ([]api.CostRow, error)
	Send(ctx context.Context, bot, text string) (string, error)
	Subscribe(ctx context.Context, bot string, afterSeq uint64) (<-chan api.Event, error)
}

// DeviceBackend is the web device administration the Devices screen needs;
// *client.Client satisfies it. A Backend without it gets an explanatory screen.
type DeviceBackend interface {
	WebDevices(ctx context.Context) ([]api.WebDevice, error)
	WebRevoke(ctx context.Context, idOrName string) error
	WebRename(ctx context.Context, idOrName, newName string) error
	WebSessions(ctx context.Context) ([]api.WebSession, error)
	WebKillSession(ctx context.Context, prefix string) error
	WebEvents(ctx context.Context, limit int) ([]api.WebEvent, error)
	WebNotices(ctx context.Context) ([]api.WebNotice, error)
	WebDismissNotice(ctx context.Context, id string) error
}
