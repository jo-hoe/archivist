package cli

import (
	"bufio"
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jo-hoe/archivist/internal/config"
	"github.com/jo-hoe/archivist/internal/pipeline"
)

func newRunCmd(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Sort and tag files from the command line (used in Docker)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c := configFromViper(v)
			return runPipeline(cmd.Context(), cmd, c)
		},
	}
}

// runPipeline builds and executes the pipeline, showing a preview first unless
// disabled, and printing a per-file report.
func runPipeline(ctx context.Context, cmd *cobra.Command, c config.Config) error {
	if err := validateOrUsage(c); err != nil {
		return err
	}

	built, err := config.Build(ctx, c)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()

	if len(built.Entries) == 0 {
		fmt.Fprintln(out, "No markdown files found to process.")
		return nil
	}

	if !c.NoPreview {
		printPreviewList(out, built.Entries, c)
		if !confirm(cmd) {
			fmt.Fprintln(out, "Aborted.")
			return nil
		}
	}

	results, err := built.Pipeline.Run(ctx, built.Entries, pipeline.ProgressHook{
		OnFile: func(_ int, r pipeline.Result) { printResult(out, r) },
	})
	if err != nil {
		return err
	}
	printSummary(out, results, c.DryRun)
	return nil
}

// confirm prompts the user to proceed unless input is non-interactive.
func confirm(cmd *cobra.Command) bool {
	fmt.Fprint(cmd.OutOrStdout(), "\nProceed? [y/N] ")
	reader := bufio.NewReader(cmd.InOrStdin())
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

// relOrBase returns a short display path for a source file.
func relOrBase(path string) string {
	return filepath.Base(path)
}
