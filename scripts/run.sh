#!/usr/bin/env bash
#
# Runs bin/ado-mcp over the stdio transport, for an MCP client that starts the server itself.
# Arguments are passed to ado-mcp; --transport http serves Streamable HTTP instead. The variables
# in .env in the repository root are exported to the server when the file exists.

set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
binary="${root}/bin/ado-mcp"

if [[ ! -x "${binary}" ]]; then
    echo "Error: ${binary} not found; run scripts/build.sh first" >&2
    exit 1
fi

# Exports the variables of the Docker environment file .env.
env_file="${root}/.env"
if [[ -f "${env_file}" ]]; then
    bom=$'\xef\xbb\xbf'
    first=1
    while IFS= read -r line || [[ -n "${line}" ]]; do
        if (( first )); then
            line="${line#"${bom}"}"
            first=0
        fi
        line="${line%$'\r'}"
        line="${line#"${line%%[![:space:]]*}"}"
        if [[ -z "${line}" || "${line}" == \#* ]]; then
            continue
        fi
        name="${line%%=*}"
        if [[ "${name}" =~ [[:space:]] ]]; then
            echo "Error: ${env_file}: variable '${name}' contains whitespace" >&2
            exit 1
        fi
        if [[ "${line}" == *=* ]]; then
            export "${name}=${line#*=}"
        fi
    done < "${env_file}"
fi

exec "${binary}" --transport stdio "$@"
