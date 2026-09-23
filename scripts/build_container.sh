#!/usr/bin/env bash
#
# Builds the ado-mcp container image with Docker, or with Apple container when Docker is not
# installed. IMAGE overrides the image name (default: ado-mcp:latest).

set -euo pipefail

cd "$(dirname "$0")/.."
image="${IMAGE:-ado-mcp:latest}"
version="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"

if command -v docker >/dev/null 2>&1; then
    docker build --build-arg "VERSION=${version}" -t "${image}" .
elif command -v container >/dev/null 2>&1; then
    # Apple container needs its background services running before any other command.
    container system status >/dev/null 2>&1 || container system start
    container build --build-arg "VERSION=${version}" -t "${image}" .
else
    echo "Error: neither docker nor Apple container is installed" >&2
    exit 1
fi
