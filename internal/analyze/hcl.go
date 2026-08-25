package analyze

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

var stringType = cty.String

func parseHCL(src []byte, filename string) (*hclsyntax.Body, hcl.Diagnostics) {
	f, diags := hclsyntax.ParseConfig(src, filename, hcl.Pos{Line: 1, Column: 1})
	if f == nil {
		return nil, diags
	}
	return f.Body.(*hclsyntax.Body), diags
}

// attrString evaluates a literal string attribute without an eval context.
// Returns the value and its source line.
func attrString(body *hclsyntax.Body, name string) (string, int, bool) {
	attr, ok := body.Attributes[name]
	if !ok {
		return "", 0, false
	}
	v, diags := attr.Expr.Value(nil)
	if diags.HasErrors() || !v.IsKnown() || v.Type() != stringType {
		return "", 0, false
	}
	return v.AsString(), attr.SrcRange.Start.Line, true
}

// literalAttrMap evaluates an object-cons attribute like
// aws = { source = "...", version = "..." } into plain key/value strings.
func literalAttrMap(expr hcl.Expression) map[string]string {
	v, diags := expr.Value(nil)
	if diags.HasErrors() || !v.IsKnown() || !v.Type().IsObjectType() {
		return nil
	}
	out := map[string]string{}
	for k, val := range v.AsValueMap() {
		if val.Type() == stringType && val.IsKnown() {
			out[k] = val.AsString()
		}
	}
	return out
}

func parseTerraformBlock(body *hclsyntax.Body, f *TFFile) {
	if rv, line, ok := attrString(body, "required_version"); ok && f.RequiredVersion == "" {
		f.RequiredVersion = rv
		if f.RequiredVersionLine == 0 {
			f.RequiredVersionLine = line
		}
	}
	for _, block := range body.Blocks {
		switch block.Type {
		case "backend":
			if len(block.Labels) > 0 {
				b := Backend{
					Type: block.Labels[0],
					File: f.Path,
					Line: block.DefRange().Start.Line,
				}
				b.Bucket, _, _ = attrString(block.Body, "bucket")
				b.Region, _, _ = attrString(block.Body, "region")
				f.Backends = append(f.Backends, b)
			}
		case "required_providers":
			for name, attr := range block.Body.Attributes {
				p := ProviderRef{Name: name, File: f.Path, Line: attr.SrcRange.Start.Line}
				for k, v := range literalAttrMap(attr.Expr) {
					switch k {
					case "source":
						p.Source = v
					case "version":
						p.Version = v
					}
				}
				f.Providers = append(f.Providers, p)
			}
		}
	}
}
