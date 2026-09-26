package tools

import (
	"context"
	"errors"

	"github.com/skoszewski/ado-mcp/internal/ado"
)

// diffPageSize is the number of changes the diffs API returns when no top is given.
const diffPageSize = 100

// displayName returns an identity's display name, or nil.
func displayName(identity *ado.IdentityRef) *string {
	if identity == nil {
		return nil
	}
	return identity.DisplayName
}

type listRunChangesInput struct {
	RunID     int `json:"run_id" jsonschema:"numeric run (build) ID whose commits to list."`
	FromRunID int `json:"from_run_id,omitempty" jsonschema:"an earlier run of the same pipeline, typically its last successful one. When given, the result lists every commit made between that run and run_id rather than the commits run_id alone picked up."`
	scopeArgs
	Top    int    `json:"top,omitempty" jsonschema:"return at most this many commits. Without from_run_id a next_cursor reports more; omit it to return them all."`
	Cursor string `json:"cursor,omitempty" jsonschema:"the next_cursor from a previous call, to continue where it stopped. Not used with from_run_id."`
}

type runChange struct {
	CommitID         *string `json:"commit_id"`
	Message          *string `json:"message"`
	MessageTruncated bool    `json:"message_truncated"`
	Author           *string `json:"author"`
	Pusher           *string `json:"pusher"`
	Timestamp        *string `json:"timestamp"`
}

type listRunChangesOutput struct {
	Organization string      `json:"organization"`
	Project      string      `json:"project"`
	RunID        int         `json:"run_id"`
	FromRunID    *int        `json:"from_run_id"`
	NextCursor   *string     `json:"next_cursor"`
	Changes      []runChange `json:"changes"`
}

func (h *handlers) listRunChanges(ctx context.Context, in listRunChangesInput) (listRunChangesOutput, error) {
	orgURL, project, err := resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return listRunChangesOutput{}, err
	}
	output := listRunChangesOutput{Organization: orgURL, Project: project, RunID: in.RunID, Changes: []runChange{}}
	var changes []ado.Change
	if in.FromRunID != 0 {
		output.FromRunID = &in.FromRunID
		changes, err = h.client.ChangesBetweenRuns(ctx, orgURL, project, in.FromRunID, in.RunID, in.Top)
	} else {
		changes, output.NextCursor, err = collectPages(in.Top, in.Cursor, func(top int, cursor string) ([]ado.Change, string, error) {
			return h.client.RunChangesPage(ctx, orgURL, project, in.RunID, top, cursor)
		})
	}
	if err != nil {
		return listRunChangesOutput{}, err
	}
	for _, change := range changes {
		output.Changes = append(output.Changes, runChange{
			CommitID: change.ID, Message: change.Message, MessageTruncated: change.MessageTruncated,
			Author: displayName(change.Author), Pusher: change.Pusher, Timestamp: change.Timestamp,
		})
	}
	return output, nil
}

type artifactSummary struct {
	ArtifactID int            `json:"artifact_id"`
	Name       string         `json:"name"`
	Source     *string        `json:"source"`
	Type       *string        `json:"type"`
	Properties map[string]any `json:"properties"`
}

type listRunArtifactsOutput struct {
	Organization string            `json:"organization"`
	Project      string            `json:"project"`
	RunID        int               `json:"run_id"`
	Artifacts    []artifactSummary `json:"artifacts"`
}

func (h *handlers) listRunArtifacts(ctx context.Context, in runInput) (listRunArtifactsOutput, error) {
	orgURL, project, err := resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return listRunArtifactsOutput{}, err
	}
	artifacts, err := h.client.RunArtifacts(ctx, orgURL, project, in.RunID)
	if err != nil {
		return listRunArtifactsOutput{}, err
	}
	output := listRunArtifactsOutput{Organization: orgURL, Project: project, RunID: in.RunID, Artifacts: []artifactSummary{}}
	for _, artifact := range artifacts {
		summary := artifactSummary{ArtifactID: artifact.ID, Name: artifact.Name, Source: artifact.Source, Properties: map[string]any{}}
		if artifact.Resource != nil {
			summary.Type = artifact.Resource.Type
			if artifact.Resource.Properties != nil {
				summary.Properties = artifact.Resource.Properties
			}
		}
		output.Artifacts = append(output.Artifacts, summary)
	}
	return output, nil
}

type getPipelineYAMLInput struct {
	Pipeline any `json:"pipeline,omitempty" jsonschema:"pipeline name, or the numeric id reported by ado_list_pipelines or any run result. Never guess one."`
	scopeArgs
	Ref       string `json:"ref,omitempty" jsonschema:"full name of the branch or tag of the pipeline's own repository to expand the YAML at, e.g. \"refs/heads/main\" or the branch ado_get_run reports. Omit it for the pipeline's default branch."`
	StartLine int    `json:"start_line,omitempty" jsonschema:"1-based line to start from."`
	LineCount int    `json:"line_count,omitempty" jsonschema:"how many lines to return. A larger value is silently capped; the result's returned_lines reports how many lines actually came back."`
}

type getPipelineYAMLOutput struct {
	Organization string  `json:"organization"`
	Project      string  `json:"project"`
	PipelineID   int     `json:"pipeline_id"`
	PipelineName string  `json:"pipeline_name"`
	Ref          *string `json:"ref"`
	linePage
}

func (h *handlers) getPipelineYAML(ctx context.Context, in getPipelineYAMLInput) (getPipelineYAMLOutput, error) {
	orgURL, project, err := resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return getPipelineYAMLOutput{}, err
	}
	pipelineID, pipelineName, err := resolvePipeline(ctx, h.client, orgURL, project, pipelineArgument(in.Pipeline), false)
	if err != nil {
		return getPipelineYAMLOutput{}, err
	}
	yaml, err := h.client.PipelineYAML(ctx, orgURL, project, pipelineID, in.Ref)
	if err != nil {
		return getPipelineYAMLOutput{}, err
	}
	return getPipelineYAMLOutput{
		Organization: orgURL, Project: project, PipelineID: pipelineID, PipelineName: pipelineName, Ref: optional(in.Ref),
		linePage: pageLines(splitLines(yaml), in.StartLine, in.LineCount, h.maxLogLines),
	}, nil
}

type getDiffInput struct {
	repositoryArg
	BaseRef   string `json:"base_ref" jsonschema:"the older version: a commit SHA, such as the source_version of the last successful run, or a branch or tag name."`
	TargetRef string `json:"target_ref" jsonschema:"the newer version: a commit SHA, such as the source_version of the failed run, or a branch or tag name."`
	scopeArgs
	BaseRefType   string `json:"base_ref_type,omitempty" jsonschema:"how to read base_ref: \"commit\" (default), \"branch\" or \"tag\"."`
	TargetRefType string `json:"target_ref_type,omitempty" jsonschema:"how to read target_ref: \"commit\" (default), \"branch\" or \"tag\"."`
	Top           int    `json:"top,omitempty" jsonschema:"how many changes this page holds, 100 by default."`
	Skip          int    `json:"skip,omitempty" jsonschema:"how many changes to skip before this page, from a previous result's next_skip."`
}

type diffChange struct {
	ChangeType       *string `json:"change_type"`
	Path             *string `json:"path"`
	OriginalPath     *string `json:"original_path"`
	ObjectID         *string `json:"object_id"`
	OriginalObjectID *string `json:"original_object_id"`
}

type getDiffOutput struct {
	Organization       string         `json:"organization"`
	Project            string         `json:"project"`
	Repository         string         `json:"repository"`
	BaseCommit         *string        `json:"base_commit"`
	TargetCommit       *string        `json:"target_commit"`
	CommonCommit       *string        `json:"common_commit"`
	AheadCount         *int           `json:"ahead_count"`
	BehindCount        *int           `json:"behind_count"`
	AllChangesIncluded *bool          `json:"all_changes_included"`
	ChangeCounts       map[string]int `json:"change_counts"`
	NextSkip           *int           `json:"next_skip"`
	Changes            []diffChange   `json:"changes"`
}

func (h *handlers) getDiff(ctx context.Context, in getDiffInput) (getDiffOutput, error) {
	orgURL, project, err := resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return getDiffOutput{}, err
	}
	if in.BaseRef == "" || in.TargetRef == "" {
		return getDiffOutput{}, errors.New("base_ref and target_ref are both required")
	}
	diffs, err := h.client.Diff(ctx, orgURL, project, in.Repository,
		ado.ItemVersion{Ref: in.BaseRef, RefType: in.BaseRefType}, ado.ItemVersion{Ref: in.TargetRef, RefType: in.TargetRefType},
		in.Top, in.Skip)
	if err != nil {
		return getDiffOutput{}, err
	}

	output := getDiffOutput{
		Organization: orgURL, Project: project, Repository: in.Repository, BaseCommit: diffs.BaseCommit,
		TargetCommit: diffs.TargetCommit, CommonCommit: diffs.CommonCommit, AheadCount: diffs.AheadCount,
		BehindCount: diffs.BehindCount, AllChangesIncluded: diffs.AllChangesIncluded, ChangeCounts: diffs.ChangeCounts,
		Changes: []diffChange{},
	}
	if output.ChangeCounts == nil {
		output.ChangeCounts = map[string]int{}
	}
	pageSize := in.Top
	if pageSize == 0 {
		pageSize = diffPageSize
	}
	if len(diffs.Changes) == pageSize {
		nextSkip := in.Skip + len(diffs.Changes)
		output.NextSkip = &nextSkip
	}
	for _, change := range diffs.Changes {
		if change.Item.IsFolder {
			continue
		}
		output.Changes = append(output.Changes, diffChange{
			ChangeType: change.ChangeType, Path: change.Item.Path, OriginalPath: change.OriginalPath,
			ObjectID: change.Item.ObjectID, OriginalObjectID: change.Item.OriginalObjectID,
		})
	}
	return output, nil
}

type listRefsInput struct {
	repositoryArg
	scopeArgs
	Filter         string `json:"filter,omitempty" jsonschema:"only refs whose name after \"refs/\" starts with this, e.g. \"tags/\" for tags, \"heads/\" for branches or \"tags/v1.\" for one version line."`
	FilterContains string `json:"filter_contains,omitempty" jsonschema:"only refs whose name contains this text."`
	Top            int    `json:"top,omitempty" jsonschema:"return at most this many refs, and a next_cursor if more remain. Omit it to return them all."`
	Cursor         string `json:"cursor,omitempty" jsonschema:"the next_cursor from a previous call, to continue where it stopped."`
}

type refSummary struct {
	Name      *string `json:"name"`
	CommitID  *string `json:"commit_id"`
	Annotated bool    `json:"annotated"`
}

type listRefsOutput struct {
	Organization string       `json:"organization"`
	Project      string       `json:"project"`
	Repository   string       `json:"repository"`
	NextCursor   *string      `json:"next_cursor"`
	Refs         []refSummary `json:"refs"`
}

func (h *handlers) listRefs(ctx context.Context, in listRefsInput) (listRefsOutput, error) {
	orgURL, project, err := resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return listRefsOutput{}, err
	}
	refs, next, err := collectPages(in.Top, in.Cursor, func(top int, cursor string) ([]ado.Ref, string, error) {
		return h.client.RefsPage(ctx, orgURL, project, in.Repository, in.Filter, in.FilterContains, top, cursor)
	})
	if err != nil {
		return listRefsOutput{}, err
	}
	output := listRefsOutput{Organization: orgURL, Project: project, Repository: in.Repository, NextCursor: next, Refs: []refSummary{}}
	for _, ref := range refs {
		summary := refSummary{Name: ref.Name, CommitID: ref.ObjectID}
		if ref.PeeledObjectID != nil {
			summary.CommitID, summary.Annotated = ref.PeeledObjectID, true
		}
		output.Refs = append(output.Refs, summary)
	}
	return output, nil
}
