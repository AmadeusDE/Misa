set shell := ["bash", "-euo", "pipefail", "-c"]
set positional-arguments := true

arch := `uname -m`
version := `git describe --tags --always --dirty 2>/dev/null || echo dev`
dist := "dist"
release_bin := dist / "misa-" + arch
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
    APPIMAGETOOL="$(just _appimagetool)" packaging/appimage/build.sh {{release_bin}} {{appimage}} {{arch}}
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

# Termux CLI (misa + ugh, no ughbot). Needs ANDROID_NDK_HOME.
# Arch is arm64 or amd64. Output is dist/misa-android-<arch>/.
android-cli arch="arm64":
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ -z "${ANDROID_NDK_HOME:-}" ]]; then
        echo "ANDROID_NDK_HOME is not set" >&2
        exit 1
    fi
    ver=$(tr -d '[:space:]' < android/ndk.version)
    case "{{arch}}" in
        arm64|aarch64) goarch=arm64; triple=aarch64-linux-android24-clang ;;
        amd64|x86_64) goarch=amd64; triple=x86_64-linux-android24-clang ;;
        *) echo "arch must be arm64 or amd64" >&2; exit 1 ;;
    esac
    prebuilt="${ANDROID_NDK_HOME}/toolchains/llvm/prebuilt"
    host=""
    case "$(uname -s)-$(uname -m)" in
        Linux-x86_64) host=linux-x86_64 ;;
        Darwin-arm64) host=darwin-arm64 ;;
        Darwin-x86_64) host=darwin-x86_64 ;;
    esac
    if [[ -z "$host" || ! -x "$prebuilt/$host/bin/$triple" ]]; then
        host=$(find "$prebuilt" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | head -n 1)
    fi
    cc="$prebuilt/$host/bin/$triple"
    if [[ ! -x "$cc" ]]; then
        echo "NDK clang not found: $cc (pin $ver)" >&2
        exit 1
    fi
    out="{{dist}}/misa-android-${goarch}"
    mkdir -p "$out"
    export GOOS=android GOARCH="$goarch" CGO_ENABLED=1 CC="$cc"
    go build -trimpath -ldflags='-s -w' -o "$out/misa" .
    go build -trimpath -ldflags='-s -w' -o "$out/ugh" ./cmd/ugh
    cp LICENSE README.md THIRD_PARTY_NOTICES.md "$out/"
    ls -lh "$out"
