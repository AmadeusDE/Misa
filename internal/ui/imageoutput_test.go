package ui

import (
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/imageout"
)

func TestImageOutputFailsClosed(t *testing.T) {
	for _, optIn := range []bool{false, true} {
		probe := imageOutputProbe(optIn)
		now := time.Unix(100, 0)
		if _, send := probe.Begin(1, now); send {
			t.Fatalf("opt-in %v: unsupported root output must not send a query", optIn)
		}
		probe.Reply(1, true, now)
		if renderer, done := probe.Result(now); renderer != imageout.HalfBlock || !done {
			t.Fatalf("opt-in %v: got renderer %v, done %v; want final half-block", optIn, renderer, done)
		}
	}
}
