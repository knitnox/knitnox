#!/usr/bin/env bash
# Build Knitnox for every supported OS/architecture into dist/.
# Usage: scripts/build-all.sh [version]      e.g. scripts/build-all.sh 1.2.0
# Works from any OS; no C toolchain needed (pure-Go SQLite).
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${1:-dev}"
OUT=dist
rm -rf "$OUT" && mkdir -p "$OUT"

targets="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64"
for t in $targets; do
  os=${t%/*}; arch=${t#*/}; ext=""
  [ "$os" = windows ] && ext=.exe
  name="knitnox-${VERSION}-${os}-${arch}"
  echo "building $name"
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags="-s -w" -o "$OUT/$name/knitnox$ext" ./cmd/knitnox
  cp README.md "$OUT/$name/"
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
