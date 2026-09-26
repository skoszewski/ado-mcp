# Upstream comparison: microsoft/azure-devops-mcp

Microsoft ships the Azure DevOps MCP Server in two forms:

- **Local server** - the TypeScript package in `microsoft/azure-devops-mcp`, run with `npx`
  over stdio. Its tool list comes from `docs/TOOLSET.md` of that repository, with tool names
  checked against `src/tools/*.ts`; `pipelines.ts` was read in full, the other domain files for
  registered tool names only.
- **Remote server** - a hosted endpoint at `https://mcp.dev.azure.com/{organization}` over
  Streamable HTTP. Its description comes from Microsoft Learn: "Set up the remote Azure DevOps
  MCP Server" and "Troubleshoot the remote Azure DevOps MCP Server". The code of the hosted
  service is not public.

Microsoft recommends the remote server wherever the client can authenticate to it.

## Upstream tools

Most tools are consolidated: one tool takes an `action` argument, and `*_write` tools hold the
mutating actions. The remote server exposes a curated, independently versioned set of the local
tools; Microsoft notes the two can differ.

| Domain | Tools | Actions |
| --- | --- | --- |
| core | `core_list_projects`, `core_list_project_teams`; local: `core_get_identity_ids`; remote: `core_list_orgs`, `core_list_group_members` (Insiders) | none |
| work | `work`, `work_iteration_write`, `work_capacity_write` | iterations, team settings, capacities; create and assign iterations; update capacity |
| work items | `wit_work_item`, `wit_work_item_write`, `wit_work_item_comment_write`, `wit_work_item_link_write`, `wit_query`, `wit_backlog`, `wit_work_item_attachment`, `search_workitem`; remote: `wit_query_by_wiql` (Insiders) | get, batch, comments, revisions, my items, create, update, links, queries, WIQL, backlogs, attachments |
| repositories | `repo_repository`, `repo_pull_request`, `repo_pull_request_thread`, `repo_branch`, `repo_file`, `repo_search_commits`, `search_code`, `repo_pull_request_write`, `repo_pull_request_thread_write`, `repo_create_branch`; local: `repo_pull_request_org` | repository and branch reads; PR get, list, list by commits, iteration changes with optional line diffs; threads and comments; file content and directory listing; commit search; create, update and vote on PRs, comment, create branches |
| pipelines | `pipelines_build`, `pipelines_build_log`, `pipelines_definition`, `pipelines_run`, `pipelines_artifact`, `pipelines_write` | build list, status and changes; log list and content; definition list and revisions; run get and list; artifact list and download; queue runs, create pipelines, cancel, retry or run a stage |
| test plans | `testplan`, `testplan_show_test_results_from_build_id`, `testplan_test_plan_write`, `testplan_test_suite_write`, `testplan_test_case_write`; remote: `testplan_test_run`, `testplan_test_run_write` | plans, suites, cases, build test results, test runs; create and update |
| wiki | `wiki`, `wiki_upsert_page`, `search_wiki` | list and read wikis and pages; create or update a page |
| advanced security | local: `advsec_get_alerts`, `advsec_get_alert_details`; remote: `advsec_alerts` | list and get alerts |
| migrations | remote: `enterprise_live_migration`, `enterprise_live_migration_write`, `enterprise_live_migration_pipelines_write` (preview) | Enterprise Live Migration status and control |
| other | local: `mcp_apps_ping` | health check |

## Upstream server characteristics

| | Local server | Remote server |
| --- | --- | --- |
| Transport | stdio | Streamable HTTP |
| Organization | Positional CLI argument, one per process | In the URL, or given per call when the URL omits it |
| Authentication | `-a`: `interactive` (default), `azcli`, `env`, `envvar` or `pat` | Microsoft Entra ID OAuth only; no PATs, no Microsoft-account organizations |
| Clients | Any stdio client | VS Code, Visual Studio, Foundry, Copilot Studio, GitHub Copilot CLI and app; Cursor and Claude Code only with a custom Entra app registration; not Claude Desktop or Codex |
| Tool selection | `-d` selects domains | `X-MCP-Toolsets` or `X-MCP-Tools` headers; `X-MCP-Readonly: true` drops write tools; `X-MCP-Insiders` for preview tools |
| Defaults | `ado_mcp_project`, `ado_mcp_team` environment variables | none documented |
| Untrusted content | Every response passes through `wrapExternalToolResponse` | not documented |
| Hosting | `npx`, Node.js 20+ | Hosted by Azure DevOps; Conditional Access applies |

## ado-mcp

30 `ado_*` tools, all read-only:

- Inventory and runs: projects, folders, pipelines, runs, run details, timeline, logs, run
  changes (alone or between two runs), run artifacts, expanded pipeline YAML.
- Repositories: repositories, file pages, directory listing, commits, commit changes, diffs
  between two versions (paths only), refs with tag commits, pull requests (by commit or by
  status and branch), PR threads, PR policy evaluations, code search.
- Pipeline configuration: service connections and their usage history, variable groups,
  environments and their deployments, agent pools and agents with capabilities.

Server: Streamable HTTP (default) and stdio; organization and project are per-call arguments;
authentication from the environment (`AZURE_DEVOPS_PAT`, service principal, Azure CLI) or forced
with `--auth`, `--auth none` for no server credential; an `Authorization` header on the MCP
request replaces the configured credential for that request (ADR-014). Static Go binary,
distroless multi-architecture container image, minimal Helm chart.

## Comparison

| Area | Local server | Remote server | ado-mcp |
| --- | --- | --- | --- |
| Scope | Most of Azure DevOps | Most of Azure DevOps, plus migrations | Pipelines, their configuration, and Git read access |
| Writes | Work items, PRs, branches, wiki, pipelines, test plans, iterations | Same; can be disabled with `X-MCP-Readonly` | None |
| Tool shape | Few tools with an `action` enum | Same | One tool per operation with a narrow schema |
| Run listing | `pipelines_run` list takes only project and pipeline ID, no filters or paging | Not documented beyond "list runs for a pipeline" | `ado_list_runs` filters by status, result, branch, time and reason, spans a project without a pipeline, pages by cursor |
| Run timeline | Not exposed | Not exposed; `pipelines_build` `get_status` reports issues | `ado_get_run_timeline` with stages, jobs, steps, results and log IDs |
| Logs | Line-range read of a raw log | Log content by ID | Paged by line, capped by `--max-log-lines`, `strip_noise`, log types, small-model tail |
| What changed | Build changes | Build changes | Run changes, or every commit between two runs |
| Diffs | No commit-changes tool | PR iteration changes with optional line diffs | Commit changes and path-level diffs between any two versions; no line diffs |
| Pull requests | Read and write | Read and write, lookup by commit | Read: lookup by commit or filters, threads, policy evaluations |
| Expanded YAML | Not exposed | Not exposed | `ado_get_pipeline_yaml` |
| Service connections, variable groups, environments, agents | Not exposed | Not exposed | Read tools for each |
| Artifacts | List and download | List and download | List only |
| Work items, wiki, test plans, security alerts | Yes | Yes | Not covered |
| Code search | `search_code` | `search_code` | `ado_search_code` |
| Output | Raw `JSON.stringify` of SDK objects | not documented | Shaped results echoing `organization` and `project` |
| Errors | Message prefix plus SDK error text | not documented | Azure DevOps codes and 401, 403, 203, 404 rewritten into guidance for the model (ADR-011) |
| Organizations | One per process | One per URL, or per call | Per call |
| Credentials | Chosen at startup | Entra sign-in of the user | Chosen at startup, or per request from the client's `Authorization` header (PAT or Entra token) |
| Clients | Any stdio client | Entra-capable clients; Claude Code with an app registration | Any Streamable HTTP or stdio client; a PAT in a header works with clients that cannot do Entra OAuth |
| Deployment | `npx` | Hosted | Binary, container image, Helm chart |

## Summary

Microsoft's servers cover far more of Azure DevOps and can change data. The remote server is
Microsoft's preferred form, but it accepts only Entra OAuth, which excludes PATs, Claude Desktop
and Codex, and needs an app registration for Claude Code.

`ado-mcp` is read-only and aimed at diagnosing failed runs. It covers what the upstream servers
do not expose: run timelines, log noise stripping, commits between runs, expanded pipeline YAML,
service connections, variable groups, environments and agents, and errors rewritten for the
model. It runs where the remote server cannot, taking a PAT or Entra token per request.

`ado-mcp` has no work item, wiki, test plan, security alert, artifact download or write tools,
and no line-level diffs.
