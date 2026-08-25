package rules

import (
	"testing"

	"github.com/ojarosch/tfdoctor/internal/analyze"
)

func runOIDC(t *testing.T, files ...string) []analyze.Result {
	t.Helper()
	repo := &analyze.Repo{}
	for _, raw := range files {
		repo.TFFiles = append(repo.TFFiles, &analyze.TFFile{Path: "main.tf", Raw: raw})
	}
	return ruleGitHubOIDC(&analyze.Context{Repo: repo})
}

func status(t *testing.T, res []analyze.Result) analyze.Status {
	t.Helper()
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d: %+v", len(res), res)
	}
	return res[0].Status
}

func TestGitHubOIDCNoPrincipal(t *testing.T) {
	res := runOIDC(t, `resource "aws_s3_bucket" "b" {}`)
	if len(res) != 0 {
		t.Fatalf("expected no results without OIDC principal, got %+v", res)
	}
}

func TestGitHubOIDCLegacySubjectWarns(t *testing.T) {
	res := runOIDC(t, `"token.actions.githubusercontent.com:sub": "repo:ojarosch/portfolio:environment:production"`)
	if got := status(t, res); got != analyze.Warn {
		t.Errorf("legacy subject should warn, got %s", got)
	}
}

func TestGitHubOIDCIDEmbeddedPasses(t *testing.T) {
	res := runOIDC(t, `"token.actions.githubusercontent.com:sub": "repo:ojarosch@9081883/portfolio@1346233646:environment:production"`)
	if got := status(t, res); got != analyze.Pass {
		t.Errorf("ID-embedded subject should pass, got %s", got)
	}
}

func TestGitHubOIDCStringLikePasses(t *testing.T) {
	res := runOIDC(t, `"StringLike": {"token.actions.githubusercontent.com:sub": "repo:ojarosch/portfolio:*"}`)
	if got := status(t, res); got != analyze.Pass {
		t.Errorf("StringLike should pass, got %s", got)
	}
}

func TestGitHubOIDCWildcardValuePasses(t *testing.T) {
	res := runOIDC(t, `"token.actions.githubusercontent.com:sub": "repo:ojarosch/*:environment:production"`)
	if got := status(t, res); got != analyze.Pass {
		t.Errorf("wildcard subject should pass, got %s", got)
	}
}
