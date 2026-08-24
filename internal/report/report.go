// Package report renders analysis results as text or JSON.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/ojarosch/tfdoctor/internal/analyze"
)

var categoryOrder = []string{"Runtime", "Providers", "Modules", "Repository", "Backend", "CI"}

// JSON writes the machine-readable report.
func JSON(w io.Writer, version, path string, results []analyze.Result) error {
	out := struct {
		Version string           `json:"version"`
		Path    string           `json:"path"`
		Summary map[string]int   `json:"summary"`
		Results []analyze.Result `json:"results"`
	}{Version: version, Path: path, Results: results, Summary: map[string]int{
		"pass": 0, "warn": 0, "fail": 0, "info": 0,
	}}
	for _, r := range results {
		out.Summary[string(r.Status)]++
	}
	if out.Results == nil {
		out.Results = []analyze.Result{}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

// Text writes the human-readable report.
func Text(w io.Writer, results []analyze.Result) {
	fancy := isTerminal(w)
	sym := map[analyze.Status]string{
		analyze.Pass: "ok", analyze.Warn: "!!", analyze.Fail: "x", analyze.Info: "-",
	}
	if fancy {
		sym = map[analyze.Status]string{
			analyze.Pass: "✓", analyze.Warn: "⚠", analyze.Fail: "✗", analyze.Info: "ℹ",
		}
	}

	byCat := map[string][]analyze.Result{}
	for _, r := range results {
		byCat[r.Category] = append(byCat[r.Category], r)
	}

	for _, cat := range categoryOrder {
		rs := byCat[cat]
		if len(rs) == 0 {
			continue
		}
		fmt.Fprintf(w, "\n%s\n", cat)
		for _, r := range rs {
			line := fmt.Sprintf("%s %s", sym[r.Status], r.Title)
			if r.Description != "" {
				line += ": " + r.Description
			}
			fmt.Fprintln(w, line)
		}
		delete(byCat, cat)
	}
	for cat, rs := range byCat { // any future category not in the fixed order
		fmt.Fprintf(w, "\n%s\n", cat)
		for _, r := range rs {
			fmt.Fprintf(w, "%s %s\n", sym[r.Status], r.Title)
		}
	}

	counts := map[analyze.Status]int{}
	for _, r := range results {
		counts[r.Status]++
	}
	fmt.Fprintln(w, "\n"+strings.Repeat("─", 26))
	fmt.Fprintf(w, "\n%d passed\n%d warnings\n%d failures\n%d info\n",
		counts[analyze.Pass], counts[analyze.Warn], counts[analyze.Fail], counts[analyze.Info])
}
