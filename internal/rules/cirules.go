package rules

import (
	"strings"

	"github.com/ojarosch/tfdoctor/internal/analyze"
)

func ruleInputDisabled(ctx *analyze.Context) []analyze.Result {
	ci := analyze.SummarizeCI(ctx.Repo.CIFiles)
	if !ci.PlanOrApply {
		return nil
	}
	if ci.InputDisabled {
		return []analyze.Result{pass("Interactive input disabled for plan/apply", "-input=false or TF_INPUT=false found")}
	}
	return []analyze.Result{warn("plan/apply may prompt for interactive input",
		"Use -input=false or set TF_INPUT=false in CI")}
}

func ruleAutomationEnv(ctx *analyze.Context) []analyze.Result {
	ci := analyze.SummarizeCI(ctx.Repo.CIFiles)
	if !ci.TFExecuted {
		return nil
	}
	if ci.AutomationEnv {
		return []analyze.Result{pass("TF_IN_AUTOMATION=true", "")}
	}
	return []analyze.Result{warn("TF_IN_AUTOMATION is not set",
		"Set TF_IN_AUTOMATION=true when running Terraform/OpenTofu in CI")}
}

func ruleAutoApprove(ctx *analyze.Context) []analyze.Result {
	ci := analyze.SummarizeCI(ctx.Repo.CIFiles)
	if !ci.AutoApprove {
		return nil
	}
	var files []string
	for _, f := range ci.Files {
		if analyze.SummarizeCI([]analyze.TextFile{f}).AutoApprove {
			files = append(files, f.Path)
		}
	}
	res := info("apply uses -auto-approve",
		"Legitimate in controlled CI environments, but deserves explicit awareness")
	if len(files) > 0 {
		res.File = strings.Join(files, ", ")
	}
	return []analyze.Result{res}
}
