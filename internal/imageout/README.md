# PR 1 contract boundaries

`imageframe` defines immutable prepared scenes, owner-local resource identities,
viewport geometry, presentation versions and acknowledgements. A lifecycle epoch
invalidates work on resize or visibility changes; a presentation generation orders
requests within that epoch. An acknowledgement means successful local submission,
not that a terminal decoded or displayed the frame. `FrameSource` is bounded and
on demand; `Clock` supplies stoppable timers without choosing playback policy.

`imageout.Session` is an **unbound adapter design**. Only an exclusive output
owner may call it, synchronously, with an injected `TransactionSink` at an
explicit complete-frame boundary. It has no stdout default, writer wrapper,
worker, timer or Bubble Tea command. `Submit` queues at most the latest prepared
frame per registered owner; `Commit` emits deletes, complete text/cell updates,
transfers and placements, with sink-owned cursor preservation. Resource IDs must
be namespaced by the sink using the owner; a future wire encoder must remap them
into its own session namespace. There is no global-delete operation.

`SetVisible`, `Resize`, `Release` and `Close` invalidate queued work and queue
owner-only cleanup. They do not write output: the host must run a final explicit
`Commit` while the boundary is still available, including during deferred exit
cleanup. Failed commits return no acknowledgements, attempt scoped cleanup and
cursor restoration, then poison the session. Recovery from a broken output sink
is not guaranteed.

## What the tests do and do not establish

Fake-sink transcripts establish deterministic phase ordering, coalescing,
owner isolation, lifecycle invalidation and failure semantics. Pending work is
bounded per owner and obsolete prepared frames are replaced rather than queued.
This proves the backpressure **shape**, not sustained transport throughput,
terminal-side atomic replacements or an animation performance target. It does not
establish a bound on time spent inside a synchronous sink; the future boundary
must handle slow writes without starving input and impose root-wide byte/fps
budgets before animation is enabled.

The visibility seam rejects hidden owners and old generations, while retaining
user playback intent separately in `imageframe.PlaybackState`. It does not carry
visibility across a PTY or an Android process boundary. Root pane/tag/help/grid
state, conservative child focus reporting, and Android tab/surface/background
signals still require the later end-to-end bridges described in the plan. Merely
stopping GUI repainting does not pause a child scheduler.

Capability probing is also a state-machine seam, not a live wire probe. It has a
250 ms deadline, per-session caching and correlated positive/negative response
handling. Its caller must supply unique tokens, bounded reply decoding and a
scheduled deadline check. No query encoder or input interception ships here.
`internal/ui/imageoutput.go` leaves all transport/consumer/input gates closed,
even for opt-in, so the current runtime cannot select Kitty.

See [TRANSPORT.md](TRANSPORT.md) for the pinned Bubble Tea v2.0.7 blocker,
source evidence and candidate integration options. There is deliberately no
program-construction or mux runtime change until a supported frame boundary is
proven. Half-block is the safe capability result; its actual renderer and viewer
are PR 2, not an implemented feature of these contracts.
