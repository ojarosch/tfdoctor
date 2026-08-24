package rules

import (
	"strings"

	"github.com/ojarosch/tfdoctor/internal/analyze"
)

func ruleRuntimeDetect(ctx *analyze.Context) []analyze.Result {
	r := ctx.Repo
	var tf, tofu bool
	for _, p := range r.Pins {
		if p.Tool == "terraform" {
			tf = true
		}
		if p.Tool == "tofu" {
			tofu = true
		}
	}
	ci := analyze.SummarizeCI(r.CIFiles)
	if !tf {
		for _, f := range r.TFFiles {
			if f.RequiredVersion != "" || len(f.Backends) > 0 || len(f.Providers) > 0 {
				tf = true
				break
			}
		}
	}
	if ci.TFExecuted {
		// Distinguish tools by binary name in CI text.
		for _, f := range r.CIFiles {
			if strings.Contains(f.Content, "tofu ") || strings.Contains(f.Content, "tofu\n") {
				tofu = true
			} else if strings.Contains(f.Content, "terraform") {
				tf = true
			}
		}
	}

	switch {
	case tf && tofu:
		return []analyze.Result{info("Ambiguous runtime", "Both Terraform and OpenTofu signals detected")}
	case tofu:
		return []analyze.Result{info("OpenTofu detected", "")}
	case tf:
		return []analyze.Result{info("Terraform detected", "")}
	default:
		return []analyze.Result{info("No Terraform/OpenTofu usage detected", "")}
	}
}

func ruleVersionPinned(ctx *analyze.Context) []analyze.Result {
	r := ctx.Repo
	var pinned, nondeterministic []string
	for _, p := range r.Pins {
		entry := p.Tool + " " + p.Version + " (" + p.File + ")"
		if p.Deterministic {
			pinned = append(pinned, entry)
		} else {
			nondeterministic = append(nondeterministic, entry)
		}
	}
	switch {
	case len(pinned) > 0:
		return []analyze.Result{pass("Runtime version pinned", strings.Join(pinned, ", "))}
	case len(nondeterministic) > 0:
		res := warn("Runtime version pin is not deterministic",
			strings.Join(nondeterministic, ", ") + " does not pin an exact version")
		return []analyze.Result{res}
	default:
		return []analyze.Result{warn("No runtime version pin",
			"No .terraform-version, .opentofu-version, .tool-versions or mise.toml pin found")}
	}
}

func ruleRequiredVersion(ctx *analyze.Context) []analyze.Result {
	for _, f := range ctx.Repo.TFFiles {
		if f.RequiredVersion != "" {
			res := pass("required_version defined", f.RequiredVersion)
			return []analyze.Result{withFile(res, f.Path, f.RequiredVersionLine)}
		}
	}
	return []analyze.Result{warn("No required_version defined",
		"No terraform { required_version = ... } constraint found in the root module")}
}
