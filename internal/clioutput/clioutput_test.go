package clioutput

import (
	"bytes"
	"testing"
)

func capture(color bool, write func()) string {
	var buffer bytes.Buffer
	originalOut, originalColored := out, colored
	defer func() { out, colored = originalOut, originalColored }()
	out, colored = &buffer, color
	write()
	return buffer.String()
}

func TestPlainOutput(t *testing.T) {
	got := capture(false, func() {
		Info("created %d", 2)
		Error("no %s", "organization")
		Table([][]string{{"Scope:", "vso.build"}, {"Authorization ID:", "a1"}})
	})
	want := "created 2\nError: no organization\n  Scope:             vso.build\n  Authorization ID:  a1\n"
	if got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestColoredOutput(t *testing.T) {
	got := capture(true, func() { Success("done") })
	if got != green+"done"+reset+"\n" {
		t.Errorf("output = %q", got)
	}
}
