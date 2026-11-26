package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/PillowPillow/envctl/v2/internal/pipeline"
	"github.com/PillowPillow/envctl/v2/internal/resolver"
	"github.com/PillowPillow/envctl/v2/internal/secret"
)

var (
	outputPath string
	dryRun     bool
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Produce an env file from template + secrets + overrides",
	RunE:  runGenerate,
}

func init() {
	generateCmd.Flags().StringVarP(&outputPath, "output", "o", "", "output file path (default: stdout)")
	generateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be generated without writing")
	generateCmd.Flags().Bool("strict", false, "fail on any missing required secret")
	rootCmd.AddCommand(generateCmd)
}

func runGenerate(cmd *cobra.Command, args []string) error {
	if envName == "" {
		return fmt.Errorf("--env is required")
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	env, err := cfg.GetEnvironment(envName)
	if err != nil {
		return err
	}

	parser, parsed, err := pipeline.ParseTemplate(cfg.Template)
	if err != nil {
		return err
	}

	strict, _ := cmd.Flags().GetBool("strict")
	strict = strict || isCI()

	rs, err := pipeline.ResolveEnv(&secret.SOPSProvider{}, env, parsed, strict)
	if err != nil {
		return err
	}

	if strict && rs.HasErrors() {
		for _, d := range rs.Diagnostics {
			if d.Severity == resolver.SeverityError {
				fmt.Fprintf(os.Stderr, "Error: %s: %s\n", d.Variable, d.Message)
			}
		}
		return errors.New(rs.FailureSummary())
	}

	output := parser.Render(parsed, rs.Values())

	dest := outputPath
	if dest == "" {
		dest = env.Output
	}

	if dryRun || dest == "" {
		fmt.Print(output)
		return nil
	}

	if err := os.WriteFile(dest, []byte(output), 0o600); err != nil {
		return fmt.Errorf("cannot write output file %s: %w", dest, err)
	}

	fmt.Fprintf(os.Stderr, "Generated %s for environment %q\n", dest, envName)
	return nil
}
