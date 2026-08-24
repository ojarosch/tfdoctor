package analyze

import (
	"os"
	"path/filepath"
	"regexp"
)

var (
	tfExecRe     = regexp.MustCompile(`\b(?:terraform|tofu)\b`)
	planApplyRe  = regexp.MustCompile(`\b(?:terraform|tofu)\s+(?:plan|apply)\b`)
	autoApplyRe  = regexp.MustCompile(`\b(?:terraform|tofu)\s+apply\b[^#\n]*-auto-approve`)
	inputOffRe   = regexp.MustCompile(`-input[= ]false\b|TF_INPUT\s*[:=]+\s*"?false`)
	automationRe = regexp.MustCompile(`TF_IN_AUTOMATION\s*[:=]+\s*"?true`)
)

// CIData summarizes CI configuration relevant to Terraform/OpenTofu.
type CIData struct {
	Files         []TextFile
	TFExecuted    bool
	PlanOrApply   bool
	InputDisabled bool
	AutoApprove   bool
	AutomationEnv bool
}

// discoverCI collects GitHub Actions and GitLab CI files and scans them for
// simple terraform/tofu command signals. No YAML parsing: text matching is
// sufficient and dependency-free.
func discoverCI(root string) []TextFile {
	var files []TextFile
	workflows := filepath.Join(root, ".github", "workflows")
	if entries, err := os.ReadDir(workflows); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if ext := filepath.Ext(e.Name()); ext == ".yml" || ext == ".yaml" {
				addTextFile(root, filepath.Join(".github", "workflows", e.Name()), &files)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".gitlab-ci.yml")); err == nil {
		addTextFile(root, ".gitlab-ci.yml", &files)
	}
	return files
}

func addTextFile(root, rel string, files *[]TextFile) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return
	}
	*files = append(*files, TextFile{Path: rel, Content: string(data)})
}

// SummarizeCI derives the CI signal flags from the discovered files.
// Each flag is evaluated per file; a single workflow disabling input
// satisfies input-disabled only if every plan/apply invocation in that
// file is covered (approximated at file granularity).
func SummarizeCI(files []TextFile) CIData {
	ci := CIData{Files: files}
	for _, f := range files {
		hasExec := tfExecRe.MatchString(f.Content)
		hasPlan := planApplyRe.MatchString(f.Content)
		ci.TFExecuted = ci.TFExecuted || hasExec
		ci.PlanOrApply = ci.PlanOrApply || hasPlan
		ci.AutoApprove = ci.AutoApprove || autoApplyRe.MatchString(f.Content)
		if hasExec && automationRe.MatchString(f.Content) {
			ci.AutomationEnv = true
		}
		// ponytail: file-level approximation — a -input=false anywhere in the
		// same workflow file counts as disabled; per-job tracking if needed later.
		if hasPlan && inputOffRe.MatchString(f.Content) {
			ci.InputDisabled = true
		}
	}
	return ci
}
