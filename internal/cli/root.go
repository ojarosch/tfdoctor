package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/ojarosch/tfdoctor/internal/analyze"
	"github.com/ojarosch/tfdoctor/internal/report"
	"github.com/ojarosch/tfdoctor/internal/rules"
	"github.com/ojarosch/tfdoctor/internal/s3check"
	"github.com/spf13/cobra"
)

// Version is the tfdoctor version reported by --version and embedded in
// JSON output.
var Version = "0.1.3"

type exitError struct{ code int }

func (e *exitError) Error() string { return fmt.Sprintf("exit %d", e.code) }

func newRootCmd() *cobra.Command {
	var format string
	var checkS3 bool
	var agentMode bool
	cmd := &cobra.Command{
		Use:           "tfdoctor [path]",
		Short:         "Check the engineering hygiene of a Terraform/OpenTofu repository",
		Version:       Version,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if agentMode && !cmd.Flags().Changed("format") {
				format = "agent"
			}
			if format != "text" && format != "json" && format != "agent" {
				return fmt.Errorf("invalid --format %q: must be \"text\", \"json\", or \"agent\"", format)
			}
			path := "."
			if len(args) == 1 {
				path = args[0]
			}
			repo, err := analyze.Discover(path)
			if err != nil {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "tfdoctor:", err)
				return &exitError{code: 2}
			}
			results := rules.RunAll(&analyze.Context{Repo: repo})
			if checkS3 {
				results = append(results, s3check.Check(cmd.Context(), repo)...)
			}
			ignored, err := applyIgnores(repo, results, cmd.ErrOrStderr())
			if err != nil {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "tfdoctor:", err)
				return &exitError{code: 2}
			}
			results = ignored.kept
			switch format {
			case "json":
				if err := report.JSON(cmd.OutOrStdout(), Version, repo.Path, results, ignored.count); err != nil {
					return &exitError{code: 2}
				}
			case "agent":
				report.Agent(cmd.OutOrStdout(), repo.Path, results, ignored.count)
			default:
				report.Text(cmd.OutOrStdout(), results, ignored.count)
			}
			for _, r := range results {
				if r.Status == analyze.Fail {
					return &exitError{code: 1}
				}
			}
			return nil
		},
	}
	cmd.SetVersionTemplate("tfdoctor {{.Version}}\n")
	cmd.Flags().StringVar(&format, "format", "text", "output format (text, json, agent)")
	cmd.Flags().BoolVar(&agentMode, "agent", false,
		"shorthand for --format agent: emit a remediation prompt for AI coding agents")
	cmd.Flags().BoolVar(&checkS3, "check-s3-backend", false,
		"inspect the S3 state bucket (versioning, encryption, public access, TLS policy)")
	return cmd
}

type ignoreOutcome struct {
	kept  []analyze.Result
	count int
}

// applyIgnores filters results whose ID is listed in .tfdoctor.yaml and warns
// about ignore entries that match no known rule ID.
func applyIgnores(repo *analyze.Repo, results []analyze.Result, stderr io.Writer) (ignoreOutcome, error) {
	ids, err := analyze.LoadIgnores(repo.Path)
	if err != nil || len(ids) == 0 {
		return ignoreOutcome{kept: results}, err
	}
	known := map[string]bool{}
	for _, r := range rules.All() {
		known[r.ID] = true
	}
	for _, id := range s3check.IDs {
		known[id] = true
	}
	var out ignoreOutcome
	out.kept = results[:0] // reuse backing array; callers own the slice
	for _, id := range ids {
		if !known[id] {
			fmt.Fprintf(stderr, "tfdoctor: warning: %s lists unknown rule ID %q\n", analyze.ConfigFile, id)
		}
	}
	for _, r := range results {
		if ignored := contains(ids, r.ID); ignored {
			out.count++
			continue
		}
		out.kept = append(out.kept, r)
	}
	return out, nil
}

func contains(ids []string, id string) bool {
	for _, i := range ids {
		if i == id {
			return true
		}
	}
	return false
}

// Run executes the CLI and returns the process exit code:
// 0 = clean, 1 = failures found, 2 = tfdoctor could not run.
func Run() int {
	err := newRootCmd().Execute()
	if err == nil {
		return 0
	}
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	fmt.Println("tfdoctor:", err)
	return 2
}
