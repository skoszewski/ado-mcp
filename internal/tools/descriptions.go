package tools

const listProjectsDescription = `List the projects in an Azure DevOps organization.

This is the entry point when nothing is known yet: its projects are what the project
argument of every other tool takes.`

const listFoldersDescription = `List the pipeline folders in a project. Pipelines are grouped into these folders.

Each path it reports is what ado_list_pipelines expects as its folder_name, verbatim.`

const listPipelinesDescription = `List the pipelines in a project, optionally only those in one folder.

Keep each pipeline's id: passing it as the pipeline argument of the other tools is
cheaper than passing a name, which has to be looked up against the whole project.`

const getPipelineDescription = `Look up one pipeline by name or ID.`

const listRunsDescription = `List runs, most recently started first, one page at a time.

With a pipeline, this lists that pipeline's runs; without one, the whole project's runs
across every pipeline it holds. Runs accumulate without limit, so this never returns all
of them: it returns one page and, while older runs remain, a next_cursor to read the
following page. Reach further back by following that cursor, not by raising top.

This is how to find a recent failure when the pipeline is not known: call it with no
pipeline and result="failed", and each run comes back naming the pipeline it belongs to.
ado_list_pipelines cannot answer that -- it reports what exists, not how anything ran.`

const getRunDescription = `Get one run's status, result, timing and source details.`

const getRunTimelineDescription = `List a run's stages, jobs and steps with their results, durations and log IDs.

This is how to find the step that failed, and the log_id to read with ado_get_run_log.`

const listRunLogsDescription = `List the logs a run produced, with the ID and line count of each.`

const getRunLogDescription = `Read one page of a run's log, given a log_id from ado_list_run_logs or ado_get_run_timeline.

Logs run to tens of thousands of lines, so this returns one page at a time. The result
carries next_start_line: while it holds a number, more of the log follows, and passing it
back as start_line (every other argument unchanged) returns the next page; it is null on
the last page. Nothing has to be worked out to continue -- the line to resume from is
handed to you, the same way a listing result hands back next_cursor.

A failed step's error is at the end of its log, so keep following next_start_line to the
end rather than answering from the first page: the beginning of a build log is setup and
routine output, and a page that shows no error means the error is further down, not that
there was none. Under the server's small-model optimization this returns the whole log,
or its last lines when it is too long, and start_line/line_count are ignored; the
result's start_line reports where what came back begins either way.`

const listRepositoriesDescription = `List the Git repositories a project holds, with their IDs and default branches.

This is how to find the repository name the other repository tools take, when a run has
not already named one. A run reports the repository it built as repository_name in
ado_get_run, so prefer that when investigating a specific run.`

const getRepositoryItemDescription = `Read one page of a file's content from a Git repository.

This is how to read the source behind a failed run: a log that names a file, a Terraform
module or a pipeline YAML template can be opened here rather than inferred from the error
text. ado_get_run reports the repository_name a run built and the source_version it built,
so passing that source_version as ref with ref_type="commit" reads the file exactly as
that run saw it, not as it stands today.

Content is paged by line the same way ado_get_run_log is: while next_start_line holds a
number more of the file follows, and passing it back as start_line returns the next page.

A directory is not readable here -- use ado_list_repository_items for one. Binary files
report is_binary with no content.`

const listRepositoryItemsDescription = `List the files and folders under a path in a Git repository.

Use this to find what a repository holds before reading a file with
ado_get_repository_item, and whenever a path from a log turns out to be a directory.`

const listCommitsDescription = `List a repository's commits, newest first, optionally only those touching one path.

This is how to answer when a change landed and who made it -- pass item_path to get one
file's history rather than the whole repository's, which is what turns "this line is
wrong" into "this commit changed it".

This list pages on skip rather than on a cursor: pass next_skip back as skip, with every
other argument unchanged, to read the next page. It is null once a page comes back short
of top, which is how this API reports the end -- a last page that happens to be exactly
full still carries a next_skip, so the call after it can legitimately return nothing.`

const getCommitChangesDescription = `List the paths one commit changed.

Use this once ado_list_commits has identified a commit worth looking at, to see what it
touched before reading any of those files with ado_get_repository_item.`

const listRunChangesDescription = `List the commits a run built: the changes it picked up since the previous run of its pipeline.

This answers "what changed" for a failed run. With from_run_id set to an earlier run, typically
the last successful one found with ado_list_runs, it lists every commit made between the two
instead, which is the set of suspects when a pipeline went from green to red. Follow up with
ado_get_diff between the two runs' source_version commits to see which files changed.`

const getDiffDescription = `List the files that differ between two versions of a repository, such as the commits of the last successful run and of the failed one.

It reports paths and change types, not line differences: read a changed file at each version
with ado_get_repository_item (ref set to the commit, ref_type="commit") to compare the content.
Folders are left out. The list pages on skip: pass next_skip back as skip, with every other
argument unchanged, while it holds a number.`

const getPipelineYAMLDescription = `Return a pipeline's final YAML with every template expanded, as Azure DevOps would run it, without queuing a run.

Use this when a failure depends on what a shared template produced -- which stages, variables,
service connections, backend settings or Terraform working directories a stage got -- rather
than on the pipeline file alone. The YAML is paged by line like ado_get_repository_item.`

const listRunArtifactsDescription = `List the artifacts a run published, such as a Terraform plan file its plan stage stored for the apply stage.

Each artifact reports source, the ID of the job that produced it.`

const listServiceConnectionsDescription = `List a project's service connections with their type, authentication scheme and readiness.

A terraform init or apply that fails to authenticate to Azure usually traces back to the
Azure Resource Manager connection its task names: this shows whether it exists, whether it is
ready, and whether it uses workload identity federation or a secret. Credentials are not
returned.`

const listServiceConnectionHistoryDescription = `List the recent uses of one service connection by pipeline runs, newest first, with each run's result.

This shows whether a connection started failing at a point in time, across every pipeline
that uses it. Pages on next_cursor.`

const listVariableGroupsDescription = `List a project's variable groups with their variables.

Pipelines take backend settings and TF_VAR_ inputs from these groups, which are not in the
repository. Secret values come back as null; a group whose type is AzureKeyVault is linked to
a key vault.`

const listEnvironmentsDescription = `List a project's deployment environments and the resources registered in them.

YAML deployment jobs target these environments; an environment's ID is what
ado_list_environment_deployments takes.`

const listEnvironmentDeploymentsDescription = `List the deployments to one environment, newest first, with the pipeline, run, stage, job and result of each.

This shows what last deployed to an environment and whether it succeeded, such as another
run's apply that changed the state after the failed run planned. Pages on next_cursor.`

const findPullRequestsDescription = `Find pull requests in a repository: those that merged a commit, or those matching a status and branches.

With commit_id set to a run's source_version this names the pull request that brought the
failing change in. Its pull_request_id is what ado_list_pull_request_threads and
ado_list_policy_evaluations take. Without commit_id the list pages on next_skip.`

const listPullRequestThreadsDescription = `List the comment threads of a pull request: the review discussion, with the file and lines each thread is on.

Plan output posted to the pull request by a validation pipeline also appears here.`

const listPolicyEvaluationsDescription = `List the branch policies evaluated on a pull request and the status of each, such as a build validation that ran terraform plan.

A build policy's settings name the pipeline it runs as buildDefinitionId.`

const listAgentPoolsDescription = `List an organization's agent pools. A pool's pool_id is what ado_list_agents takes.`

const listAgentsDescription = `List the agents of a pool with their status, version, current job and last completed job.

This is how to check failures that come from the agent rather than the code: an offline agent,
a job stuck in the queue, or a self-hosted agent whose installed tools (such as the Terraform
version) differ from what the pipeline expects, which include_capabilities shows.`

const listRefsDescription = `List a repository's branches and tags with the commit each points to.

Use this to resolve a module source pinned with ?ref=<tag> to the commit it builds, and to find
the tags of a version line with filter, e.g. "tags/v1.". An annotated tag reports the commit it
points to, not the tag object.`

const searchCodeDescription = `Search the code of the organization's repositories for text, such as a Terraform variable, resource, module or output named in an error.

It needs the Code Search extension installed in the organization. info_code 0 means the search
ran normally; other values mean the index is incomplete or the query was not supported, so a
search that found nothing is not proof the text is absent. Read a file it reports with
ado_get_repository_item. Pages on next_skip.`

const accessDeniedMessage = `Azure DevOps refused this request for the identity this server authenticates as (%s):
%s
That identity has no access to what this call names, or, for a personal access token, the
token lacks the scope this tool's area needs, so it is out of reach through these tools.
Tell the user so, naming what was refused, instead of retrying with other names or answering
from guesswork.`

const notFoundMessage = `Azure DevOps reports that what this call names does not exist:
%s
It answers the same way when the identity this server authenticates as (%s) is not allowed to
see it, so this is either a wrong name or no access. Check the name against ado_list_projects, ado_list_pipelines or
ado_list_repositories before concluding. If the name is right, tell the user this server cannot
reach it instead of guessing.`

const refNotFoundMessage = `Azure DevOps could not find the branch, tag or commit this call names as ref:
%s
The repository exists; the ref does not. ado_list_repositories reports each repository's
default_branch, and omitting ref reads that branch. Tell the user the ref does not exist in that
repository rather than concluding the repository or its files are missing.`

const pathNotFoundMessage = `Azure DevOps could not find this path in the repository at the version the call reads:
%s
The repository and the version exist; the path does not. ado_list_repository_items lists what a
folder holds at that version, so check the path there before concluding the file is missing.`

const folderPathMessage = `'%s' is a folder, not a file, so it has no content to read. Use ado_list_repository_items
with scope_path set to that path to list what it holds, then read one of the files it reports.`
