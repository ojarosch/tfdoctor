package rules

import (
	"fmt"
	"strings"

	"github.com/ojarosch/tfdoctor/internal/analyze"
)

func ignoreOf(repo *analyze.Repo) analyze.Ignore {
	return analyze.NewIgnore(repo.Gitignore)
}

func ruleTerraformDirIgnored(ctx *analyze.Context) []analyze.Result {
	if ignoreOf(ctx.Repo).Matches(".terraform") {
		return []analyze.Result{pass(".terraform/ ignored", "")}
	}
	return []analyze.Result{warn(".terraform/ is not ignored",
		"Add .terraform/ to .gitignore")}
}

func ruleStateIgnored(ctx *analyze.Context) []analyze.Result {
	ig := ignoreOf(ctx.Repo)
	candidates := []string{
		"terraform.tfstate",
		"terraform.tfstate.backup",
		"env/prod.tfstate",
	}
	for _, c := range candidates {
		if ig.Matches(c) {
			return []analyze.Result{pass("State files ignored", "")}
		}
	}
	return []analyze.Result{fail("State files are not ignored",
		"Add *.tfstate and *.tfstate.* to .gitignore")}
}

func ruleStatePresent(ctx *analyze.Context) []analyze.Result {
	found := ctx.Repo.StateFiles
	if len(found) == 0 {
		return []analyze.Result{pass("No state files found", "")}
	}
	var names []string
	for _, f := range found {
		label := f.Path
		if ctx.Repo.GitAvailable {
			if f.Tracked {
				label += " (tracked)"
			} else {
				label += " (untracked)"
			}
		}
		names = append(names, label)
	}
	res := fail(fmt.Sprintf("State file found: %s", strings.Join(names, ", ")),
		"Terraform state can contain secrets; it must not live in the repository")
	return []analyze.Result{withFile(res, found[0].Path, 0)}
}

func ruleTfvars(ctx *analyze.Context) []analyze.Result {
	r := ctx.Repo
	if len(r.TfvarsFiles) == 0 {
		return nil
	}
	ig := ignoreOf(r)
	var unignored []string
	for _, f := range r.TfvarsFiles {
		if !ig.Matches(f.Path) {
			unignored = append(unignored, f.Path)
		}
	}
	if len(unignored) > 0 {
		res := warn(fmt.Sprintf("tfvars file(s) not ignored: %s", strings.Join(unignored, ", ")),
			"If these hold environment-specific values, add them to .gitignore")
		return []analyze.Result{withFile(res, unignored[0], 0)}
	}
	return []analyze.Result{pass("All tfvars files ignored", strings.Join(tfvarPaths(r), ", "))}
}

func tfvarPaths(r *analyze.Repo) []string {
	var out []string
	for _, f := range r.TfvarsFiles {
		out = append(out, f.Path)
	}
	return out
}

func ruleBackendDetect(ctx *analyze.Context) []analyze.Result {
	var types []string
	var firstFile string
	firstLine := 0
	for _, f := range ctx.Repo.TFFiles {
		for _, b := range f.Backends {
			types = append(types, b.Type)
			if firstFile == "" {
				firstFile, firstLine = b.File, b.Line
			}
		}
	}
	switch len(types) {
	case 0:
		return []analyze.Result{info("Backend: local/default", "No backend block configured")}
	case 1:
		res := info("Backend: "+types[0], "")
		return []analyze.Result{withFile(res, firstFile, firstLine)}
	default:
		return []analyze.Result{info(fmt.Sprintf("Backend: %s", strings.Join(types, ", ")),
			"Multiple backends configured")}
	}
}
