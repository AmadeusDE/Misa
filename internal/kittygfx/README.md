# kittygfx

The bounded subset of the [Kitty graphics protocol](https://sw.kovidgoyal.net/kitty/graphics-protocol/)
that Misa's terminal emulator accepts and its image output emits. Standard library only, no UI, GUI,
terminal or Bubble Tea imports, so both sides of the wire share one parser and one encoder.

Misa never claims full Kitty conformance. Everything outside the subset below is rejected cleanly.

## API

```go
cmd, err := kittygfx.Parse(apcBody, kittygfx.DefaultLimits()) // APC body: no leading "G", no ST
raster, err := kittygfx.Decode(cmd, cmd.Payload, kittygfx.DefaultLimits())
wire, err := kittygfx.Encode(cmd, cmd.Payload)  // one complete APC
reply := kittygfx.Reply(cmd, "EINVAL", "reason") // nil when q suppresses it
```

- `Command` is one parsed command. `Has(key)` reports whether a key was present, so `m=0` and an
  absent `m`, or `C=0` and an absent `C`, stay distinguishable.
- `Parse` applies the protocol defaults: a missing `a` is `t`, and a transmit without `f` is `f=32`.
- `Parse` returns the fields decoded so far together with the error, so a terminal can honour `q` and
  address `i`/`I` in its error reply even when an earlier key was invalid.
- `Decode` accepts one *complete* encoded transfer (all chunks concatenated) and returns an immutable
  `Raster`: `Bounds`, `At`, `Format`, `CopyPixels`. No mutable pixel slice escapes.
- `Encode` writes one APC from an already base64 encoded payload; chunking a large transfer into
  several APCs is the caller's job. Only non-zero fields are written, in the order
  `a t f i I p s v c r x y w h X Y C z q m U d`. `m`, `q` and `C` keep their key when `Has` reports
  it was present, so a parsed `m=0` (last chunk) re-encodes as `m=0` instead of disappearing.
  `Encode` rejects the same malformed values and action incompatibilities as `Parse` — negative
  sizes or offsets, `t` on anything but `a=t`, `a=T` or `a=q`, an unknown delete specifier, and so
  on — but it does not apply the size limits, which belong to the receiver.
- `Reply` writes `ESC _ G [i=<id>][,I=<number>][,p=<placement id>];CODE[:message] ST`. `q=1`
  suppresses `OK` replies, `q=2` suppresses every reply (nil). Messages are truncated and stripped of
  control bytes and `;`.
- `Raster` stores the transferred bytes exactly as sent (RGB or RGBA, 3 or 4 bytes per pixel,
  row-major). It does not expand RGB to four bytes, premultiply or copy. `At` returns `color.NRGBA`;
  `CopyPixels` writes into a caller-owned buffer, so a frontend that needs BGRA or premultiplied
  pixels converts while copying.

## Accepted subset

| Key | Field | Accepted |
|---|---|---|
| `a` | `Action` | `t` transmit, `T` transmit and place, `p` place, `d` delete, `q` query |
| `t` | `Transmission` | `d` direct only |
| `d` | `Delete` | `a`, `A`, `i`, `I`, `n`, `N` (screen-local; lowercase keeps the data, uppercase frees it when unplaced) |
| `f` | `Format` | `24` RGB, `32` RGBA |
| `s` `v` | `Width` `Height` | source pixels, bounded by `MaxWidth`/`MaxHeight` |
| `i` `I` `p` | `ID` `Number` `PlacementID` | uint32 |
| `c` `r` | `Columns` `Rows` | non-negative |
| `x` `y` `w` `h` | `SourceX` `SourceY` `SourceWidth` `SourceHeight` | non-negative source rectangle |
| `X` `Y` | `OffsetX` `OffsetY` | non-negative pixel offsets |
| `C` | `CursorMovement` | `0` or `1` |
| `z` | `Z` | any signed value, ignored: Misa draws a single layer |
| `q` | `Quiet` | `0`, `1`, `2` |
| `m` | `More` | `0` or `1` |
| `U` | `Unicode` | `0` only; placeholders are unsupported |
| `o` | — | rejected: no compression |

A key may appear once; repeated keys are `ErrMalformed` rather than last-one-wins.

Delete selectors: `d=i`/`d=I` select the images named by `i` (and `p` when present), `d=n`/`d=N`
select by image number `I`, and `d=a`/`d=A` select every image on the screen. Lowercase selectors
keep the image data; uppercase ones free it once no placement is left. `d=p`/`d=P` are not part of
this subset.

Rejected with `ErrUnsupported`: `a=f`/`a=a`/`a=c` (native animation and composition), file,
temporary-file and shared-memory transport, `f=100` (PNG payloads), `o=` compression, `U!=0`,
unicode placeholders, the placement selectors `d=p`/`d=P`, and any key outside the table. `t=d` is accepted with `a=t`, `a=T` and `a=q`;
the query describes the transfer it asks about. Malformed values are `ErrMalformed`; sizes and
payloads over the limits are `ErrTooLarge`. All three are `errors.Is`-able.

`Decode` requires `a=t`/`a=T`, `t=d` (or absent), `f=24`/`32`, positive `s` and `v`, and an encoded
payload whose exact decoded length equals the pixel count. Both caps are checked before anything is
allocated: `s`/`v` against `MaxWidth`/`MaxHeight`, and `s*v*bytesPerPixel` (overflow checked) against
`MaxImageBytes` and `MaxTerminalBytes`. A full 16384x16384 RGBA image is therefore rejected by the byte
budget (1 GiB against 256 MiB), not accepted because it fits the dimension cap.

## Limits

`DefaultLimits()`:

| Field | Default | Meaning |
|---|---|---|
| `MaxWidth`, `MaxHeight` | 16384 | pixels per image |
| `MaxImageBytes` | 256 MiB | decoded bytes per image |
| `MaxTerminalBytes` | 512 MiB | decoded bytes for all images in one terminal |
| `MaxImages` | 256 | stored images per terminal |
| `MaxPlacements` | 1024 | placements per terminal |
| `MaxInFlightBytes` | 360 MiB | encoded bytes of all incomplete transfers |
| `MaxPayload` | 8 KiB | encoded payload bytes in one APC |

`Limits.Normalize()` replaces zero and negative fields with their defaults, so a zero `Limits`
value is the safe default rather than "allow nothing". `Parse` and `Decode` normalize what they are
given. `MaxTerminalBytes` (across images), `MaxImages`, `MaxPlacements` and `MaxInFlightBytes`
describe terminal-side state and are enforced by the emulator's graphics store, not by these
per-command functions.

## Not in this package

Placement, clipping, scrolling, alternate-screen isolation, image and placement lifetime, and the
atomic replacement of a current frame belong to the terminal-side store (`internal/term`) and the
output side (`internal/imageout`). This package only defines the wire format, the immutable decoded
image and the reply.

## Tests

```sh
go test ./internal/kittygfx/
go test -run xxx -fuzz FuzzParse -fuzztime 30s ./internal/kittygfx/
```

`FuzzParse` checks that parsing never panics, that accepted commands stay inside the limits, and
that `Encode`/`Parse` round-trip every accepted command.
