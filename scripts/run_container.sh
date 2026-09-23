#!/usr/bin/env bash
#
# Runs the ado-mcp container image with Docker, or with Apple container when Docker is not
# installed. Arguments are passed to ado-mcp. Credentials come from .env in the repository
# root when it exists, and from AZURE_DEVOPS_PAT, AZURE_TENANT_ID, AZURE_CLIENT_ID and
# AZURE_CLIENT_SECRET when they are set. IMAGE overrides the image name (default:
# ado-mcp:latest) and PORT the host port published on 127.0.0.1 (default: 8888).

set -euo pipefail

cd "$(dirname "$0")/.."
image="${IMAGE:-ado-mcp:latest}"
port="${PORT:-8888}"

run_args=(run --rm -i -p "127.0.0.1:${port}:8888")
if [[ -f .env ]]; then
    run_args+=(--env-file .env)
fi
for name in AZURE_DEVOPS_PAT AZURE_TENANT_ID AZURE_CLIENT_ID AZURE_CLIENT_SECRET; do
    if [[ -n "${!name:-}" ]]; then
        run_args+=(-e "${name}")
    fi
done

if command -v docker >/dev/null 2>&1; then
    exec docker "${run_args[@]}" "${image}" "$@"
elif command -v container >/dev/null 2>&1; then
    # Apple container needs its background services running before any other command.
    container system status >/dev/null 2>&1 || container system start

    # Stops the container on INT and TERM.
    name="ado-mcp-$$"
    trap 'container stop "${name}" >/dev/null' INT TERM
    container "${run_args[0]}" --name "${name}" "${run_args[@]:1}" "${image}" "$@" <&0 &
    client=$!
    status=0
    while kill -0 "${client}" 2>/dev/null; do
        wait "${client}" || status=$?
    done
    exit "${status}"
else
    echo "Error: neither docker nor Apple container is installed" >&2
    exit 1
fi
