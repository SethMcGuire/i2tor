#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
APPDIR="$ROOT_DIR/dist/AppDir"
APPIMAGETOOL="${APPIMAGETOOL:-$ROOT_DIR/.tools/appimagetool}"
VERSION="${VERSION:-dev}"
OUTPUT_NAME="${OUTPUT_NAME:-i2tor-${VERSION}-linux-x86_64.AppImage}"

mkdir -p "$ROOT_DIR/dist" "$ROOT_DIR/.tools" "$APPDIR/usr/bin" "$APPDIR/usr/share/applications" "$APPDIR/usr/share/icons/hicolor/512x512/apps" "$APPDIR/usr/share/icons/hicolor/256x256/apps" "$APPDIR/usr/share/icons/hicolor/128x128/apps"
rm -rf "$APPDIR"
mkdir -p "$APPDIR/usr/bin" "$APPDIR/usr/share/applications" "$APPDIR/usr/share/icons/hicolor/512x512/apps" "$APPDIR/usr/share/icons/hicolor/256x256/apps" "$APPDIR/usr/share/icons/hicolor/128x128/apps"

env GOCACHE="${GOCACHE:-/tmp/i2tor-gocache}" go build -o "$APPDIR/usr/bin/i2tor" "$ROOT_DIR/cmd/i2tor"
cp "$ROOT_DIR/packaging/linux/i2tor.desktop" "$APPDIR/i2tor.desktop"
cp "$ROOT_DIR/packaging/linux/i2tor.desktop" "$APPDIR/usr/share/applications/i2tor.desktop"
cp "$ROOT_DIR/packaging/linux/AppRun" "$APPDIR/AppRun"
cp "$ROOT_DIR/i2tor.png" "$APPDIR/i2tor.png"
cp "$ROOT_DIR/i2tor.png" "$APPDIR/.DirIcon"
cp "$ROOT_DIR/i2tor.png" "$APPDIR/usr/share/icons/hicolor/512x512/apps/i2tor.png"
cp "$ROOT_DIR/i2tor.png" "$APPDIR/usr/share/icons/hicolor/256x256/apps/i2tor.png"
cp "$ROOT_DIR/i2tor.png" "$APPDIR/usr/share/icons/hicolor/128x128/apps/i2tor.png"
chmod +x "$APPDIR/AppRun" "$APPDIR/usr/bin/i2tor"

if [ ! -x "$APPIMAGETOOL" ]; then
  curl -fsSL "https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage" -o "$APPIMAGETOOL"
  chmod +x "$APPIMAGETOOL"
fi

ARCH=x86_64 "$APPIMAGETOOL" "$APPDIR" "$ROOT_DIR/dist/$OUTPUT_NAME"
