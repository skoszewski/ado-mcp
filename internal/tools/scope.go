package tools

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/skoszewski/ado-mcp/internal/ado"
)

const noOrganizationMessage = `no organization given. The organization is a separate name from the project, pipeline and
repository, and cannot be derived from any of them. Every result from these tools carries the
organization it came from, so use the one an earlier result in this conversation reported. If
none has, ask the user which organization to work in rather than guessing.`

const noProjectMessage = `no project given. This call is not answerable as it stands, but it is retryable: every result
from these tools carries the project it came from, and the run, log or pipeline you are asking
about came from one, so repeat this same call with that project added. Do that rather than
reporting the failure or asking the user, which wastes a turn on something you were already
told. Only if no earlier result named a project: ado_list_projects lists the projects an
organization holds, and the user can say which one to work in. Never guess a name.`

const noPipelineMessage = `no pipeline given for project '%s'. Pass the pipeline argument as a name or an id:
ado_list_pipelines reports the pipelines this project holds, and every run result carries the
pipeline_id and pipeline_name it belongs to. Do not guess a name.`

// resolveScope returns one tool call's organization URL and project, failing with guidance for
// the model when either is missing.
func resolveScope(organization, project string, needsProject bool) (string, string, error) {
	orgURL := ado.NormalizeOrgURL(organization)
	if orgURL == "" {
		return "", "", errors.New(noOrganizationMessage)
	}
	if needsProject && project == "" {
		return "", "", errors.New(noProjectMessage)
	}
	return orgURL, project, nil
}

// resolvePipeline resolves a tool's pipeline argument to its ID and name. When optional is
// set, an omitted pipeline resolves to ID 0 for a project-wide call.
func resolvePipeline(ctx context.Context, client *ado.Client, orgURL, project, pipeline string, optional bool) (int, string, error) {
	if pipeline != "" {
		return client.ResolvePipeline(ctx, orgURL, project, pipeline)
	}
	if optional {
		return 0, "", nil
	}
	return 0, "", fmt.Errorf(noPipelineMessage, project)
}

// pipelineArgument returns a pipeline argument given as a JSON number or string as a string.
func pipelineArgument(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

// NormalizeFolderPath returns a folder path as the literal path Azure DevOps matches on: a
// leading backslash and backslash separators. Azure DevOps reports a path without the leading
// backslash as an empty list rather than an error, so forward slashes and a missing leading
// backslash are corrected here.
func NormalizeFolderPath(folderName string) string {
	if folderName == "" {
		return ""
	}
	path := strings.TrimRight(strings.ReplaceAll(folderName, "/", `\`), `\`)
	if path == "" {
		return `\`
	}
	return `\` + strings.TrimLeft(path, `\`)
}

// InFolder reports whether a definition at definitionPath belongs to folderName, compared
// case-insensitively as Azure DevOps does, and when recursive also to folders below it.
func InFolder(definitionPath, folderName string, recursive bool) bool {
	path := strings.ToLower(definitionPath)
	folder := strings.ToLower(folderName)
	if path == folder {
		return true
	}
	prefix := folder
	if !strings.HasSuffix(prefix, `\`) {
		prefix += `\`
	}
	return recursive && strings.HasPrefix(path, prefix)
}
