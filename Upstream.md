# Upstream comparison: microsoft/azure-devops-mcp

This document compares the local `azure-devops-mcp` server (TypeScript, stdio only) with
`ado-mcp`. The tool list comes from `docs/TOOLSET.md` of the upstream repository, and the tool
names were checked against `src/tools/*.ts`. The upstream `pipelines.ts` was read in full; the
other domain files were checked for registered tool names only.

## Upstream tools

Most upstream tools are consolidated: one tool takes an `action` argument, and the `*_write`
variants hold the mutating actions.

| Domain | Tools | Actions |
| --- | --- | --- |
| core | `core_list_projects`, `core_list_project_teams`, `core_get_identity_ids` | none |
| work | `work`, `work_iteration_write`, `work_capacity_write` | list iterations, team iterations, team settings, capacities; create and assign iterations; update capacity |
| work-items | `wit_work_item`, `wit_work_item_write`, `wit_work_item_comment_write`, `wit_work_item_link_write`, `wit_query`, `wit_backlog`, `wit_work_item_attachment` | get, batch get, comments, revisions, my items, create, update, child creation, links (including PR and artifact links), saved queries, ad-hoc WIQL, backlog listing and reorder, attachment download |
| repositories | `repo_repository`, `repo_pull_request`, `repo_pull_request_org`, `repo_pull_request_thread`, `repo_branch`, `repo_file`, `repo_search_commits`, `repo_pull_request_write`, `repo_pull_request_thread_write`, `repo_create_branch` | repository and branch reads, PR and thread reads, file content and directory listing, commit search; create, update and vote on PRs, comment on threads, create branches |
| pipelines | `pipelines_build`, `pipelines_build_log`, `pipelines_definition`, `pipelines_run`, `pipelines_artifact`, `pipelines_write` | build list, status and changes; log list and content by line range; definition list and revisions; run get and list; artifact list and download; run, create and rename pipelines; cancel, retry or run a build stage |
| test-plans | `testplan`, `testplan_show_test_results_from_build_id`, `testplan_test_plan_write`, `testplan_test_suite_write`, `testplan_test_case_write` | list plans, suites and cases; build test results; create plans, suites and cases; update case steps |
| wiki | `wiki`, `wiki_upsert_page` | list wikis, get wiki, list pages, page metadata and content; create or update a page |
| search | `search_code`, `search_wiki`, `search_workitem` | none |
| advanced-security | `advsec_get_alerts`, `advsec_get_alert_details` | none |
| mcp-apps | `mcp_apps_ping` | health check |

TOOLSET.md prefixes some tools with `mcp_ado_`, for example `mcp_ado_core_list_projects`. The
names registered in code have no such prefix.

## Upstream server characteristics

- Transport: stdio only.
- Configuration: the organization is a positional CLI argument.
- Authentication: `interactive` (default), `azcli`, `env`, `envvar` or `pat`, chosen with `-a`.
- Domains: `-d` selects which domains load.
- Defaults: project and team defaults come from the `ado_mcp_project` and `ado_mcp_team`
  environment variables.
- Untrusted content: responses from every domain pass through a content-safety wrapper
  (`wrapExternalToolResponse`).
- Direction: the upstream README recommends the hosted remote MCP server (`mcp.dev.azure.com`)
  over the local one.

## Comparison with ado-mcp

`ado-mcp` has 14 `ado_*` tools, all read-only: projects, folders, pipelines, runs, run timeline,
run logs, repositories, repository items, commits and commit changes.

| Area | azure-devops-mcp | ado-mcp |
| --- | --- | --- |
| Scope | Most of Azure DevOps: work items, PRs, wiki, test plans, search, security | Pipelines and Git read access |
| Writes | Work items, PRs, branches, wiki, pipelines, test plans, iterations | None |
| Tool shape | Few tools with an `action` enum and many optional arguments | One tool per operation with a narrow schema |
| Pipeline runs | `pipelines_run` list takes only project and pipeline ID; no status, result, branch or time filters, no paging | `ado_list_runs` filters by status, result, branch, time and reason, works project-wide without a pipeline, pages by cursor |
| Pipeline lookup | `pipelines_definition` filters by many fields | `ado_get_pipeline` takes a name or an ID |
| Build listing | `pipelines_build` list has many filters and returns the raw API JSON | `ado_list_runs` returns a summarized result |
| Run timeline | Not exposed | `ado_get_run_timeline` lists stages, jobs and steps with log IDs and results |
| Logs | Line-range read of a raw log; no stripping of ANSI codes, timestamps or blank lines | Paged by line, capped by `--max-log-lines`, `strip_noise`; log types `job`, `task`, `all`; small-model mode returns the log tail |
| Pipeline folders | Filter by `path` | `ado_list_folders`; `folder_name` filtering in the server (ADR-008) |
| Git reads | Repositories, branches, file content, directory listing, commit search | Repositories, file pages with line paging and binary detection, tree listing, commits with `next_skip` paging, per-commit changes |
| Git gaps | No commit-changes tool; no tag or commit `ref_type` on file reads | No branch list, PR or code search tools |
| Artifacts and stages | Artifact list and download; stage cancel, retry or run | Not covered |
| Test results | Test plans and results by build | Not covered |
| Output | Raw `JSON.stringify` of SDK objects | Shaped structs echoing `organization` and `project` in every result |
| Errors | Message prefix plus the SDK error text | Azure DevOps codes (`TF401175`, `TF401174`, 401, 403, 404) rewritten into guidance for the model (ADR-011) |
| Organization | One organization fixed at startup | Per-call `organization` argument |
| Authentication | Interactive, Azure CLI, environment variables, PAT | `AZURE_DEVOPS_PAT`, then service principal, then Azure CLI, chosen from the environment; `create-pat` CLI |
| Transport | stdio | Streamable HTTP (default) and stdio |
| Deployment | npm package run with `npx`; no container image | Static Go binary and distroless container image |
| Logging | Logger with levels | Human and daemon log styles, `--debug` levels |

## Summary

`azure-devops-mcp` covers far more of Azure DevOps and can change data. `ado-mcp` covers a
narrow read-only slice in more depth.

Upstream has no run timeline, no commit-changes tool, no log noise stripping and no error
rewriting.

`ado-mcp` has no work item, PR, wiki, test plan, search, security, artifact or write tools.
