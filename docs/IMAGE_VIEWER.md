# Image viewer

`imgview [-r|--recursive] [--thumbnail] [--renderer auto|halfblock|kitty] FILE|DIR...`
opens files in order, either as an in-pane Misa viewer for simple invocations or
as a standalone terminal program. `--` ends option parsing. Relative paths are
resolved against the shell's working directory. A directory operand is expanded
into the images it contains (see [Directories](#directories)). Missing or
unreadable files remain in the collection and show an error in the status line.

Rendering currently uses colored half-block cells, including when `auto` is
selected. Kitty graphics transport is not implemented: requesting `kitty` in
configuration or on the command line safely falls back to half-blocks and
shows a warning in the status line. No terminal graphics escapes are emitted.
The viewer is read-only: rotations, flips and marks exist only for the session.
Static PNG, JPEG, GIF, WebP, BMP and TIFF are supported. Animated GIF, WebP and
APNG use bounded animation decoders, full-canvas compositing and application
scheduled playback; they play, pause and step in the half-block renderer. APNG
prefers a separate default poster when present (the poster is not included in
the animation sequence). SVG, editing, file-browser previews and external
viewer launching are not implemented.

## Directories

Directory operands follow the sxiv/nsxiv convention. Each one is expanded in
place, so `imgview a.png DIR b.gif` opens the explicit files around the images
found in `DIR`, in that order. Operands given explicitly are always kept — even
with an unusual extension, a leading dot, or when they do not exist — while
entries found inside a directory are filtered quietly:

* Only names with a supported image extension are collected (`.png`, `.jpg`,
  `.jpeg`, `.gif`, `.webp`, `.bmp`, `.tif`, `.tiff`, case-insensitive).
  Detection is extension-based; a file with an image extension that does not
  decode stays in the collection and reports the error in its status line.
  Other entries, unreadable entries and permission errors are skipped without
  a per-file message.
* Hidden entries (a leading dot) are skipped inside a directory. A hidden file
  or hidden directory that the user names explicitly is still opened or scanned.
* Only direct children are scanned by default. `-r`/`--recursive` descends into
  subdirectories. Symlinked directories are never followed, so a link loop
  cannot recurse; a symlink to a regular image file is accepted.
* The images of each directory are sorted naturally and case-insensitively by
  their path relative to that directory (`img2.png` before `img10.png`), so
  recursive results are grouped per directory deterministically.
* If nothing is left to show, `imgview` fails once with `imgview: no images
  found` instead of opening an empty viewer.

The walk is bounded so a huge or deep tree cannot stall the command:
`max_images` images (10000), `max_directory_depth` levels (32) and
`max_directory_entries` directory entries (200000). Reaching a bound stops
collecting (a depth bound prunes only the deeper subtree) and shows one
status-line warning such as `showing first 10000 images; limit reached`; it is
not an error, and the images already collected stay usable.

## Cell aspect

A half-block cell carries one sample per column and two per row, so the image
only keeps its shape when the viewer knows the cell's real pixel size. It
resolves the size in this order:

1. `cell_pixels: [9, 20]` in `imageviewer.yaml`: an exact cell width and height.
2. `cell_aspect: 0.45`, the cell's width divided by its height, when only the
   ratio is known; the height is then taken as 20 pixels.
3. The host terminal's `TIOCGWINSZ` pixel dimensions. Misa's own terminal fills
   `ws_xpixel`/`ws_ypixel` from the GUI font metrics and answers `CSI 16t` (cell
   size) and `CSI 14t` (text area size), so a viewer started inside Misa, in a
   pane or in the Wayland/Android GUI, uses the real font metrics. Terminals
   that report nothing, including most plain consoles, leave the size unknown.
4. Otherwise a 10x20 cell, twice as tall as wide, which is what earlier versions
   assumed unconditionally.

Fit modes, zoom, panning, the thumbnail grid and the half-block output all work
in physical pixels, so images are not stretched for any cell size. `f` (actual
size) and `W` (shrink-only) mean one image pixel per physical pixel when the
cell size is known, and per assumed pixel otherwise; the status line's
percentage is relative to that. The cell size is read when the viewer opens, so
a font-size change in the GUI applies to the next viewer.

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
| `f` | Toggle actual size (one image pixel per physical pixel) / fit whole |
| `<`, `>` | Rotate left / right |
| `?`, `\|` | Flip horizontally / vertically |
| `m`, `M` | Toggle current mark / invert all marks |
| Ctrl+u, Ctrl+m | Clear marks |
| `N`, `P` | Next / previous marked image |
| Ctrl+a | Toggle animation playback (grid: no action) |
| Ctrl+n, Ctrl+p | Pause and step one frame forward / backward; a numeric prefix steps N |
| `r` | Reload current image and restart its animation |
| `o` | Show external-viewer-unavailable notice |

A numeric prefix (at most 9999) applies to next/previous navigation and to
Ctrl+n/Ctrl+p. Ctrl+a discards a pending prefix with a status hint instead of
consuming it. In the grid, click to select and click the same item again to open
it. In single view, drag to pan. The mouse wheel navigates images (rows in the
grid); Ctrl+wheel zooms or changes thumbnail size.

## Animation playback

Animated GIF, WebP and APNG play in the single-image view by re-rendering the
composited frame on the viewer's own clock; no terminal animation protocol is
used. Autoplay follows `animation_autoplay`; with autoplay off the sequence starts
paused. Ctrl+a toggles playback. After a finite sequence has ended Ctrl+a
restarts it from the first frame, otherwise it resumes at the saved position.
Ctrl+n/Ctrl+p pause and step forward/backward, honouring a numeric prefix.
Stepping clamps at the first and last frame, never changes the play counter and
keeps the paused state. Static images only show a brief status hint. Ctrl+a in
the grid reports that thumbnails stay still; frame stepping has no effect
there, and entering the grid suspends playback.

The status line reports the committed frame, not a requested decode:
`frame 3/24 · playing`, `paused` or `ended`, plus the loop counter (`loop 2/5`,
`loop ∞` when the sequence is unbounded). Frame 0 is decoded like any other
frame, so a paused animation never leaves a still poster on screen: formats
whose poster is a separate image (APNG) present frame 0 as soon as it decodes.
Until that first frame is committed the status line claims no frame number and
says `4 frames · loading` instead. `animation_loops` can override the file's
play count with `once` or `infinite`; otherwise the file's own count and the GIF
repeat convention described below apply. `halfblock_animation_fps` caps
presentation: when short frames would exceed it, frames are skipped so overall
timing is preserved rather than playback slowing down. Frame delays are never
shorter than `animation_min_delay_ms`.

At most one frame decode and one clock boundary are outstanding, so slow frames
coalesce to the latest needed one instead of queueing, and a heavy sequence
cannot starve key handling. A boundary starts only once the frame it follows is
on screen: a decode slower than the frame delay delays the animation instead of
skipping frames, so the opening frame and a finite sequence's last frame are
always presented. Frames are dropped only when `halfblock_animation_fps` samples
them. The sequence is indexed only for the visible single-image item; the
poster is decoded first, and switching images, entering the grid, reloading
(`r`), or quitting stops and closes the sequence. A decode or index failure is
reported as `animation: <reason>` and leaves the poster visible.

Playback pauses, with no timer and no new decode, while the viewer is hidden or
unfocused: blur, another Misa screen or tag, an unfocused split pane, or the
viewer's own grid or help overlay. Revealing resumes with the remaining delay
and keeps the position and loop counter; a user's own pause survives
visibility changes. Hiding cancels the pending boundary, so the reveal returns
the command that arms the next one: `SetVisible` hands it to the host, which
returns it from its own `Update`. Without it playback would stay frozen until an
unrelated message happened to reach the viewer.

Terminal focus messages reach one pane at a time, so a viewer blurred while it
was active keeps its own focus flag when the active pane changes underneath it.
A host reports visible only for a screen that is on screen and owns the keyboard,
so that push also settles focus and the revealed viewer resumes; hiding leaves
focus alone. In-pane Misa computes visibility from the active tag, pane focus and
window focus. The standalone program has no host: it enables terminal focus
reporting and relies on those messages alone. A terminal that never reports
focus is treated as focused, so the animation keeps playing there.

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
cell_aspect: 0                 # cell width/height; 0 measures the terminal
cell_pixels: []                # exact cell size in pixels, e.g. [9, 20]
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
max_images: 10000              # images collected from directory operands
max_directory_depth: 32        # levels scanned below a directory operand
max_directory_entries: 200000  # directory entries read while walking
external_command: []           # argv, e.g. ["viewer", "{file}"]
```

On Android the animation fps defaults are 20, 8 and 8 respectively; animation
cache defaults to 24 MiB, frame limit to 2000, metadata to 2 MiB and transport
to 4 MiB/s. Animation decode limits, delay policies, `animation_autoplay`,
`animation_loops`, the half-block presentation cap
(`halfblock_animation_fps`) and the three directory collection bounds are
active. The remaining values belong to later work: `animation_fps` and
`animation_transport_mib_s` to the pixel renderers,
and `preview`, `preview_delay_ms`, `preview_animation`, `preview_animation_fps`
and `open_mode` to file-browser previews and external opening. There are no
image-viewer controls in the settings screen yet; these options are YAML-only.
`external_command` is stored as an argument vector, not a shell command;
external opening is not yet implemented by this configuration alone.

`cell_pixels`, `cell_aspect` and the measured terminal size decide the cell shape
described above; the first two are the user's override and win over any
measurement.

Numeric bounds: thumbnail size 32–1024 pixels; cell aspect 0.1–4 (or 0 to
measure); cell pixels 1–256 per axis; preview delay 0–60000 ms;
fps 1–120; frame delays 1–60000 ms; cache 1–1024 MiB; animation cache 1 MiB
through the cache limit; frame count 1–10000; metadata 1–64 MiB; transport
1–256 MiB/s; file size 1–1024 MiB; pixels 1–32000000; collected images
1–1000000; directory depth 1–256; directory entries 1–10000000. These are
configuration bounds, not a guarantee that all allowed files can be decoded on
every device.


## Animation decoding and limits

`internal/imgview/animdecode` provides an on-demand `imageframe.FrameSource`;
`imgview.OpenAnimation` opens a regular local file and returns a source owned by
its caller, which serializes access and closes it. Frames are immutable,
full-canvas NRGBA images. Encoded and effective delays are available separately.
The viewer decodes the poster first and indexes the sequence only when the item
is the visible single image; navigation, entering the grid, reload and exit
close it. At most one frame decode and one clock boundary are outstanding, so
slow frames coalesce to the latest needed one instead of queueing. Stepping
backward may need to replay earlier frames to rebuild the canvas, so it can take
longer than stepping forward; the work stays bounded either way.

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
absent GIF loop extension means once. `animation_loops` can override the file's
count with `once` or `infinite`.

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

## Terminal graphics store

Misa's terminal emulator has an opt-in bounded Kitty-protocol image store and
immutable snapshot placements for a future pixel compositor. Graphics are
**disabled by default**, and no frontend enables them yet: disabled terminals
consume graphics APCs without displaying payload text, storing images or
answering queries positively. This does not enable `renderer: kitty` in the
viewer. Host-Kitty output/runtime integration is PR 5b; GUI and mux pixel
composition is PR 6. See [the supported protocol subset](../internal/kittygfx/README.md)
for formats, limits and lifecycle semantics.

### Terminal graphics limits

`graphics.yaml` is shared by Misa panes and GUI terminals. It follows the normal
configuration search order (next to the executable, then the Misa user config
directory, never the working directory). These settings change resource limits,
**not** capability gating: they cannot enable graphics without a consumer.

```yaml
kitty:
  max_width: 16384           # pixels per image
  max_height: 16384
  max_image_mib: 256         # decoded bytes per image, binary MiB
  max_terminal_mib: 512      # stored decoded bytes, both screens combined
  max_images: 256            # both screens combined
  max_placements: 1024       # both screens combined
  max_in_flight_mib: 360     # encoded bytes in an incomplete transfer
  max_payload: 8192          # bytes per graphics APC body
```

Every field accepts zero for its default. Negative values fall back to defaults
with a diagnostic; values above the validated maxima are clamped with a
diagnostic. Maximum configurable dimensions are 1,048,576 pixels per axis;
byte budgets are 65,536 MiB each; image and placement counts are 1,000,000 each;
APC bodies are at most 16 MiB. These maxima allow raising the defaults, not a
promise that allocating that much memory is safe on a particular device. The
dimension and byte caps both apply: a 16384×16384 RGBA image needs 1024 MiB and
is rejected by the default 256 MiB image budget before allocation. Arithmetic
and platform-addressability checks still apply with raised limits.

The stored-byte budget is not a peak-process-RSS limit: a completed transfer
also temporarily holds its encoded payload and decoded replacement; retained
snapshots can keep older immutable rasters alive. Consumers should reuse or
release snapshots promptly. No terminal-side animation timer is started.

## Remaining later work

The viewer deliberately stops at half-block playback. These parts are not
implemented yet and should not be assumed from this document:

* Host-Kitty graphics transport and capability probing. `renderer: kitty` still
  falls back to half-blocks with a status warning. The emulator's opt-in image
  store is not enabled by any consumer; the GUI does not display image escapes.
* Pixel animation in Misa's Wayland GUI, the Android APK and nested mux panes,
  including the shared compositor, damage tracking and byte-rate limits.
* File-browser integration: opening images from the browser, thumbnail
  previews and opt-in animated previews. `preview`, `preview_delay_ms`,
  `preview_animation` and `preview_animation_fps` are reserved for that work,
  and the full viewer's playback state will stay independent of any preview.
* External/default viewer opening: the `o` key only reports that it is
  unavailable, and `open_mode` and `external_command` are not wired to a
  launcher.
* Android-specific lifecycle visibility, playback extra-key controls and
  handing the original file to a system viewer through a content URI.
* Settings-screen controls for image-viewer options; only `imageviewer.yaml`
  configures them today.
* SVG support, which remains an optional follow-up. Sixel, animated
  AVIF/HEIC/JPEG XL and MNG, video containers, slideshow,
  editing/saving transforms and freedesktop thumbnail-cache interoperability
  are out of scope rather than pending work.
* Optional native Kitty animation offload. Even on Kitty, the intended baseline
  is application-scheduled frame replacement so pause, stepping and loop status
  stay authoritative.
