#!/usr/bin/env bash
set -euo pipefail
VERSION="${1:-dev}"
mkdir -p dist
for target in linux/amd64 linux/arm64; do
  GOOS=${target%/*}; GOARCH=${target#*/}
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o "dist/mesh-agent-$GOOS-$GOARCH" ./cmd/mesh-agent
done
