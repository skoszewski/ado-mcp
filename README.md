# ado-mcp

An MCP server that gives an AI client read-only access to Azure DevOps pipeline inventory, run
history, run timelines and run logs, and to the Git repositories those runs build. It runs as a
local binary or as a container, over Streamable HTTP or stdio.

## Tools

| Tool | Purpose |
| --- | --- |
| `ado_list_projects` | Projects in an organization |
| `ado_list_folders` | Pipeline folders in a project |
| `ado_list_pipelines` | Pipelines in a project, optionally in one folder |
| `ado_get_pipeline` | One pipeline by name or ID |
| `ado_list_runs` | Runs of a pipeline or a whole project, newest first, paged by cursor |
| `ado_get_run` | One run's status, result, timing, repository and commit |
| `ado_get_run_timeline` | A run's stages, jobs and steps with results and log IDs |
| `ado_list_run_logs` | The logs a run produced |
| `ado_get_run_log` | One page of a run log |
| `ado_list_repositories` | Git repositories in a project |
| `ado_get_repository_item` | One page of a file, at a branch, tag or commit |
| `ado_list_repository_items` | Files and folders under a path |
| `ado_list_commits` | Commits, optionally touching one path, paged by skip |
| `ado_get_commit_changes` | Paths one commit changed |

## Authentication

The first method whose environment variables are set is used:

1. `AZURE_DEVOPS_PAT` - a personal access token.
2. `AZURE_TENANT_ID`, `AZURE_CLIENT_ID` and `AZURE_CLIENT_SECRET` - a service principal client
   secret.
3. None of the above - the identity signed in to the Azure CLI (`az login`). The container image
   does not include the Azure CLI, so this method works only with the local binary.

The method in use is printed at startup. A personal access token needs these scopes:

- Project and Team: Read
- Build: Read
- Code: Read

A service principal or Azure CLI identity needs read access to the same areas in the
organization.

## Running locally

Requires Go 1.27.

```bash
go build -o ado-mcp ./cmd/ado-mcp
./ado-mcp -O myorg
```

The MCP endpoint is then `http://127.0.0.1:8888/mcp`. For a client that starts the server
itself, use the stdio transport:

```bash
./ado-mcp --transport stdio -O myorg
```

## Running in a container

The scripts use Docker when it is installed and Apple `container` otherwise.

```bash
scripts/build.sh
scripts/run.sh -O myorg
```

`scripts/run.sh` publishes the server on `127.0.0.1:8888` and passes its arguments to
`ado-mcp`. Credentials are read from `.env` in the repository root when it exists, as plain
`NAME=value` lines, and from the authentication variables above when they are set in the
calling shell. `IMAGE` overrides the image name (`ado-mcp:latest`) and `PORT` the host port.

## Flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `--organization`, `--org`, `-O` | | Serve this organization only, by name or `https://dev.azure.com/<org>` URL |
| `--project`, `-p` | | Serve this project only |
| `--folder-name`, `-f` | | Serve this pipeline folder and the folders below it only |
| `--pipeline`, `-n` | | Serve this pipeline only, by name or ID; requires `-O` and `-p` |
| `--transport` | `http` | `http` (Streamable HTTP) or `stdio` |
| `--host` | `127.0.0.1` | Address the HTTP server binds to; the container image sets `0.0.0.0` |
| `--port` | `8888` | HTTP port |
| `--path` | `/mcp` | HTTP path of the MCP endpoint |
| `--max-log-lines` | `2000` | Maximum lines `ado_get_run_log` and `ado_get_repository_item` return in one call |
| `--optimize` | | `small-model` makes `ado_get_run_log` return the whole log or its last `--max-log-lines` lines; `log-type=job\|task\|all` sets the log type `ado_list_run_logs` lists by default (`job`) |
| `--debug[=N]` | `0` | `1` logs tool calls, `2` adds incoming HTTP requests, `3` adds the MCP library's own logging |

The scope flags are fixed for the server's lifetime. They are announced to the client at
initialize, and every call is checked against them: a call naming anything outside the scope
is refused.

## Client configuration

Streamable HTTP, for a client that supports it:

```json
{
  "mcpServers": {
    "azure-devops": {
      "type": "http",
      "url": "http://127.0.0.1:8888/mcp"
    }
  }
}
```

stdio:

```json
{
  "mcpServers": {
    "azure-devops": {
      "command": "/path/to/ado-mcp",
      "args": ["--transport", "stdio", "-O", "myorg"],
      "env": { "AZURE_DEVOPS_PAT": "<token>" }
    }
  }
}
```

## Development

```bash
go vet ./...
go test ./...
```
