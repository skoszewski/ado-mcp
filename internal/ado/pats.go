package ado

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PAT scopes CreatePAT accepts.
const (
	// ToolScopes are the scopes the MCP tools need: projects, builds, code, service connections,
	// variable groups and agent pools to read, and environments, which have no read-only scope.
	ToolScopes = "vso.project vso.build vso.code vso.serviceendpoint vso.variablegroups_read vso.agentpools vso.environment_manage"
	// FullScope grants full access to every Azure DevOps resource the user can reach.
	FullScope = "app_token"
)

const patAPIVersion = "7.1-preview.1"

// vsspsURL is the base URL of the Azure DevOps service that manages tokens.
var vsspsURL = "https://vssps.dev.azure.com"

// PATRequest describes the personal access token CreatePAT creates.
type PATRequest struct {
	DisplayName string    `json:"displayName"`
	Scope       string    `json:"scope"`
	ValidTo     time.Time `json:"validTo"`
	AllOrgs     bool      `json:"allOrgs"`
}

// PAT is a personal access token created by CreatePAT. Token holds the secret, which Azure
// DevOps returns only when the token is created.
type PAT struct {
	AuthorizationID string `json:"authorizationId"`
	DisplayName     string `json:"displayName"`
	Scope           string `json:"scope"`
	Token           string `json:"token"`
	ValidFrom       string `json:"validFrom"`
	ValidTo         string `json:"validTo"`
}

// CreatePAT creates a personal access token for the user the client authenticates as, through
// the PAT Lifecycle Management API. The API accepts only a Microsoft Entra token issued to a
// user; organization is the organization name or URL.
func (c *Client) CreatePAT(ctx context.Context, organization string, request PATRequest) (PAT, error) {
	name := strings.TrimSuffix(strings.TrimPrefix(NormalizeOrgURL(organization), "https://dev.azure.com/"), "/")
	requestURL := fmt.Sprintf("%s/%s/_apis/tokens/pats?api-version=%s", vsspsURL, url.PathEscape(name), patAPIVersion)

	body, _, err := c.do(ctx, http.MethodPost, requestURL, request, true)
	if err != nil {
		return PAT{}, err
	}
	var result struct {
		PatToken      PAT    `json:"patToken"`
		PatTokenError string `json:"patTokenError"`
	}
	if err := decodeJSON(requestURL, body, &result); err != nil {
		return PAT{}, err
	}
	if result.PatTokenError != "" && result.PatTokenError != "none" {
		return PAT{}, fmt.Errorf("Azure DevOps did not create the PAT: %s", result.PatTokenError)
	}
	if result.PatToken.Token == "" {
		return PAT{}, fmt.Errorf("the response from %s carried no token", requestURL)
	}
	return result.PatToken, nil
}
