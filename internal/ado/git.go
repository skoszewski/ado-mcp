package ado

import (
	"context"
	"net/http"
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

// CommitDiffs is the list of paths that differ between two versions of a repository.
type CommitDiffs struct {
	BaseCommit         *string        `json:"baseCommit"`
	TargetCommit       *string        `json:"targetCommit"`
	CommonCommit       *string        `json:"commonCommit"`
	AheadCount         *int           `json:"aheadCount"`
	BehindCount        *int           `json:"behindCount"`
	AllChangesIncluded *bool          `json:"allChangesIncluded"`
	ChangeCounts       map[string]int `json:"changeCounts"`
	Changes            []struct {
		ChangeType   *string `json:"changeType"`
		OriginalPath *string `json:"originalPath"`
		Item         struct {
			GitItem
			OriginalObjectID *string `json:"originalObjectId"`
		} `json:"item"`
	} `json:"changes"`
}

// Diff lists the paths that differ between the base and target versions of a repository, one
// page of top changes after skip. A top or skip of 0 is not sent.
func (c *Client) Diff(ctx context.Context, orgURL, project, repository string, base, target ItemVersion, top, skip int) (CommitDiffs, error) {
	query := url.Values{"diffCommonCommit": {"false"}}
	for prefix, version := range map[string]ItemVersion{"base": base, "target": target} {
		query.Set(prefix+"Version", version.Ref)
		if version.RefType != "" {
			query.Set(prefix+"VersionType", version.RefType)
		}
	}
	if top > 0 {
		query.Set("$top", strconv.Itoa(top))
	}
	if skip > 0 {
		query.Set("$skip", strconv.Itoa(skip))
	}
	var diffs CommitDiffs
	err := c.getJSON(ctx, gitURL(orgURL, project, repositoryPath(repository, "/diffs/commits"), query), &diffs)
	return diffs, err
}

// Ref is a branch, tag or other Git ref.
type Ref struct {
	Name           *string `json:"name"`
	ObjectID       *string `json:"objectId"`
	PeeledObjectID *string `json:"peeledObjectId"`
}

// RefsPage fetches one page of a repository's refs whose names, without the "refs/" prefix,
// start with filter and contain filterContains, returning the continuation token for the next
// page, or "" on the last one. An annotated tag reports the commit it points to as its
// PeeledObjectID.
func (c *Client) RefsPage(ctx context.Context, orgURL, project, repository, filter, filterContains string, top int, cursor string) ([]Ref, string, error) {
	query := pageQuery(url.Values{"peelTags": {"true"}}, top, cursor)
	if filter != "" {
		query.Set("filter", filter)
	}
	if filterContains != "" {
		query.Set("filterContains", filterContains)
	}
	return getPage[Ref](ctx, c, gitURL(orgURL, project, repositoryPath(repository, "/refs"), query))
}

// PullRequest is a Git pull request.
type PullRequest struct {
	PullRequestID   int          `json:"pullRequestId"`
	Title           *string      `json:"title"`
	Description     *string      `json:"description"`
	Status          *string      `json:"status"`
	IsDraft         bool         `json:"isDraft"`
	SourceRefName   *string      `json:"sourceRefName"`
	TargetRefName   *string      `json:"targetRefName"`
	CreatedBy       *IdentityRef `json:"createdBy"`
	CreationDate    *string      `json:"creationDate"`
	ClosedDate      *string      `json:"closedDate"`
	MergeStatus     *string      `json:"mergeStatus"`
	LastMergeCommit *struct {
		CommitID *string `json:"commitId"`
	} `json:"lastMergeCommit"`
	Reviewers []struct {
		DisplayName *string `json:"displayName"`
		Vote        int     `json:"vote"`
		IsRequired  bool    `json:"isRequired"`
	} `json:"reviewers"`
}

// PullRequestQuery selects the pull requests PullRequests returns. Empty fields do not filter.
type PullRequestQuery struct {
	Status        string
	SourceRefName string
	TargetRefName string
	Top           int
	Skip          int
}

// PullRequests lists one page of the pull requests into a repository. Without a status Azure
// DevOps returns only active pull requests.
func (c *Client) PullRequests(ctx context.Context, orgURL, project, repository string, q PullRequestQuery) ([]PullRequest, error) {
	query := url.Values{}
	for name, value := range map[string]string{
		"searchCriteria.status": q.Status, "searchCriteria.sourceRefName": q.SourceRefName,
		"searchCriteria.targetRefName": q.TargetRefName,
	} {
		if value != "" {
			query.Set(name, value)
		}
	}
	if q.Top > 0 {
		query.Set("$top", strconv.Itoa(q.Top))
	}
	if q.Skip > 0 {
		query.Set("$skip", strconv.Itoa(q.Skip))
	}
	return getValues[PullRequest](ctx, c, gitURL(orgURL, project, repositoryPath(repository, "/pullrequests"), query))
}

// PullRequestsByCommit returns the pull requests whose merge created commitID, followed by those
// that merged it, each once.
func (c *Client) PullRequestsByCommit(ctx context.Context, orgURL, project, repository, commitID string) ([]PullRequest, error) {
	body := map[string]any{"queries": []map[string]any{
		{"type": "lastMergeCommit", "items": []string{commitID}},
		{"type": "commit", "items": []string{commitID}},
	}}
	queryURL := gitURL(orgURL, project, repositoryPath(repository, "/pullrequestquery"), url.Values{})
	response, _, err := c.do(ctx, http.MethodPost, queryURL, body, true)
	if err != nil {
		return nil, err
	}
	var result struct {
		Results []map[string][]PullRequest `json:"results"`
	}
	if err := decodeJSON(queryURL, response, &result); err != nil {
		return nil, err
	}
	pullRequests := []PullRequest{}
	seen := map[int]bool{}
	for _, byCommit := range result.Results {
		for _, pullRequest := range byCommit[commitID] {
			if !seen[pullRequest.PullRequestID] {
				seen[pullRequest.PullRequestID] = true
				pullRequests = append(pullRequests, pullRequest)
			}
		}
	}
	return pullRequests, nil
}

// Thread is a comment thread of a pull request.
type Thread struct {
	ID            int     `json:"id"`
	Status        *string `json:"status"`
	IsDeleted     bool    `json:"isDeleted"`
	PublishedDate *string `json:"publishedDate"`
	ThreadContext *struct {
		FilePath       *string `json:"filePath"`
		RightFileStart *struct {
			Line int `json:"line"`
		} `json:"rightFileStart"`
		RightFileEnd *struct {
			Line int `json:"line"`
		} `json:"rightFileEnd"`
	} `json:"threadContext"`
	Comments []struct {
		ID            int          `json:"id"`
		Author        *IdentityRef `json:"author"`
		Content       *string      `json:"content"`
		CommentType   *string      `json:"commentType"`
		PublishedDate *string      `json:"publishedDate"`
		IsDeleted     bool         `json:"isDeleted"`
	} `json:"comments"`
}

// PullRequestThreads lists the comment threads of a pull request.
func (c *Client) PullRequestThreads(ctx context.Context, orgURL, project, repository string, pullRequestID int) ([]Thread, error) {
	path := repositoryPath(repository, "/pullRequests/"+strconv.Itoa(pullRequestID)+"/threads")
	return getValues[Thread](ctx, c, gitURL(orgURL, project, path, url.Values{}))
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
