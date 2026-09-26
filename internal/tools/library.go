package tools

import (
	"context"
	"slices"
	"strings"

	"github.com/skoszewski/ado-mcp/internal/ado"
)

// organizationArg is the organization argument of the tools that apply to a whole organization.
type organizationArg struct {
	Organization string `json:"organization,omitempty" jsonschema:"Azure DevOps organization name or URL. Omit it unless this conversation has named an organization. Never guess one, and never reuse a project, pipeline or repository name as the organization: they are unrelated names."`
}

// ownerRun is the pipeline and run an execution record reports.
type ownerRun struct {
	PipelineID   *int    `json:"pipeline_id"`
	PipelineName *string `json:"pipeline_name"`
	RunID        *int    `json:"run_id"`
	RunName      *string `json:"run_name"`
}

func newOwnerRun(definition, owner *ado.OrchestrationOwner) ownerRun {
	var run ownerRun
	if definition != nil {
		run.PipelineID, run.PipelineName = &definition.ID, definition.Name
	}
	if owner != nil {
		run.RunID, run.RunName = &owner.ID, owner.Name
	}
	return run
}

type listServiceConnectionsInput struct {
	scopeArgs
	Type string `json:"type,omitempty" jsonschema:"only connections of this type, e.g. \"azurerm\" for Azure Resource Manager."`
}

type serviceConnectionSummary struct {
	ConnectionID        *string `json:"connection_id"`
	Name                *string `json:"name"`
	Type                *string `json:"type"`
	URL                 *string `json:"url"`
	AuthorizationScheme *string `json:"authorization_scheme"`
	IsReady             *bool   `json:"is_ready"`
	IsShared            *bool   `json:"is_shared"`
	Owner               *string `json:"owner"`
	Description         *string `json:"description"`
}

type listServiceConnectionsOutput struct {
	Organization string                     `json:"organization"`
	Project      string                     `json:"project"`
	Connections  []serviceConnectionSummary `json:"connections"`
}

func (h *handlers) listServiceConnections(ctx context.Context, in listServiceConnectionsInput) (listServiceConnectionsOutput, error) {
	orgURL, project, err := resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return listServiceConnectionsOutput{}, err
	}
	endpoints, err := h.client.ServiceEndpoints(ctx, orgURL, project, in.Type)
	if err != nil {
		return listServiceConnectionsOutput{}, err
	}
	output := listServiceConnectionsOutput{Organization: orgURL, Project: project, Connections: []serviceConnectionSummary{}}
	for _, endpoint := range endpoints {
		summary := serviceConnectionSummary{
			ConnectionID: endpoint.ID, Name: endpoint.Name, Type: endpoint.Type, URL: endpoint.URL,
			IsReady: endpoint.IsReady, IsShared: endpoint.IsShared, Owner: endpoint.Owner, Description: endpoint.Description,
		}
		if endpoint.Authorization != nil {
			summary.AuthorizationScheme = endpoint.Authorization.Scheme
		}
		output.Connections = append(output.Connections, summary)
	}
	return output, nil
}

type listServiceConnectionHistoryInput struct {
	ConnectionID string `json:"connection_id" jsonschema:"the connection_id ado_list_service_connections reports."`
	scopeArgs
	Top    int    `json:"top,omitempty" jsonschema:"how many uses this page holds, 25 by default; older ones are reached through next_cursor."`
	Cursor string `json:"cursor,omitempty" jsonschema:"the next_cursor from a previous call, to continue where it stopped."`
}

type connectionUse struct {
	ownerRun
	PlanType   *string `json:"plan_type"`
	StartTime  *string `json:"start_time"`
	FinishTime *string `json:"finish_time"`
	Result     *string `json:"result"`
}

type listServiceConnectionHistoryOutput struct {
	Organization string          `json:"organization"`
	Project      string          `json:"project"`
	ConnectionID string          `json:"connection_id"`
	NextCursor   *string         `json:"next_cursor"`
	Uses         []connectionUse `json:"uses"`
}

func (h *handlers) listServiceConnectionHistory(ctx context.Context, in listServiceConnectionHistoryInput) (listServiceConnectionHistoryOutput, error) {
	orgURL, project, err := resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return listServiceConnectionHistoryOutput{}, err
	}
	records, next, err := collectPages(in.Top, in.Cursor, func(top int, cursor string) ([]ado.ServiceEndpointExecution, string, error) {
		return h.client.ServiceEndpointHistoryPage(ctx, orgURL, project, in.ConnectionID, top, cursor)
	})
	if err != nil {
		return listServiceConnectionHistoryOutput{}, err
	}
	output := listServiceConnectionHistoryOutput{
		Organization: orgURL, Project: project, ConnectionID: in.ConnectionID, NextCursor: next, Uses: []connectionUse{},
	}
	for _, record := range records {
		output.Uses = append(output.Uses, connectionUse{
			ownerRun: newOwnerRun(record.Data.Definition, record.Data.Owner), PlanType: record.Data.PlanType,
			StartTime: record.Data.StartTime, FinishTime: record.Data.FinishTime, Result: record.Data.Result,
		})
	}
	return output, nil
}

type listVariableGroupsInput struct {
	scopeArgs
	GroupName string `json:"group_name,omitempty" jsonschema:"only groups with this name; \"*\" matches any text, e.g. \"terraform-*\"."`
	Top       int    `json:"top,omitempty" jsonschema:"return at most this many groups, and a next_cursor if more remain. Omit it to return them all."`
	Cursor    string `json:"cursor,omitempty" jsonschema:"the next_cursor from a previous call, to continue where it stopped."`
}

type variableSummary struct {
	Name       string  `json:"name"`
	Value      *string `json:"value"`
	IsSecret   bool    `json:"is_secret"`
	IsReadOnly bool    `json:"is_read_only"`
}

type variableGroupSummary struct {
	GroupID     int               `json:"group_id"`
	Name        *string           `json:"name"`
	Type        *string           `json:"type"`
	Description *string           `json:"description"`
	ModifiedOn  *string           `json:"modified_on"`
	ModifiedBy  *string           `json:"modified_by"`
	Variables   []variableSummary `json:"variables"`
}

type listVariableGroupsOutput struct {
	Organization string                 `json:"organization"`
	Project      string                 `json:"project"`
	NextCursor   *string                `json:"next_cursor"`
	Groups       []variableGroupSummary `json:"groups"`
}

func (h *handlers) listVariableGroups(ctx context.Context, in listVariableGroupsInput) (listVariableGroupsOutput, error) {
	orgURL, project, err := resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return listVariableGroupsOutput{}, err
	}
	groups, next, err := collectPages(in.Top, in.Cursor, func(top int, cursor string) ([]ado.VariableGroup, string, error) {
		return h.client.VariableGroupsPage(ctx, orgURL, project, in.GroupName, top, cursor)
	})
	if err != nil {
		return listVariableGroupsOutput{}, err
	}
	output := listVariableGroupsOutput{Organization: orgURL, Project: project, NextCursor: next, Groups: []variableGroupSummary{}}
	for _, group := range groups {
		summary := variableGroupSummary{
			GroupID: group.ID, Name: group.Name, Type: group.Type, Description: group.Description,
			ModifiedOn: group.ModifiedOn, ModifiedBy: displayName(group.ModifiedBy), Variables: []variableSummary{},
		}
		for name, variable := range group.Variables {
			summary.Variables = append(summary.Variables, variableSummary{
				Name: name, Value: variable.Value, IsSecret: variable.IsSecret, IsReadOnly: variable.IsReadOnly,
			})
		}
		slices.SortFunc(summary.Variables, func(a, b variableSummary) int { return strings.Compare(a.Name, b.Name) })
		output.Groups = append(output.Groups, summary)
	}
	return output, nil
}

type listEnvironmentsInput struct {
	scopeArgs
	Name   string `json:"name,omitempty" jsonschema:"only the environment with this name."`
	Top    int    `json:"top,omitempty" jsonschema:"return at most this many environments, and a next_cursor if more remain. Omit it to return them all."`
	Cursor string `json:"cursor,omitempty" jsonschema:"the next_cursor from a previous call, to continue where it stopped."`
}

type environmentResource struct {
	ResourceID int      `json:"resource_id"`
	Name       *string  `json:"name"`
	Type       *string  `json:"type"`
	Tags       []string `json:"tags"`
}

type environmentSummary struct {
	EnvironmentID  int                   `json:"environment_id"`
	Name           *string               `json:"name"`
	Description    *string               `json:"description"`
	CreatedOn      *string               `json:"created_on"`
	LastModifiedOn *string               `json:"last_modified_on"`
	Resources      []environmentResource `json:"resources"`
}

type listEnvironmentsOutput struct {
	Organization string               `json:"organization"`
	Project      string               `json:"project"`
	NextCursor   *string              `json:"next_cursor"`
	Environments []environmentSummary `json:"environments"`
}

func (h *handlers) listEnvironments(ctx context.Context, in listEnvironmentsInput) (listEnvironmentsOutput, error) {
	orgURL, project, err := resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return listEnvironmentsOutput{}, err
	}
	environments, next, err := collectPages(in.Top, in.Cursor, func(top int, cursor string) ([]ado.Environment, string, error) {
		return h.client.EnvironmentsPage(ctx, orgURL, project, in.Name, top, cursor)
	})
	if err != nil {
		return listEnvironmentsOutput{}, err
	}
	output := listEnvironmentsOutput{Organization: orgURL, Project: project, NextCursor: next, Environments: []environmentSummary{}}
	for _, environment := range environments {
		summary := environmentSummary{
			EnvironmentID: environment.ID, Name: environment.Name, Description: environment.Description,
			CreatedOn: environment.CreatedOn, LastModifiedOn: environment.LastModifiedOn, Resources: []environmentResource{},
		}
		for _, resource := range environment.Resources {
			tags := resource.Tags
			if tags == nil {
				tags = []string{}
			}
			summary.Resources = append(summary.Resources, environmentResource{
				ResourceID: resource.ID, Name: resource.Name, Type: resource.Type, Tags: tags,
			})
		}
		output.Environments = append(output.Environments, summary)
	}
	return output, nil
}

type listEnvironmentDeploymentsInput struct {
	EnvironmentID int `json:"environment_id" jsonschema:"the environment_id ado_list_environments reports."`
	scopeArgs
	Top    int    `json:"top,omitempty" jsonschema:"how many deployments this page holds, 25 by default; older ones are reached through next_cursor."`
	Cursor string `json:"cursor,omitempty" jsonschema:"the next_cursor from a previous call, to continue where it stopped."`
}

type deploymentSummary struct {
	ownerRun
	StageName    *string `json:"stage_name"`
	JobName      *string `json:"job_name"`
	StageAttempt *int    `json:"stage_attempt"`
	JobAttempt   *int    `json:"job_attempt"`
	QueueTime    *string `json:"queue_time"`
	StartTime    *string `json:"start_time"`
	FinishTime   *string `json:"finish_time"`
	Result       *string `json:"result"`
}

type listEnvironmentDeploymentsOutput struct {
	Organization  string              `json:"organization"`
	Project       string              `json:"project"`
	EnvironmentID int                 `json:"environment_id"`
	NextCursor    *string             `json:"next_cursor"`
	Deployments   []deploymentSummary `json:"deployments"`
}

func (h *handlers) listEnvironmentDeployments(ctx context.Context, in listEnvironmentDeploymentsInput) (listEnvironmentDeploymentsOutput, error) {
	orgURL, project, err := resolveScope(in.Organization, in.Project, true)
	if err != nil {
		return listEnvironmentDeploymentsOutput{}, err
	}
	records, next, err := collectPages(in.Top, in.Cursor, func(top int, cursor string) ([]ado.EnvironmentDeployment, string, error) {
		return h.client.EnvironmentDeploymentsPage(ctx, orgURL, project, in.EnvironmentID, top, cursor)
	})
	if err != nil {
		return listEnvironmentDeploymentsOutput{}, err
	}
	output := listEnvironmentDeploymentsOutput{
		Organization: orgURL, Project: project, EnvironmentID: in.EnvironmentID, NextCursor: next, Deployments: []deploymentSummary{},
	}
	for _, record := range records {
		output.Deployments = append(output.Deployments, deploymentSummary{
			ownerRun: newOwnerRun(record.Definition, record.Owner), StageName: record.StageName, JobName: record.JobName,
			StageAttempt: record.StageAttempt, JobAttempt: record.JobAttempt, QueueTime: record.QueueTime,
			StartTime: record.StartTime, FinishTime: record.FinishTime, Result: record.Result,
		})
	}
	return output, nil
}

type listAgentPoolsInput struct {
	organizationArg
	PoolName string `json:"pool_name,omitempty" jsonschema:"only the pool with this name."`
}

type agentPoolSummary struct {
	PoolID   int     `json:"pool_id"`
	Name     *string `json:"name"`
	PoolType *string `json:"pool_type"`
	IsHosted bool    `json:"is_hosted"`
	Size     *int    `json:"size"`
	Options  *string `json:"options"`
}

type listAgentPoolsOutput struct {
	Organization string             `json:"organization"`
	Pools        []agentPoolSummary `json:"pools"`
}

func (h *handlers) listAgentPools(ctx context.Context, in listAgentPoolsInput) (listAgentPoolsOutput, error) {
	orgURL, _, err := resolveScope(in.Organization, "", false)
	if err != nil {
		return listAgentPoolsOutput{}, err
	}
	pools, err := h.client.AgentPools(ctx, orgURL, in.PoolName)
	if err != nil {
		return listAgentPoolsOutput{}, err
	}
	output := listAgentPoolsOutput{Organization: orgURL, Pools: []agentPoolSummary{}}
	for _, pool := range pools {
		output.Pools = append(output.Pools, agentPoolSummary{
			PoolID: pool.ID, Name: pool.Name, PoolType: pool.PoolType, IsHosted: pool.IsHosted, Size: pool.Size, Options: pool.Options,
		})
	}
	return output, nil
}

type listAgentsInput struct {
	PoolID int `json:"pool_id" jsonschema:"the pool_id ado_list_agent_pools reports."`
	organizationArg
	AgentName           string `json:"agent_name,omitempty" jsonschema:"only the agent with this name."`
	IncludeCapabilities bool   `json:"include_capabilities,omitempty" jsonschema:"also return each agent's system and user capabilities, such as installed tool versions and environment variables. They are long, so ask for them for one agent_name."`
}

type agentJob struct {
	ownerRun
	JobName    *string `json:"job_name"`
	QueueTime  *string `json:"queue_time"`
	FinishTime *string `json:"finish_time"`
	Result     *string `json:"result"`
}

func newAgentJob(request *ado.JobRequest) *agentJob {
	if request == nil {
		return nil
	}
	return &agentJob{
		ownerRun: newOwnerRun(request.Definition, request.Owner), JobName: request.JobName,
		QueueTime: request.QueueTime, FinishTime: request.FinishTime, Result: request.Result,
	}
}

type agentSummary struct {
	AgentID            int               `json:"agent_id"`
	Name               *string           `json:"name"`
	Status             *string           `json:"status"`
	Enabled            *bool             `json:"enabled"`
	Version            *string           `json:"version"`
	OSDescription      *string           `json:"os_description"`
	StatusChangedOn    *string           `json:"status_changed_on"`
	CurrentJob         *agentJob         `json:"current_job"`
	LastJob            *agentJob         `json:"last_job"`
	SystemCapabilities map[string]string `json:"system_capabilities,omitempty"`
	UserCapabilities   map[string]string `json:"user_capabilities,omitempty"`
}

type listAgentsOutput struct {
	Organization string         `json:"organization"`
	PoolID       int            `json:"pool_id"`
	Agents       []agentSummary `json:"agents"`
}

func (h *handlers) listAgents(ctx context.Context, in listAgentsInput) (listAgentsOutput, error) {
	orgURL, _, err := resolveScope(in.Organization, "", false)
	if err != nil {
		return listAgentsOutput{}, err
	}
	agents, err := h.client.Agents(ctx, orgURL, in.PoolID, in.AgentName, in.IncludeCapabilities)
	if err != nil {
		return listAgentsOutput{}, err
	}
	output := listAgentsOutput{Organization: orgURL, PoolID: in.PoolID, Agents: []agentSummary{}}
	for _, agent := range agents {
		output.Agents = append(output.Agents, agentSummary{
			AgentID: agent.ID, Name: agent.Name, Status: agent.Status, Enabled: agent.Enabled, Version: agent.Version,
			OSDescription: agent.OSDescription, StatusChangedOn: agent.StatusChangedOn,
			CurrentJob: newAgentJob(agent.AssignedRequest), LastJob: newAgentJob(agent.LastCompletedRequest),
			SystemCapabilities: agent.SystemCapabilities, UserCapabilities: agent.UserCapabilities,
		})
	}
	return output, nil
}
