package reporter

import (
	"fmt"
	"io"

	"github.com/fatih/color"

	"github.com/PillowPillow/envctl/internal/resolver"
	"github.com/PillowPillow/envctl/internal/template"
)

var (
	green  = color.New(color.FgGreen).SprintFunc()
	red    = color.New(color.FgRed).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	bold   = color.New(color.Bold).SprintFunc()
)

// VerifyResult contains the outcome of a verify report.
type VerifyResult struct {
	MissingCount int
}

// PrintVerifyResult outputs the verification result to the writer.
func PrintVerifyResult(w io.Writer, parsed *template.ParseResult, rs *resolver.ResolvedSet) VerifyResult {
	required, optional, static := parsed.Stats()
	total := len(parsed.Variables)

	fmt.Fprintf(w, "\nParsing template...\n")
	fmt.Fprintf(w, "  Found %d variables (%d required, %d optional, %d static)\n\n",
		total, required, optional, static)

	fmt.Fprintf(w, "Checking secrets...\n")

	// Build classification lookup for O(1) access
	classification := make(map[string]template.Classification, len(parsed.Variables))
	for _, v := range parsed.Variables {
		classification[v.Name] = v.Classification
	}

	// Count resolved required (stripped vars are intentionally excluded, not missing)
	resolvedRequired := 0
	for _, v := range rs.Vars {
		if classification[v.Name] != template.RequiredSecret {
			continue
		}
		if v.Status == resolver.StatusOK || v.Status == resolver.StatusDefaulted || v.Status == resolver.StatusStripped {
			resolvedRequired++
		}
	}

	if required > 0 {
		if resolvedRequired == required {
			fmt.Fprintf(w, "  %s %d/%d required secrets found\n", green("OK"), resolvedRequired, required)
		} else {
			fmt.Fprintf(w, "  %s %d/%d required secrets found\n", red("FAIL"), resolvedRequired, required)
		}
	}

	// Detail each required secret's status
	for _, v := range rs.Vars {
		if classification[v.Name] != template.RequiredSecret {
			continue
		}
		switch v.Status {
		case resolver.StatusOK:
			fmt.Fprintf(w, "    %s %s (%s)\n", green("✓"), v.Name, v.Source)
		case resolver.StatusDefaulted:
			fmt.Fprintf(w, "    %s %s (default)\n", green("✓"), v.Name)
		case resolver.StatusStripped:
			fmt.Fprintf(w, "    %s %s (stripped)\n", yellow("-"), v.Name)
		case resolver.StatusMissing:
			fmt.Fprintf(w, "    %s %s\n", red("✗"), bold(v.Name))
		}
	}

	// List unused secrets
	for _, d := range rs.Diagnostics {
		if d.Severity == resolver.SeverityWarning {
			fmt.Fprintf(w, "    %s %s (unused)\n", yellow("!"), d.Variable)
		}
	}

	fmt.Fprintln(w)

	missingCount := required - resolvedRequired

	switch {
	case missingCount > 0 || rs.HasErrors():
		fmt.Fprintf(w, "%d missing required secret(s)\n", missingCount)
	case rs.WarningCount() > 0:
		fmt.Fprintf(w, "%d warning(s)\n", rs.WarningCount())
	default:
		fmt.Fprintf(w, "%s\n", green("All checks passed."))
	}

	return VerifyResult{MissingCount: missingCount}
}
