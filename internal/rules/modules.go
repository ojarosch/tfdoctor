package rules

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/ojarosch/tfdoctor/internal/analyze"
)

func ruleRemotePinned(ctx *analyze.Context) []analyze.Result {
	var out []analyze.Result
	pinned := 0
	for _, f := range ctx.Repo.TFFiles {
		for _, m := range f.Modules {
			if !registrySource(m.Source) {
				continue
			}
			if m.Version == "" {
				res := warn(fmt.Sprintf("Module %q is not version pinned", m.Name),
					fmt.Sprintf("Registry module %s has no version attribute", m.Source))
				out = append(out, withFile(res, m.File, m.Line))
			} else {
				pinned++
			}
		}
	}
	if pinned > 0 && len(out) == 0 {
		out = append(out, pass(fmt.Sprintf("%d registry module(s) pinned", pinned), ""))
	}
	return out
}

func ruleGitRefPinned(ctx *analyze.Context) []analyze.Result {
	var out []analyze.Result
	ok := 0
	for _, f := range ctx.Repo.TFFiles {
		for _, m := range f.Modules {
			if !gitSource(m.Source) {
				continue
			}
			ref := gitRef(m.Source)
			switch {
			case ref == "":
				res := warn(fmt.Sprintf("Module %q uses a Git source without ref", m.Name),
					fmt.Sprintf("%s has no ?ref= pin", m.Source))
				out = append(out, withFile(res, m.File, m.Line))
			case !refIsDeterministic(ref):
				res := warn(fmt.Sprintf("Module %q uses non-deterministic git ref %q", m.Name, ref),
					fmt.Sprintf("%s should use a tag or commit SHA instead of %q", m.Source, ref))
				out = append(out, withFile(res, m.File, m.Line))
			default:
				ok++
			}
		}
	}
	if ok > 0 && len(out) == 0 {
		out = append(out, pass(fmt.Sprintf("%d git module(s) pinned to tag or SHA", ok), ""))
	}
	return out
}

// gitRef extracts the ref query parameter without full URL parsing.
func gitRef(src string) string {
	if i := strings.Index(src, "?"); i >= 0 {
		q, err := url.ParseQuery(strings.TrimPrefix(src[i+1:], "//"))
		if err == nil && q.Get("ref") != "" {
			return q.Get("ref")
		}
	}
	return ""
}
