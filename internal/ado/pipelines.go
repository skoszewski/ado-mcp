package ado

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Project is an Azure DevOps project.
type Project struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	State       *string `json:"state"`
	Visibility  *string `json:"visibility"`
	Description *string `json:"description"`
}

// Folder is a pipeline (build definition) folder.
type Folder struct {
	Path        *string `json:"path"`
	Description *string `json:"description"`
	CreatedOn   *string `json:"createdOn"`
}

// Definition is a pipeline (build definition).
type Definition struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Path        *string `json:"path"`
	QueueStatus *string `json:"queueStatus"`
	Revision    *int    `json:"revision"`
	Queue       *struct {
		Name *string `json:"name"`
	} `json:"queue"`
}

// QueueName returns the name of the definition's default agent queue, or nil.
func (d Definition) QueueName() *string {
	if d.Queue == nil {
		return nil
	}
	return d.Queue.Name
}

func pageQuery(query url.Values, top int, cursor string) url.Values {
	if top > 0 {
		query.Set("$top", strconv.Itoa(top))
	}
	if cursor != "" {
		query.Set("continuationToken", cursor)
	}
	return query
}

// ProjectsPage fetches one page of an organization's projects, returning the continuation
// token for the next page, or "" on the last one. A top of 0 uses the API's own page size.
func (c *Client) ProjectsPage(ctx context.Context, orgURL string, top int, cursor string) ([]Project, string, error) {
	query := pageQuery(url.Values{"api-version": {APIVersion}}, top, cursor)
	return getPage[Project](ctx, c, orgURL+"/_apis/projects?"+encodeQuery(query))
}

// Folders fetches the pipeline folders in a project, below path when it is not empty.
func (c *Client) Folders(ctx context.Context, orgURL, project, path string) ([]Folder, error) {
	foldersURL := projectURL(orgURL, project) + "/_apis/build/folders"
	if path != "" {
		foldersURL += "/" + url.PathEscape(path)
	}
	return getValues[Folder](ctx, c, foldersURL+"?queryOrder=folderAscending&api-version=7.1-preview.2")
}

// DefinitionsPage fetches one page of a project's pipeline definitions in ascending name order,
// returning the continuation token for the next page, or "" on the last one.
func (c *Client) DefinitionsPage(ctx context.Context, orgURL, project string, top int, cursor string) ([]Definition, string, error) {
	query := pageQuery(url.Values{"queryOrder": {"definitionNameAscending"}, "api-version": {APIVersion}}, top, cursor)
	return getPage[Definition](ctx, c, projectURL(orgURL, project)+"/_apis/build/definitions?"+encodeQuery(query))
}

// Definition fetches one pipeline definition by its ID.
func (c *Client) Definition(ctx context.Context, orgURL, project string, id int) (Definition, error) {
	var definition Definition
	definitionURL := fmt.Sprintf("%s/_apis/build/definitions/%d?api-version=%s", projectURL(orgURL, project), id, APIVersion)
	err := c.getJSON(ctx, definitionURL, &definition)
	return definition, err
}

// PipelineYAML returns a pipeline's final YAML with its templates expanded, through a preview
// run that queues nothing. A non-empty ref, such as "refs/heads/main", previews the YAML on that
// branch or tag of the pipeline's own repository.
func (c *Client) PipelineYAML(ctx context.Context, orgURL, project string, pipelineID int, ref string) (string, error) {
	body := map[string]any{"previewRun": true}
	if ref != "" {
		body["resources"] = map[string]any{"repositories": map[string]any{"self": map[string]string{"refName": ref}}}
	}
	previewURL := fmt.Sprintf("%s/_apis/pipelines/%d/preview?api-version=%s", projectURL(orgURL, project), pipelineID, APIVersion)
	response, _, err := c.do(ctx, http.MethodPost, previewURL, body, true)
	if err != nil {
		return "", err
	}
	var preview struct {
		FinalYAML string `json:"finalYaml"`
	}
	err = decodeJSON(previewURL, response, &preview)
	return preview.FinalYAML, err
}

// ProjectID returns the ID of a project given by name or ID.
func (c *Client) ProjectID(ctx context.Context, orgURL, project string) (string, error) {
	var result Project
	err := c.getJSON(ctx, orgURL+"/_apis/projects/"+url.PathEscape(project)+"?api-version="+APIVersion, &result)
	return result.ID, err
}

// ResolvePipeline resolves a pipeline name or numeric ID to its ID and name. A name that does
// not exist fails with the closest name matches.
func (c *Client) ResolvePipeline(ctx context.Context, orgURL, project, pipeline string) (int, string, error) {
	if id, err := strconv.Atoi(pipeline); err == nil {
		definition, err := c.Definition(ctx, orgURL, project, id)
		return id, definition.Name, err
	}

	var definitions []Definition
	cursor := ""
	for {
		page, next, err := c.DefinitionsPage(ctx, orgURL, project, 0, cursor)
		if err != nil {
			return 0, "", err
		}
		definitions = append(definitions, page...)
		if next == "" {
			break
		}
		cursor = next
	}

	for _, definition := range definitions {
		if definition.Name == pipeline {
			return definition.ID, definition.Name, nil
		}
	}

	lines := []string{
		fmt.Sprintf("Pipeline '%s' not found", pipeline),
		fmt.Sprintf("%d pipeline(s) returned for this project. Closest name matches:", len(definitions)),
	}
	needle := strings.ToLower(pipeline)
	for _, definition := range definitions {
		if len(lines) == 12 {
			break
		}
		if strings.Contains(strings.ToLower(definition.Name), needle) {
			path := ""
			if definition.Path != nil {
				path = *definition.Path
			}
			lines = append(lines, fmt.Sprintf("    %d\t%s\t%s", definition.ID, path, definition.Name))
		}
	}
	return 0, "", fmt.Errorf("%s", strings.Join(lines, "\n"))
}
