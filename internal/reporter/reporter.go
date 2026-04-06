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

	// List missing required secrets
	hasMissing := false
	for _, v := range rs.Vars {
		if classification[v.Name] == template.RequiredSecret && v.Status == resolver.StatusMissing {
			if !hasMissing {
				fmt.Fprintf(w, "\n  Missing secrets:\n")
				hasMissing = true
			}
			fmt.Fprintf(w, "    %s %s\n", red("x"), bold(v.Name))
		}
	}

	// List unused secrets
	hasWarnings := false
	for _, d := range rs.Diagnostics {
		if d.Severity == resolver.SeverityWarning {
			if !hasWarnings {
				fmt.Fprintf(w, "\n  Unused secrets:\n")
				hasWarnings = true
			}
			fmt.Fprintf(w, "    %s %s\n", yellow("!"), d.Variable)
		}
	}

	fmt.Fprintln(w)

	missingCount := required - resolvedRequired

	switch {
	case hasMissing || rs.HasErrors():
		fmt.Fprintf(w, "%d missing required secret(s)\n", missingCount)
	case rs.WarningCount() > 0:
		fmt.Fprintf(w, "%d warning(s)\n", rs.WarningCount())
	default:
		fmt.Fprintf(w, "%s\n", green("All checks passed."))
	}

	return VerifyResult{MissingCount: missingCount}
}
