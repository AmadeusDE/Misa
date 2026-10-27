#!/bin/sh
# Builds the AppImage that opens Misa's window.
# Usage: build.sh <misa binary> <output .AppImage> <arch>
# APPIMAGETOOL names appimagetool (default: on PATH); APPIMAGE_RUNTIME, when
# set, is the runtime file to embed instead of the one appimagetool downloads.
set -eu
bin=$1 out=$2 arch=$3
here=$(dirname "$0")
appdir=$(mktemp -d)
trap 'rm -rf "$appdir"' EXIT

install -Dm755 "$bin" "$appdir/usr/bin/misa"
install -Dm755 "$here/AppRun" "$appdir/AppRun"
install -Dm644 "$here/misa.desktop" "$appdir/misa.desktop"
install -Dm644 "$here/misa.desktop" "$appdir/usr/share/applications/misa.desktop"
install -Dm644 "$here/misa.svg" "$appdir/misa.svg"
install -Dm644 "$here/misa.svg" "$appdir/usr/share/icons/hicolor/scalable/apps/misa.svg"

set -- --no-appstream
if [ -n "${APPIMAGE_RUNTIME:-}" ]; then
	set -- "$@" --runtime-file "$APPIMAGE_RUNTIME"
fi
ARCH=$arch APPIMAGE_EXTRACT_AND_RUN=1 "${APPIMAGETOOL:-appimagetool}" "$@" "$appdir" "$out"
