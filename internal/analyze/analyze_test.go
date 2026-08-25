package analyze

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIgnoreMatching(t *testing.T) {
	ig := NewIgnore([]string{
		".terraform/",
		"*.tfstate",
		"*.tfstate.*",
		"*.tfvars",
		"/build-output",
		"secrets/prod.env",
	})
	cases := []struct {
		path string
		want bool
	}{
		{".terraform", true},
		{".terraform/modules/x", true},
		{"terraform.tfstate", true},
		{"envs/prod/app.tfstate", true},
		{"terraform.tfstate.backup", true},
		{"a/b/c.tfstate.1", true},
		{"terraform.tfvars", true},
		{"main.tf", false},
		{"terraform.tfstate.json", true},
		{"notatfstatebackup", false},
		{"build-output", true},
		{"sub/build-output", false}, // leading slash anchors to root
		{"secrets/prod.env", true},
		{"x/secrets/prod.env", false}, // embedded slash anchors to root
	}
	for _, c := range cases {
		if got := ig.Matches(c.path); got != c.want {
			t.Errorf("Matches(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestDiscoverReadsRootGitignoreFromSubdir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"),
		[]byte(".terraform/\n*.tfstate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "terraform")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "main.tf"),
		[]byte("terraform {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo, err := Discover(sub)
	if err != nil {
		t.Fatal(err)
	}
	ig := NewIgnore(repo.Gitignore)
	if !ig.Matches(".terraform") {
		t.Errorf(".terraform not matched; gitignore = %v", repo.Gitignore)
	}
	if !ig.Matches("x.tfstate") {
		t.Errorf("x.tfstate not matched; gitignore = %v", repo.Gitignore)
	}
}

func TestSummarizeCI(t *testing.T) {
	ci := SummarizeCI([]TextFile{{Path: ".github/workflows/ci.yml", Content: `
jobs:
  - run: terraform plan -input=false
`}})
	if !ci.PlanOrApply || !ci.InputDisabled || ci.AutoApprove || ci.AutomationEnv {
		t.Fatalf("unexpected summary: %+v", ci)
	}
	ci = SummarizeCI([]TextFile{{Path: "x.yml", Content: `
env:
  TF_IN_AUTOMATION: true
script: tofu apply -auto-approve
`}})
	if !ci.AutoApprove || !ci.AutomationEnv {
		t.Fatalf("unexpected summary: %+v", ci)
	}
}
