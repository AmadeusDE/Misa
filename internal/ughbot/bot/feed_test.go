package bot

import (
	"context"
	"testing"
	"time"
)

func TestFeedSeqRisesAndReplaysAfter(t *testing.T) {
	f := newFeed()
	for i := 0; i < 3; i++ {
		f.publish("text", map[string]int{"i": i})
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	all := f.subscribe(ctx, 0)
	first := <-all
	second := <-all
	if second.Seq != first.Seq+1 {
		t.Fatalf("seq %d then %d", first.Seq, second.Seq)
	}
	resumed := f.subscribe(ctx, first.Seq)
	if ev := <-resumed; ev.Seq != second.Seq {
		t.Fatalf("resume from %d began at %d, want %d", first.Seq, ev.Seq, second.Seq)
	}
	f.publish("text", nil)
	<-all
	if ev := <-all; ev.Type != "text" {
		t.Fatalf("live event = %+v", ev)
	}
}

func TestFeedDropsSlowSubscriberWithoutBlocking(t *testing.T) {
	f := newFeed()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := f.subscribe(ctx, 0)
	done := make(chan struct{})
	go func() {
		for i := 0; i < subBuffer+10; i++ {
			f.publish("text", nil)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("publish blocked on a slow subscriber")
	}
	n := 0
	for range ch {
		n++
	}
	if n != subBuffer {
		t.Fatalf("read %d buffered events, want %d", n, subBuffer)
	}
}

func TestFeedClosesChannelWhenContextEnds(t *testing.T) {
	f := newFeed()
	ctx, cancel := context.WithCancel(context.Background())
	ch := f.subscribe(ctx, 0)
	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("unexpected event")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("channel not closed")
	}
}
