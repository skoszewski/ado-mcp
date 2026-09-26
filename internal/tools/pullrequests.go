package tools

import (
	"context"

	"github.com/skoszewski/ado-mcp/internal/ado"
)

type findPullRequestsInput struct {
	repositoryArg
	scopeArgs
	CommitID     string `json:"commit_id,omitempty" jsonschema:"find the pull requests that merged this commit or whose merge created it, e.g. the source_version of a run. The other filters are then ignored."`
	Status       string `json:"status,omitempty" jsonschema:"only pull requests in this state: \"active\" (the API's default), \"completed\", \"abandoned\" or \"all\"."`
	SourceBranch string `json:"source_branch,omitempty" jsonschema:"only pull requests from this branch, e.g. \"refs/heads/feature/x\"."`
	TargetBranch string `json:"target_branch,omitempty" jsonschema:"only pull requests into this branch, e.g. \"refs/heads/main\"."`
	Top          int    `json:"top,omitempty" jsonschema:"how many pull requests this page holds, 25 by default."`
	Skip         int    `json:"skip,omitempty" jsonschema:"how many pull requests to skip before this page, from a previous result's next_skip."`
}

type reviewerSummary struct {
	Name       *string `json:"name"`
	Vote       int     `json:"vote"`
	IsRequired bool    `json:"is_required"`
}

type pullRequestSummary struct {
	PullRequestID int               `json:"pull_request_id"`
	Title         *string           `json:"title"`
	Description   *string           `json:"description"`
	Status        *string           `json:"status"`
	IsDraft       bool              `json:"is_draft"`
	SourceBranch  *string           `json:"source_branch"`
	TargetBranch  *string           `json:"target_branch"`
	CreatedBy     *string           `json:"created_by"`
	CreationDate  *string           `json:"creation_date"`
	ClosedDate    *string           `json:"closed_date"`
	MergeStatus   *string           `json:"merge_status"`
	MergeCommit   *string           `json:"merge_commit"`
	Reviewers     []reviewerSummary `json:"reviewers"`
}

type findPullRequestsOutput struct {
	Organization string               `json:"organization"`
	Project      string               `json:"project"`
	Repository   string               `json:"repository"`
	CommitID     *string              `json:"commit_id"`
	NextSkip     *int                 `json:"next_skip"`
	PullRequests []pullRequestSummary `json:"pull_requests"`
}

func (h *handlers) findPullRequests(ctx context.Context, in findPullRequestsInput) (findPullRequestsOutput, error) {
	orgURL, project, err := resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return findPullRequestsOutput{}, err
	}
	output := findPullRequestsOutput{
		Organization: orgURL, Project: project, Repository: in.Repository, CommitID: optional(in.CommitID),
		PullRequests: []pullRequestSummary{},
	}
	var pullRequests []ado.PullRequest
	if in.CommitID != "" {
		pullRequests, err = h.client.PullRequestsByCommit(ctx, orgURL, project, in.Repository, in.CommitID)
	} else {
		pullRequests, err = h.client.PullRequests(ctx, orgURL, project, in.Repository, ado.PullRequestQuery{
			Status: in.Status, SourceRefName: in.SourceBranch, TargetRefName: in.TargetBranch, Top: in.Top, Skip: in.Skip,
		})
		if in.Top > 0 && len(pullRequests) == in.Top {
			nextSkip := in.Skip + len(pullRequests)
			output.NextSkip = &nextSkip
		}
	}
	if err != nil {
		return findPullRequestsOutput{}, err
	}
	for _, pullRequest := range pullRequests {
		summary := pullRequestSummary{
			PullRequestID: pullRequest.PullRequestID, Title: pullRequest.Title, Description: pullRequest.Description,
			Status: pullRequest.Status, IsDraft: pullRequest.IsDraft, SourceBranch: pullRequest.SourceRefName,
			TargetBranch: pullRequest.TargetRefName, CreatedBy: displayName(pullRequest.CreatedBy),
			CreationDate: pullRequest.CreationDate, ClosedDate: pullRequest.ClosedDate, MergeStatus: pullRequest.MergeStatus,
			Reviewers: []reviewerSummary{},
		}
		if pullRequest.LastMergeCommit != nil {
			summary.MergeCommit = pullRequest.LastMergeCommit.CommitID
		}
		for _, reviewer := range pullRequest.Reviewers {
			summary.Reviewers = append(summary.Reviewers, reviewerSummary{
				Name: reviewer.DisplayName, Vote: reviewer.Vote, IsRequired: reviewer.IsRequired,
			})
		}
		output.PullRequests = append(output.PullRequests, summary)
	}
	return output, nil
}

type pullRequestInput struct {
	repositoryArg
	PullRequestID int `json:"pull_request_id" jsonschema:"the pull_request_id ado_find_pull_requests reports."`
	scopeArgs
}

type listPullRequestThreadsInput struct {
	pullRequestInput
	IncludeSystem bool `json:"include_system,omitempty" jsonschema:"also return the comments Azure DevOps writes itself, such as votes, pushes and merge attempts."`
}

type commentSummary struct {
	Author        *string `json:"author"`
	PublishedDate *string `json:"published_date"`
	Type          *string `json:"type"`
	Content       *string `json:"content"`
}

type threadSummary struct {
	ThreadID      int              `json:"thread_id"`
	Status        *string          `json:"status"`
	FilePath      *string          `json:"file_path"`
	StartLine     *int             `json:"start_line"`
	EndLine       *int             `json:"end_line"`
	PublishedDate *string          `json:"published_date"`
	Comments      []commentSummary `json:"comments"`
}

type listPullRequestThreadsOutput struct {
	Organization  string          `json:"organization"`
	Project       string          `json:"project"`
	Repository    string          `json:"repository"`
	PullRequestID int             `json:"pull_request_id"`
	Threads       []threadSummary `json:"threads"`
}

func (h *handlers) listPullRequestThreads(ctx context.Context, in listPullRequestThreadsInput) (listPullRequestThreadsOutput, error) {
	orgURL, project, err := resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return listPullRequestThreadsOutput{}, err
	}
	threads, err := h.client.PullRequestThreads(ctx, orgURL, project, in.Repository, in.PullRequestID)
	if err != nil {
		return listPullRequestThreadsOutput{}, err
	}
	output := listPullRequestThreadsOutput{
		Organization: orgURL, Project: project, Repository: in.Repository, PullRequestID: in.PullRequestID,
		Threads: []threadSummary{},
	}
	for _, thread := range threads {
		if thread.IsDeleted {
			continue
		}
		summary := threadSummary{ThreadID: thread.ID, Status: thread.Status, PublishedDate: thread.PublishedDate, Comments: []commentSummary{}}
		if position := thread.ThreadContext; position != nil {
			summary.FilePath = position.FilePath
			if position.RightFileStart != nil {
				summary.StartLine = &position.RightFileStart.Line
			}
			if position.RightFileEnd != nil {
				summary.EndLine = &position.RightFileEnd.Line
			}
		}
		for _, comment := range thread.Comments {
			if comment.IsDeleted || (!in.IncludeSystem && comment.CommentType != nil && *comment.CommentType == "system") {
				continue
			}
			summary.Comments = append(summary.Comments, commentSummary{
				Author: displayName(comment.Author), PublishedDate: comment.PublishedDate, Type: comment.CommentType,
				Content: comment.Content,
			})
		}
		if len(summary.Comments) > 0 {
			output.Threads = append(output.Threads, summary)
		}
	}
	return output, nil
}

type policyEvaluationSummary struct {
	PolicyType    *string        `json:"policy_type"`
	IsBlocking    bool           `json:"is_blocking"`
	IsEnabled     bool           `json:"is_enabled"`
	Status        *string        `json:"status"`
	StartedDate   *string        `json:"started_date"`
	CompletedDate *string        `json:"completed_date"`
	Settings      map[string]any `json:"settings"`
}

type listPolicyEvaluationsOutput struct {
	Organization  string                    `json:"organization"`
	Project       string                    `json:"project"`
	PullRequestID int                       `json:"pull_request_id"`
	Evaluations   []policyEvaluationSummary `json:"evaluations"`
}

func (h *handlers) listPolicyEvaluations(ctx context.Context, in pullRequestInput) (listPolicyEvaluationsOutput, error) {
	orgURL, project, err := resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return listPolicyEvaluationsOutput{}, err
	}
	projectID, err := h.client.ProjectID(ctx, orgURL, project)
	if err != nil {
		return listPolicyEvaluationsOutput{}, err
	}
	evaluations, err := h.client.PolicyEvaluations(ctx, orgURL, projectID, in.PullRequestID)
	if err != nil {
		return listPolicyEvaluationsOutput{}, err
	}
	output := listPolicyEvaluationsOutput{
		Organization: orgURL, Project: project, PullRequestID: in.PullRequestID, Evaluations: []policyEvaluationSummary{},
	}
	for _, evaluation := range evaluations {
		summary := policyEvaluationSummary{
			Status: evaluation.Status, StartedDate: evaluation.StartedDate, CompletedDate: evaluation.CompletedDate,
			Settings: map[string]any{},
		}
		if configuration := evaluation.Configuration; configuration != nil {
			summary.IsBlocking, summary.IsEnabled = configuration.IsBlocking, configuration.IsEnabled
			if configuration.Type != nil {
				summary.PolicyType = configuration.Type.DisplayName
			}
			if configuration.Settings != nil {
				summary.Settings = configuration.Settings
			}
		}
		output.Evaluations = append(output.Evaluations, summary)
	}
	return output, nil
}
