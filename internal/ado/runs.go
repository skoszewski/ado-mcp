package ado

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	ansiEscapeRE   = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	adoTimestampRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z\s+`)
)

// Build is one pipeline run.
type Build struct {
	ID            int      `json:"id"`
	BuildNumber   *string  `json:"buildNumber"`
	Status        *string  `json:"status"`
	Result        *string  `json:"result"`
	SourceBranch  *string  `json:"sourceBranch"`
	SourceVersion *string  `json:"sourceVersion"`
	QueueTime     *string  `json:"queueTime"`
	StartTime     *string  `json:"startTime"`
	FinishTime    *string  `json:"finishTime"`
	Reason        *string  `json:"reason"`
	Tags          []string `json:"tags"`
	RequestedFor  *struct {
		DisplayName *string `json:"displayName"`
	} `json:"requestedFor"`
	Definition *struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"definition"`
	Repository *struct {
		Name *string `json:"name"`
		Type *string `json:"type"`
	} `json:"repository"`
}

// TimelineRecord is one stage, job or step in a run's timeline.
type TimelineRecord struct {
	ID         *string          `json:"id"`
	ParentID   *string          `json:"parentId"`
	Type       *string          `json:"type"`
	Name       *string          `json:"name"`
	Order      *int             `json:"order"`
	State      *string          `json:"state"`
	Result     *string          `json:"result"`
	StartTime  *string          `json:"startTime"`
	FinishTime *string          `json:"finishTime"`
	Issues     []map[string]any `json:"issues"`
	Log        *struct {
		ID int `json:"id"`
	} `json:"log"`
}

// RunLog is one log a run produced, carrying a signed URL to its content.
type RunLog struct {
	ID            int     `json:"id"`
	LineCount     *int    `json:"lineCount"`
	CreatedOn     *string `json:"createdOn"`
	SignedContent *struct {
		URL string `json:"url"`
	} `json:"signedContent"`
}

// BuildQuery selects the runs BuildsPage returns. Empty fields do not filter.
type BuildQuery struct {
	PipelineID int
	Top        int
	Cursor     string
	Status     string
	Result     string
	Branch     string
	MinTime    string
	MaxTime    string
	Reason     string
}

// BuildsPage fetches one page of a project's runs, most recently started first, returning the
// continuation token for the next page, or "" on the last one.
func (c *Client) BuildsPage(ctx context.Context, orgURL, project string, q BuildQuery) ([]Build, string, error) {
	query := pageQuery(url.Values{"queryOrder": {"startTimeDescending"}, "api-version": {APIVersion}}, q.Top, q.Cursor)
	if q.PipelineID != 0 {
		query.Set("definitions", strconv.Itoa(q.PipelineID))
	}
	for name, value := range map[string]string{
		"statusFilter": q.Status, "resultFilter": q.Result, "branchName": q.Branch,
		"minTime": q.MinTime, "maxTime": q.MaxTime, "reasonFilter": q.Reason,
	} {
		if value != "" {
			query.Set(name, value)
		}
	}
	return getPage[Build](ctx, c, projectURL(orgURL, project)+"/_apis/build/builds?"+encodeQuery(query))
}

// Build fetches one run by its ID.
func (c *Client) Build(ctx context.Context, orgURL, project string, runID int) (Build, error) {
	var build Build
	buildURL := fmt.Sprintf("%s/_apis/build/builds/%d?api-version=%s", projectURL(orgURL, project), runID, APIVersion)
	err := c.getJSON(ctx, buildURL, &build)
	return build, err
}

// Timeline fetches the stage, job and step records of a run.
func (c *Client) Timeline(ctx context.Context, orgURL, project string, runID int) ([]TimelineRecord, error) {
	var timeline struct {
		Records []TimelineRecord `json:"records"`
	}
	timelineURL := fmt.Sprintf("%s/_apis/build/builds/%d/timeline?api-version=%s", projectURL(orgURL, project), runID, APIVersion)
	err := c.getJSON(ctx, timelineURL, &timeline)
	return timeline.Records, err
}

// RunLogs fetches the logs of a run, each with a signed content URL.
func (c *Client) RunLogs(ctx context.Context, orgURL, project string, pipelineID, runID int) ([]RunLog, error) {
	var logs struct {
		Logs []RunLog `json:"logs"`
	}
	logsURL := fmt.Sprintf("%s/_apis/pipelines/%d/runs/%d/logs?$expand=signedContent&api-version=%s",
		projectURL(orgURL, project), pipelineID, runID, APIVersion)
	err := c.getJSON(ctx, logsURL, &logs)
	return logs.Logs, err
}

// LogContent fetches a log's content from its signed URL, which needs no authorization.
func (c *Client) LogContent(ctx context.Context, log RunLog) (string, error) {
	if log.SignedContent == nil || log.SignedContent.URL == "" {
		return "", fmt.Errorf("log %d has no signed content URL available", log.ID)
	}
	body, _, err := c.get(ctx, log.SignedContent.URL, false)
	return strings.ToValidUTF8(string(body), "�"), err
}

// StripLogNoise removes ANSI escape codes, the per-line timestamp prefix and blank lines from
// log text.
func StripLogNoise(text string) string {
	var lines []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = ansiEscapeRE.ReplaceAllString(line, "")
		line = adoTimestampRE.ReplaceAllString(line, "")
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
