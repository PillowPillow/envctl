package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/PillowPillow/envctl/internal/pipeline"
	"github.com/PillowPillow/envctl/internal/reporter"
	"github.com/PillowPillow/envctl/internal/secret"
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Check that all required secrets are present for an environment",
	RunE:  runVerify,
}

func init() {
	verifyCmd.Flags().Bool("strict", false, "treat warnings as errors (auto-enabled when CI=true)")
	rootCmd.AddCommand(verifyCmd)
}

func runVerify(cmd *cobra.Command, args []string) error {
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

	_, parsed, err := pipeline.ParseTemplate(cfg.Template)
	if err != nil {
		return err
	}

	strict, _ := cmd.Flags().GetBool("strict")
	rs, err := pipeline.ResolveEnv(&secret.SOPSProvider{}, env, parsed, strict || isCI())
	if err != nil {
		return err
	}

	reporter.PrintVerifyResult(os.Stdout, parsed, rs)

	if rs.HasErrors() {
		return fmt.Errorf("%d missing required secret(s)", rs.ErrorCount())
	}

	return nil
}
