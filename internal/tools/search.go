package tools

import (
	"context"

	"github.com/skoszewski/ado-mcp/internal/ado"
)

type searchCodeInput struct {
	SearchText   string `json:"search_text" jsonschema:"text to search for, e.g. a Terraform variable, resource or module name from an error message."`
	Organization string `json:"organization,omitempty" jsonschema:"Azure DevOps organization name or URL. Omit it unless this conversation has named an organization. Never guess one."`
	Project      string `json:"project,omitempty" jsonschema:"only search this project. Omit it to search every project of the organization, which is how to find a shared module kept in another project."`
	Repository   string `json:"repository,omitempty" jsonschema:"only search this repository."`
	Path         string `json:"path,omitempty" jsonschema:"only search under this path, e.g. \"/modules\"."`
	Branch       string `json:"branch,omitempty" jsonschema:"only search this branch; omit it for each repository's default branch."`
	Top          int    `json:"top,omitempty" jsonschema:"how many files this page holds, 25 by default."`
	Skip         int    `json:"skip,omitempty" jsonschema:"how many files to skip before this page, from a previous result's next_skip."`
}

type codeMatch struct {
	Project    *string  `json:"project"`
	Repository *string  `json:"repository"`
	Path       *string  `json:"path"`
	Branches   []string `json:"branches"`
	MatchCount int      `json:"match_count"`
}

type searchCodeOutput struct {
	Organization string      `json:"organization"`
	Project      *string     `json:"project"`
	Count        int         `json:"count"`
	InfoCode     int         `json:"info_code"`
	NextSkip     *int        `json:"next_skip"`
	Files        []codeMatch `json:"files"`
}

func (h *handlers) searchCode(ctx context.Context, in searchCodeInput) (searchCodeOutput, error) {
	orgURL, project, err := resolveScope(in.Organization, in.Project, false)
	if err != nil {
		return searchCodeOutput{}, err
	}
	result, err := h.client.SearchCode(ctx, orgURL, project, ado.CodeSearchQuery{
		Text: in.SearchText, Repository: in.Repository, Path: in.Path, Branch: in.Branch, Top: in.Top, Skip: in.Skip,
	})
	if err != nil {
		return searchCodeOutput{}, err
	}
	output := searchCodeOutput{
		Organization: orgURL, Project: optional(project), Count: result.Count, InfoCode: result.InfoCode, Files: []codeMatch{},
	}
	if in.Skip+len(result.Results) < result.Count {
		nextSkip := in.Skip + len(result.Results)
		output.NextSkip = &nextSkip
	}
	for _, file := range result.Results {
		match := codeMatch{Path: file.Path, Branches: []string{}, MatchCount: len(file.Matches["content"])}
		if file.Project != nil {
			match.Project = file.Project.Name
		}
		if file.Repository != nil {
			match.Repository = file.Repository.Name
		}
		for _, version := range file.Versions {
			if version.BranchName != nil {
				match.Branches = append(match.Branches, *version.BranchName)
			}
		}
		output.Files = append(output.Files, match)
	}
	return output, nil
}
