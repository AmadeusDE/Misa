package bot

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
)

const (
	ringSize  = 4096
	subBuffer = 256
)

// feed numbers a bot's events, remembers the latest for resuming subscribers and
// fans them out. Seq starts at the wall clock in milliseconds so it keeps rising
// across daemon restarts.
type feed struct {
	mu   sync.Mutex
	seq  uint64
	ring []api.Event
	subs map[*sub]struct{}
}

type sub struct {
	ch     chan api.Event
	closed bool
}

func newFeed() *feed {
	return &feed{seq: uint64(time.Now().UnixMilli()), subs: map[*sub]struct{}{}}
}

func (f *feed) publish(typ string, data any) {
	raw, _ := json.Marshal(data)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	ev := api.Event{Seq: f.seq, Type: typ, Data: raw}
	if len(f.ring) == ringSize {
		f.ring = append(f.ring[:0], f.ring[1:]...)
	}
	f.ring = append(f.ring, ev)
	for s := range f.subs {
		select {
		case s.ch <- ev:
		default: // a reader that cannot keep up is dropped; it resumes by Last-Event-ID
			f.drop(s)
		}
	}
}

func (f *feed) drop(s *sub) {
	if !s.closed {
		s.closed = true
		close(s.ch)
		delete(f.subs, s)
	}
}

func (f *feed) subscribe(ctx context.Context, after uint64) <-chan api.Event {
	f.mu.Lock()
	var replay []api.Event
	for _, ev := range f.ring {
		if ev.Seq > after {
			replay = append(replay, ev)
		}
	}
	s := &sub{ch: make(chan api.Event, len(replay)+subBuffer)}
	for _, ev := range replay {
		s.ch <- ev
	}
	f.subs[s] = struct{}{}
	f.mu.Unlock()
	go func() {
		<-ctx.Done()
		f.mu.Lock()
		f.drop(s)
		f.mu.Unlock()
	}()
	return s.ch
}

func (f *feed) closeAll() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for s := range f.subs {
		f.drop(s)
	}
}
