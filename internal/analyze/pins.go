package analyze

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	miseToolRe   = regexp.MustCompile(`(?m)^\s*(terraform|tofu|opentofu)\s*=\s*"([^"]+)"`)
	toolVersRe   = regexp.MustCompile(`(?m)^\s*(terraform|tofu|opentofu)\s+(\S+)`)
	pinFileTools = map[string]string{
		".terraform-version": "terraform",
		".opentofu-version":  "tofu",
	}
)

// collectPin records a runtime version pin from the given file.
func collectPin(rel, abs string, pins *[]PinInfo) {
	data, err := os.ReadFile(abs)
	if err != nil {
		return
	}
	content := strings.TrimSpace(string(data))
	base := filepath.Base(rel)

	if tool, ok := pinFileTools[base]; ok && content != "" {
		add := PinInfo{Tool: tool, Version: firstLine(content), File: rel}
		add.Deterministic = deterministic(add.Version)
		*pins = append(*pins, add)
		return
	}
	if base == ".tool-versions" {
		for _, m := range toolVersRe.FindAllStringSubmatch(content, -1) {
			add := PinInfo{Tool: normalizeTool(m[1]), Version: m[2], File: rel}
			add.Deterministic = deterministic(add.Version)
			*pins = append(*pins, add)
		}
		return
	}
	for _, m := range miseToolRe.FindAllStringSubmatch(content, -1) {
		add := PinInfo{Tool: normalizeTool(m[1]), Version: m[2], File: rel}
		add.Deterministic = deterministic(add.Version)
		*pins = append(*pins, add)
	}
}

func normalizeTool(t string) string {
	if t == "opentofu" {
		return "tofu"
	}
	return t
}

func firstLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func deterministic(v string) bool {
	return v != "" && !strings.HasPrefix(strings.ToLower(v), "latest")
}

// parseTF parses one .tf file into a TFFile using hclsyntax.
func parseTF(rootDir, rel string) (*TFFile, error) {
	f := &TFFile{Path: rel}
	src, err := os.ReadFile(filepath.Join(rootDir, rel))
	if err != nil {
		return nil, err
	}
	f.Raw = string(src)
	body, diags := parseHCL(src, rel)
	_ = diags // tolerate partial parses; extract whatever is valid
	if body == nil {
		return nil, fmt.Errorf("unparsable %s", rel)
	}

	for _, block := range body.Blocks {
		switch block.Type {
		case "terraform":
			parseTerraformBlock(block.Body, f)
		case "module":
			if len(block.Labels) < 1 {
				continue
			}
			m := ModuleRef{Name: block.Labels[0], File: rel, Line: block.DefRange().Start.Line}
			if v, line, ok := attrString(block.Body, "source"); ok {
				m.Source, m.Line = v, line
			}
			if v, _, ok := attrString(block.Body, "version"); ok {
				m.Version = v
			}
			f.Modules = append(f.Modules, m)
		}
	}
	return f, nil
}
