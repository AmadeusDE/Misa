package imgview

import (
	"context"
	"sync"
)

// Request carries an owner-supplied generation so consumers can discard stale
// results. Visible requests are dequeued before queued prefetch requests.
type Request struct {
	Context    context.Context
	Path       string
	Limits     Limits
	Generation uint64
	Visible    bool
}
type Result struct {
	Request Request
	Image   Decoded
	Err     error
}

// Loader bounds both concurrent decodes and queued requests. Results must be
// drained or Close called. Submit returns false rather than blocking on load.
type Loader struct {
	mu       sync.Mutex
	cond     *sync.Cond
	queue    []Request
	capacity int
	closed   bool
	done     chan struct{}
	results  chan Result
	wg       sync.WaitGroup
}

func NewLoader(workers, capacity int) *Loader {
	if workers < 1 {
		workers = DefaultWorkers()
	}
	if workers > 32 {
		workers = 32
	}
	if capacity < 1 {
		capacity = 1
	}
	if capacity > 4096 {
		capacity = 4096
	}
	l := &Loader{capacity: capacity, done: make(chan struct{}), results: make(chan Result, capacity)}
	l.cond = sync.NewCond(&l.mu)
	l.wg.Add(workers)
	for range workers {
		go l.worker()
	}
	return l
}
func (l *Loader) Results() <-chan Result { return l.results }
func (l *Loader) Submit(r Request) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || len(l.queue) >= l.capacity {
		return false
	}
	if r.Context == nil {
		r.Context = context.Background()
	}
	l.queue = append(l.queue, r)
	l.cond.Signal()
	return true
}
func (l *Loader) worker() {
	defer l.wg.Done()
	for {
		l.mu.Lock()
		for len(l.queue) == 0 && !l.closed {
			l.cond.Wait()
		}
		if l.closed {
			l.mu.Unlock()
			return
		}
		i := 0
		for j := range l.queue {
			if l.queue[j].Visible {
				i = j
				break
			}
		}
		r := l.queue[i]
		copy(l.queue[i:], l.queue[i+1:])
		l.queue[len(l.queue)-1] = Request{}
		l.queue = l.queue[:len(l.queue)-1]
		l.mu.Unlock()
		img, err := DecodeContext(r.Context, r.Path, r.Limits)
		if r.Context.Err() != nil {
			continue
		}
		select {
		case l.results <- Result{r, img, err}:
		case <-l.done:
			return
		case <-r.Context.Done():
		}
	}
}

// Close waits for in-flight codecs, which cannot be forcibly interrupted.
func (l *Loader) Close() {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return
	}
	l.closed = true
	l.queue = nil
	close(l.done)
	l.cond.Broadcast()
	l.mu.Unlock()
	l.wg.Wait()
	close(l.results)
}
