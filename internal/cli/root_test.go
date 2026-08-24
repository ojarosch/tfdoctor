package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ojarosch/tfdoctor/internal/analyze"
	"github.com/ojarosch/tfdoctor/internal/report"
	"github.com/ojarosch/tfdoctor/internal/rules"
)

func TestJSONReport(t *testing.T) {
	repo, err := analyze.Discover("../../testdata/healthy-opentofu")
	if err != nil {
		t.Fatal(err)
	}
	results := rules.RunAll(&analyze.Context{Repo: repo})
	var buf bytes.Buffer
	if err := report.JSON(&buf, "test", repo.Path, results); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Version string         `json:"version"`
		Summary map[string]int `json:"summary"`
		Results []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			File   string `json:"file"`
		} `json:"results"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if doc.Summary["fail"] != 0 {
		t.Fatalf("expected no failures, got %+v", doc.Summary)
	}
	found := map[string]bool{}
	for _, r := range doc.Results {
		found[r.ID] = true
	}
	if !found["runtime.version-pinned"] {
		t.Fatal("runtime.version-pinned missing from JSON")
	}
}

func TestTextReportContainsCategories(t *testing.T) {
	repo, err := analyze.Discover("../../testdata/committed-state")
	if err != nil {
		t.Fatal(err)
	}
	results := rules.RunAll(&analyze.Context{Repo: repo})
	var buf bytes.Buffer
	report.Text(&buf, results)
	out := buf.String()
	for _, want := range []string{"Runtime", "Repository", "failure"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q:\n%s", want, out)
		}
	}
}

func TestExitCodeMapping(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"../../testdata/committed-state"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected exit error for failing fixture")
	}
	var ee *exitError
	if !asExit(err, &ee) || ee.code != 1 {
		t.Fatalf("expected exit code 1, got %v", err)
	}

	cmd = newRootCmd()
	cmd.SetArgs([]string{"../../testdata/healthy-opentofu"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected clean run on healthy fixture, got %v", err)
	}
}

func asExit(err error, target **exitError) bool {
	if e, ok := err.(*exitError); ok {
		*target = e
		return true
	}
	return false
}

func TestBadFormatIsUsageError(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--format", "yaml", "../../testdata/healthy-opentofu"})
	err := cmd.Execute()
	if err == nil || strings.Contains(err.Error(), "exit") {
		t.Fatalf("expected format validation error, got %v", err)
	}
}
