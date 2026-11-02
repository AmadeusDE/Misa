// Package imageout contains output-boundary image presentation contracts.
package imageout

import "time"

// Renderer is the usable image presentation mode. HalfBlock is always safe.
type Renderer uint8

const (
	HalfBlock Renderer = iota
	Kitty
)

// ProbeTimeout bounds the time a session may wait for a capability reply.
const ProbeTimeout = 250 * time.Millisecond

// ProbeGates must describe the actual output and input path for this session,
// not merely the terminal name or an environment variable. All are required.
// SerializedOutput means a supported output hook serializes queries with frames.
// ConsumerSupport means every consumer on that path supports Kitty images.
// BoundedInput means the caller has a supported input decoder and bounded reply
// parsing integrated without stealing input or blocking normal event handling.
type ProbeGates struct {
	OptIn            bool
	SerializedOutput bool
	ConsumerSupport  bool
	BoundedInput     bool
}

// QueryToken is a positive correlation ID. The caller must allocate IDs unique
// across all probes sharing an input stream, including earlier sessions and
// late replies; IDs must not be reused while such replies could arrive.
type QueryToken uint64

// ProbeQuery is metadata only, not a protocol query or serialized output.
type ProbeQuery struct {
	Token    QueryToken
	Deadline time.Time
}

// Probe caches one capability decision for one output session. It performs no
// IO, scheduling or parsing. Its owner must serialize calls and schedule a
// Result call at the deadline. A new output session needs a new Probe.
// This seam does not implement runtime probing; wire support belongs elsewhere.
type Probe struct {
	eligible  bool
	started   bool
	done      bool
	renderer  Renderer
	query     ProbeQuery
	startedAt time.Time
}

// NewProbe creates a fail-closed, one-shot session capability probe.
func NewProbe(g ProbeGates) *Probe {
	eligible := g.OptIn && g.SerializedOutput && g.ConsumerSupport && g.BoundedInput
	return &Probe{eligible: eligible, done: !eligible}
}

// Begin returns a query only once, when all gates are satisfied and token is
// nonzero. The caller is responsible for encoding and serializing the query;
// if it cannot send it, the deadline still resolves to HalfBlock.
func (p *Probe) Begin(token QueryToken, now time.Time) (ProbeQuery, bool) {
	if !p.eligible || p.started || p.done || token == 0 {
		return ProbeQuery{}, false
	}
	p.started = true
	p.startedAt = now
	p.query = ProbeQuery{Token: token, Deadline: now.Add(ProbeTimeout)}
	return p.query, true
}

// Reply accepts only a bounded, already-validated response correlated to this
// session's query. supported must reflect a positive protocol response, not
// the mere presence of a reply. Unmatched, premature and late replies cannot
// enable Kitty. Calls must use the same nondecreasing clock as Begin.
func (p *Probe) Reply(token QueryToken, supported bool, now time.Time) {
	p.expire(now)
	if p.done || !p.started || token == 0 || token != p.query.Token || now.Before(p.startedAt) {
		return
	}
	p.done = true
	if supported {
		p.renderer = Kitty
	}
}

// Result returns HalfBlock while pending and the cached decision once done.
// At or after the deadline a pending probe permanently falls back to HalfBlock.
func (p *Probe) Result(now time.Time) (renderer Renderer, done bool) {
	if !p.eligible {
		return HalfBlock, true
	}
	p.expire(now)
	return p.renderer, p.done
}

func (p *Probe) expire(now time.Time) {
	if p.started && !p.done && !now.Before(p.query.Deadline) {
		p.done = true
	}
}
