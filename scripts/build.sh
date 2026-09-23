#!/usr/bin/env bash
#
# Builds the ado-mcp binary into bin/ado-mcp. GOOS and GOARCH select another target platform.

set -euo pipefail

cd "$(dirname "$0")/.."
version="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"

CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${version}" -o bin/ado-mcp ./cmd/ado-mcp
echo "bin/ado-mcp ${version}"
