package sshclient

import (
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeSize struct {
	mu   sync.Mutex
	w, h int
	err  error
}

func (f *fakeSize) set(w, h int, err error) {
	f.mu.Lock()
	f.w, f.h, f.err = w, h, err
	f.mu.Unlock()
}

func (f *fakeSize) read() (int, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.w, f.h, f.err
}

func collect(t *testing.T, ch <-chan [2]int, want [2]int) {
	t.Helper()
	select {
	case got := <-ch:
		if got != want {
			t.Fatalf("applied %v, want %v", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("no resize applied, want %v", want)
	}
}

func TestWatchSizePollsWithoutEvents(t *testing.T) {
	src := &fakeSize{w: 80, h: 24}
	applied := make(chan [2]int, 8)
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		watchSize(src.read, func(w, h int) { applied <- [2]int{w, h} }, nil, time.Millisecond, 80, 24, done)
		close(finished)
	}()

	select {
	case got := <-applied:
		t.Fatalf("unchanged size applied %v", got)
	case <-time.After(30 * time.Millisecond):
	}
	src.set(100, 30, nil)
	collect(t, applied, [2]int{100, 30})

	src.set(0, 0, errors.New("not a terminal"))
	select {
	case got := <-applied:
		t.Fatalf("read error applied %v", got)
	case <-time.After(30 * time.Millisecond):
	}
	src.set(100, 30, nil)
	select {
	case got := <-applied:
		t.Fatalf("size equal to the last applied was applied again: %v", got)
	case <-time.After(30 * time.Millisecond):
	}

	close(done)
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("watchSize ignored done")
	}
}

func TestWatchSizeReactsToEventBeforePoll(t *testing.T) {
	src := &fakeSize{w: 80, h: 24}
	applied := make(chan [2]int, 1)
	events := make(chan struct{}, 1)
	done := make(chan struct{})
	defer close(done)
	go watchSize(src.read, func(w, h int) { applied <- [2]int{w, h} }, events, time.Hour, 80, 24, done)

	src.set(120, 40, nil)
	events <- struct{}{}
	collect(t, applied, [2]int{120, 40})
}
