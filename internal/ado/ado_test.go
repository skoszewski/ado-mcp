package ado

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewAuthorizerPAT(t *testing.T) {
	env := map[string]string{"AZURE_DEVOPS_PAT": "secret", "AZURE_TENANT_ID": "t", "AZURE_CLIENT_ID": "c", "AZURE_CLIENT_SECRET": "s"}
	authorizer, err := NewAuthorizer(func(name string) string { return env[name] })
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
	authorizer, err := NewAuthorizer(func(name string) string { return servicePrincipal[name] })
	if err != nil || !strings.HasPrefix(authorizer.Method(), "service principal") {
		t.Errorf("service principal: %v %v", authorizer, err)
	}

	partial := map[string]string{"AZURE_TENANT_ID": "t"}
	authorizer, err = NewAuthorizer(func(name string) string { return partial[name] })
	if err != nil || authorizer.Method() != "Azure CLI" {
		t.Errorf("Azure CLI fallback: %v %v", authorizer, err)
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
