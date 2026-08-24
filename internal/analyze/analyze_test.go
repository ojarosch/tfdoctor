package analyze

import "testing"

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
