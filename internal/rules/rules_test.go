package rules

import (
	"strings"
	"testing"

	"github.com/ojarosch/tfdoctor/internal/analyze"
)

func analyzeFixture(t *testing.T, name string) []analyze.Result {
	t.Helper()
	repo, err := analyze.Discover("../../testdata/" + name)
	if err != nil {
		t.Fatalf("discover %s: %v", name, err)
	}
	return RunAll(&analyze.Context{Repo: repo})
}

// find returns the result for a rule ID (empty if the rule emitted nothing).
func find(results []analyze.Result, id string) analyze.Result {
	for _, r := range results {
		if r.ID == id {
			return r
		}
	}
	return analyze.Result{}
}

func assertStatus(t *testing.T, results []analyze.Result, id string, want analyze.Status) {
	t.Helper()
	got := find(results, id)
	if got.ID == "" {
		t.Fatalf("%s: rule emitted no result; want %s", id, want)
	}
	if got.Status != want {
		t.Fatalf("%s: got status %q, want %q", id, got.Status, want)
	}
}

func TestHealthyOpenTofu(t *testing.T) {
	res := analyzeFixture(t, "healthy-opentofu")
	assertStatus(t, res, "runtime.version-pinned", analyze.Pass)
	assertStatus(t, res, "runtime.required-version", analyze.Pass)
	assertStatus(t, res, "providers.lockfile-present", analyze.Pass)
	assertStatus(t, res, "providers.version-constraints", analyze.Pass)
	assertStatus(t, res, "providers.source-explicit", analyze.Pass)
	assertStatus(t, res, "modules.remote-version-pinned", analyze.Pass)
	assertStatus(t, res, "modules.git-ref-pinned", analyze.Pass)
	assertStatus(t, res, "repository.terraform-directory-ignored", analyze.Pass)
	assertStatus(t, res, "repository.state-files-ignored", analyze.Pass)
	assertStatus(t, res, "repository.state-file-present", analyze.Pass)

	for _, r := range res {
		if r.Status == analyze.Fail {
			t.Fatalf("healthy-opentofu produced failure: %+v", r)
		}
	}
}

func TestHealthyTerraformBackendAndCI(t *testing.T) {
	res := analyzeFixture(t, "healthy-terraform")
	b := find(res, "backend.detect")
	if b.Title != "Backend: azurerm" || b.File != "main.tf" {
		t.Fatalf("backend.detect = %+v", b)
	}
	assertStatus(t, res, "ci.input-disabled", analyze.Pass)
	assertStatus(t, res, "ci.automation-env", analyze.Pass)

	d := find(res, "runtime.detect")
	if d.Status != analyze.Info || d.Title != "Terraform detected" {
		t.Fatalf("runtime.detect = %+v", d)
	}
}

func TestMissingLockfile(t *testing.T) {
	res := analyzeFixture(t, "missing-lockfile")
	assertStatus(t, res, "providers.lockfile-present", analyze.Warn)
	assertStatus(t, res, "providers.version-constraints", analyze.Pass)
}

func TestCommittedState(t *testing.T) {
	res := analyzeFixture(t, "committed-state")
	assertStatus(t, res, "repository.state-file-present", analyze.Fail)
	assertStatus(t, res, "repository.state-files-ignored", analyze.Fail)
	assertStatus(t, res, "repository.terraform-directory-ignored", analyze.Warn)
}

func TestUnpinnedProvider(t *testing.T) {
	res := analyzeFixture(t, "unpinned-provider")
	r := find(res, "providers.version-constraints")
	if r.Status != analyze.Warn || r.File != "main.tf" {
		t.Fatalf("providers.version-constraints = %+v", r)
	}
	// terraform.io/builtin-style entry must not trigger a warning
	for _, x := range res {
		if x.ID == "providers.version-constraints" && x.Description != "" &&
			contains(x.Description, `"terraform"`) {
			t.Fatalf("builtin provider flagged: %+v", x)
		}
	}
	// provider without source should warn on source-explicit too
	s := find(res, "providers.source-explicit")
	if s.Status != analyze.Warn || !contains(s.Title, "kubernetes") {
		t.Fatalf("providers.source-explicit = %+v", s)
	}
}

func TestUnpinnedModule(t *testing.T) {
	res := analyzeFixture(t, "unpinned-module")
	r1 := find(res, "modules.remote-version-pinned")
	if r1.Status != analyze.Warn || !contains(r1.Title, "vpc") {
		t.Fatalf("modules.remote-version-pinned = %+v", r1)
	}
	g := find(res, "modules.git-ref-pinned")
	if g.Status != analyze.Warn {
		t.Fatalf("modules.git-ref-pinned = %+v", g)
	}
}

func TestGitHubActionsCI(t *testing.T) {
	res := analyzeFixture(t, "github-actions")
	in := find(res, "ci.input-disabled")
	if in.Status != analyze.Warn {
		t.Fatalf("ci.input-disabled = %+v", in)
	}
	env := find(res, "ci.automation-env")
	if env.Status != analyze.Warn {
		t.Fatalf("ci.automation-env = %+v", env)
	}
	aa := find(res, "ci.apply-auto-approve")
	if aa.Status != analyze.Info {
		t.Fatalf("ci.apply-auto-approve = %+v", aa)
	}
}

func TestGitLabCI(t *testing.T) {
	res := analyzeFixture(t, "gitlab-ci")
	assertStatus(t, res, "ci.input-disabled", analyze.Warn)
	assertStatus(t, res, "ci.automation-env", analyze.Warn)
	assertStatus(t, res, "ci.apply-auto-approve", analyze.Info)
}

func TestStableRuleIDsPresent(t *testing.T) {
	got := map[string]bool{}
	for _, r := range All() {
		if got[r.ID] {
			t.Fatalf("duplicate rule id %s", r.ID)
		}
		got[r.ID] = true
	}
	for _, want := range []string{
		"runtime.detect", "runtime.version-pinned", "runtime.required-version",
		"providers.lockfile-present", "providers.version-constraints", "providers.source-explicit",
		"modules.remote-version-pinned", "modules.git-ref-pinned",
		"repository.terraform-directory-ignored", "repository.state-files-ignored",
		"repository.state-file-present", "repository.tfvars-sensitive-files",
		"backend.detect", "ci.input-disabled", "ci.automation-env", "ci.apply-auto-approve",
	} {
		if !got[want] {
			t.Errorf("missing rule %s", want)
		}
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func TestGitRefExtraction(t *testing.T) {
	cases := map[string]string{
		"git::https://github.com/o/r.git?ref=v1.2.3": "v1.2.3",
		"github.com/o/r//mod?ref=abcdef1234567":      "abcdef1234567",
		"git::https://github.com/o/r.git":            "",
	}
	for src, want := range cases {
		if got := gitRef(src); got != want {
			t.Errorf("gitRef(%q) = %q, want %q", src, got, want)
		}
	}
}

func TestRefDeterminism(t *testing.T) {
	deterministic := []string{"v1.2.3", "1.0.0", "0123456789abcdef", "2.1"}
	nondeterministic := []string{"main", "master", "HEAD", "", "release"}
	for _, r := range deterministic {
		if !refIsDeterministic(r) {
			t.Errorf("ref %q should be deterministic", r)
		}
	}
	for _, r := range nondeterministic {
		if refIsDeterministic(r) {
			t.Errorf("ref %q should not be deterministic", r)
		}
	}
}
