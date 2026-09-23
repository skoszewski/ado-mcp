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
	unrestricted := Limits{}
	if _, _, err := unrestricted.resolveScope("", "p", true); err == nil || !strings.HasPrefix(err.Error(), "no organization given") {
		t.Errorf("missing organization: got %v", err)
	}
	if _, _, err := unrestricted.resolveScope("org", "", true); err == nil || !strings.HasPrefix(err.Error(), "no project given") {
		t.Errorf("missing project: got %v", err)
	}
	orgURL, project, err := unrestricted.resolveScope("org", "", false)
	if err != nil || orgURL != "https://dev.azure.com/org" || project != "" {
		t.Errorf("project not needed: got %q %q %v", orgURL, project, err)
	}

	limited := Limits{Organization: "https://dev.azure.com/Org", Project: "Proj"}
	orgURL, project, err = limited.resolveScope("org", "proj", true)
	if err != nil || orgURL != "https://dev.azure.com/Org" || project != "Proj" {
		t.Errorf("case-insensitive match: got %q %q %v", orgURL, project, err)
	}
	orgURL, project, err = limited.resolveScope("", "", true)
	if err != nil || orgURL != limited.Organization || project != limited.Project {
		t.Errorf("filled from limits: got %q %q %v", orgURL, project, err)
	}
	if _, _, err := limited.resolveScope("other", "", true); err == nil || !strings.Contains(err.Error(), "out of reach") {
		t.Errorf("other organization: got %v", err)
	}
	if _, _, err := limited.resolveScope("", "other", true); err == nil || !strings.Contains(err.Error(), "out of reach") {
		t.Errorf("other project: got %v", err)
	}
}

func TestResolveFolder(t *testing.T) {
	limited := Limits{FolderName: `\lab`}
	if got, err := limited.resolveFolder(""); err != nil || got != `\lab` {
		t.Errorf("omitted folder: got %q %v", got, err)
	}
	if got, err := limited.resolveFolder("lab/dev"); err != nil || got != `\lab\dev` {
		t.Errorf("folder inside limit: got %q %v", got, err)
	}
	if _, err := limited.resolveFolder("other"); err == nil {
		t.Error("folder outside limit: expected an error")
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
