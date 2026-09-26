package tools

const instructionsOverview = `Read-only access to Azure DevOps pipeline inventory and run history, and to the Git
repositories those runs build.

Explore top-down: ado_list_projects, ado_list_folders, ado_list_pipelines, ado_list_runs, then
ado_get_run_timeline to find the step that matters, ado_list_run_logs for its log IDs, and
ado_get_run_log to read the content a page at a time.

That order is a path, not a precondition. A question about a project rather than one pipeline
goes straight to ado_list_runs with no pipeline argument, which spans every pipeline the project
holds; listing the pipelines first only to reach the runs is wasted work.

"Why did the pipeline fail recently", naming a project but no pipeline, is answered that way:
call ado_list_runs on the project with result="failed", and the newest failed runs come back
with the pipeline_id and pipeline_name each belongs to, identifying the pipeline rather than
requiring it up front. Then ado_get_run_timeline on that run_id for the step that failed, and
ado_get_run_log for what it printed. Do not reach for ado_list_pipelines here and do not ask the
user which pipeline they meant: an inventory of pipelines carries no run results, so it cannot
say which one failed.

Almost every tool needs the project, and a run or log ID means nothing without it. Each result
states the organization and project it came from, so carry those forward from the earlier steps
of the conversation rather than asking again: the project the pipeline was listed in is the
project its runs, timelines and logs belong to.

What the conversation has not given you, ask the user for -- never invent it. The organization
is a separate name from the project, the pipeline and the repository, and cannot be derived
from any of them, so a request that names only a project does not tell you the organization.
Omitting an argument you do not know returns an error saying how to find it; passing a guess
returns someone else's data or a misleading "not found".

A listing result carries a next_cursor. When it holds a value there are more rows than this
result contains: pass it back as the cursor argument, with every other argument unchanged, to
read the next page. When it is null the list is complete. ado_list_runs always works this way,
one page at a time -- reach older runs by following its cursor, never by raising top, which only
makes a single response larger. Watch for a page that matches nothing: a folder-filtered
ado_list_pipelines page can come back empty while next_cursor still points at further pages, so
a folder is only empty once you have followed the cursor to a null.

ado_get_run_log pages the same way, on next_start_line rather than a cursor: while it holds a
number there is more of that log, and passing it back as start_line returns the next page. A
failed step's error sits at the end of its log, so follow it to a null instead of concluding
from the first page -- neither the page size nor the tool is a limit on how much you can read.

When a log names a file, a module or a template, read it rather than inferring what it says:
ado_get_repository_item returns a file's content a page at a time, ado_list_repository_items
lists what a directory holds, and ado_list_repositories reports the repositories a project has.
ado_get_run already names the repository a run built as repository_name and the commit it built
as source_version, so passing that source_version as ref with ref_type="commit" reads the file
as that run saw it rather than as it stands now. For when a change landed and who made it, use
ado_list_commits with item_path set to the one file, then ado_get_commit_changes on a commit it
reports. ado_list_commits pages on next_skip rather than on a cursor.

To find what changed before a failure, find the last successful run of the same pipeline with
ado_list_runs, then ado_list_run_changes with that run as from_run_id for the commits in
between, ado_get_diff between the two runs' source_version commits for the files, and
ado_find_pull_requests with a commit for the pull request that brought it in. A failure that
depends on configuration outside the repository is examined with ado_get_pipeline_yaml for the
expanded templates, ado_list_service_connections and ado_list_variable_groups for what the
stage authenticated with and was given, ado_list_environment_deployments for what else deployed
to the same environment, and ado_list_agents for the agent that ran the job.`

const instructionsScope = `This server remembers nothing between calls, so every call carries the organization and
project it applies to. Keeping track
of what you were told is your job, not the user's, but that is not licence to fill a gap with a
guess: as the conversation settles on an organization, a project, a folder, a pipeline and a
run, hold on to each one and pass it into every later call, so the user never has to repeat it.
Keep using that scope until the user points somewhere else, and change only the part they
actually changed -- naming a different pipeline does not change the project. If you have lost
track, the last result you received restates the organization and project it came from.`

const instructionsAccess = `Azure DevOps decides what this server can reach: every call runs as one identity, and a
request for something that identity cannot see fails. The user may ask about an organization,
project, pipeline or repository that is out of its reach. When a result says access was refused,
or that something does not exist although its name is right, tell the user this server cannot
reach it rather than looking for a way around it or answering from guesswork.`

// Instructions are the server instructions sent to the client at initialize.
const Instructions = instructionsOverview + "\n\n" + instructionsScope + "\n\n" + instructionsAccess
