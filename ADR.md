# Architecture Decision Records

## ADR-001: Port of the ado-pipeline-logs-analyzer MCP tools

**Decision:** The server implements the `ado_` tools of the Python `ado-pipeline-logs-analyzer`
MCP server with the same names, arguments, result fields, descriptions and error messages. The
GCP tools, AI analysis and CLI subcommands are not part of it.

**Reason:** Clients and prompts written against the Python server keep working, and the
model-facing text that was tuned there is reused unchanged.

## ADR-002: Azure Identity for Microsoft Entra tokens

**Decision:** Service principal and Azure CLI tokens are acquired through
`github.com/Azure/azure-sdk-for-go/sdk/azidentity` (`ClientSecretCredential`,
`AzureCLICredential`), which builds on MSAL for Go. A personal access token is sent as a Basic
header with an empty user name and needs no library.

**Reason:** The standard library has no equivalent of MSAL, and Microsoft advises against
implementing the token protocol directly.

**Consequence:** `AzureCLICredential` does not cache tokens, so the server keeps each token until
five minutes before it expires instead of running `az` on every request.

## ADR-003: Authentication selected from the environment

**Decision:** The first configured method wins: `AZURE_DEVOPS_PAT`, then the service principal
variables when all three are set, then the Azure CLI. There is no flag to choose the method.

**Reason:** The same binary runs unchanged locally and in a container, configured only by its
environment.

## ADR-004: No Azure CLI in the container image

**Decision:** The image is a static Go binary on `distroless/static`. Inside the container only
the personal access token and service principal methods work.

**Reason:** The Azure CLI would add a Python runtime and several hundred megabytes to an
otherwise minimal image, and its token cache would have to be mounted from the host.

## ADR-005: Stateless Streamable HTTP and stdio transports

**Decision:** The server offers stateless Streamable HTTP, the default and what the container
serves, and stdio for clients that start the server themselves.

**Reason:** No tool keeps state between calls, so stateless HTTP needs no session handling. stdio
covers local clients without a network listener.

## ADR-006: Tool defaults declared in the input schema

**Decision:** Argument defaults such as `top=25` or `strip_noise=true` are declared as JSON Schema
defaults, which the MCP SDK applies before decoding the arguments.

**Reason:** The client sees the defaults in the tool schema, and the handlers never have to tell
an omitted argument from a zero value.

## ADR-007: Container scripts prefer Docker, fall back to Apple container

**Decision:** `scripts/build.sh` and `scripts/run.sh` use `docker` when it is installed and Apple
`container` otherwise, starting the Apple container services when `container system status`
reports them stopped.

**Reason:** Docker is the common runtime, and Apple `container` is the only runtime on the
development host. For the options these scripts use (`build -t --build-arg`,
`run --rm -i -p -e --env-file`) the two command lines take the same form.

**Consequence:** Apple `container run` 1.4.1 does not forward SIGINT or SIGTERM to the container
("failed to send signal ... missing signal in xpc message"). For that runtime `scripts/run.sh`
names the container and stops it with `container stop` from a signal trap. The client runs as a
background child with stdin passed through explicitly, because bash runs a trap only after the
foreground command finishes and gives a background command `/dev/null` as stdin. The server
handles SIGINT and SIGTERM itself as PID 1, so the image needs no init process such as tini.

## ADR-008: Pipeline listing filtered by folder in the server

**Decision:** `ado_list_pipelines` pages through the definitions in `definitionNameAscending`
order and applies `folder_name` itself, case-insensitively and optionally recursively. Folder
paths are normalized to a leading backslash and backslash separators.

**Reason:** The API's `path` parameter matches one folder exactly and drops a definition whose
name is shared by another in the same folder. Only the name order pages through every
definition: a timestamp order drops rows, and a continuation token without an order is
rejected. Azure DevOps answers a folder path without the leading backslash with an empty list
rather than an error.

## ADR-009: Listings without top return every page

**Decision:** A listing tool called without `top` follows the continuation token to the end;
with `top` it returns that one page and its cursor.

**Reason:** The API's first page alone would silently truncate a large project or organization.

## ADR-010: small-model optimization returns the end of a log

**Decision:** Under `--optimize small-model`, `ado_get_run_log` returns the whole log, or its last
`--max-log-lines` lines.

**Reason:** A small model tends to answer from the first page it reads, and a failed step's error
is at the end of its log.
