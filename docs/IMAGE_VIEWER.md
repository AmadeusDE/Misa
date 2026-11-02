# Image viewer

`imgview [--thumbnail] [--renderer auto|halfblock|kitty] FILE...` opens
files in order, either as an in-pane Misa viewer for simple invocations or as a
standalone terminal program. `--` ends option parsing. Relative paths are
resolved against the shell's working directory. Directories and recursive
scanning are not supported. Missing or unreadable files remain in the collection
and show an error in the status line.

Rendering currently uses colored half-block cells, including when `auto` is
selected. Kitty graphics transport is not implemented: requesting `kitty` in
configuration or on the command line safely falls back to half-blocks and
shows a warning in the status line. No terminal graphics escapes are emitted.
The viewer is read-only: rotations, flips and marks exist only for the session.
Static PNG, JPEG, GIF, WebP, BMP and TIFF are supported. Animated GIF, WebP and
APNG use bounded animation decoders and full-canvas compositing. The viewer
still shows a still poster with a status notice; playback, timers and animation
keys are not implemented yet. APNG prefers a separate default poster when present
(the poster is not included in the animation sequence). SVG, editing, file-browser previews and external
viewer launching are not implemented.

## Controls

| Key | Action |
| --- | --- |
| `q`, `Esc` | Quit (`Esc` first closes help or clears a numeric prefix) |
| `F1` | Toggle help |
| `Enter` | Toggle single image and thumbnail grid |
| `n` / Space, `p` / Backspace | Next / previous image |
| `g`, `G` | First / last image |
| Arrows or `h`, `j`, `k`, `l` | Pan in single view; select in grid |
| `+` / `=`, `-` | Zoom in / out; change thumbnail size in grid |
| `w`, `W`, `e`, `E` | Fit whole, shrink only, fit width, fit height |
| `f` | Toggle actual size / fit whole |
| `<`, `>` | Rotate left / right |
| `?`, `\|` | Flip horizontally / vertically |
| `m`, `M` | Toggle current mark / invert all marks |
| Ctrl+u, Ctrl+m | Clear marks |
| `N`, `P` | Next / previous marked image |
| `r` | Reload current image |
| `o` | Show external-viewer-unavailable notice |

A numeric prefix applies to next/previous navigation. In the grid, click to
select and click the same item again to open it. In single view, drag to pan.
The mouse wheel navigates images (rows in the grid); Ctrl+wheel zooms or
changes thumbnail size.

## Configuration

`imageviewer.yaml` follows the standard Misa search order: next to the executable,
then in the Misa user configuration directory (normally
`~/.config/misa/imageviewer.yaml` on Linux); the current directory is not
searched. Missing fields use defaults. Invalid enum values revert to defaults;
numeric values are clamped to the ranges below. Invalid YAML falls back to all
defaults. Diagnostics are written to stderr. Saving updates the first existing
file or creates the user configuration file.

```yaml
open_mode: builtin             # builtin | external
renderer: auto                 # auto | kitty | halfblock
initial_mode: single           # single | thumbnails
fit: shrink                    # shrink | contain | width | height | actual
thumbnail_size: 128
preview: true
preview_delay_ms: 120
animation_autoplay: true
preview_animation: false
animation_loops: file          # file | once | infinite
animation_fps: 30
halfblock_animation_fps: 10
preview_animation_fps: 10
animation_min_delay_ms: 20
gif_short_delay_ms: 100
animation_cache_mib: 64
animation_max_frames: 10000
animation_metadata_mib: 8
animation_transport_mib_s: 16
cache_mib: 128
max_file_mib: 64
max_pixels: 32000000
external_command: []           # argv, e.g. ["viewer", "{file}"]
```

On Android the animation fps defaults are 20, 8 and 8 respectively; animation
cache defaults to 24 MiB, frame limit to 2000, metadata to 2 MiB and transport
to 4 MiB/s. Animation decode limits and delay policies are active. Playback, preview and
`open_mode` options are reserved for later integration.
`external_command` is stored as an argument vector, not a shell command;
external opening is not yet implemented by this configuration alone.

Numeric bounds: thumbnail size 32–1024 pixels; preview delay 0–60000 ms;
fps 1–120; frame delays 1–60000 ms; cache 1–1024 MiB; animation cache 1 MiB
through the cache limit; frame count 1–10000; metadata 1–64 MiB; transport
1–256 MiB/s; file size 1–1024 MiB; pixels 1–32000000. These are configuration
bounds, not a guarantee that all allowed files can be decoded on every device.


## Animation decoding and limits

`internal/imgview/animdecode` provides an on-demand `imageframe.FrameSource`.
`imgview.OpenAnimation` opens a regular local file and returns a source owned by
its caller; callers serialize access and close the source. Frames are immutable,
full-canvas NRGBA images. Encoded and effective delays are available separately.
The half-block viewer only requests a poster and closes its source immediately;
no animation state, background goroutines or clocks are retained.

The original GIF/APNG/WebP container parsers validate the complete index before
publishing a source. GIF uses Go's LZW/image decoder, APNG reconstructs streaming
PNG frames for Go's PNG decoder, and WebP repackages ANMF image chunks for the
pure-Go VP8/VP8L codecs. No external codec process, cgo, new module dependency or
copied viewer code is used. The pinned Go 1.27.1 GIF decoder is eager for DecodeAll;
PNG does not expose APNG controls; x/image v0.38.0 WebP has no animation demux API. CRCs, chunk sizes/padding, counts, offsets, flags and
frame rectangles are checked before the relevant allocations. Lazy decoding
can still report corrupt compressed pixel data when a later frame is requested.

GIF disposal 0/1 retains, 2 clears to the logical palette background, and 3
restores pre-frame pixels. When the first image's transparent palette index
matches the logical background index, the canvas/disposal background is transparent.
APNG supports SOURCE/OVER and NONE/BACKGROUND/PREVIOUS (transparent background).
WebP supports source replacement/alpha blend and disposal to the ANIM background,
including background alpha. Compositing never substitutes the UI checkerboard
for transparent image pixels. GIF plain-text extensions are structurally parsed
but text is not rendered; ICC correction remains unsupported.

GIF encoded 0/10 ms delays default to 100 ms; APNG's zero denominator means 100,
and APNG/WebP short delays default to a 20 ms floor. The configured minimum also
acts as a safety floor. GIF positive loop counts become repeats + 1 total plays;
APNG/WebP positive counts already mean total plays. Zero means infinite; an
absent GIF loop extension means once. No loop is played in this PR.

Desktop defaults are 10,000 frames, 8 MiB index metadata and 64 MiB animation
working storage; Android uses 2,000, 2 MiB and 24 MiB. Encoded input (64 MiB) and
canvas pixels (32 million) have independent caps. Working storage conservatively
reserves codec scratch, decoded output, compositor and previous-disposal canvases,
index/reader storage and eager paletted data. A file under the pixel cap can still
exceed its working budget. Small GIFs use DecodeAll only after structural preflight
and when their conservative predecode charge fits one quarter of the work budget.
Larger allowed sequences decode frames lazily, with one post-disposal canvas
checkpoint; backward seeks replay from the beginning and check cancellation
between frames. There is no unbounded full-frame slice or per-frame goroutine.
The viewer's raster cache temporarily reserves animation working storage from
its overall cache budget, evicting cached pixels if necessary.

This version deliberately retains a size-capped immutable encoded snapshot,
rather than file-backed ReaderAt offsets. Identity/size/mtime are checked while
reading; subsequent source-file changes cannot mix frame bytes. The trade-off is
up to one encoded-file cap retained per open source, plus a temporary input copy
while opening. File-backed indexing and sparse checkpoint/LRU seek acceleration
remain hardening/performance follow-ups. Results retained by callers, renderers,
Go's allocator/GC, encoded snapshots and transport buffers are not a hard
peak-process-RSS guarantee. In-process codecs are not a sandbox or a hard CPU
deadline: cancellation suppresses stale publication but cannot interrupt every
individual codec call.


### Bounded WebP codec adapter

`x/image v0.38.0/vp8l` can allocate a very large number of Huffman groups from a
tiny meta-Huffman image, independently of canvas size. The viewer therefore uses
`internal/imgview/webpcodec`, a minimal BSD-noticed adaptation of the pinned
WebP/VP8L reader with allocation checks before backing storage is allocated.
VP8 still uses the module's decoder after conservative padded-image/partition
reservations. Both animated VP8L frames and compressed ALPH data use the bounded
lossless decoder. Static WebP uses the same adapter and independently checks raw
codec dimensions against VP8X before pixel decoding.

A small pre-parser was considered, but parsing the meta-Huffman image requires
lossless transforms, Huffman decoding and recursive image data; an independent
pre-parser would duplicate most of the codec and risk disagreement with the
actual decoder. The local adaptation keeps codec behavior aligned with the
pinned dependency while imposing explicit cumulative allocation limits. See its
`NOTICE`/`LICENSE` and compatibility tests; future x/image upgrades must review
these guards rather than blindly replace the local files. This does not make
in-process decoding a sandbox or promise allocator-exact RSS bounds.
