// Package ado reads Azure DevOps pipelines, runs, logs and Git repositories over its REST API.
package ado

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// APIVersion is the REST API version every request asks for, unless a resource needs its own.
const APIVersion = "7.1"

// Error codes Azure DevOps starts an error message with.
const (
	CodeRefNotFound  = "TF401175"
	CodePathNotFound = "TF401174"
)

var errorCodeRE = regexp.MustCompile(`^TF\d+$`)

// Client performs authenticated Azure DevOps REST requests.
type Client struct {
	HTTP *http.Client
	Auth Authorizer
}

// RequestError reports a failed request, carrying the message Azure DevOps answered with.
type RequestError struct {
	URL        string
	StatusCode int
	Status     string
	Message    string
}

// Unauthorized reports whether Azure DevOps refused the request for the authenticated identity.
func (e *RequestError) Unauthorized() bool {
	return e.StatusCode == http.StatusUnauthorized || e.StatusCode == http.StatusForbidden ||
		e.StatusCode == http.StatusNonAuthoritativeInfo
}

// NotFound reports whether Azure DevOps answered that the requested resource does not exist,
// which it also answers for a resource the identity is not allowed to see. A missing project is
// reported as 400 with error code TF200016.
func (e *RequestError) NotFound() bool {
	return e.StatusCode == http.StatusNotFound || e.Code() == "TF200016"
}

// Code returns the Azure DevOps error code the message starts with, such as TF401175, or "".
func (e *RequestError) Code() string {
	code, _, found := strings.Cut(e.Message, ":")
	if !found || !errorCodeRE.MatchString(code) {
		return ""
	}
	return code
}

func (e *RequestError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("request to %s failed: %s", e.URL, e.Status)
	}
	return fmt.Sprintf("request to %s failed: %s: %s", e.URL, e.Status, e.Message)
}

// NormalizeOrgURL returns an organization name or URL as its full https://dev.azure.com/<org>
// URL, or "" when org is empty.
func NormalizeOrgURL(org string) string {
	if org == "" || strings.HasPrefix(org, "https://dev.azure.com/") {
		return org
	}
	return "https://dev.azure.com/" + org
}

// projectURL returns the base URL of a project's REST resources.
func projectURL(orgURL, project string) string {
	return orgURL + "/" + url.PathEscape(project)
}

// encodeQuery encodes query parameters, leaving "$" unescaped in names such as $top.
func encodeQuery(query url.Values) string {
	return strings.ReplaceAll(query.Encode(), "%24", "$")
}

// get performs a GET request, authenticated unless authorize is false, and returns the body
// and response headers.
func (c *Client) get(ctx context.Context, requestURL string, authorize bool) ([]byte, http.Header, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, nil, err
	}
	request.Header.Set("Accept", "application/json")
	if authorize {
		authorization, err := c.Auth.Authorization(ctx)
		if err != nil {
			return nil, nil, err
		}
		request.Header.Set("Authorization", authorization)
	}

	response, err := c.HTTP.Do(request)
	if err != nil {
		return nil, nil, fmt.Errorf("request to %s failed: %w", requestURL, err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("request to %s failed: %w", requestURL, err)
	}
	// Azure DevOps answers credentials it does not accept with 203 and an HTML sign-in page.
	if response.StatusCode >= 300 || response.StatusCode == http.StatusNonAuthoritativeInfo {
		var failure struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &failure)
		return nil, nil, &RequestError{
			URL: requestURL, StatusCode: response.StatusCode, Status: response.Status, Message: failure.Message,
		}
	}
	return body, response.Header, nil
}

// getJSON performs an authenticated GET request and decodes the JSON body into out. An empty
// body leaves out unchanged.
func (c *Client) getJSON(ctx context.Context, requestURL string, out any) error {
	body, _, err := c.get(ctx, requestURL, true)
	if err != nil {
		return err
	}
	return decodeJSON(requestURL, body, out)
}

// getPage performs an authenticated GET of one page of a collection, returning its values and
// the continuation token Azure DevOps reports in the x-ms-continuationtoken header, which is
// absent on the last page.
func getPage[T any](ctx context.Context, c *Client, requestURL string) ([]T, string, error) {
	body, header, err := c.get(ctx, requestURL, true)
	if err != nil {
		return nil, "", err
	}
	var page struct {
		Value []T `json:"value"`
	}
	if err := decodeJSON(requestURL, body, &page); err != nil {
		return nil, "", err
	}
	return page.Value, header.Get("X-Ms-Continuationtoken"), nil
}

// getValues performs an authenticated GET of a collection and returns its value array.
func getValues[T any](ctx context.Context, c *Client, requestURL string) ([]T, error) {
	var page struct {
		Value []T `json:"value"`
	}
	err := c.getJSON(ctx, requestURL, &page)
	return page.Value, err
}

func decodeJSON(requestURL string, body []byte, out any) error {
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("response from %s is not the expected JSON: %w", requestURL, err)
	}
	return nil
}
