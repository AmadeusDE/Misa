# Image output transport investigation

This is a source investigation and a contract target, not an implemented
transport. Misa pins `charm.land/bubbletea/v2` **v2.0.7** and
`github.com/charmbracelet/ultraviolet` **v0.0.0-20260525132238-948f4557a654**
(`go.mod`). References below are to those versions unless a Misa path is given.

## Finding

The supported APIs do not expose a frame-boundary hook with which to guarantee
this transaction for a particular composed root frame:

1. Delete only obsolete placements owned by Misa before affected text changes.
2. Emit the corresponding text frame.
3. Emit the new graphics after that text, without disturbing the renderer's
   cursor and screen state.
4. Acknowledge the submitted generation, or report that it was superseded or
   failed.

`tea.WithOutput(io.Writer)` is supported (`options.go`), but it provides a byte
sink, not frame identities or before/after-render callbacks. A writer cannot
safely infer those boundaries from write calls or escape contents. The following
implementation details explain why the other available mechanisms do not fill
that gap:

- `tea.Raw` is a supported command for unformatted output and terminal queries
  (`raw.go`). The event loop handles `RawMsg` by adding its content to a
  separate program output buffer (`tea.go`, `eventLoop` and `execute`). The
  renderer goroutine flushes that buffer **before** the renderer on each tick
  (`startRenderer`). It is not a post-text hook.
- Renderer `render` stores the latest view; the asynchronous renderer flush
  may coalesce views or skip unchanged views (`cursed_renderer.go`, `render`
  and `flush`). Processing a message or returning from an update therefore is
  not proof that its text was emitted. `tea.Sequence` orders command execution
  and message sends, not renderer flushes (`tea.go`, `execSequenceMsg`).
- `tea.Println` and `tea.Printf` insert unmanaged lines above the program and
  explicitly do not print in the alternate screen (`renderer.go`). Misa's root
  view uses the alternate screen (`internal/ui/app.go`, `View`).
- Bubble Tea's renderer interface is private (`renderer.go`), and the cited
  version does not expose a supported renderer replacement or commit callback.
  Its ticker discards flush errors (`tea.go`, `startRenderer`), so a transport
  cannot infer successful submission merely from an update or command.

A successful, fully checked output write would acknowledge **local transport
submission**, not terminal decoding, rendering, or physical presentation. A
terminal protocol reply, when requested, has its own semantics and must not be
confused with a per-frame presentation acknowledgement.

## Input and capability queries

`tea.Raw` can issue a bounded Kitty capability query, but probing alone does not
solve frame ordering. In the pinned Ultraviolet version, `parseApc` recognizes
Kitty graphics replies and returns `uv.KittyGraphicsEvent` with options and a
payload (`decoder.go`, `parseApc`; `event.go`, `KittyGraphicsEvent`). Bubble
Tea translates its known input events and returns other event types unchanged
(`input.go`, `translateInputEvent`), so a recognized complete reply can reach
the root as that Ultraviolet event. This is not a Bubble Tea Kitty-specific
message API and depends on the pinned decoder behavior.

The decoder ignores errors from options unmarshalling and splits the response
payload at semicolons. A consumer must not treat the event alone as strict
validation. Bound and correlate outstanding query IDs and responses; handle
malformed, oversized, fragmented, unsolicited and late replies, and time out to
the cell fallback. Verify whether required bounds can be enforced before the
decoder buffers input. Test that unrelated keyboard, paste and terminal events
are preserved and replies do not become typed text. A terminal or intermediary
that drops the query or reply is not evidence of support.

## Root ownership and the missing boundary

`main.go` constructs the root with `tea.NewProgram(model)` and default output.
The root composes pane strings in `internal/ui/mux.go`; shell panes render their
terminal cells with `RenderFull`, and box composition truncates or pads text.
`internal/ui/muxframe.go` may reuse a cached text frame. Pane commands are
routed in `internal/ui/route.go`, but Bubble Tea messages pass through to the
program globally. Consequently neither a pane-issued raw command nor a graphic
embedded in a view string has root geometry, clipping, resource isolation or a
reliable output phase.

The desired transport seam is a root-owned coordinator receiving immutable,
bounded and generation-tagged prepared effects, not worker writes to stdout.
Its commit boundary must explicitly associate the composed text frame with
owner-scoped deletions and post-text placements, serialize them, preserve
cursor/screen state, and report submitted, superseded and failed generations.
It must invalidate stale work on resize, screen or owner lifecycle changes and
never delete another application's resources. The host must distinguish this
local submission acknowledgement from a terminal or display acknowledgement.

Candidate ways to obtain the missing boundary need separate evaluation:

- A custom output writer supplied at program construction via `WithOutput`,
  **only if coupled to an explicit supported frame-boundary mechanism** that
  provides the required ordering and error reporting. Wrapping the writer alone
  or guessing boundaries from writes is insufficient; terminal detection and
  lifecycle behavior also need verification.
- An upstream Bubble Tea feature exposing appropriate serialized frame hooks
  and acknowledgement/error semantics.
- An isolated, maintained Bubble Tea fork providing those semantics, if the
  project explicitly accepts that dependency and maintenance cost.

These are candidates, not proof that any particular integration is safe.
No raw graphics sequences belong in UI styling or pane view strings.

## Scope and verification status

Kitty capability must remain disabled until the output boundary and query path
are proven end to end; use half-block cells as the safe fallback. This
investigation does not add runtime hooks or change mux behavior. Graphics
protocol implementation, terminal image storage, GUI compositing, nested PTY
forwarding and viewer features are later work and must not be inferred from
this document.

Tests against a fake transaction adapter can establish the intended ordering,
owner isolation, stale-generation handling and backpressure contract. They do
**not** prove the pinned Bubble Tea runtime supplies the required ordering.
Runtime acceptance needs tests against the actual integrated boundary, including
unchanged and coalesced frames, image-only changes, resize, exit, cursor state,
errors and query replies, plus terminal interoperability tests. This document
records source inspection only; it is not a runtime experiment.
