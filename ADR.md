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

**Amendment:** `--auth` (default `auto`, the behavior above) forces `pat`, `service-principal` or
`azure-cli` when several are configured, or `none` to hold no credential and rely on the
`Authorization` header of ADR-014. A forced method whose variables are missing fails at startup
instead of falling through to another identity.

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

**Decision:** `scripts/build_container.sh` and `scripts/run_container.sh` use `docker` when it is installed and Apple
`container` otherwise, starting the Apple container services when `container system status`
reports them stopped.

**Reason:** Docker is the common runtime, and Apple `container` is the only runtime on the
development host. For the options these scripts use (`build -t --build-arg`,
`run --rm -i -p -e --env-file`) the two command lines take the same form.

**Consequence:** Apple `container run` 1.4.1 does not forward SIGINT or SIGTERM to the container
("failed to send signal ... missing signal in xpc message"). For that runtime `scripts/run_container.sh`
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

## ADR-011: Azure DevOps errors rewritten into guidance for the model

**Decision:** A tool error from Azure DevOps reaches the client as a message that names the
cause and the next step: a missing branch, tag or commit (`TF401175`), a missing path
(`TF401174`), a refusal for the authenticated identity (401, 403, or 203 with a sign-in page),
and a missing resource (404, or `TF200016` for a project).

**Reason:** Azure DevOps decides what the server can reach and answers a resource the identity may
not see the same way as one that does not exist. A bare status line leaves the model to guess, and
a generic not-found message sends it looking for a wrong repository name when only the branch is
missing.

## ADR-012: No repository size in ado_list_repositories

**Decision:** `ado_list_repositories` does not report the repository size.

**Reason:** Azure DevOps reports a size of 0 for repositories that hold files, which reads as an
empty repository.

## ADR-013: Human or daemon log style

**Decision:** `ado-mcp` logs in a human style, a banner and one colored line per event, or in a
daemon style, timestamped `slog` text records. `--log-style` chooses one; its default, `auto`,
chooses human when stdout is a terminal. Terminals are detected with `golang.org/x/term`.

**Reason:** A person running the server in a terminal reads its output directly, while a
container runtime, a service manager or an MCP client on the stdio transport collects it and
needs timestamps and levels. A character-device check is not enough to detect a terminal:
`/dev/null` is a character device. The flag covers cases detection gets wrong, such as a
terminal multiplexer or a log collector attached to a pseudo-terminal.

## ADR-014: Authorization header of the MCP request overrides the configured method

**Decision:** When the HTTP request carrying a tool call has an `Authorization` header, the
server sends its value unchanged as the `Authorization` header of that call's Azure DevOps
requests, in place of the method selected from the environment. The header is not logged.

**Reason:** Different organizations and users need different credentials, while the configured
method gives every client the server's single identity. The standard header name is one that
MCP clients can set, and Zed starts an OAuth flow for a remote server unless it is configured.
Forwarding the value unchanged supports both forms Azure DevOps accepts, `Basic` for a personal
access token and `Bearer` for a Microsoft Entra token, without the server parsing credentials.

## ADR-015: Tools for diagnosing Terraform deployment runs

**Decision:** The server adds tools for what a failed Terraform run's logs refer to but do not
contain: the commits between runs, the files that changed, the expanded pipeline YAML, service
connections, variable groups, environments and their deployments, run artifacts, pull requests
with their threads and policy evaluations, agent pools and agents, refs, and code search.
`ado_get_diff` reports changed paths only. Approvals and checks, agent job requests, work
items, classic release pipelines and test results have no tools.

**Reason:** The organizations served deploy Terraform with YAML pipelines and use neither work
items, classic pipelines nor Azure DevOps test management. The diffs API returns paths and
object IDs, not line differences, and the model can read both versions of a file with
`ado_get_repository_item`. The approvals API cannot be queried by run and check evaluations need
a check suite ID the documentation does not say how to obtain; the documented job request API
covers agent clouds only.

**Consequence:** The environment APIs accept only the `vso.environment_manage` scope, a
high-privilege scope that also manages agent pools, queues and agents, so the token
`create-pat` creates can manage them. A token without a tool's scope gets the
access-denied guidance of ADR-011 for that tool alone.
