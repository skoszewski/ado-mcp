package ado

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewAuthorizerPAT(t *testing.T) {
	env := map[string]string{"AZURE_DEVOPS_PAT": "secret", "AZURE_TENANT_ID": "t", "AZURE_CLIENT_ID": "c", "AZURE_CLIENT_SECRET": "s"}
	authorizer, err := NewAuthorizer(AuthAuto, func(name string) string { return env[name] })
	if err != nil {
		t.Fatal(err)
	}
	header, err := authorizer.Authorization(context.Background())
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte(":secret"))
	if err != nil || header != want {
		t.Errorf("Authorization() = %q, %v; want %q", header, err, want)
	}
}

func TestNewAuthorizerSelection(t *testing.T) {
	servicePrincipal := map[string]string{"AZURE_TENANT_ID": "00000000-0000-0000-0000-000000000000", "AZURE_CLIENT_ID": "c", "AZURE_CLIENT_SECRET": "s"}
	authorizer, err := NewAuthorizer(AuthAuto, func(name string) string { return servicePrincipal[name] })
	if err != nil || !strings.HasPrefix(authorizer.Method(), "service principal") {
		t.Errorf("service principal: %v %v", authorizer, err)
	}

	partial := map[string]string{"AZURE_TENANT_ID": "t"}
	authorizer, err = NewAuthorizer(AuthAuto, func(name string) string { return partial[name] })
	if err != nil || authorizer.Method() != "Azure CLI" {
		t.Errorf("Azure CLI fallback: %v %v", authorizer, err)
	}
}

func TestNewAuthorizerForced(t *testing.T) {
	both := map[string]string{"AZURE_DEVOPS_PAT": "secret", "AZURE_TENANT_ID": "00000000-0000-0000-0000-000000000000", "AZURE_CLIENT_ID": "c", "AZURE_CLIENT_SECRET": "s"}
	getenv := func(name string) string { return both[name] }

	authorizer, err := NewAuthorizer(AuthServicePrincipal, getenv)
	if err != nil || !strings.HasPrefix(authorizer.Method(), "service principal") {
		t.Errorf("forced service principal: %v %v", authorizer, err)
	}
	authorizer, err = NewAuthorizer(AuthAzureCLI, getenv)
	if err != nil || authorizer.Method() != "Azure CLI" {
		t.Errorf("forced Azure CLI: %v %v", authorizer, err)
	}

	empty := func(string) string { return "" }
	for _, method := range []string{AuthPAT, AuthServicePrincipal, "bogus"} {
		if _, err := NewAuthorizer(method, empty); err == nil {
			t.Errorf("%s without settings: expected an error", method)
		}
	}

	authorizer, err = NewAuthorizer(AuthNone, getenv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authorizer.Authorization(context.Background()); err == nil {
		t.Error("none: Authorization without a header should fail")
	}
	client := &Client{HTTP: http.DefaultClient, Auth: authorizer}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer x" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.Write([]byte(`{"value":[]}`))
	}))
	defer server.Close()
	if _, _, err := client.ProjectsPage(WithAuthorization(context.Background(), "Bearer x"), server.URL+"/org", 1, ""); err != nil {
		t.Errorf("none with a header: %v", err)
	}
}

func TestGetPageAndErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Basic test" {
			t.Errorf("Authorization header = %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/org/_apis/projects":
			if r.URL.Query().Get("$top") != "1" || r.URL.Query().Get("continuationToken") != "a b" {
				t.Errorf("query = %q", r.URL.RawQuery)
			}
			w.Header().Set("x-ms-continuationtoken", "next")
			w.Write([]byte(`{"value":[{"id":"1","name":"P"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"TF200016: The following project does not exist: X"}`))
		}
	}))
	defer server.Close()

	client := &Client{HTTP: server.Client(), Auth: patAuthorizer{header: "Basic test"}}
	projects, next, err := client.ProjectsPage(context.Background(), server.URL+"/org", 1, "a b")
	if err != nil || len(projects) != 1 || projects[0].Name != "P" || next != "next" {
		t.Errorf("ProjectsPage = %v %q %v", projects, next, err)
	}

	_, err = client.Build(context.Background(), server.URL+"/org", "X", 1)
	var requestError *RequestError
	if !errors.As(err, &requestError) || !strings.Contains(requestError.Message, "TF200016") {
		t.Errorf("Build error = %v", err)
	}
}

func TestRequestAuthorizationOverride(t *testing.T) {
	var received string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = r.Header.Get("Authorization")
		w.Write([]byte(`{"value":[]}`))
	}))
	defer server.Close()

	client := &Client{HTTP: server.Client(), Auth: patAuthorizer{header: "Basic test"}}
	if _, _, err := client.ProjectsPage(context.Background(), server.URL+"/org", 1, ""); err != nil || received != "Basic test" {
		t.Errorf("configured: Authorization = %q, error %v", received, err)
	}
	ctx := WithAuthorization(context.Background(), "Bearer override")
	if _, _, err := client.ProjectsPage(ctx, server.URL+"/org", 1, ""); err != nil || received != "Bearer override" {
		t.Errorf("override: Authorization = %q, error %v", received, err)
	}
}

func TestDiagnosisRequests(t *testing.T) {
	var method, path, query string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path, query, body = r.Method, r.URL.Path, r.URL.RawQuery, nil
		if r.Body != nil {
			json.NewDecoder(r.Body).Decode(&body)
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/preview"):
			w.Write([]byte(`{"finalYaml":"stages:\n- stage: plan"}`))
		case strings.HasSuffix(r.URL.Path, "/pullrequestquery"):
			w.Write([]byte(`{"results":[{"c1":[{"pullRequestId":7}]},{"c1":[{"pullRequestId":7},{"pullRequestId":8}]}]}`))
		case strings.HasSuffix(r.URL.Path, "/codesearchresults"):
			w.Write([]byte(`{"count":1,"infoCode":0,"results":[{"path":"/main.tf","matches":{"content":[{"charOffset":1,"length":2}]}}]}`))
		case strings.HasSuffix(r.URL.Path, "/diffs/commits"):
			w.Write([]byte(`{"baseCommit":"a","targetCommit":"b","changes":[{"changeType":"edit","item":{"path":"/main.tf","objectId":"o2","originalObjectId":"o1"}}]}`))
		case strings.HasPrefix(r.URL.Path, "/org/_apis/projects/"):
			w.Write([]byte(`{"id":"p-id","name":"P"}`))
		default:
			w.Write([]byte(`{"value":[]}`))
		}
	}))
	defer server.Close()
	defer func(original string) { searchURL = original }(searchURL)
	searchURL = server.URL

	ctx := context.Background()
	org := server.URL + "/org"
	client := &Client{HTTP: server.Client(), Auth: patAuthorizer{header: "Basic test"}}
	check := func(name, wantMethod, wantPath string, wantQuery ...string) {
		t.Helper()
		if method != wantMethod || path != wantPath {
			t.Errorf("%s: request = %s %s", name, method, path)
		}
		for _, want := range wantQuery {
			if !strings.Contains(query, want) {
				t.Errorf("%s: query %q lacks %q", name, query, want)
			}
		}
	}

	client.RunChangesPage(ctx, org, "P", 5, 10, "c")
	check("RunChangesPage", "GET", "/org/P/_apis/build/builds/5/changes", "$top=10", "continuationToken=c")
	client.ChangesBetweenRuns(ctx, org, "P", 3, 5, 0)
	check("ChangesBetweenRuns", "GET", "/org/P/_apis/build/changes", "fromBuildId=3", "toBuildId=5")
	client.RunArtifacts(ctx, org, "P", 5)
	check("RunArtifacts", "GET", "/org/P/_apis/build/builds/5/artifacts")

	yaml, err := client.PipelineYAML(ctx, org, "P", 9, "refs/heads/main")
	check("PipelineYAML", "POST", "/org/P/_apis/pipelines/9/preview")
	ref, _ := body["resources"].(map[string]any)["repositories"].(map[string]any)["self"].(map[string]any)["refName"]
	if err != nil || yaml != "stages:\n- stage: plan" || body["previewRun"] != true || ref != "refs/heads/main" {
		t.Errorf("PipelineYAML = %q, %v; body %v", yaml, err, body)
	}

	diffs, err := client.Diff(ctx, org, "P", "infra", ItemVersion{Ref: "a", RefType: "commit"}, ItemVersion{Ref: "main", RefType: "branch"}, 0, 0)
	check("Diff", "GET", "/org/P/_apis/git/repositories/infra/diffs/commits",
		"baseVersion=a", "baseVersionType=commit", "targetVersion=main", "targetVersionType=branch", "diffCommonCommit=false")
	if err != nil || len(diffs.Changes) != 1 || *diffs.Changes[0].Item.OriginalObjectID != "o1" {
		t.Errorf("Diff = %+v, %v", diffs, err)
	}

	client.RefsPage(ctx, org, "P", "infra", "tags/", "v1", 0, "")
	check("RefsPage", "GET", "/org/P/_apis/git/repositories/infra/refs", "filter=tags%2F", "filterContains=v1", "peelTags=true")
	client.PullRequests(ctx, org, "P", "infra", PullRequestQuery{Status: "completed", TargetRefName: "refs/heads/main", Top: 5})
	check("PullRequests", "GET", "/org/P/_apis/git/repositories/infra/pullrequests",
		"searchCriteria.status=completed", "searchCriteria.targetRefName=refs%2Fheads%2Fmain", "$top=5")

	pullRequests, err := client.PullRequestsByCommit(ctx, org, "P", "infra", "c1")
	check("PullRequestsByCommit", "POST", "/org/P/_apis/git/repositories/infra/pullrequestquery")
	if err != nil || len(pullRequests) != 2 || pullRequests[0].PullRequestID != 7 || len(body["queries"].([]any)) != 2 {
		t.Errorf("PullRequestsByCommit = %+v, %v; body %v", pullRequests, err, body)
	}

	client.PullRequestThreads(ctx, org, "P", "infra", 7)
	check("PullRequestThreads", "GET", "/org/P/_apis/git/repositories/infra/pullRequests/7/threads")
	projectID, err := client.ProjectID(ctx, org, "P")
	check("ProjectID", "GET", "/org/_apis/projects/P")
	if err != nil || projectID != "p-id" {
		t.Errorf("ProjectID = %q, %v", projectID, err)
	}
	client.PolicyEvaluations(ctx, org, "p-id", 7)
	check("PolicyEvaluations", "GET", "/org/p-id/_apis/policy/evaluations",
		"artifactId=vstfs%3A%2F%2F%2FCodeReview%2FCodeReviewId%2Fp-id%2F7", "api-version=7.1-preview.1")

	client.ServiceEndpoints(ctx, org, "P", "azurerm")
	check("ServiceEndpoints", "GET", "/org/P/_apis/serviceendpoint/endpoints", "type=azurerm")
	client.ServiceEndpointHistoryPage(ctx, org, "P", "e1", 25, "c")
	check("ServiceEndpointHistoryPage", "GET", "/org/P/_apis/serviceendpoint/e1/executionhistory", "top=25", "continuationToken=c")
	client.VariableGroupsPage(ctx, org, "P", "tf-*", 0, "")
	check("VariableGroupsPage", "GET", "/org/P/_apis/distributedtask/variablegroups", "groupName=tf-%2A")
	client.EnvironmentsPage(ctx, org, "P", "prod", 0, "")
	check("EnvironmentsPage", "GET", "/org/P/_apis/distributedtask/environments", "name=prod")
	client.EnvironmentDeploymentsPage(ctx, org, "P", 4, 25, "")
	check("EnvironmentDeploymentsPage", "GET", "/org/P/_apis/distributedtask/environments/4/environmentdeploymentrecords", "top=25")
	client.AgentPools(ctx, org, "Default")
	check("AgentPools", "GET", "/org/_apis/distributedtask/pools", "poolName=Default")
	client.Agents(ctx, org, 1, "agent-1", true)
	check("Agents", "GET", "/org/_apis/distributedtask/pools/1/agents",
		"agentName=agent-1", "includeCapabilities=true", "includeLastCompletedRequest=true")

	result, err := client.SearchCode(ctx, "https://dev.azure.com/org", "P", CodeSearchQuery{Text: "backend", Repository: "infra", Top: 25})
	check("SearchCode", "POST", "/org/P/_apis/search/codesearchresults")
	filters, _ := body["filters"].(map[string]any)
	if err != nil || result.Count != 1 || body["searchText"] != "backend" || filters["Repository"] == nil || filters["Path"] != nil {
		t.Errorf("SearchCode = %+v, %v; body %v", result, err, body)
	}
}

func TestRequestErrorCode(t *testing.T) {
	cases := map[string]string{
		"TF401175:The version descriptor <Branch: main > could not be resolved": "TF401175",
		"TF200016: The following project does not exist: X":                     "TF200016",
		"Not Found": "",
		"":          "",
		"TFabc: x":  "",
	}
	for message, want := range cases {
		if got := (&RequestError{Message: message}).Code(); got != want {
			t.Errorf("Code() of %q = %q, want %q", message, got, want)
		}
	}
	if !(&RequestError{StatusCode: 400, Message: "TF200016: missing"}).NotFound() {
		t.Error("TF200016 with status 400: expected NotFound")
	}
}

func TestCreatePAT(t *testing.T) {
	var received PATRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/myorg/_apis/tokens/pats" || r.URL.Query().Get("api-version") != patAPIVersion {
			t.Errorf("request = %s %s", r.Method, r.URL)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		if received.Scope == "bogus" {
			w.Write([]byte(`{"patToken":null,"patTokenError":"invalidScope"}`))
			return
		}
		w.Write([]byte(`{"patToken":{"displayName":"ado-mcp","scope":"vso.project vso.build vso.code","token":"secret","authorizationId":"a1"},"patTokenError":"none"}`))
	}))
	defer server.Close()
	defer func(original string) { vsspsURL = original }(vsspsURL)
	vsspsURL = server.URL

	client := &Client{HTTP: server.Client(), Auth: patAuthorizer{header: "Bearer test"}}
	validTo := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	pat, err := client.CreatePAT(context.Background(), "https://dev.azure.com/myorg", PATRequest{DisplayName: "ado-mcp", Scope: ToolScopes, ValidTo: validTo})
	if err != nil || pat.Token != "secret" || pat.AuthorizationID != "a1" {
		t.Errorf("CreatePAT = %+v, %v", pat, err)
	}
	if received.DisplayName != "ado-mcp" || received.Scope != ToolScopes || !received.ValidTo.Equal(validTo) || received.AllOrgs {
		t.Errorf("request body = %+v", received)
	}

	if _, err := client.CreatePAT(context.Background(), "myorg", PATRequest{Scope: "bogus"}); err == nil || !strings.Contains(err.Error(), "invalidScope") {
		t.Errorf("rejected scope: got %v", err)
	}
}

func TestNormalizeOrgURL(t *testing.T) {
	cases := map[string]string{"": "", "org": "https://dev.azure.com/org", "https://dev.azure.com/org": "https://dev.azure.com/org"}
	for input, want := range cases {
		if got := NormalizeOrgURL(input); got != want {
			t.Errorf("NormalizeOrgURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestStripLogNoise(t *testing.T) {
	text := "2026-09-01T10:00:00.1234567Z \x1b[32mhello\x1b[0m\r\n\n  \n2026-09-01T10:00:01.0000000Z world  \n"
	if got := StripLogNoise(text); got != "hello\nworld" {
		t.Errorf("StripLogNoise = %q", got)
	}
}

func TestTimeline(t *testing.T) {
	name := func(value string) *string { return &value }
	order := func(value int) *int { return &value }
	records := []TimelineRecord{
		{Name: name("Build"), Type: name("Job"), Order: order(2), Log: &struct {
			ID int `json:"id"`
		}{ID: 5}},
		{Name: name("Checkout"), Type: name("Task"), Order: order(1), Log: &struct {
			ID int `json:"id"`
		}{ID: 3}},
		{Name: name("Stage"), Type: name("Stage")},
	}
	logInfo := MapLogsToTimeline(records)

	if got := DescribeLog(5, logInfo); got != "Build (combined job output)" {
		t.Errorf("DescribeLog(5) = %q", got)
	}
	if got := DescribeLog(3, logInfo); got != "Checkout (Task)" {
		t.Errorf("DescribeLog(3) = %q", got)
	}
	if got := DescribeLog(9, logInfo); got != "Log 9" {
		t.Errorf("DescribeLog(9) = %q", got)
	}

	if steps := StepEntries(records, logInfo, "job"); len(steps) != 1 || *steps[0].Name != "Build" {
		t.Errorf("job steps = %+v", steps)
	}
	steps := StepEntries(records, logInfo, "all")
	if len(steps) != 3 || *steps[0].Name != "Checkout" || *steps[2].Name != "Stage" {
		t.Errorf("all steps out of order: %+v", steps)
	}

	logs := []RunLog{{ID: 3}, {ID: 5}, {ID: 9}}
	if selected, err := SelectLogs(logs, logInfo, "task"); err != nil || len(selected) != 2 {
		t.Errorf("task logs = %+v %v", selected, err)
	}
	if _, err := SelectLogs(logs, logInfo, "bogus"); err == nil {
		t.Error("unknown log type: expected an error")
	}
}

func TestDurationSeconds(t *testing.T) {
	start, finish := "2026-09-01T10:00:00.5Z", "2026-09-01T10:01:00Z"
	if got := DurationSeconds(&start, &finish); got == nil || *got != 59.5 {
		t.Errorf("DurationSeconds = %v", got)
	}
	if DurationSeconds(&start, nil) != nil {
		t.Error("missing finish: expected nil")
	}
}
