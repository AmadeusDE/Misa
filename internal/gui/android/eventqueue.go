package android

import "github.com/AmadeusDE/Misa/internal/gui"

// overflow runs under b.mu. Collapse lifecycle state before dropping input:
// losing a blur or invalidate would leave hidden playback or a blank surface.
func (b *Backend) overflow(ev gui.Event) {
	pending := make([]gui.Event, 0, cap(b.events)+1)
drain:
	for {
		select {
		case old := <-b.events:
			pending = append(pending, old)
		default:
			break drain
		}
	}
	pending = append(pending, ev)
	var seen [3]bool
	for i := len(pending) - 1; i >= 0; i-- {
		if slot := lifecycleSlot(pending[i].Kind); slot >= 0 {
			if seen[slot] {
				pending = append(pending[:i], pending[i+1:]...)
			} else {
				seen[slot] = true
			}
		}
	}
	if len(pending) > cap(b.events) {
		for i, old := range pending {
			if lifecycleSlot(old.Kind) < 0 {
				pending = append(pending[:i], pending[i+1:]...)
				break
			}
		}
	}
	// Producers share b.mu; consumers can only free more room. The bounded
	// batch therefore fits even if the GUI drains it while it is restored.
	for _, old := range pending {
		b.events <- old
	}
}

func lifecycleSlot(kind gui.EventKind) int {
	switch kind {
	case gui.EventFocus:
		return 0
	case gui.EventInvalidate:
		return 1
	case gui.EventClose:
		return 2
	default:
		return -1
	}
}
