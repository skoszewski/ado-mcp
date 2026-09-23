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

const organizationNotAllowedMessage = `this server serves the %[1]s organization only, so %[2]s is out of reach here. Drop
the organization argument, or set it to %[1]s. Tell the user this server cannot reach
%[2]s instead of retrying.`

const projectNotAllowedMessage = `this server serves the '%[1]s' project only, so '%[2]s' is out of reach here. Drop
the project argument, or set it to '%[1]s'. Tell the user this server cannot reach
'%[2]s' instead of retrying.`

const pipelineNotAllowedMessage = `this server serves the pipeline '%[1]s' (id %[2]d) only, so '%[3]s' is out of
reach here. Drop the pipeline argument, or set it to %[2]d. Tell the user this server
cannot reach '%[3]s' instead of retrying.`

const folderNotAllowedMessage = `this server serves the folder '%[1]s' and the folders below it only, so '%[2]s' is
out of reach here. Drop the folder_name argument, or name a folder inside '%[1]s'. Tell the
user this server cannot reach '%[2]s' instead of retrying.`

const noPipelineMessage = `no pipeline given for project '%s'. Pass the pipeline argument as a name or an id:
ado_list_pipelines reports the pipelines this project holds, and every run result carries the
pipeline_id and pipeline_name it belongs to. Do not guess a name.`

// Limits is the scope a server is confined to for its lifetime. Empty fields do not limit.
type Limits struct {
	Organization string
	Project      string
	FolderName   string
	PipelineID   int
	PipelineName string
}

// ResolveLimits builds the server's limits, normalizing the organization and folder and
// resolving a pipeline name to its ID so that a name that does not exist fails the launch.
func ResolveLimits(ctx context.Context, client *ado.Client, organization, project, folderName, pipeline string) (Limits, error) {
	limits := Limits{
		Organization: ado.NormalizeOrgURL(organization),
		Project:      project,
		FolderName:   NormalizeFolderPath(folderName),
	}
	if pipeline == "" {
		return limits, nil
	}
	if limits.Organization == "" || limits.Project == "" {
		return limits, errors.New("--pipeline requires --organization and --project")
	}
	var err error
	limits.PipelineID, limits.PipelineName, err = client.ResolvePipeline(ctx, limits.Organization, limits.Project, pipeline)
	return limits, err
}

// resolveScope resolves one tool call's organization URL and project against the limits. A
// call that names something outside them is refused rather than answered from elsewhere.
func (l Limits) resolveScope(organization, project string, needsProject bool) (string, string, error) {
	orgURL := ado.NormalizeOrgURL(organization)
	if l.Organization != "" {
		if orgURL != "" && !strings.EqualFold(orgURL, l.Organization) {
			return "", "", fmt.Errorf(organizationNotAllowedMessage, l.Organization, orgURL)
		}
		orgURL = l.Organization
	}
	if orgURL == "" {
		return "", "", errors.New(noOrganizationMessage)
	}

	if l.Project != "" {
		if project != "" && !strings.EqualFold(project, l.Project) {
			return "", "", fmt.Errorf(projectNotAllowedMessage, l.Project, project)
		}
		project = l.Project
	}
	if needsProject && project == "" {
		return "", "", errors.New(noProjectMessage)
	}
	return orgURL, project, nil
}

// resolveFolder resolves a tool's folder_name argument against the folder limit.
func (l Limits) resolveFolder(folderName string) (string, error) {
	folderName = NormalizeFolderPath(folderName)
	if l.FolderName == "" {
		return folderName, nil
	}
	if folderName != "" && !InFolder(folderName, l.FolderName, true) {
		return "", fmt.Errorf(folderNotAllowedMessage, l.FolderName, folderName)
	}
	if folderName == "" {
		return l.FolderName, nil
	}
	return folderName, nil
}

// resolvePipeline resolves a tool's pipeline argument to its ID and name, enforcing the
// pipeline limit. When optional is set, an omitted pipeline without a pipeline limit resolves
// to ID 0 for a project-wide call.
func (l Limits) resolvePipeline(ctx context.Context, client *ado.Client, orgURL, project, pipeline string, optional bool) (int, string, error) {
	if l.PipelineID == 0 {
		if pipeline != "" {
			return client.ResolvePipeline(ctx, orgURL, project, pipeline)
		}
		if optional {
			return 0, "", nil
		}
		return 0, "", fmt.Errorf(noPipelineMessage, project)
	}

	if pipeline == "" {
		return l.PipelineID, l.PipelineName, nil
	}
	id, name, err := client.ResolvePipeline(ctx, orgURL, project, pipeline)
	if err != nil {
		return 0, "", err
	}
	if id != l.PipelineID {
		return 0, "", fmt.Errorf(pipelineNotAllowedMessage, l.PipelineName, l.PipelineID, name)
	}
	return id, name, nil
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
