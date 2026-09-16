#!/usr/bin/env bash
# Build release archives for dns-bench. Usage: scripts/build-release.sh [version]
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VERSION="${1:-}"
if [[ -z "$VERSION" ]]; then
  VERSION="$(git describe --tags --always --dirty 2>/dev/null || true)"
fi
if [[ -z "$VERSION" ]]; then
  VERSION="dev"
fi
if [[ "$VERSION" != v* && "$VERSION" != dev* ]]; then
  VERSION="v${VERSION}"
fi
VER_NUM="${VERSION#v}"

LDFLAGS="-s -w -X github.com/78tacos/dns-bench/internal/version.Version=${VER_NUM}"
OUT="${ROOT}/dist"
rm -rf "$OUT"
mkdir -p "$OUT"

build_windows() {
  local arch="$1"
  local tmp
  tmp="$(mktemp -d)"
  CGO_ENABLED=0 GOOS=windows GOARCH="$arch" go build -trimpath -ldflags "$LDFLAGS" -o "${tmp}/dns-bench.exe" ./cmd/dns-bench
  cp LICENSE README.md "$tmp/"
  (cd "$tmp" && zip -q "${OUT}/dns-bench-${VERSION}-windows-${arch}.zip" dns-bench.exe LICENSE README.md)
  rm -rf "$tmp"
}

build_unix() {
  local os="$1"
  local arch="$2"
  local tmp
  tmp="$(mktemp -d)"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags "$LDFLAGS" -o "${tmp}/dns-bench" ./cmd/dns-bench
  cp LICENSE README.md "$tmp/"
  tar -C "$tmp" -czf "${OUT}/dns-bench-${VERSION}-${os}-${arch}.tar.gz" dns-bench LICENSE README.md
  rm -rf "$tmp"
}

build_windows amd64
build_windows arm64
build_unix linux amd64
build_unix linux arm64
build_unix darwin amd64
build_unix darwin arm64

(cd "$OUT" && sha256sum -- * > SHA256SUMS)
ls -la "$OUT"
