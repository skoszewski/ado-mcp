# ado-mcp

An MCP server that gives an AI client read-only access to Azure DevOps pipeline inventory, run
history, run timelines and run logs, and to the Git repositories those runs build. It runs as a
binary or as a container, over Streamable HTTP or stdio.

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

With the default `--auth auto`, the first method whose environment variables are set is used:

1. `AZURE_DEVOPS_PAT` - a personal access token.
2. `AZURE_TENANT_ID`, `AZURE_CLIENT_ID` and `AZURE_CLIENT_SECRET` - a service principal client
   secret.
3. None of the above - the identity signed in to the Azure CLI (`az login`). The container image
   does not include the Azure CLI, so this method works only when the binary runs outside a
   container.

`--auth` forces one method when several are configured: `pat`, `service-principal` or
`azure-cli`. The server fails at startup when the forced method's variables are missing.
`--auth none` configures no credential; every call then needs an `Authorization` header from the
MCP client, described below.

The method in use is logged at startup. A personal access token needs these scopes:

- Project and Team: Read
- Build: Read
- Code: Read

A service principal or Azure CLI identity needs read access to the same areas in the
organization.

### Credentials sent by the MCP client

Over the HTTP transport, an `Authorization` header on the MCP request replaces the configured
method for that request. The server forwards the value unchanged to Azure DevOps, so it takes
one of the two forms Azure DevOps accepts:

- `Basic <value>` for a personal access token, where `<value>` is the Base64 encoding of the
  token preceded by a colon: `printf ':%s' "$AZURE_DEVOPS_PAT" | base64`.
- `Bearer <token>` for a Microsoft Entra access token issued for Azure DevOps.

A client without the header, and every client on stdio, uses the configured method; with
`--auth none` such a call fails. For example, in a Claude Code configuration:

```json
{
  "mcpServers": {
    "azure-devops": {
      "type": "http",
      "url": "http://127.0.0.1:8888/mcp",
      "headers": { "Authorization": "Basic <value>" }
    }
  }
}
```

### Creating a personal access token

`create-pat` creates a personal access token for the user signed in to the Azure CLI through the
Azure DevOps PAT Lifecycle Management API, and shows it with its scope, expiry and authorization
ID. With `--bare` it prints only the token on stdout.

```bash
create-pat -O myorg
create-pat -O myorg --days 7 --name ado-mcp-ci
create-pat -O myorg --full-scope
export AZURE_DEVOPS_PAT="$(create-pat -O myorg --bare)"
```

| Flag | Default | Meaning |
| --- | --- | --- |
| `--organization`, `-O` | | Organization name or `https://dev.azure.com/<org>` URL; required |
| `--name` | `ado-mcp` | Display name of the token |
| `--days` | `30` | Number of days the token is valid for |
| `--full-scope` | | Grant full access instead of the read-only scopes above |
| `--bare` | | Print only the token on stdout, for use in scripts |

The API accepts only a user identity, not a service principal. Organization policies can
restrict PAT creation, scopes and lifetime.

## Running

```bash
ado-mcp
ado-mcp --transport stdio
```

The first form serves Streamable HTTP at `http://127.0.0.1:8888/mcp`; the second serves stdio for
an MCP client that starts the server itself. The container image runs `ado-mcp` bound to
`0.0.0.0`:

```bash
docker run --rm -p 127.0.0.1:8888:8888 -e AZURE_DEVOPS_PAT ado-mcp:latest
```

### Kubernetes

The Helm chart in `charts/ado-mcp` runs the container image as a Deployment behind a Service.
The Deployment and the Service are named after the Helm release. The server reads its
credentials from a Kubernetes Secret, which the chart does not create; it must exist in the
namespace before the release is installed. The endpoint has no authentication of its own, so
anyone who can reach the Service uses the credentials in the Secret.

| Value | Default | Meaning |
| --- | --- | --- |
| `image` | `ado-mcp:latest` | Container image; it must be pullable by the cluster |
| `secretName` | `ado-mcp` | Secret holding `AZURE_DEVOPS_PAT`, or `AZURE_TENANT_ID`, `AZURE_CLIENT_ID` and `AZURE_CLIENT_SECRET`; every key becomes an environment variable of the server |
| `port` | `8888` | Port the server listens on and the Service exposes |
| `serviceType` | `LoadBalancer` | Service type, e.g. `ClusterIP` for access from inside the cluster only |

Create the Secret from a Docker environment file, then install the chart:

```bash
kubectl create secret generic ado-mcp --from-env-file=.env
helm install ado-mcp charts/ado-mcp --set image=<registry>/ado-mcp:latest
```

A values file overrides the defaults in `charts/ado-mcp/values.yaml`; values it omits keep their
defaults:

```yaml
# my-values.yaml
image: <registry>/ado-mcp:1.0.0
secretName: ado-mcp-credentials
port: 9000
```

```bash
helm install ado-mcp charts/ado-mcp -f my-values.yaml
```

#### Two servers with different credentials

Each Helm release is an independent server, so one release per identity gives two servers that
reach different Azure DevOps organizations or run with different permissions. Give each release
its own Secret, and its own `port`, because two `LoadBalancer` Services in one cluster can
contend for the same port:

```bash
kubectl create secret generic ado-mcp-contoso --from-env-file=contoso.env
kubectl create secret generic ado-mcp-fabrikam --from-env-file=fabrikam.env

helm install ado-mcp-contoso charts/ado-mcp --set secretName=ado-mcp-contoso --set port=8888
helm install ado-mcp-fabrikam charts/ado-mcp --set secretName=ado-mcp-fabrikam --set port=8889
```

The endpoints are then `http://<contoso-address>:8888/mcp` and `http://<fabrikam-address>:8889/mcp`,
where `kubectl get service` reports each address. Upgrade or remove one server with its release
name, e.g. `helm uninstall ado-mcp-fabrikam`; the Secrets are not part of a release and stay
until deleted with `kubectl delete secret`.

## Development scripts

The `scripts` directory of the repository holds helpers for development.

Building the binaries requires Go 1.27.

```bash
scripts/build.sh
scripts/run.sh
```

`scripts/build.sh` builds `bin/ado-mcp` and `bin/create-pat` in the repository; `GOOS` and
`GOARCH` select another target platform. `scripts/run.sh` runs `bin/ado-mcp` over the stdio
transport and passes its arguments to it, so `scripts/run.sh --transport http` serves
`http://127.0.0.1:8888/mcp` instead. It exports the variables in the `.env` file in the repository
root to the server when that file exists. `.env` is a Docker environment file.

The Bash container scripts use Docker when it is installed and Apple `container` otherwise.

```bash
scripts/build_container.sh
scripts/run_container.sh
scripts/run_container.sh --transport stdio
```

`scripts/build_container.sh` builds the `ado-mcp:latest` image for the host's architecture;
`ARCH` set to `amd64`, `arm64` or `amd64,arm64` selects others, e.g.
`ARCH=amd64,arm64 scripts/build_container.sh` for a multi-architecture image. `scripts/run_container.sh`
publishes the server on `127.0.0.1:8888` and passes its arguments to `ado-mcp`. It passes
credentials to the container from the `.env` file in the repository root when that file exists,
and from the authentication variables above when they are set in the calling shell. `IMAGE`
overrides the image name (`ado-mcp:latest`) and `PORT` the host port.

PowerShell 7 scripts do the same on any platform, with Docker as the container runtime:

```powershell
scripts/build.ps1
scripts/run.ps1
scripts/build_container.ps1
scripts/run_container.ps1
```

`scripts/build.ps1` gives binaries built for Windows the `.exe` extension.

## Flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `--transport` | `http` | `http` (Streamable HTTP) or `stdio` |
| `--auth` | `auto` | `auto`, `pat`, `service-principal`, `azure-cli` or `none`; see Authentication |
| `--host` | `127.0.0.1` | Address the HTTP server binds to; the container image sets `0.0.0.0` |
| `--port` | `8888` | HTTP port |
| `--path` | `/mcp` | HTTP path of the MCP endpoint |
| `--max-log-lines` | `2000` | Maximum lines `ado_get_run_log` and `ado_get_repository_item` return in one call |
| `--optimize` | | `small-model` makes `ado_get_run_log` return the whole log or its last `--max-log-lines` lines; `log-type=job\|task\|all` sets the log type `ado_list_run_logs` lists by default (`job`) |
| `--debug[=N]` | `0` | `1` logs tool calls, `2` adds incoming HTTP requests, `3` adds the MCP library's own logging |
| `--log-style` | `auto` | `human`, `daemon`, or `auto` for `human` when stdout is a terminal; see Logging |

## Logging

`ado-mcp` logs to stderr in one of two styles. `human` starts with a banner listing the version,
authentication method, tools and endpoint, and writes one readable line per event, colored when
stderr is a terminal. `daemon` writes timestamped `key=value` records for a container runtime,
service manager or MCP client to collect. `--log-style auto`, the default, uses `human` when
stdout is a terminal and `daemon` otherwise.

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

stdio, with the binary:

```json
{
  "mcpServers": {
    "azure-devops": {
      "command": "/usr/local/bin/ado-mcp",
      "args": ["--transport", "stdio"],
      "env": { "AZURE_DEVOPS_PAT": "<token>" }
    }
  }
}
```

stdio, with the container image; `-e AZURE_DEVOPS_PAT` passes the token set in `env` into the
container:

```json
{
  "mcpServers": {
    "azure-devops": {
      "command": "docker",
      "args": ["run", "--rm", "-i", "-e", "AZURE_DEVOPS_PAT", "ado-mcp:latest", "--transport", "stdio"],
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
