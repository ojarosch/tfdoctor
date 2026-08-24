package analyze

import (
	"regexp"
	"strings"
)

// Ignore is a minimal gitignore-style matcher covering the patterns
// tfdoctor cares about: *, **, ?, trailing-/ directory patterns and
// slash-anchored patterns. Comments and blank lines are already filtered.
type Ignore struct{ patterns []string }

func NewIgnore(patterns []string) Ignore { return Ignore{patterns: patterns} }

// Matches reports whether the slash-separated repo-relative path is ignored.
func (ig Ignore) Matches(rel string) bool {
	for _, p := range ig.patterns {
		if matchPattern(p, rel) {
			return true
		}
	}
	return false
}

var patternCache = map[string]*regexp.Regexp{}

func matchPattern(pat, rel string) bool {
	re, ok := patternCache[pat]
	if !ok {
		re = compilePattern(pat)
		patternCache[pat] = re
	}
	if re == nil {
		return false
	}
	return re.MatchString(rel)
}

func compilePattern(pat string) *regexp.Regexp {
	dirOnly := strings.HasSuffix(pat, "/")
	pat = strings.TrimSuffix(pat, "/")
	if pat == "" {
		return nil
	}
	anchored := strings.HasPrefix(pat, "/")
	pat = strings.TrimPrefix(pat, "/")
	if !anchored && strings.Contains(pat, "/") {
		anchored = true // gitignore: embedded slash anchors to root
	}

	var b strings.Builder
	if anchored {
		b.WriteString("^")
	} else {
		b.WriteString("(?:.*/)?")
	}
	for i, seg := range strings.Split(pat, "/") {
		if i > 0 {
			b.WriteString("/")
		}
		if seg == "**" {
			b.WriteString(".*")
			continue
		}
		for _, r := range seg {
			switch r {
			case '*':
				b.WriteString("[^/]*")
			case '?':
				b.WriteString("[^/]")
			default:
				b.WriteString(regexp.QuoteMeta(string(r)))
			}
		}
	}
	_ = dirOnly // a matched dir ignores everything beneath it, covered below
	b.WriteString("(?:/.*)?$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil
	}
	return re
}
