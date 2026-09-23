// Package tools registers the Azure DevOps MCP tools and resolves the scope each call may
// reach.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/skoszewski/ado-mcp/internal/ado"
)

const (
	defaultLineCount   = 500
	defaultRunCount    = 25
	defaultCommitCount = 25
	treeNotBlobMarker  = "resolved to a Tree"
)

// scopeArgs are the organization and project arguments almost every tool takes.
type scopeArgs struct {
	Project      string `json:"project,omitempty" jsonschema:"Azure DevOps project name. Omit it unless this conversation has named a project -- the user's own words count, as does the result that introduced this pipeline or run. Never guess one: a wrong project makes a run or log ID mean nothing, and omitting it returns an error saying how to find the right one."`
	Organization string `json:"organization,omitempty" jsonschema:"Azure DevOps organization name or URL. Omit it unless this conversation has named an organization. Never guess one, and never reuse the project, pipeline or repository name as the organization: they are unrelated names."`
}

// repositoryArg is the repository argument every Git tool takes.
type repositoryArg struct {
	Repository string `json:"repository" jsonschema:"repository name or ID, as ado_list_repositories reports it, or the repository_name from ado_get_run."`
}

// handlers implements the tools, bound to the client and the scope they may reach.
type handlers struct {
	client        *ado.Client
	limits        Limits
	maxLogLines   int
	optimizations Optimizations
}

// Register adds every Azure DevOps tool to server and returns their names in registration
// order.
func Register(server *mcp.Server, client *ado.Client, limits Limits, maxLogLines int, optimizations Optimizations) []string {
	h := &handlers{client: client, limits: limits, maxLogLines: maxLogLines, optimizations: optimizations}
	return []string{
		addTool(server, "ado_list_projects", listProjectsDescription, nil, h.listProjects),
		addTool(server, "ado_list_folders", listFoldersDescription, nil, h.listFolders),
		addTool(server, "ado_list_pipelines", listPipelinesDescription, map[string]any{"recursive": true}, h.listPipelines),
		addTool(server, "ado_get_pipeline", getPipelineDescription, nil, h.getPipeline),
		addTool(server, "ado_list_runs", listRunsDescription, map[string]any{"top": defaultRunCount}, h.listRuns),
		addTool(server, "ado_get_run", getRunDescription, nil, h.getRun),
		addTool(server, "ado_get_run_timeline", getRunTimelineDescription, map[string]any{"log_type": "all"}, h.getRunTimeline),
		addTool(server, "ado_list_run_logs", listRunLogsDescription, nil, h.listRunLogs),
		addTool(server, "ado_get_run_log", getRunLogDescription,
			map[string]any{"start_line": 1, "line_count": defaultLineCount, "strip_noise": true}, h.getRunLog),
		addTool(server, "ado_list_repositories", listRepositoriesDescription, nil, h.listRepositories),
		addTool(server, "ado_get_repository_item", getRepositoryItemDescription,
			map[string]any{"ref_type": "branch", "start_line": 1, "line_count": defaultLineCount}, h.getRepositoryItem),
		addTool(server, "ado_list_repository_items", listRepositoryItemsDescription,
			map[string]any{"scope_path": "/", "recursion_level": "oneLevel", "ref_type": "branch"}, h.listRepositoryItems),
		addTool(server, "ado_list_commits", listCommitsDescription,
			map[string]any{"ref_type": "branch", "top": defaultCommitCount}, h.listCommits),
		addTool(server, "ado_get_commit_changes", getCommitChangesDescription, nil, h.getCommitChanges),
	}
}

// addTool registers handler as the tool name. The input schema is inferred from In, with the
// given property defaults, which the SDK fills into the arguments before they are decoded, and
// with a pipeline property accepting a name or a numeric ID. Every call is logged at debug
// level, and a handler error reaches the caller as a tool error result.
func addTool[In, Out any](server *mcp.Server, name, description string, defaults map[string]any, handler func(context.Context, In) (Out, error)) string {
	schema, err := jsonschema.For[In](nil)
	if err != nil {
		panic(fmt.Sprintf("tool %s: %v", name, err))
	}
	for property, value := range defaults {
		raw, err := json.Marshal(value)
		if err != nil {
			panic(fmt.Sprintf("tool %s: default for %s: %v", name, property, err))
		}
		schema.Properties[property].Default = raw
	}
	if pipeline, ok := schema.Properties["pipeline"]; ok {
		pipeline.Type = ""
		pipeline.Types = []string{"null", "integer", "string"}
	}

	mcp.AddTool(server, &mcp.Tool{Name: name, Description: description, InputSchema: schema},
		func(ctx context.Context, request *mcp.CallToolRequest, input In) (*mcp.CallToolResult, Out, error) {
			slog.Debug("tool call", "tool", name, "arguments", string(request.Params.Arguments))
			output, err := handler(ctx, input)
			if err != nil {
				slog.Debug("tool failed", "tool", name, "error", err)
			}
			return nil, output, err
		})
	return name
}

// optional returns nil for an empty string, which a result reports as null.
func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

type listProjectsInput struct {
	Organization string `json:"organization,omitempty" jsonschema:"Azure DevOps organization name, e.g. \"myorg\", or its full URL. Omit it unless this conversation has named an organization. Never guess one, and never reuse a project or pipeline name as the organization: they are unrelated names."`
	Top          int    `json:"top,omitempty" jsonschema:"return at most this many projects, and a next_cursor if more remain. Omit it to return every project, which is what an organization-sized list allows."`
	Cursor       string `json:"cursor,omitempty" jsonschema:"the next_cursor from a previous call, to continue where it stopped."`
}

type listProjectsOutput struct {
	Organization string        `json:"organization"`
	NextCursor   *string       `json:"next_cursor"`
	Projects     []ado.Project `json:"projects"`
}

func (h *handlers) listProjects(ctx context.Context, in listProjectsInput) (listProjectsOutput, error) {
	orgURL, _, err := h.limits.resolveScope(in.Organization, "", false)
	if err != nil {
		return listProjectsOutput{}, err
	}
	projects, next, err := collectPages(in.Top, in.Cursor, func(top int, cursor string) ([]ado.Project, string, error) {
		return h.client.ProjectsPage(ctx, orgURL, top, cursor)
	})
	if err != nil {
		return listProjectsOutput{}, err
	}
	if h.limits.Project != "" {
		projects = slices.DeleteFunc(projects, func(project ado.Project) bool {
			return !strings.EqualFold(project.Name, h.limits.Project)
		})
	}
	return listProjectsOutput{Organization: orgURL, NextCursor: next, Projects: projects}, nil
}

type listFoldersInput struct {
	scopeArgs
	Path string `json:"path,omitempty" jsonschema:"folder path to start from, starting with a backslash as in \"\\pipeline-templates\"; omit for the whole project."`
}

type folderSummary struct {
	Path        *string `json:"path"`
	Description *string `json:"description"`
	CreatedOn   *string `json:"created_on"`
}

type listFoldersOutput struct {
	Organization string          `json:"organization"`
	Project      string          `json:"project"`
	Folders      []folderSummary `json:"folders"`
}

func (h *handlers) listFolders(ctx context.Context, in listFoldersInput) (listFoldersOutput, error) {
	orgURL, project, err := h.limits.resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return listFoldersOutput{}, err
	}
	folders, err := h.client.Folders(ctx, orgURL, project, NormalizeFolderPath(in.Path))
	if err != nil {
		return listFoldersOutput{}, err
	}
	output := listFoldersOutput{Organization: orgURL, Project: project, Folders: []folderSummary{}}
	for _, folder := range folders {
		output.Folders = append(output.Folders, folderSummary(folder))
	}
	return output, nil
}

type listPipelinesInput struct {
	scopeArgs
	FolderName string `json:"folder_name,omitempty" jsonschema:"only list pipelines in this folder, as ado_list_folders reports it -- a path starting with a backslash, as in \"\\pipeline-templates\" or \"\\dom-lab-azure-lz\\dev\", with \"\\\" alone for the root folder. Omit it to list every pipeline in the project. Each pipeline's own folder comes back in its path field either way."`
	Recursive  bool   `json:"recursive,omitempty" jsonschema:"also list pipelines in the folders below folder_name, which is the default: a folder often holds nothing itself and keeps its pipelines in subfolders. Pass false for that one folder alone."`
	Top        int    `json:"top,omitempty" jsonschema:"read at most this many pipelines from the project in this call, and report a next_cursor if more remain. Omit it to read them all, which is the default. Any folder_name filters what that call read, so a bounded call can return fewer pipelines than top -- even none -- while next_cursor still points at more: follow the cursor before concluding a folder is empty."`
	Cursor     string `json:"cursor,omitempty" jsonschema:"the next_cursor from a previous call, to continue where it stopped."`
}

type pipelineSummary struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Path         *string `json:"path"`
	QueueStatus  *string `json:"queue_status"`
	DefaultQueue *string `json:"default_queue"`
}

type listPipelinesOutput struct {
	Organization string            `json:"organization"`
	Project      string            `json:"project"`
	FolderName   *string           `json:"folder_name"`
	NextCursor   *string           `json:"next_cursor"`
	Pipelines    []pipelineSummary `json:"pipelines"`
}

func (h *handlers) listPipelines(ctx context.Context, in listPipelinesInput) (listPipelinesOutput, error) {
	orgURL, project, err := h.limits.resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return listPipelinesOutput{}, err
	}
	folderName, err := h.limits.resolveFolder(in.FolderName)
	if err != nil {
		return listPipelinesOutput{}, err
	}
	// Filtered here rather than through the API's own path parameter, which matches one folder
	// exactly and drops a definition whose name is shared by another in the same folder.
	definitions, next, err := collectPages(in.Top, in.Cursor, func(top int, cursor string) ([]ado.Definition, string, error) {
		return h.client.DefinitionsPage(ctx, orgURL, project, top, cursor)
	})
	if err != nil {
		return listPipelinesOutput{}, err
	}

	output := listPipelinesOutput{
		Organization: orgURL, Project: project, FolderName: optional(folderName), NextCursor: next,
		Pipelines: []pipelineSummary{},
	}
	for _, definition := range definitions {
		if folderName != "" && (definition.Path == nil || !InFolder(*definition.Path, folderName, in.Recursive)) {
			continue
		}
		output.Pipelines = append(output.Pipelines, pipelineSummary{
			ID: definition.ID, Name: definition.Name, Path: definition.Path,
			QueueStatus: definition.QueueStatus, DefaultQueue: definition.QueueName(),
		})
	}
	return output, nil
}

type getPipelineInput struct {
	Pipeline any `json:"pipeline,omitempty" jsonschema:"pipeline name, or the numeric id reported by ado_list_pipelines or any run result. Omit it unless this conversation has named a pipeline. Never guess one: ado_list_pipelines reports the pipelines that exist."`
	scopeArgs
}

type getPipelineOutput struct {
	Organization string  `json:"organization"`
	Project      string  `json:"project"`
	PipelineID   int     `json:"pipeline_id"`
	PipelineName string  `json:"pipeline_name"`
	Path         *string `json:"path"`
	QueueStatus  *string `json:"queue_status"`
	Revision     *int    `json:"revision"`
	DefaultQueue *string `json:"default_queue"`
}

func (h *handlers) getPipeline(ctx context.Context, in getPipelineInput) (getPipelineOutput, error) {
	orgURL, project, err := h.limits.resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return getPipelineOutput{}, err
	}
	pipelineID, _, err := h.limits.resolvePipeline(ctx, h.client, orgURL, project, pipelineArgument(in.Pipeline), false)
	if err != nil {
		return getPipelineOutput{}, err
	}
	definition, err := h.client.Definition(ctx, orgURL, project, pipelineID)
	if err != nil {
		return getPipelineOutput{}, err
	}
	return getPipelineOutput{
		Organization: orgURL, Project: project, PipelineID: definition.ID, PipelineName: definition.Name,
		Path: definition.Path, QueueStatus: definition.QueueStatus, Revision: definition.Revision,
		DefaultQueue: definition.QueueName(),
	}, nil
}

type listRunsInput struct {
	Pipeline any `json:"pipeline,omitempty" jsonschema:"pipeline name, or the numeric id reported by ado_list_pipelines or any run result. Omit it to list the project's runs across every pipeline, which is how to answer a question about a project rather than a pipeline; each run reports the pipeline_id and pipeline_name it belongs to either way. Never guess a name: ado_list_pipelines reports the ones that exist."`
	scopeArgs
	Top     int    `json:"top,omitempty" jsonschema:"how many runs this page holds, 25 by default. It bounds one page, not the search: older runs are reached through next_cursor."`
	Cursor  string `json:"cursor,omitempty" jsonschema:"the next_cursor from a previous call, to continue where it stopped. Every other argument must stay as it was for the cursor to mean anything."`
	Status  string `json:"status,omitempty" jsonschema:"only runs with this status: \"inProgress\", \"completed\", \"cancelling\", \"postponed\", \"notStarted\" or \"all\"."`
	Result  string `json:"result,omitempty" jsonschema:"only runs with this result: \"succeeded\", \"partiallySucceeded\", \"failed\" or \"canceled\"."`
	Branch  string `json:"branch,omitempty" jsonschema:"only runs that built this branch, e.g. \"refs/heads/main\"."`
	MinTime string `json:"min_time,omitempty" jsonschema:"only runs started at or after this ISO 8601 timestamp."`
	MaxTime string `json:"max_time,omitempty" jsonschema:"only runs started at or before this ISO 8601 timestamp."`
	Reason  string `json:"reason,omitempty" jsonschema:"only runs queued for this reason, e.g. \"manual\", \"individualCI\", \"schedule\" or \"pullRequest\"."`
}

type runSummary struct {
	RunID           int      `json:"run_id"`
	BuildNumber     *string  `json:"build_number"`
	Status          *string  `json:"status"`
	Result          *string  `json:"result"`
	Branch          *string  `json:"branch"`
	QueueTime       *string  `json:"queue_time"`
	StartTime       *string  `json:"start_time"`
	FinishTime      *string  `json:"finish_time"`
	DurationSeconds *float64 `json:"duration_seconds"`
	Reason          *string  `json:"reason"`
	RequestedFor    *string  `json:"requested_for"`
	PipelineID      *int     `json:"pipeline_id"`
	PipelineName    *string  `json:"pipeline_name"`
}

func newRunSummary(build ado.Build) runSummary {
	summary := runSummary{
		RunID: build.ID, BuildNumber: build.BuildNumber, Status: build.Status, Result: build.Result,
		Branch: build.SourceBranch, QueueTime: build.QueueTime, StartTime: build.StartTime,
		FinishTime: build.FinishTime, DurationSeconds: ado.DurationSeconds(build.StartTime, build.FinishTime),
		Reason: build.Reason,
	}
	if build.RequestedFor != nil {
		summary.RequestedFor = build.RequestedFor.DisplayName
	}
	if build.Definition != nil {
		summary.PipelineID = &build.Definition.ID
		summary.PipelineName = &build.Definition.Name
	}
	return summary
}

type listRunsOutput struct {
	Organization string       `json:"organization"`
	Project      string       `json:"project"`
	PipelineID   *int         `json:"pipeline_id"`
	PipelineName *string      `json:"pipeline_name"`
	NextCursor   *string      `json:"next_cursor"`
	Runs         []runSummary `json:"runs"`
}

func (h *handlers) listRuns(ctx context.Context, in listRunsInput) (listRunsOutput, error) {
	orgURL, project, err := h.limits.resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return listRunsOutput{}, err
	}
	pipelineID, pipelineName, err := h.limits.resolvePipeline(ctx, h.client, orgURL, project, pipelineArgument(in.Pipeline), true)
	if err != nil {
		return listRunsOutput{}, err
	}
	builds, next, err := h.client.BuildsPage(ctx, orgURL, project, ado.BuildQuery{
		PipelineID: pipelineID, Top: in.Top, Cursor: in.Cursor, Status: in.Status, Result: in.Result,
		Branch: in.Branch, MinTime: in.MinTime, MaxTime: in.MaxTime, Reason: in.Reason,
	})
	if err != nil {
		return listRunsOutput{}, err
	}

	output := listRunsOutput{
		Organization: orgURL, Project: project, PipelineName: optional(pipelineName),
		NextCursor: optional(next), Runs: []runSummary{},
	}
	if pipelineID != 0 {
		output.PipelineID = &pipelineID
	}
	for _, build := range builds {
		output.Runs = append(output.Runs, newRunSummary(build))
	}
	return output, nil
}

type runInput struct {
	RunID int `json:"run_id" jsonschema:"numeric run (build) ID."`
	scopeArgs
}

type getRunOutput struct {
	Organization string `json:"organization"`
	Project      string `json:"project"`
	runSummary
	SourceVersion  *string  `json:"source_version"`
	RepositoryName *string  `json:"repository_name"`
	RepositoryType *string  `json:"repository_type"`
	Tags           []string `json:"tags"`
	URL            string   `json:"url"`
}

func (h *handlers) getRun(ctx context.Context, in runInput) (getRunOutput, error) {
	orgURL, project, err := h.limits.resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return getRunOutput{}, err
	}
	build, err := h.client.Build(ctx, orgURL, project, in.RunID)
	if err != nil {
		return getRunOutput{}, err
	}
	output := getRunOutput{
		Organization: orgURL, Project: project, runSummary: newRunSummary(build),
		SourceVersion: build.SourceVersion, Tags: build.Tags,
		URL: fmt.Sprintf("%s/%s/_build/results?buildId=%d", orgURL, url.PathEscape(project), in.RunID),
	}
	if output.Tags == nil {
		output.Tags = []string{}
	}
	if build.Repository != nil {
		output.RepositoryName = build.Repository.Name
		output.RepositoryType = build.Repository.Type
	}
	return output, nil
}

type getRunTimelineInput struct {
	runInput
	LogType string `json:"log_type,omitempty" jsonschema:"which records to include: \"all\" (default), \"job\" for only the one combined-output record per job, or \"task\" for only the individual steps."`
}

type getRunTimelineOutput struct {
	Organization string     `json:"organization"`
	Project      string     `json:"project"`
	RunID        int        `json:"run_id"`
	LogType      string     `json:"log_type"`
	Steps        []ado.Step `json:"steps"`
}

func (h *handlers) getRunTimeline(ctx context.Context, in getRunTimelineInput) (getRunTimelineOutput, error) {
	orgURL, project, err := h.limits.resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return getRunTimelineOutput{}, err
	}
	if !slices.Contains(logTypes, in.LogType) {
		return getRunTimelineOutput{}, fmt.Errorf("unknown log_type '%s'; use 'job', 'task' or 'all'", in.LogType)
	}
	records, err := h.client.Timeline(ctx, orgURL, project, in.RunID)
	if err != nil {
		return getRunTimelineOutput{}, err
	}
	if len(records) == 0 {
		return getRunTimelineOutput{}, fmt.Errorf("no timeline records found for run %d", in.RunID)
	}
	return getRunTimelineOutput{
		Organization: orgURL, Project: project, RunID: in.RunID, LogType: in.LogType,
		Steps: ado.StepEntries(records, ado.MapLogsToTimeline(records), in.LogType),
	}, nil
}

type listRunLogsInput struct {
	runInput
	LogType string `json:"log_type,omitempty" jsonschema:"which logs to list: \"job\" for each job's combined output log, which already contains every step's output; \"task\" for each step's own log, which is far smaller and points at the step that actually failed; or \"all\" for every log with no filtering. Omit it to use the one this server is configured with."`
}

type logSummary struct {
	LogID     int     `json:"log_id"`
	Name      string  `json:"name"`
	Type      *string `json:"type"`
	LineCount *int    `json:"line_count"`
	CreatedOn *string `json:"created_on"`
}

type listRunLogsOutput struct {
	Organization string       `json:"organization"`
	Project      string       `json:"project"`
	RunID        int          `json:"run_id"`
	PipelineID   int          `json:"pipeline_id"`
	PipelineName string       `json:"pipeline_name"`
	LogType      string       `json:"log_type"`
	Logs         []logSummary `json:"logs"`
}

// runLogs fetches a run and its logs, returning the run's pipeline alongside.
func (h *handlers) runLogs(ctx context.Context, orgURL, project string, runID int) (ado.Build, []ado.RunLog, error) {
	build, err := h.client.Build(ctx, orgURL, project, runID)
	if err != nil {
		return build, nil, err
	}
	if build.Definition == nil {
		return build, nil, fmt.Errorf("run %d reports no pipeline", runID)
	}
	logs, err := h.client.RunLogs(ctx, orgURL, project, build.Definition.ID, runID)
	return build, logs, err
}

func (h *handlers) listRunLogs(ctx context.Context, in listRunLogsInput) (listRunLogsOutput, error) {
	orgURL, project, err := h.limits.resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return listRunLogsOutput{}, err
	}
	logType := in.LogType
	if logType == "" {
		logType = h.optimizations.LogType
	}
	build, logs, err := h.runLogs(ctx, orgURL, project, in.RunID)
	if err != nil {
		return listRunLogsOutput{}, err
	}
	records, err := h.client.Timeline(ctx, orgURL, project, in.RunID)
	if err != nil {
		return listRunLogsOutput{}, err
	}
	logInfo := ado.MapLogsToTimeline(records)
	selected, err := ado.SelectLogs(logs, logInfo, logType)
	if err != nil {
		return listRunLogsOutput{}, err
	}

	output := listRunLogsOutput{
		Organization: orgURL, Project: project, RunID: in.RunID, PipelineID: build.Definition.ID,
		PipelineName: build.Definition.Name, LogType: logType, Logs: []logSummary{},
	}
	for _, log := range selected {
		summary := logSummary{
			LogID: log.ID, Name: ado.DescribeLog(log.ID, logInfo), LineCount: log.LineCount, CreatedOn: log.CreatedOn,
		}
		if info, known := logInfo[log.ID]; known {
			summary.Type = &info.Type
		}
		output.Logs = append(output.Logs, summary)
	}
	return output, nil
}

type getRunLogInput struct {
	RunID int `json:"run_id" jsonschema:"numeric run (build) ID."`
	LogID int `json:"log_id" jsonschema:"numeric log ID, from ado_list_run_logs or a step's log_id in ado_get_run_timeline."`
	scopeArgs
	StartLine  int  `json:"start_line,omitempty" jsonschema:"1-based line to start from, counted after any noise stripping."`
	LineCount  int  `json:"line_count,omitempty" jsonschema:"how many lines to return. A larger value is silently capped; the result's returned_lines reports how many lines actually came back."`
	StripNoise bool `json:"strip_noise,omitempty" jsonschema:"drop ANSI colour codes, per-line timestamps and blank lines."`
}

type getRunLogOutput struct {
	Organization string `json:"organization"`
	Project      string `json:"project"`
	RunID        int    `json:"run_id"`
	LogID        int    `json:"log_id"`
	linePage
}

func (h *handlers) getRunLog(ctx context.Context, in getRunLogInput) (getRunLogOutput, error) {
	orgURL, project, err := h.limits.resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return getRunLogOutput{}, err
	}
	_, logs, err := h.runLogs(ctx, orgURL, project, in.RunID)
	if err != nil {
		return getRunLogOutput{}, err
	}
	index := slices.IndexFunc(logs, func(log ado.RunLog) bool { return log.ID == in.LogID })
	if index < 0 {
		return getRunLogOutput{}, fmt.Errorf("no log with id %d found for run %d", in.LogID, in.RunID)
	}

	text, err := h.client.LogContent(ctx, logs[index])
	if err != nil {
		return getRunLogOutput{}, err
	}
	if in.StripNoise {
		text = ado.StripLogNoise(text)
	}
	lines := splitLines(text)

	output := getRunLogOutput{Organization: orgURL, Project: project, RunID: in.RunID, LogID: in.LogID}
	if h.optimizations.SmallModel {
		// A small model tends to answer from the first page, and a failed step's error is at the end.
		output.linePage = tailLines(lines, h.maxLogLines)
	} else {
		output.linePage = pageLines(lines, in.StartLine, in.LineCount, h.maxLogLines)
	}
	return output, nil
}

type repositorySummary struct {
	ID            *string `json:"repository_id"`
	Name          *string `json:"name"`
	DefaultBranch *string `json:"default_branch"`
	IsDisabled    *bool   `json:"is_disabled"`
	IsFork        *bool   `json:"is_fork"`
	Size          *int64  `json:"size"`
	WebURL        *string `json:"web_url"`
	RemoteURL     *string `json:"remote_url"`
}

type listRepositoriesOutput struct {
	Organization string              `json:"organization"`
	Project      string              `json:"project"`
	Repositories []repositorySummary `json:"repositories"`
}

func (h *handlers) listRepositories(ctx context.Context, in scopeArgs) (listRepositoriesOutput, error) {
	orgURL, project, err := h.limits.resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return listRepositoriesOutput{}, err
	}
	repositories, err := h.client.Repositories(ctx, orgURL, project)
	if err != nil {
		return listRepositoriesOutput{}, err
	}
	output := listRepositoriesOutput{Organization: orgURL, Project: project, Repositories: []repositorySummary{}}
	for _, repository := range repositories {
		output.Repositories = append(output.Repositories, repositorySummary(repository))
	}
	return output, nil
}

type getRepositoryItemInput struct {
	repositoryArg
	Path string `json:"path" jsonschema:"path of the file within the repository, e.g. \"modules/ar/main.tf\"."`
	scopeArgs
	Ref       string `json:"ref,omitempty" jsonschema:"branch name, tag name or commit SHA to read at. Omit it for the repository's default branch, which is not necessarily the branch a run built."`
	RefType   string `json:"ref_type,omitempty" jsonschema:"how to read ref: \"branch\" (default), \"tag\" or \"commit\"."`
	StartLine int    `json:"start_line,omitempty" jsonschema:"1-based line to start from."`
	LineCount int    `json:"line_count,omitempty" jsonschema:"how many lines to return. A larger value is silently capped; the result's returned_lines reports how many lines actually came back."`
}

type getRepositoryItemOutput struct {
	Organization string  `json:"organization"`
	Project      string  `json:"project"`
	Repository   string  `json:"repository"`
	Path         string  `json:"path"`
	Ref          *string `json:"ref"`
	CommitID     *string `json:"commit_id"`
	ObjectID     *string `json:"object_id"`
	IsBinary     bool    `json:"is_binary"`
	linePage
}

func (h *handlers) getRepositoryItem(ctx context.Context, in getRepositoryItemInput) (getRepositoryItemOutput, error) {
	orgURL, project, err := h.limits.resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return getRepositoryItemOutput{}, err
	}
	item, err := h.client.RepositoryItem(ctx, orgURL, project, in.Repository, in.Path, ado.ItemVersion{Ref: in.Ref, RefType: in.RefType})
	if err != nil {
		var requestError *ado.RequestError
		if errors.As(err, &requestError) && strings.Contains(requestError.Message, treeNotBlobMarker) {
			return getRepositoryItemOutput{}, fmt.Errorf(folderPathMessage, in.Path)
		}
		return getRepositoryItemOutput{}, err
	}

	output := getRepositoryItemOutput{
		Organization: orgURL, Project: project, Repository: in.Repository, Path: in.Path,
		Ref: optional(in.Ref), CommitID: item.CommitID, ObjectID: item.ObjectID,
		IsBinary: item.ContentMetadata != nil && item.ContentMetadata.IsBinary,
	}
	if item.Path != nil && *item.Path != "" {
		output.Path = *item.Path
	}
	if output.IsBinary {
		output.linePage = linePage{StartLine: 1}
	} else {
		output.linePage = pageLines(splitLines(item.Content), in.StartLine, in.LineCount, h.maxLogLines)
	}
	return output, nil
}

type listRepositoryItemsInput struct {
	repositoryArg
	scopeArgs
	ScopePath      string `json:"scope_path,omitempty" jsonschema:"path to list under, \"/\" for the repository root."`
	RecursionLevel string `json:"recursion_level,omitempty" jsonschema:"how deep to list: \"oneLevel\" (default) for the direct children of scope_path, \"none\" for scope_path itself, or \"full\" for every descendant, which on a large repository returns a great many rows."`
	Ref            string `json:"ref,omitempty" jsonschema:"branch name, tag name or commit SHA to list at. Omit it for the repository's default branch."`
	RefType        string `json:"ref_type,omitempty" jsonschema:"how to read ref: \"branch\" (default), \"tag\" or \"commit\"."`
}

type itemSummary struct {
	Path          *string `json:"path"`
	IsFolder      bool    `json:"is_folder"`
	GitObjectType *string `json:"git_object_type"`
	ObjectID      *string `json:"object_id"`
}

type listRepositoryItemsOutput struct {
	Organization   string        `json:"organization"`
	Project        string        `json:"project"`
	Repository     string        `json:"repository"`
	ScopePath      string        `json:"scope_path"`
	RecursionLevel string        `json:"recursion_level"`
	Ref            *string       `json:"ref"`
	Items          []itemSummary `json:"items"`
}

func (h *handlers) listRepositoryItems(ctx context.Context, in listRepositoryItemsInput) (listRepositoryItemsOutput, error) {
	orgURL, project, err := h.limits.resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return listRepositoryItemsOutput{}, err
	}
	items, err := h.client.RepositoryItems(ctx, orgURL, project, in.Repository, in.ScopePath, in.RecursionLevel,
		ado.ItemVersion{Ref: in.Ref, RefType: in.RefType})
	if err != nil {
		return listRepositoryItemsOutput{}, err
	}
	output := listRepositoryItemsOutput{
		Organization: orgURL, Project: project, Repository: in.Repository, ScopePath: in.ScopePath,
		RecursionLevel: in.RecursionLevel, Ref: optional(in.Ref), Items: []itemSummary{},
	}
	for _, item := range items {
		output.Items = append(output.Items, itemSummary{
			Path: item.Path, IsFolder: item.IsFolder, GitObjectType: item.GitObjectType, ObjectID: item.ObjectID,
		})
	}
	return output, nil
}

type listCommitsInput struct {
	repositoryArg
	scopeArgs
	ItemPath string `json:"item_path,omitempty" jsonschema:"only commits that changed this path, e.g. \"modules/ar/main.tf\". Omit it for the whole repository's history."`
	Ref      string `json:"ref,omitempty" jsonschema:"branch name, tag name or commit SHA to walk history from. Omit it for the repository's default branch."`
	RefType  string `json:"ref_type,omitempty" jsonschema:"how to read ref: \"branch\" (default), \"tag\" or \"commit\"."`
	Author   string `json:"author,omitempty" jsonschema:"only commits by this author alias or display name."`
	FromDate string `json:"from_date,omitempty" jsonschema:"only commits after this date, e.g. \"2026-09-01\"."`
	ToDate   string `json:"to_date,omitempty" jsonschema:"only commits before this date."`
	Top      int    `json:"top,omitempty" jsonschema:"how many commits this page holds, 25 by default. It bounds one page, not the search: older commits are reached through next_skip."`
	Skip     int    `json:"skip,omitempty" jsonschema:"how many commits to skip before this page, from a previous result's next_skip."`
}

type commitSummary struct {
	CommitID         *string        `json:"commit_id"`
	Comment          *string        `json:"comment"`
	CommentTruncated bool           `json:"comment_truncated"`
	AuthorName       *string        `json:"author_name"`
	AuthorEmail      *string        `json:"author_email"`
	AuthorDate       *string        `json:"author_date"`
	CommitterDate    *string        `json:"committer_date"`
	ChangeCounts     map[string]int `json:"change_counts"`
	RemoteURL        *string        `json:"remote_url"`
}

type listCommitsOutput struct {
	Organization string          `json:"organization"`
	Project      string          `json:"project"`
	Repository   string          `json:"repository"`
	ItemPath     *string         `json:"item_path"`
	NextSkip     *int            `json:"next_skip"`
	Commits      []commitSummary `json:"commits"`
}

func (h *handlers) listCommits(ctx context.Context, in listCommitsInput) (listCommitsOutput, error) {
	orgURL, project, err := h.limits.resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return listCommitsOutput{}, err
	}
	commits, err := h.client.Commits(ctx, orgURL, project, in.Repository, ado.CommitQuery{
		ItemPath: in.ItemPath, Version: ado.ItemVersion{Ref: in.Ref, RefType: in.RefType}, Author: in.Author,
		FromDate: in.FromDate, ToDate: in.ToDate, Top: in.Top, Skip: in.Skip,
	})
	if err != nil {
		return listCommitsOutput{}, err
	}

	output := listCommitsOutput{
		Organization: orgURL, Project: project, Repository: in.Repository, ItemPath: optional(in.ItemPath),
		Commits: []commitSummary{},
	}
	if in.Top > 0 && len(commits) == in.Top {
		nextSkip := in.Skip + len(commits)
		output.NextSkip = &nextSkip
	}
	for _, commit := range commits {
		summary := commitSummary{
			CommitID: commit.CommitID, Comment: commit.Comment, CommentTruncated: commit.CommentTruncated,
			ChangeCounts: commit.ChangeCounts, RemoteURL: commit.RemoteURL,
		}
		if summary.ChangeCounts == nil {
			summary.ChangeCounts = map[string]int{}
		}
		if commit.Author != nil {
			summary.AuthorName, summary.AuthorEmail, summary.AuthorDate = commit.Author.Name, commit.Author.Email, commit.Author.Date
		}
		if commit.Committer != nil {
			summary.CommitterDate = commit.Committer.Date
		}
		output.Commits = append(output.Commits, summary)
	}
	return output, nil
}

type getCommitChangesInput struct {
	repositoryArg
	CommitID string `json:"commit_id" jsonschema:"SHA of the commit, from ado_list_commits or the source_version reported by ado_get_run."`
	scopeArgs
	Top  int `json:"top,omitempty" jsonschema:"maximum number of changes to return. Omit it for the API's own page size."`
	Skip int `json:"skip,omitempty" jsonschema:"number of changes to skip, for a commit with more than one page of them."`
}

type changeSummary struct {
	ChangeType    *string `json:"change_type"`
	Path          *string `json:"path"`
	GitObjectType *string `json:"git_object_type"`
	IsFolder      bool    `json:"is_folder"`
}

type getCommitChangesOutput struct {
	Organization string          `json:"organization"`
	Project      string          `json:"project"`
	Repository   string          `json:"repository"`
	CommitID     string          `json:"commit_id"`
	ChangeCounts map[string]int  `json:"change_counts"`
	Changes      []changeSummary `json:"changes"`
}

func (h *handlers) getCommitChanges(ctx context.Context, in getCommitChangesInput) (getCommitChangesOutput, error) {
	orgURL, project, err := h.limits.resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return getCommitChangesOutput{}, err
	}
	changes, err := h.client.CommitChanges(ctx, orgURL, project, in.Repository, in.CommitID, in.Top, in.Skip)
	if err != nil {
		return getCommitChangesOutput{}, err
	}
	output := getCommitChangesOutput{
		Organization: orgURL, Project: project, Repository: in.Repository, CommitID: in.CommitID,
		ChangeCounts: changes.ChangeCounts, Changes: []changeSummary{},
	}
	if output.ChangeCounts == nil {
		output.ChangeCounts = map[string]int{}
	}
	for _, change := range changes.Changes {
		output.Changes = append(output.Changes, changeSummary{
			ChangeType: change.ChangeType, Path: change.Item.Path,
			GitObjectType: change.Item.GitObjectType, IsFolder: change.Item.IsFolder,
		})
	}
	return output, nil
}
