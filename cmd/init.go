package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/PillowPillow/envctl/internal/config"
	"github.com/PillowPillow/envctl/internal/template"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Bootstrap envctl in an existing project",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check prerequisites
	if err := checkPrerequisites(); err != nil {
		return err
	}

	// Check if already initialized
	if _, err := os.Stat(config.DefaultConfigFile); err == nil {
		return fmt.Errorf("%s already exists, remove it to re-initialize", config.DefaultConfigFile)
	}

	// Detect template file
	templateFile := detectTemplateFile()
	if templateFile == "" {
		return fmt.Errorf("no .env.example or similar template file found\n\n  Fix: Create a .env.example file first")
	}

	// Parse template to show stats
	parser := template.DetectParser(templateFile)
	f, err := os.Open(templateFile) //nolint:gosec // path from detectTemplateFile
	if err != nil {
		return fmt.Errorf("cannot open %s: %w", templateFile, err)
	}
	defer f.Close() //nolint:errcheck // read-only or best-effort close

	parsed, err := parser.Parse(f)
	if err != nil {
		return fmt.Errorf("cannot parse %s: %w", templateFile, err)
	}

	required, optional, static := parsed.Stats()
	fmt.Printf("Found %s with %d variables (%d required, %d optional, %d static)\n\n",
		templateFile, len(parsed.Variables), required, optional, static)

	// Create .envctl directory
	if err := os.MkdirAll(".envctl", 0o755); err != nil {
		return fmt.Errorf("cannot create .envctl directory: %w", err)
	}

	// Generate config
	cfg := config.Config{
		Version:  "1",
		Template: templateFile,
		Environments: map[string]config.Environment{
			"local": {
				Secrets: ".envctl/secrets.local.enc.yaml",
			},
			"production": {
				Secrets: ".envctl/secrets.production.enc.yaml",
			},
		},
		SOPS: config.SOPSConfig{
			AgeRecipients: []string{"# Add your age public key here: age1..."},
		},
	}

	cfgData, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("cannot marshal config: %w", err)
	}

	if err := os.WriteFile(config.DefaultConfigFile, cfgData, 0o644); err != nil {
		return fmt.Errorf("cannot write %s: %w", config.DefaultConfigFile, err)
	}

	fmt.Printf("Created %s\n", config.DefaultConfigFile)

	// Update .gitignore
	if err := updateGitignore(); err != nil {
		fmt.Printf("Warning: could not update .gitignore: %v\n", err)
	}

	// Print next steps
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Generate an age key pair:")
	fmt.Println("       age-keygen -o ~/.config/sops/age/keys.txt")
	fmt.Println("  2. Add your public key to .envctl.yaml under sops.age_recipients")
	fmt.Println("  3. Create your first secrets file:")
	fmt.Println("       envctl edit --env local")
	fmt.Println("  4. Verify your setup:")
	fmt.Println("       envctl verify --env local")

	return nil
}

func checkPrerequisites() error {
	missing := []string{}
	if _, err := exec.LookPath("sops"); err != nil {
		missing = append(missing, "sops")
	}
	if _, err := exec.LookPath("age"); err != nil {
		missing = append(missing, "age")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required tools: %v\n\n  Fix: brew install %s", missing, strings.Join(missing, " "))
	}
	return nil
}

func detectTemplateFile() string {
	candidates := []string{
		".env.example",
		".env.sample",
		".env.template",
		".env.dist",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	// Check for any file matching *.env* pattern
	matches, _ := filepath.Glob("*.env*")
	for _, m := range matches {
		if m != ".env" { // skip actual .env file
			return m
		}
	}
	return ""
}

func updateGitignore() error {
	const marker = "# envctl"
	entries := []string{
		marker,
		".env",
		".env.local",
		"secrets.*.yaml",
		"!*.enc.yaml",
	}

	// Read existing .gitignore if present
	existing := ""
	if data, err := os.ReadFile(".gitignore"); err == nil {
		existing = string(data)
		// Already has envctl entries
		if strings.Contains(existing, marker) {
			return nil
		}
	}

	// Append envctl entries
	addition := "\n" + strings.Join(entries, "\n") + "\n"
	f, err := os.OpenFile(".gitignore", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // read-only or best-effort close

	if _, err := f.WriteString(addition); err != nil {
		return err
	}

	fmt.Println("Updated .gitignore")
	return nil
}
