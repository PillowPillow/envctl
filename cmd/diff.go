package cmd

import (
	"fmt"
	"sort"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/PillowPillow/envctl/v2/internal/pipeline"
	"github.com/PillowPillow/envctl/v2/internal/secret"
)

var (
	fromEnv string
	toEnv   string
	reveal  bool
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Compare secrets between environments",
	RunE:  runDiff,
}

func init() {
	diffCmd.Flags().StringVar(&fromEnv, "from", "", "source environment")
	diffCmd.Flags().StringVar(&toEnv, "to", "", "target environment")
	diffCmd.Flags().BoolVar(&reveal, "reveal", false, "show actual secret values")
	rootCmd.AddCommand(diffCmd)
}

func runDiff(cmd *cobra.Command, args []string) error {
	if fromEnv == "" || toEnv == "" {
		return fmt.Errorf("both --from and --to are required")
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	fromCfg, err := cfg.GetEnvironment(fromEnv)
	if err != nil {
		return err
	}
	toCfg, err := cfg.GetEnvironment(toEnv)
	if err != nil {
		return err
	}

	_, parsed, err := pipeline.ParseTemplate(cfg.Template)
	if err != nil {
		return err
	}

	provider := &secret.SOPSProvider{}

	fromRS, err := pipeline.ResolveEnv(provider, fromCfg, parsed, false)
	if err != nil {
		return fmt.Errorf("cannot resolve %s: %w", fromEnv, err)
	}

	toRS, err := pipeline.ResolveEnv(provider, toCfg, parsed, false)
	if err != nil {
		return fmt.Errorf("cannot resolve %s: %w", toEnv, err)
	}

	fromVals := fromRS.Values()
	toVals := toRS.Values()

	// Collect all keys
	allKeys := make(map[string]bool)
	for k := range fromVals {
		allKeys[k] = true
	}
	for k := range toVals {
		allKeys[k] = true
	}

	sorted := make([]string, 0, len(allKeys))
	for k := range allKeys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	bold := color.New(color.Bold).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()

	fmt.Printf("\nComparing %s -> %s:\n\n", bold(fromEnv), bold(toEnv))

	diffCount := 0
	for _, key := range sorted {
		fromVal, fromOk := fromVals[key]
		toVal, toOk := toVals[key]

		if fromOk && toOk && fromVal == toVal {
			continue // same value, skip
		}

		diffCount++
		fmt.Printf("  %s:\n", bold(key))
		if fromOk {
			fmt.Printf("    %s: %s\n", yellow(fromEnv), formatValue(fromVal, reveal))
		} else {
			fmt.Printf("    %s: %s\n", yellow(fromEnv), red("(not set)"))
		}
		if toOk {
			fmt.Printf("    %s: %s\n", yellow(toEnv), formatValue(toVal, reveal))
		} else {
			fmt.Printf("    %s: %s\n", yellow(toEnv), red("(not set)"))
		}
		fmt.Println()
	}

	if diffCount == 0 {
		fmt.Println(green("  No differences found."))
	} else {
		fmt.Printf("  %d difference(s)\n", diffCount)
	}

	return nil
}

func formatValue(val string, showReveal bool) string {
	if showReveal {
		return fmt.Sprintf("%q", val)
	}
	if len(val) > 0 {
		return fmt.Sprintf("(set, %d chars)", len(val))
	}
	return "(empty)"
}
