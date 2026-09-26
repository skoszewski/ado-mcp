package ado

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// IdentityRef names a user or service identity.
type IdentityRef struct {
	DisplayName *string `json:"displayName"`
	UniqueName  *string `json:"uniqueName"`
}

// OrchestrationOwner is the pipeline or run an execution record belongs to.
type OrchestrationOwner struct {
	ID   int     `json:"id"`
	Name *string `json:"name"`
}

// ServiceEndpoint is a service connection. Its authorization parameters and data are not
// decoded.
type ServiceEndpoint struct {
	ID            *string `json:"id"`
	Name          *string `json:"name"`
	Type          *string `json:"type"`
	URL           *string `json:"url"`
	Description   *string `json:"description"`
	Owner         *string `json:"owner"`
	IsReady       *bool   `json:"isReady"`
	IsShared      *bool   `json:"isShared"`
	Authorization *struct {
		Scheme *string `json:"scheme"`
	} `json:"authorization"`
}

// ServiceEndpointExecution is one use of a service connection by a pipeline run.
type ServiceEndpointExecution struct {
	Data struct {
		ID         int64               `json:"id"`
		PlanType   *string             `json:"planType"`
		Definition *OrchestrationOwner `json:"definition"`
		Owner      *OrchestrationOwner `json:"owner"`
		StartTime  *string             `json:"startTime"`
		FinishTime *string             `json:"finishTime"`
		Result     *string             `json:"result"`
	} `json:"data"`
}

// VariableGroup is a variable group from the pipeline library. Azure DevOps returns secret
// values as null.
type VariableGroup struct {
	ID          int          `json:"id"`
	Name        *string      `json:"name"`
	Type        *string      `json:"type"`
	Description *string      `json:"description"`
	ModifiedOn  *string      `json:"modifiedOn"`
	ModifiedBy  *IdentityRef `json:"modifiedBy"`
	Variables   map[string]struct {
		Value      *string `json:"value"`
		IsSecret   bool    `json:"isSecret"`
		IsReadOnly bool    `json:"isReadOnly"`
	} `json:"variables"`
}

// Environment is a deployment environment of YAML pipelines.
type Environment struct {
	ID             int     `json:"id"`
	Name           *string `json:"name"`
	Description    *string `json:"description"`
	CreatedOn      *string `json:"createdOn"`
	LastModifiedOn *string `json:"lastModifiedOn"`
	Resources      []struct {
		ID   int      `json:"id"`
		Name *string  `json:"name"`
		Type *string  `json:"type"`
		Tags []string `json:"tags"`
	} `json:"resources"`
}

// EnvironmentDeployment is one deployment job that ran against an environment.
type EnvironmentDeployment struct {
	ID           int64               `json:"id"`
	Definition   *OrchestrationOwner `json:"definition"`
	Owner        *OrchestrationOwner `json:"owner"`
	StageName    *string             `json:"stageName"`
	JobName      *string             `json:"jobName"`
	StageAttempt *int                `json:"stageAttempt"`
	JobAttempt   *int                `json:"jobAttempt"`
	QueueTime    *string             `json:"queueTime"`
	StartTime    *string             `json:"startTime"`
	FinishTime   *string             `json:"finishTime"`
	Result       *string             `json:"result"`
}

// AgentPool is an organization's agent pool.
type AgentPool struct {
	ID       int     `json:"id"`
	Name     *string `json:"name"`
	PoolType *string `json:"poolType"`
	IsHosted bool    `json:"isHosted"`
	Size     *int    `json:"size"`
	Options  *string `json:"options"`
}

// JobRequest is a job an agent ran or is running.
type JobRequest struct {
	JobName    *string             `json:"jobName"`
	Definition *OrchestrationOwner `json:"definition"`
	Owner      *OrchestrationOwner `json:"owner"`
	QueueTime  *string             `json:"queueTime"`
	FinishTime *string             `json:"finishTime"`
	Result     *string             `json:"result"`
}

// Agent is a pipeline agent.
type Agent struct {
	ID                   int               `json:"id"`
	Name                 *string           `json:"name"`
	Status               *string           `json:"status"`
	Enabled              *bool             `json:"enabled"`
	Version              *string           `json:"version"`
	OSDescription        *string           `json:"osDescription"`
	StatusChangedOn      *string           `json:"statusChangedOn"`
	SystemCapabilities   map[string]string `json:"systemCapabilities"`
	UserCapabilities     map[string]string `json:"userCapabilities"`
	AssignedRequest      *JobRequest       `json:"assignedRequest"`
	LastCompletedRequest *JobRequest       `json:"lastCompletedRequest"`
}

// libraryURL returns the URL of a project's resource under _apis.
func libraryURL(orgURL, project, path string, query url.Values) string {
	query.Set("api-version", APIVersion)
	return projectURL(orgURL, project) + "/_apis/" + path + "?" + encodeQuery(query)
}

// historyQuery sets the top and continuationToken parameters of the execution history APIs,
// which take top without the $ prefix.
func historyQuery(top int, cursor string) url.Values {
	query := url.Values{}
	if top > 0 {
		query.Set("top", strconv.Itoa(top))
	}
	if cursor != "" {
		query.Set("continuationToken", cursor)
	}
	return query
}

// ServiceEndpoints lists a project's service connections, only those of endpointType when it is
// not empty.
func (c *Client) ServiceEndpoints(ctx context.Context, orgURL, project, endpointType string) ([]ServiceEndpoint, error) {
	query := url.Values{}
	if endpointType != "" {
		query.Set("type", endpointType)
	}
	return getValues[ServiceEndpoint](ctx, c, libraryURL(orgURL, project, "serviceendpoint/endpoints", query))
}

// ServiceEndpointHistoryPage fetches one page of a service connection's uses, returning the
// continuation token for the next page, or "" on the last one.
func (c *Client) ServiceEndpointHistoryPage(ctx context.Context, orgURL, project, endpointID string, top int, cursor string) ([]ServiceEndpointExecution, string, error) {
	path := "serviceendpoint/" + url.PathEscape(endpointID) + "/executionhistory"
	return getPage[ServiceEndpointExecution](ctx, c, libraryURL(orgURL, project, path, historyQuery(top, cursor)))
}

// VariableGroupsPage fetches one page of a project's variable groups, only those matching
// groupName when it is not empty, returning the continuation token for the next page, or "" on
// the last one.
func (c *Client) VariableGroupsPage(ctx context.Context, orgURL, project, groupName string, top int, cursor string) ([]VariableGroup, string, error) {
	query := pageQuery(url.Values{}, top, cursor)
	if groupName != "" {
		query.Set("groupName", groupName)
	}
	return getPage[VariableGroup](ctx, c, libraryURL(orgURL, project, "distributedtask/variablegroups", query))
}

// EnvironmentsPage fetches one page of a project's environments, only the one named name when it
// is not empty, returning the continuation token for the next page, or "" on the last one.
func (c *Client) EnvironmentsPage(ctx context.Context, orgURL, project, name string, top int, cursor string) ([]Environment, string, error) {
	query := pageQuery(url.Values{}, top, cursor)
	if name != "" {
		query.Set("name", name)
	}
	return getPage[Environment](ctx, c, libraryURL(orgURL, project, "distributedtask/environments", query))
}

// EnvironmentDeploymentsPage fetches one page of the deployments to an environment, returning
// the continuation token for the next page, or "" on the last one.
func (c *Client) EnvironmentDeploymentsPage(ctx context.Context, orgURL, project string, environmentID, top int, cursor string) ([]EnvironmentDeployment, string, error) {
	path := fmt.Sprintf("distributedtask/environments/%d/environmentdeploymentrecords", environmentID)
	return getPage[EnvironmentDeployment](ctx, c, libraryURL(orgURL, project, path, historyQuery(top, cursor)))
}

// AgentPools lists an organization's agent pools, only those named poolName when it is not
// empty.
func (c *Client) AgentPools(ctx context.Context, orgURL, poolName string) ([]AgentPool, error) {
	query := url.Values{"api-version": {APIVersion}}
	if poolName != "" {
		query.Set("poolName", poolName)
	}
	return getValues[AgentPool](ctx, c, orgURL+"/_apis/distributedtask/pools?"+encodeQuery(query))
}

// Agents lists the agents of a pool with their current and last completed jobs, only the agent
// named agentName when it is not empty, and with their capabilities when includeCapabilities is
// set.
func (c *Client) Agents(ctx context.Context, orgURL string, poolID int, agentName string, includeCapabilities bool) ([]Agent, error) {
	query := url.Values{
		"includeCapabilities":         {strconv.FormatBool(includeCapabilities)},
		"includeAssignedRequest":      {"true"},
		"includeLastCompletedRequest": {"true"},
		"api-version":                 {APIVersion},
	}
	if agentName != "" {
		query.Set("agentName", agentName)
	}
	return getValues[Agent](ctx, c, fmt.Sprintf("%s/_apis/distributedtask/pools/%d/agents?%s", orgURL, poolID, encodeQuery(query)))
}
