package api

import "context"

type Backend interface {
	ListBots(ctx context.Context) ([]BotInfo, error)
	Status(ctx context.Context) (Status, error)
	Cost(ctx context.Context, q CostQuery) ([]CostRow, error)
	Send(ctx context.Context, bot, text string) (ticket string, err error)
	// Subscribe returns events with Seq > afterSeq, then live ones. The backend
	// closes the channel when ctx ends. It must not block on a slow reader of
	// its own accord: dropping the subscription is its choice to make.
	Subscribe(ctx context.Context, bot string, afterSeq uint64) (<-chan Event, error)
	Stop(ctx context.Context) error
}
