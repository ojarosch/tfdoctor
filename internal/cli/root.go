package cli

import (
	"errors"
	"fmt"

	"github.com/ojarosch/tfdoctor/internal/analyze"
	"github.com/ojarosch/tfdoctor/internal/report"
	"github.com/ojarosch/tfdoctor/internal/rules"
	"github.com/spf13/cobra"
)

// Version is the tfdoctor version reported by --version and embedded in
// JSON output.
var Version = "0.1.0"

type exitError struct{ code int }

func (e *exitError) Error() string { return fmt.Sprintf("exit %d", e.code) }

func newRootCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:           "tfdoctor [path]",
		Short:         "Check the engineering hygiene of a Terraform/OpenTofu repository",
		Version:       Version,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "text" && format != "json" {
				return fmt.Errorf("invalid --format %q: must be \"text\" or \"json\"", format)
			}
			path := "."
			if len(args) == 1 {
				path = args[0]
			}
			repo, err := analyze.Discover(path)
			if err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "tfdoctor:", err)
				return &exitError{code: 2}
			}
			results := rules.RunAll(&analyze.Context{Repo: repo})
			if format == "json" {
				if err := report.JSON(cmd.OutOrStdout(), Version, repo.Path, results); err != nil {
					return &exitError{code: 2}
				}
			} else {
				report.Text(cmd.OutOrStdout(), results)
			}
			for _, r := range results {
				if r.Status == analyze.Fail {
					return &exitError{code: 1}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "text", "output format (text, json)")
	return cmd
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
