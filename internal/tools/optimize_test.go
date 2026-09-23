package tools

import "testing"

func TestParseOptimizations(t *testing.T) {
	got, err := ParseOptimizations("small-model, log-type=task")
	if err != nil || !got.SmallModel || got.LogType != "task" {
		t.Errorf("valid settings: %+v %v", got, err)
	}
	if got.String() != "small-model, log-type=task" {
		t.Errorf("String() = %q", got.String())
	}

	if got, err := ParseOptimizations(""); err != nil || got != DefaultOptimizations() || got.String() != "" {
		t.Errorf("empty value: %+v %v", got, err)
	}

	for _, invalid := range []string{"unknown", "small-model=1", "log-type", "log-type=everything", "Bad_Name"} {
		if _, err := ParseOptimizations(invalid); err == nil {
			t.Errorf("ParseOptimizations(%q): expected an error", invalid)
		}
	}
}
