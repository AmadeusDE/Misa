package webui

import (
	"net/netip"
	"sync"
	"time"
)

// limiter locks a key (and, with a global cap, everyone) out after too many failures
// inside a sliding window.
type limiter struct {
	mu        sync.Mutex
	perKey    map[string][]time.Time
	global    []time.Time
	keyMax    int
	globalMax int
	window    time.Duration
}

// newLimiter allows keyMax failures per client and globalMax in all (0: no
// global cap) within window.
func newLimiter(keyMax, globalMax int, window time.Duration) *limiter {
	return &limiter{perKey: map[string][]time.Time{}, keyMax: keyMax, globalMax: globalMax, window: window}
}

func (l *limiter) prune(ts []time.Time, now time.Time) []time.Time {
	i := 0
	for i < len(ts) && now.Sub(ts[i]) >= l.window {
		i++
	}
	return ts[i:]
}

func (l *limiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.global = l.prune(l.global, now)
	k := l.prune(l.perKey[key], now)
	if len(k) == 0 {
		delete(l.perKey, key)
	} else {
		l.perKey[key] = k
	}
	return len(k) < l.keyMax && (l.globalMax == 0 || len(l.global) < l.globalMax)
}

func (l *limiter) fail(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.perKey[key] = append(l.prune(l.perKey[key], now), now)
	if l.globalMax > 0 {
		l.global = append(l.prune(l.global, now), now)
	}
}

func (l *limiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.perKey, key)
}

// clientKey is what limits count against: the address, or for IPv6 its /64,
// since one host usually holds a whole /64.
func clientKey(ip string) string {
	a, err := netip.ParseAddr(ip)
	if err != nil || !a.Is6() || a.Is4In6() {
		return ip
	}
	p, _ := a.Prefix(64)
	return p.String()
}
