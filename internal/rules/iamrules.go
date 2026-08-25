package rules

import (
	"regexp"
	"strings"

	"github.com/ojarosch/tfdoctor/internal/analyze"
)

const ghOIDCIssuer = "token.actions.githubusercontent.com"

var (
	// repo:owner@12345/repo — GitHub's new subject format with embedded IDs.
	idEmbeddedSubRe = regexp.MustCompile(`repo:[A-Za-z0-9_.-]+@\d+/`)
	// repo:owner/* or similar wildcard subject values.
	wildcardSubRe = regexp.MustCompile(`repo:[^"'\s]*\*`)
)

func ruleGitHubOIDC(ctx *analyze.Context) []analyze.Result {
	var out []analyze.Result
	for _, f := range ctx.Repo.TFFiles {
		if !strings.Contains(f.Raw, ghOIDCIssuer) {
			continue
		}
		if idEmbeddedSubRe.MatchString(f.Raw) || strings.Contains(f.Raw, "StringLike") ||
			wildcardSubRe.MatchString(f.Raw) {
			out = append(out, withFile(pass("GitHub OIDC trust policy handles ID-embedded subjects", ""), f.Path, 0))
			continue
		}
		res := warn("GitHub OIDC subjects now include owner/repo IDs; add ID-embedded variants or use StringLike",
			`Trust policies matching only "repo:owner/repo:..." stop authorizing once GitHub embeds IDs `+
				"(owner@<id>/repo@<id>). Debug the real subject via CloudTrail: "+
				"aws cloudtrail lookup-events --lookup-attributes AttributeKey=EventName,AttributeValue=AssumeRoleWithWebIdentity")
		out = append(out, withFile(res, f.Path, 0))
	}
	return out
}
