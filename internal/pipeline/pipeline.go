package pipeline

import (
	"fmt"
	"os"

	"github.com/PillowPillow/envctl/internal/config"
	"github.com/PillowPillow/envctl/internal/resolver"
	"github.com/PillowPillow/envctl/internal/secret"
	"github.com/PillowPillow/envctl/internal/template"
)

// ParseTemplate detects the appropriate parser for the given template file,
// opens it, and parses it. Returns the parser (needed for Render) and the result.
func ParseTemplate(templatePath string) (template.Parser, *template.ParseResult, error) {
	parser := template.DetectParser(templatePath)

	f, err := os.Open(templatePath) //nolint:gosec // path from project config file
	if err != nil {
		return nil, nil, fmt.Errorf("cannot open template %s: %w", templatePath, err)
	}
	defer f.Close() //nolint:errcheck // read-only file

	parsed, err := parser.Parse(f)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot parse template %s: %w", templatePath, err)
	}

	return parser, parsed, nil
}

// ResolveEnv loads secrets for one environment and resolves all template variables.
// The provider parameter enables testing with MemoryProvider.
func ResolveEnv(
	provider secret.Provider,
	env *config.Environment,
	parsed *template.ParseResult,
	strict bool,
) (*resolver.ResolvedSet, error) {
	secrets, err := provider.Load(env.Secrets)
	if err != nil {
		return nil, err
	}

	rs := resolver.Resolve(parsed, resolver.Options{
		Secrets:   secrets,
		Overrides: env.Overrides,
		Strip:     env.Strip,
		Strict:    strict,
	})

	return rs, nil
}
