# Terminal graphics (internal/term)

Terminal-side support for the Kitty graphics protocol: the emulator parses the
APC commands a program writes to its PTY, keeps a per-screen store of images and
placements, and exposes the visible ones through `Snapshot`. Drawing them is the
frontend's job (PR6); this package only stores geometry
and immutable rasters.

Everything here is off until a frontend says it can draw images.

## API

```go
func (t *Terminal) SetGraphicsEnabled(on bool)          // default false
func (t *Terminal) GraphicsEnabled() bool
func (t *Terminal) SetGraphicsLimits(kittygfx.Limits)   // zero fields = defaults
func (t *Terminal) GraphicsLimits() kittygfx.Limits     // what is actually in force
func (t *Terminal) SetCellPixels(w, h int)              // existing: used for c/r math
```

`Snapshot` carries the graphics that are on screen:

```go
type Snapshot struct {
    // ...
    GraphicsGeneration uint64          // bumps whenever visible graphics change
    Graphics           []GraphicsPlacement
}

type GraphicsPlacement struct {
    ImageID         uint32
    PlacementID     uint32
    ImageGeneration uint64
    Raster          image.Image       // immutable, shared, never copied
    CellRect        image.Rectangle   // cells, may extend past the screen
    Clip            image.Rectangle   // CellRect ∩ screen ∩ scroll-region clip
    Source          image.Rectangle   // pixels inside Raster
    PixelOffset     image.Point       // pixels inside the first cell
}
```

`Raster` is a `*kittygfx.Raster` (raw RGB/RGBA). A replacement installs a new
raster and bumps `ImageGeneration`; a snapshot taken before the replacement keeps
pointing at the old pixels. Nothing copies pixel data per frame, and `Snapshot`
reuses the caller's `Graphics` slice, clearing the tail so an evicted image is
not kept alive by an old snapshot.

## Protocol subset

| key | meaning |
| --- | --- |
| `a` | `t` transmit, `T` transmit+place, `p` place, `d` delete, `q` query |
| `f` | `24` (RGB) and `32` (RGBA) only |
| `s`,`v` | source width/height in pixels |
| `i`,`I` | image id, image number (per screen; `I` alone allocates an id) |
| `p` | placement id (`p=0`/absent: a new placement each time) |
| `c`,`r` | placement size in cells |
| `x`,`y`,`w`,`h` | source rectangle in pixels |
| `X`,`Y` | pixel offset inside the first cell |
| `C` | `1`: leave the cursor where it was |
| `q` | `0` reply always, `1` only errors, `2` never |
| `m` | `1` more chunks follow, `0` last chunk |
| `z` | accepted, any signed value, ignored: one fixed layer |
| `U` | `0` only; unicode placeholders are refused |
| `t` | `d` (direct data) only; file and shared-memory transfer are refused |

Chunked transfers: only the first chunk needs `i`/`I`, later chunks may carry
just `m` and their payload. A chunk with no transfer to join and no identity is
dropped without a reply. Transfers are reclaimed by `RIS`, `ESC c`, `Close`,
`SetGraphicsEnabled(false)`, `SetGraphicsLimits`, a new image id, an overflowed
payload, or a chunk larger than the in-flight budget.

Replies are `ESC _ G <image identifiers, optional p=placement id> ; code:message ESC \` with codes `OK`,
`EINVAL` (malformed, or a feature outside this fixed subset: PNG payloads,
compression, file or shared-memory transfer, animation, unicode placeholders,
an unsupported delete selector), `ENOENT` (no such image) and `ENOSPC` (over a
limit). There is no separate "unsupported" code: the subset is fixed, so
anything outside it is a bad request. With graphics disabled the terminal says
nothing at all, so a probe gets no answer and the client falls back.

Deletes are the standard subset:

* `d=a` drops every placement on the current screen, keeps the image data.
* `d=A` drops every placement and every image on the current screen.
* `d=i` / `d=n` (`i=`, `I=`, optional `p=`) drop one image's placements, or one
  placement of it, and keep the data.
* `d=I` / `d=N` do the same and free the data once no placement uses it.
* Anything else, including the cursor-intersection selectors `p`/`P`, is `EINVAL`.

## Lifecycle

* Disabled by default: `ESC _ G …` is swallowed like any other APC, nothing is
  stored, nothing is answered. Other APCs keep their old swallow-until-ST
  behaviour.
* Payload bytes are never written to the screen. `ST` and `BEL` end an APC; a
  stray `ESC` inside one aborts it and the rest is swallowed until `ST`/`BEL`,
  except `ESC c`, which resets the terminal and returns to the ground state.
* `RIS` (`ESC c`) clears every image and placement.
* `Close` drops everything and refuses graphics for good; bytes still buffered
  for the parser cannot store an image afterwards.
* Images, ids and numbers are per screen. The main and alternate screens are
  isolated: a command on one cannot see or delete the other's images. Entering
  the alternate screen clears its graphics alongside its freshly cleared cells;
  the main screen's resources are preserved.
* Switching screens bumps the generation even when no cell changes, and cancels
  a transfer in flight: a chunked transfer never spans the switch.
* Placements follow content. A placement with any row inside the scroll region
  moves with it (an anchor-only test would pin a tall placement above the margin
  once its anchor scrolled out). From the first scroll it is also clipped to the
  region, so the rows that left it are gone and the rest stays visible until the
  whole placement has scrolled out; only then is it evicted. Rows outside the
  region stay put.
* Resize shifts main-screen anchors up by the rows the grid moved, drops
  placements that no longer intersect the screen, and bumps the generation even
  when nothing moves (every `Clip` is the cell rectangle intersected with the
  new grid).
* A grid change or a cell-size change (`SetCellPixels`) bumps the epoch, so a
  decode that is still running on the old geometry is dropped instead of
  committing a stale placement.
* Replacing an image with the same id is atomic: the new raster and its
  placements are published together. A placement keeps the source window it was
  made with, so when the replacement is smaller the window is clamped to the
  pixels that exist (and the placement is skipped if nothing is left), which
  keeps `Snapshot.Source` inside `Raster.Bounds()`.
* The cursor moves only when a placement is actually committed: a malformed or
  failed `a=T`/`a=p` leaves it where it was.

## Limits and memory

`graphicsLimits` mirrors `kittygfx.Limits` and is derived from
`kittygfx.DefaultLimits()`, so the protocol package is the only place the
numbers live (per-image pixels and bytes, bytes per terminal, image count,
placement count, in-flight encoded bytes, APC payload bytes). Raising the
limits does not relax the arithmetic: source rectangles, offsets, cell counts
and byte totals are clamped or rejected before they are added.

The APC payload cap counts the **whole body**, control keys included, while
`kittygfx.Parse` applies `MaxPayload` to the bytes after the `;`. The terminal
is therefore slightly stricter than the standalone parser (a body that only
just fits the protocol limit is refused), which bounds the buffer before any
key is parsed. Encoders that chunk at 4096 bytes stay well inside both.

Eviction is oldest-image-first, then oldest placements. The byte caps bound what
is *stored*: committing a transfer also holds its encoded payload and one
decoded image at the same time, so the peak is the stored total plus one image
and its encoded transfer.

## Ordering and concurrency

`ProcessOutput` and `Finish` serialise on `processMu`, so graphics commands keep
the order the program wrote them in, even when a decode has to drop `t.mu`. A
finished APC is decoded and committed immediately, before the next byte is
parsed: text, scrolling or a screen switch later in the same write sees the
image.

Decoding runs with `t.mu` released and re-checks an epoch counter before
committing, so `SetGraphicsEnabled`, `SetGraphicsLimits`, `RIS` or `Close` that
happen while a large image is being decoded discard its result.

## Not implemented

* Unicode placeholders (`U=1`), file and shared-memory transmission (`t=f`,
  `t=s`), zlib payloads (`o=z`), frame animation (`a=f`) and z-layers.
* Graphics are not re-shown from scrollback: once a placement scrolls out it is
  gone.
* Only direct (inline base64) transmission is accepted; the encoded payload is
  capped per APC and per transfer.

## Tests

```sh
go test ./internal/term/                     # parser, store, replies, review
go test -race ./internal/term/
go test ./internal/term/ -run XXX -fuzz FuzzGraphicsAPC -fuzztime 20s
go test ./internal/term/ -run XXX -fuzz FuzzGraphicsReviewOutput -fuzztime 20s
```

`FuzzGraphicsAPC` feeds graphics traffic one byte per write, so every split of a
chunked transfer, escape or reset is exercised; the store, screen, resize and
scroll cases live in `graphics_store_test.go`, `graphics_test.go` and
`graphics_reply_test.go`.
