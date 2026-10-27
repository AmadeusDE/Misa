set shell := ["bash", "-euo", "pipefail", "-c"]
set positional-arguments := true

arch := `uname -m`
version := `git describe --tags --always --dirty 2>/dev/null || echo dev`
dist := "dist"
release_bin := dist / "misa-" + arch
appdir := dist / "Misa.AppDir"
appimage := dist / "Misa-" + version + "-" + arch + ".AppImage"
appimagetool_cache := ".cache" / "appimagetool-" + arch + ".AppImage"
appimagetool_url := "https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-" + arch + ".AppImage"

# List recipes
default:
    @just --list

# Build ./misa with everything (ugh agent + Wayland window)
build:
    go build -o misa .

# Build ./misa without the embedded ugh agent
build-nough:
    go build -tags nough -o misa .

# Build ./misa without the Wayland window
build-nogui:
    go build -tags nogui -o misa .

# Build ./misa with neither ugh nor the window
build-minimal:
    go build -tags nough,nogui -o misa .

# Static, stripped binary in dist/
release:
    mkdir -p {{dist}}
    CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o {{release_bin}} .
    @ls -lh {{release_bin}}

# Run tests (default ./...; e.g. `just test -run Foo ./internal/term/`)
test *args="./...":
    go test "$@"

vet:
    go vet ./...

# vet + tests
check: vet test

# Terminal parser throughput
bench:
    go test -run '^$' -bench ProcessOutput ./internal/term/

# Build and run Misa in this terminal
run *args: build
    ./misa "$@"

# Build and open Misa in its Wayland window (args run there instead)
gui *args: build
    ./misa --gui "$@"

# Build and open the window as an ssh client
ssh +args: build
    ./misa --ssh "$@"

# Install the release binary plus misa-gui/misa-ssh links into PREFIX/bin
install prefix="~/.local": release
    install -Dm755 {{release_bin}} {{prefix}}/bin/misa
    ln -sf misa {{prefix}}/bin/misa-gui
    ln -sf misa {{prefix}}/bin/misa-ssh
    install -Dm644 packaging/appimage/misa.desktop {{prefix}}/share/applications/misa.desktop
    install -Dm644 packaging/appimage/misa.svg {{prefix}}/share/icons/hicolor/scalable/apps/misa.svg

# AppImage that opens Misa's window (args run in the window: `Misa.AppImage sshc host`)
appimage: release
    rm -rf {{appdir}}
    install -Dm755 {{release_bin}} {{appdir}}/usr/bin/misa
    install -Dm755 packaging/appimage/AppRun {{appdir}}/AppRun
    install -Dm644 packaging/appimage/misa.desktop {{appdir}}/misa.desktop
    install -Dm644 packaging/appimage/misa.desktop {{appdir}}/usr/share/applications/misa.desktop
    install -Dm644 packaging/appimage/misa.svg {{appdir}}/misa.svg
    install -Dm644 packaging/appimage/misa.svg {{appdir}}/usr/share/icons/hicolor/scalable/apps/misa.svg
    ARCH={{arch}} APPIMAGE_EXTRACT_AND_RUN=1 "$(just _appimagetool)" --no-appstream {{appdir}} {{appimage}}
    @ls -lh {{appimage}}

# Path to appimagetool: the installed one, else a cached download
_appimagetool:
    #!/usr/bin/env bash
    set -euo pipefail
    if command -v appimagetool >/dev/null; then command -v appimagetool; exit; fi
    if [[ ! -x {{appimagetool_cache}} ]]; then
        mkdir -p "$(dirname {{appimagetool_cache}})"
        curl -fL# -o {{appimagetool_cache}}.part {{appimagetool_url}} >&2
        chmod +x {{appimagetool_cache}}.part
        mv {{appimagetool_cache}}.part {{appimagetool_cache}}
    fi
    realpath {{appimagetool_cache}}

clean:
    rm -rf {{dist}} misa
