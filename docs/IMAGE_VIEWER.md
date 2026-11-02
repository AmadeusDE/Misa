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
APNG are shown as a first-frame poster when decodable, with a status notice;
playback is not supported. SVG, editing, file-browser previews and external
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
to 4 MiB/s. Animation, preview and `open_mode` options are reserved for later integration.
`external_command` is stored as an argument vector, not a shell command;
external opening is not yet implemented by this configuration alone.

Numeric bounds: thumbnail size 32–1024 pixels; preview delay 0–60000 ms;
fps 1–120; frame delays 1–60000 ms; cache 1–1024 MiB; animation cache 1 MiB
through the cache limit; frame count 1–10000; metadata 1–64 MiB; transport
1–256 MiB/s; file size 1–1024 MiB; pixels 1–32000000. These are configuration
bounds, not a guarantee that all allowed files can be decoded on every device.
