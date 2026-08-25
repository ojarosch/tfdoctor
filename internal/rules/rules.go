// Package rules contains the tfdoctor diagnostic rules. Each rule operates
// on the pre-discovered repository model; rules never touch the filesystem
// themselves.
package rules

import (
	"regexp"
	"strings"

	"github.com/ojarosch/tfdoctor/internal/analyze"
)

// Rule is a single named diagnostic.
type Rule struct {
	ID       string
	Category string
	Fn       func(ctx *analyze.Context) []analyze.Result
}

// All returns the full rule set in stable reporting order.
func All() []Rule {
	return []Rule{
		{ID: "runtime.detect", Category: "Runtime", Fn: ruleRuntimeDetect},
		{ID: "runtime.version-pinned", Category: "Runtime", Fn: ruleVersionPinned},
		{ID: "runtime.required-version", Category: "Runtime", Fn: ruleRequiredVersion},
		{ID: "providers.lockfile-present", Category: "Providers", Fn: ruleLockfilePresent},
		{ID: "providers.version-constraints", Category: "Providers", Fn: ruleVersionConstraints},
		{ID: "providers.source-explicit", Category: "Providers", Fn: ruleSourceExplicit},
		{ID: "modules.remote-version-pinned", Category: "Modules", Fn: ruleRemotePinned},
		{ID: "modules.git-ref-pinned", Category: "Modules", Fn: ruleGitRefPinned},
		{ID: "repository.terraform-directory-ignored", Category: "Repository", Fn: ruleTerraformDirIgnored},
		{ID: "repository.state-files-ignored", Category: "Repository", Fn: ruleStateIgnored},
		{ID: "repository.state-file-present", Category: "Repository", Fn: ruleStatePresent},
		{ID: "repository.tfvars-sensitive-files", Category: "Repository", Fn: ruleTfvars},
		{ID: "backend.detect", Category: "Backend", Fn: ruleBackendDetect},
		{ID: "iam.github-oidc-legacy-subject", Category: "IAM", Fn: ruleGitHubOIDC},
		{ID: "ci.input-disabled", Category: "CI", Fn: ruleInputDisabled},
		{ID: "ci.automation-env", Category: "CI", Fn: ruleAutomationEnv},
		{ID: "ci.apply-auto-approve", Category: "CI", Fn: ruleAutoApprove},
	}
}

// RunAll executes every rule against the context, preserving rule order.
func RunAll(ctx *analyze.Context) []analyze.Result {
	var out []analyze.Result
	for _, r := range All() {
		for _, res := range r.Fn(ctx) {
			res.ID = r.ID
			res.Category = r.Category
			out = append(out, res)
		}
	}
	return out
}

// --- helpers shared by rules ---

func pass(title, desc string) analyze.Result {
	return analyze.Result{Status: analyze.Pass, Title: title, Description: desc}
}
func warn(title, desc string) analyze.Result {
	return analyze.Result{Status: analyze.Warn, Title: title, Description: desc}
}
func fail(title, desc string) analyze.Result {
	return analyze.Result{Status: analyze.Fail, Title: title, Description: desc}
}
func info(title, desc string) analyze.Result {
	return analyze.Result{Status: analyze.Info, Title: title, Description: desc}
}

func withFile(res analyze.Result, file string, line int) analyze.Result {
	res.File, res.Line = file, line
	return res
}

// allProviders dedupes provider entries across files by local name,
// merging source/version info.
func allProviders(repo *analyze.Repo) map[string]*analyze.ProviderRef {
	byName := map[string]*analyze.ProviderRef{}
	for _, f := range repo.TFFiles {
		for i := range f.Providers {
			p := &f.Providers[i]
			existing, ok := byName[p.Name]
			if !ok {
				cp := *p
				byName[p.Name] = &cp
				continue
			}
			if existing.Source == "" {
				existing.Source = p.Source
			}
			if existing.Version == "" {
				existing.Version = p.Version
			}
		}
	}
	return byName
}

var builtinRe = regexp.MustCompile(`^terraform\.io/builtin/`)

// registrySource reports whether a module source looks like a public/private
// registry module (no scheme, at least one slash).
func registrySource(src string) bool {
	if src == "" || strings.Contains(src, "::") || strings.Contains(src, "://") {
		return false
	}
	return strings.Count(src, "/") >= 1 && !strings.HasPrefix(src, "./") && !strings.HasPrefix(src, "../")
}

func gitSource(src string) bool {
	if strings.HasPrefix(src, "git::") || strings.HasPrefix(src, "github.com/") ||
		strings.HasPrefix(src, "git@") {
		return true
	}
	return strings.Contains(src, "//") && strings.Contains(src, ".git")
}

var shaRefRe = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// refIsDeterministic accepts semver-ish tags and commit SHAs; branches like
// main/master/develop or anything else are treated as non-deterministic.
func refIsDeterministic(ref string) bool {
	if ref == "" {
		return false
	}
	lower := strings.ToLower(ref)
	switch lower {
	case "main", "master", "develop", "head", "latest":
		return false
	}
	return shaRefRe.MatchString(ref) || regexp.MustCompile(`^v?\d+(\.\d+){0,2}$`).MatchString(ref)
}
