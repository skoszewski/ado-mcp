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
