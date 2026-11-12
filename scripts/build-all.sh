#!/usr/bin/env bash
# Build Knitnox for every supported OS/architecture into dist/.
# Usage: scripts/build-all.sh [version]      e.g. scripts/build-all.sh 1.2.0  (or v1.2.0)
# Works from any OS; no C toolchain needed (pure-Go SQLite).
#
# File names carry no version (knitnox-<os>-<arch>), so the website can link to
# github.com/knitnox/knitnox/releases/latest/download/<file> for ever. The
# version is built into the program instead: `knitnox version` prints it.
# .github/workflows/release.yml runs this script for every v* tag.
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${1:-dev}"
VERSION="${VERSION#v}"
OUT=dist
rm -rf "$OUT" && mkdir -p "$OUT"

targets="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64"
for t in $targets; do
  os=${t%/*}; arch=${t#*/}; ext=""; label=$os
  [ "$os" = windows ] && ext=.exe
  [ "$os" = darwin ] && label=macos
  name="knitnox-${label}-${arch}"
  echo "building $name ($VERSION)"
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o "$OUT/$name/knitnox$ext" ./cmd/knitnox
  chmod +x "$OUT/$name/knitnox$ext"
  cp README.md docs/INSTALL.md "$OUT/$name/"
  (
    cd "$OUT"
    if [ "$os" = windows ]; then
      # .zip for Windows users: use zip, else PowerShell (Git Bash on Windows), else fall back to tar.gz
      if command -v zip >/dev/null; then zip -qr "$name.zip" "$name"
      elif command -v powershell.exe >/dev/null; then powershell.exe -NoProfile -Command "Compress-Archive -Path '$name' -DestinationPath '$name.zip'"
      else tar -czf "$name.tar.gz" "$name"; fi
    else
      tar -czf "$name.tar.gz" "$name"
    fi
    rm -r "$name"
  )
done

( cd "$OUT" && (sha256sum knitnox-* 2>/dev/null || shasum -a 256 knitnox-*) > SHA256SUMS.txt )
echo; ls -la "$OUT"
