package term

import "sync"

// inputQueue feeds the PTY from its own goroutine. A program that has stopped
// reading its terminal (busy, or in raw mode) fills the tty buffer and makes
// writes block, which must stall neither the UI (keys, pastes) nor the reader
// (query replies).
type inputQueue struct {
	mu             sync.Mutex
	pending        [][]byte
	pendingReplies int // reply bytes in pending: capped, unlike keys and pastes
	wake           chan struct{}
	start          sync.Once
	dead           bool
}

// maxPendingReplies bounds query replies waiting for a program that has
// stopped reading; replies past it are dropped.
const maxPendingReplies = 64 << 10

// enqueueReply queues a query reply unless too many are already waiting.
func (t *Terminal) enqueueReply(data []byte) {
	q := &t.input
	q.mu.Lock()
	full := q.pendingReplies+len(data) > maxPendingReplies
	if !full {
		q.pendingReplies += len(data)
	}
	q.mu.Unlock()
	if !full {
		t.enqueue(data)
	}
}

// enqueue schedules data for the program's stdin, in order.
func (t *Terminal) enqueue(data []byte) {
	q := &t.input
	q.start.Do(func() {
		q.wake = make(chan struct{}, 1)
		go t.drainInput()
	})
	q.mu.Lock()
	if !q.dead {
		q.pending = append(q.pending, append([]byte(nil), data...))
	}
	q.mu.Unlock()
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func (t *Terminal) drainInput() {
	q := &t.input
	for {
		select {
		case <-q.wake:
		case <-t.exited:
			q.stop()
			return
		}
		q.mu.Lock()
		batch, replies := q.pending, q.pendingReplies
		q.pending = nil
		q.mu.Unlock()
		for _, b := range batch {
			if _, err := t.conn.Write(b); err != nil {
				q.stop()
				return
			}
		}
		q.mu.Lock()
		q.pendingReplies -= replies // written: those replies no longer count
		q.mu.Unlock()
	}
}

func (q *inputQueue) stop() {
	q.mu.Lock()
	q.dead = true
	q.pending = nil
	q.pendingReplies = 0
	q.mu.Unlock()
}
