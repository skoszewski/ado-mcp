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

const folderPathMessage = `'%s' is a folder, not a file, so it has no content to read. Use ado_list_repository_items
with scope_path set to that path to list what it holds, then read one of the files it reports.`
