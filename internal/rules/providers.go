package rules

import (
	"fmt"

	"github.com/ojarosch/tfdoctor/internal/analyze"
)

func ruleLockfilePresent(ctx *analyze.Context) []analyze.Result {
	r := ctx.Repo
	if r.Lockfile != "" {
		return []analyze.Result{pass(".terraform.lock.hcl present", r.Lockfile)}
	}
	if len(allProviders(r)) > 0 {
		return []analyze.Result{warn("No provider lockfile",
			"Providers are referenced but .terraform.lock.hcl is missing")}
	}
	return nil
}

func ruleVersionConstraints(ctx *analyze.Context) []analyze.Result {
	var out []analyze.Result
	total := 0
	for _, p := range allProviders(ctx.Repo) {
		if builtinRe.MatchString(p.Source) || p.Name == "terraform" {
			continue
		}
		total++
		if p.Version == "" {
			res := warn(fmt.Sprintf("Provider %q has no version constraint", p.Name),
				fmt.Sprintf("Add version = \"...\" for %s", displayName(p)))
			out = append(out, withFile(res, p.File, p.Line))
		}
	}
	if len(out) == 0 && total > 0 {
		out = append(out, pass(fmt.Sprintf("All %d provider(s) have version constraints", total), ""))
	}
	return out
}

func ruleSourceExplicit(ctx *analyze.Context) []analyze.Result {
	var out []analyze.Result
	total := 0
	for _, p := range allProviders(ctx.Repo) {
		if builtinRe.MatchString(p.Source) || p.Name == "terraform" {
			continue
		}
		total++
		if p.Source == "" {
			res := warn(fmt.Sprintf("Provider %q has no explicit source", p.Name),
				fmt.Sprintf("Add source = \"registry.../%s\" to required_providers", p.Name))
			out = append(out, withFile(res, p.File, p.Line))
		}
	}
	if len(out) == 0 && total > 0 {
		out = append(out, pass(fmt.Sprintf("All %d provider(s) have explicit sources", total), ""))
	}
	return out
}

func displayName(p *analyze.ProviderRef) string {
	if p.Source != "" {
		return p.Source
	}
	return p.Name
}
