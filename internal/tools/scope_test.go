package tools

import (
	"strings"
	"testing"
)

func TestNormalizeFolderPath(t *testing.T) {
	cases := map[string]string{
		"":                      "",
		"pipeline-templates":    `\pipeline-templates`,
		`\pipeline-templates`:   `\pipeline-templates`,
		"dom-lab-azure-lz/dev/": `\dom-lab-azure-lz\dev`,
		`\\double\`:             `\double`,
		"/":                     `\`,
		`\`:                     `\`,
	}
	for input, want := range cases {
		if got := NormalizeFolderPath(input); got != want {
			t.Errorf("NormalizeFolderPath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestInFolder(t *testing.T) {
	cases := []struct {
		path, folder string
		recursive    bool
		want         bool
	}{
		{`\Lab\Dev`, `\lab\dev`, false, true},
		{`\lab\dev\sub`, `\lab\dev`, true, true},
		{`\lab\dev\sub`, `\lab\dev`, false, false},
		{`\lab\development`, `\lab\dev`, true, false},
		{`\anything`, `\`, true, true},
	}
	for _, c := range cases {
		if got := InFolder(c.path, c.folder, c.recursive); got != c.want {
			t.Errorf("InFolder(%q, %q, %v) = %v, want %v", c.path, c.folder, c.recursive, got, c.want)
		}
	}
}

func TestResolveScope(t *testing.T) {
	if _, _, err := resolveScope("", "p", true); err == nil || !strings.HasPrefix(err.Error(), "no organization given") {
		t.Errorf("missing organization: got %v", err)
	}
	if _, _, err := resolveScope("org", "", true); err == nil || !strings.HasPrefix(err.Error(), "no project given") {
		t.Errorf("missing project: got %v", err)
	}
	orgURL, project, err := resolveScope("org", "", false)
	if err != nil || orgURL != "https://dev.azure.com/org" || project != "" {
		t.Errorf("project not needed: got %q %q %v", orgURL, project, err)
	}
	orgURL, project, err = resolveScope("https://dev.azure.com/org", "proj", true)
	if err != nil || orgURL != "https://dev.azure.com/org" || project != "proj" {
		t.Errorf("full URL: got %q %q %v", orgURL, project, err)
	}
}

func TestPipelineArgument(t *testing.T) {
	cases := map[any]string{nil: "", float64(42): "42", "deploy": "deploy"}
	for input, want := range cases {
		if got := pipelineArgument(input); got != want {
			t.Errorf("pipelineArgument(%v) = %q, want %q", input, got, want)
		}
	}
}
