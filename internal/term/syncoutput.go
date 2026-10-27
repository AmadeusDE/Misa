package term

import (
	"bytes"
	"time"
)

const maxSyncPending = 4 * 1024 * 1024

// syncTimeout bounds how long a frame may stay open: a program that begins a
// frame and then waits for input must not freeze its pane.
var syncTimeout = 500 * time.Millisecond

var (
	syncBegin = []byte("\x1b[?2026h")
	syncEnd   = []byte("\x1b[?2026l")
)

// feedSync buffers DEC mode 2026 synchronized-output blocks so a frame is
// applied atomically, carrying partial markers across reads. The caller holds
// t.mu. It reports whether anything was applied to the screen.
func (t *Terminal) feedSync(data []byte) bool {
	t.syncPending = append(t.syncPending, data...)
	applied := false
	for {
		marker := syncBegin
		if t.synced {
			marker = syncEnd
		}
		if pos := bytes.Index(t.syncPending, marker); pos >= 0 {
			through := pos + len(marker)
			chunk := t.syncPending[:through]
			t.syncPending = append([]byte(nil), t.syncPending[through:]...)
			t.processVT(chunk)
			t.synced = !t.synced
			if t.synced {
				t.armSyncTimeout()
			}
			applied = applied || !t.synced || pos > 0 // a frame ended, or output preceded a begin
			continue
		}
		if t.synced {
			if len(t.syncPending) > maxSyncPending {
				chunk := t.syncPending
				t.syncPending = nil
				t.processVT(chunk)
				t.synced = false
				applied = true
			}
			return applied
		}
		keep := partialMarkerSuffix(t.syncPending, marker)
		flush := len(t.syncPending) - keep
		if flush > 0 {
			t.processVT(t.syncPending[:flush])
			applied = true
		}
		t.syncPending = append([]byte(nil), t.syncPending[flush:]...)
		return applied
	}
}

func (t *Terminal) armSyncTimeout() {
	t.syncGen++
	gen := t.syncGen
	time.AfterFunc(syncTimeout, func() { t.expireSync(gen) })
}

// expireSync shows a frame that is still open after syncTimeout.
func (t *Terminal) expireSync(gen uint64) {
	t.mu.Lock()
	if !t.synced || t.syncGen != gen {
		t.mu.Unlock()
		return
	}
	chunk := t.syncPending
	t.syncPending = nil
	t.synced = false
	t.processVT(chunk)
	t.mu.Unlock()
	t.markDirty()
}

func partialMarkerSuffix(buf, marker []byte) int {
	for n := len(marker) - 1; n > 0; n-- {
		if bytes.HasSuffix(buf, marker[:n]) {
			return n
		}
	}
	return 0
}
