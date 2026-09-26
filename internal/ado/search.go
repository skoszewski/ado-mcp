package ado

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// searchURL is the base URL of the Azure DevOps search service.
var searchURL = "https://almsearch.dev.azure.com"

// CodeSearchQuery selects the files SearchCode returns. Empty fields do not filter.
type CodeSearchQuery struct {
	Text       string
	Repository string
	Path       string
	Branch     string
	Top        int
	Skip       int
}

// CodeSearchResult is the answer to a code search.
type CodeSearchResult struct {
	Count    int `json:"count"`
	InfoCode int `json:"infoCode"`
	Results  []struct {
		FileName *string `json:"fileName"`
		Path     *string `json:"path"`
		Project  *struct {
			Name *string `json:"name"`
		} `json:"project"`
		Repository *struct {
			Name *string `json:"name"`
		} `json:"repository"`
		Versions []struct {
			BranchName *string `json:"branchName"`
			ChangeID   *string `json:"changeId"`
		} `json:"versions"`
		Matches map[string][]struct {
			CharOffset int `json:"charOffset"`
			Length     int `json:"length"`
		} `json:"matches"`
	} `json:"results"`
}

// SearchCode searches the code of an organization's repositories, in one project when project
// is not empty. It needs the Code Search extension installed in the organization.
func (c *Client) SearchCode(ctx context.Context, orgURL, project string, q CodeSearchQuery) (CodeSearchResult, error) {
	requestURL := searchURL + "/" + url.PathEscape(strings.TrimPrefix(orgURL, "https://dev.azure.com/"))
	if project != "" {
		requestURL += "/" + url.PathEscape(project)
	}
	requestURL += "/_apis/search/codesearchresults?api-version=" + APIVersion

	filters := map[string][]string{}
	for name, value := range map[string]string{"Repository": q.Repository, "Path": q.Path, "Branch": q.Branch} {
		if value != "" {
			filters[name] = []string{value}
		}
	}
	body := map[string]any{"searchText": q.Text, "$top": q.Top, "$skip": q.Skip, "filters": filters}
	response, _, err := c.do(ctx, http.MethodPost, requestURL, body, true)
	if err != nil {
		return CodeSearchResult{}, err
	}
	var result CodeSearchResult
	err = decodeJSON(requestURL, response, &result)
	return result, err
}
