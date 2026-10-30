package chess

import (
	"slices"
	"testing"
	"time"
)

func TestQueuedDeadlinePreservesEngineAndAllowsReuse(t *testing.T) {
	p := NewPosition(builtin(t, "chess"))
	e := NewEngine(p.V)
	previous := e.Think(p, nil, Limits{Depth: 2})
	e.mu.Lock()
	nodes, deadline, stop := e.nodes, e.deadline, e.stop
	path, history, table := slices.Clone(e.path), slices.Clone(e.history), slices.Clone(e.tt)
	killers := e.killers
	done := make(chan Result, 1)
	go func() {
		done <- e.ThinkLevel(p, nil, Hard, Limits{Time: 10 * time.Millisecond}, 0)
	}()
	var result Result
	select {
	case result = <-done:
	case <-time.After(time.Second):
		e.mu.Unlock()
		t.Fatal("expired waiter did not return while the shared engine was locked")
	}
	unchanged := e.nodes == nodes && e.deadline == deadline && e.stop == stop &&
		slices.Equal(e.path, path) && slices.Equal(e.history, history) &&
		slices.Equal(e.tt, table) && e.killers == killers
	e.mu.Unlock()
	if !unchanged {
		t.Fatal("expired waiter changed the active engine's search state")
	}
	if !result.Found || !slices.Contains(p.Moves(), result.Move) || result.Depth != 0 || result.Nodes != 0 || result.Score != 0 {
		t.Fatalf("expired waiter returned analysed or illegal fallback: %+v", result)
	}
	go func() { done <- e.Think(p, nil, Limits{Depth: 2, Time: time.Second}) }()
	select {
	case next := <-done:
		if !next.Found || next.Move != previous.Move || next.Score != previous.Score || next.Depth != previous.Depth {
			t.Fatalf("engine did not resume its previous policy: %+v, previous %+v", next, previous)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("subsequent search could not acquire the engine")
	}
}
