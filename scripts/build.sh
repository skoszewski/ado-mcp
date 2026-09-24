#!/usr/bin/env bash
#
# Builds the ado-mcp and create-pat binaries into bin/. GOOS and GOARCH select another target
# platform.

set -euo pipefail

cd "$(dirname "$0")/.."
version="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"

for command in ado-mcp create-pat; do
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${version}" -o "bin/${command}" "./cmd/${command}"
    echo "bin/${command} ${version}"
done
