package ado

import (
	"context"
	"net/url"
	"strconv"
)

// Repository is a Git repository.
type Repository struct {
	ID            *string `json:"id"`
	Name          *string `json:"name"`
	DefaultBranch *string `json:"defaultBranch"`
	IsDisabled    *bool   `json:"isDisabled"`
	IsFork        *bool   `json:"isFork"`
	Size          *int64  `json:"size"`
	WebURL        *string `json:"webUrl"`
	RemoteURL     *string `json:"remoteUrl"`
}

// GitItem is a file or folder in a Git repository.
type GitItem struct {
	Path            *string `json:"path"`
	IsFolder        bool    `json:"isFolder"`
	GitObjectType   *string `json:"gitObjectType"`
	ObjectID        *string `json:"objectId"`
	CommitID        *string `json:"commitId"`
	Content         string  `json:"content"`
	ContentMetadata *struct {
		IsBinary bool `json:"isBinary"`
	} `json:"contentMetadata"`
}

// GitUserDate is the author or committer of a commit.
type GitUserDate struct {
	Name  *string `json:"name"`
	Email *string `json:"email"`
	Date  *string `json:"date"`
}

// Commit is a Git commit.
type Commit struct {
	CommitID         *string        `json:"commitId"`
	Comment          *string        `json:"comment"`
	CommentTruncated bool           `json:"commentTruncated"`
	Author           *GitUserDate   `json:"author"`
	Committer        *GitUserDate   `json:"committer"`
	ChangeCounts     map[string]int `json:"changeCounts"`
	RemoteURL        *string        `json:"remoteUrl"`
}

// CommitChanges is the list of paths a commit changed.
type CommitChanges struct {
	ChangeCounts map[string]int `json:"changeCounts"`
	Changes      []struct {
		ChangeType *string `json:"changeType"`
		Item       GitItem `json:"item"`
	} `json:"changes"`
}

// ItemVersion pins a Git request to a branch, tag or commit. An empty Ref reads the default
// branch.
type ItemVersion struct {
	Ref     string
	RefType string
}

func (v ItemVersion) apply(query url.Values, prefix string) {
	if v.Ref == "" {
		return
	}
	refType := v.RefType
	if refType == "" {
		refType = "branch"
	}
	query.Set(prefix+".version", v.Ref)
	query.Set(prefix+".versionType", refType)
}

func gitURL(orgURL, project, path string, query url.Values) string {
	query.Set("api-version", APIVersion)
	return projectURL(orgURL, project) + "/_apis/git/" + path + "?" + encodeQuery(query)
}

func repositoryPath(repository, rest string) string {
	return "repositories/" + url.PathEscape(repository) + rest
}

// Repositories lists the Git repositories a project holds.
func (c *Client) Repositories(ctx context.Context, orgURL, project string) ([]Repository, error) {
	return getValues[Repository](ctx, c, gitURL(orgURL, project, "repositories", url.Values{}))
}

// RepositoryItem reads one file, with its content and content metadata. Reading a folder fails
// with a message saying the path resolved to a Tree.
func (c *Client) RepositoryItem(ctx context.Context, orgURL, project, repository, path string, version ItemVersion) (GitItem, error) {
	query := url.Values{
		"path":                   {path},
		"includeContent":         {"true"},
		"includeContentMetadata": {"true"},
		"$format":                {"json"},
	}
	version.apply(query, "versionDescriptor")
	var item GitItem
	err := c.getJSON(ctx, gitURL(orgURL, project, repositoryPath(repository, "/items"), query), &item)
	return item, err
}

// RepositoryItems lists the entries under scopePath without their content.
func (c *Client) RepositoryItems(ctx context.Context, orgURL, project, repository, scopePath, recursionLevel string, version ItemVersion) ([]GitItem, error) {
	query := url.Values{"recursionLevel": {recursionLevel}, "$format": {"json"}}
	if scopePath != "" {
		query.Set("scopePath", scopePath)
	}
	version.apply(query, "versionDescriptor")
	return getValues[GitItem](ctx, c, gitURL(orgURL, project, repositoryPath(repository, "/items"), query))
}

// CommitQuery selects the commits Commits returns. Empty fields do not filter.
type CommitQuery struct {
	ItemPath string
	Version  ItemVersion
	Author   string
	FromDate string
	ToDate   string
	Top      int
	Skip     int
}

// Commits lists one page of a repository's commits, newest first. This endpoint pages on
// $top and $skip rather than on a continuation token.
func (c *Client) Commits(ctx context.Context, orgURL, project, repository string, q CommitQuery) ([]Commit, error) {
	query := url.Values{}
	q.Version.apply(query, "searchCriteria.itemVersion")
	for name, value := range map[string]string{
		"searchCriteria.itemPath": q.ItemPath, "searchCriteria.author": q.Author,
		"searchCriteria.fromDate": q.FromDate, "searchCriteria.toDate": q.ToDate,
	} {
		if value != "" {
			query.Set(name, value)
		}
	}
	if q.Top > 0 {
		query.Set("searchCriteria.$top", strconv.Itoa(q.Top))
	}
	if q.Skip > 0 {
		query.Set("searchCriteria.$skip", strconv.Itoa(q.Skip))
	}
	return getValues[Commit](ctx, c, gitURL(orgURL, project, repositoryPath(repository, "/commits"), query))
}

// CommitChanges lists the paths one commit changed. A top or skip of 0 is not sent.
func (c *Client) CommitChanges(ctx context.Context, orgURL, project, repository, commitID string, top, skip int) (CommitChanges, error) {
	query := url.Values{}
	if top > 0 {
		query.Set("top", strconv.Itoa(top))
	}
	if skip > 0 {
		query.Set("skip", strconv.Itoa(skip))
	}
	var changes CommitChanges
	path := repositoryPath(repository, "/commits/"+url.PathEscape(commitID)+"/changes")
	err := c.getJSON(ctx, gitURL(orgURL, project, path, query), &changes)
	return changes, err
}
