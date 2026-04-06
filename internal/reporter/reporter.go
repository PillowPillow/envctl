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

// PrintVerifyResult outputs the verification result to the writer.
func PrintVerifyResult(w io.Writer, parsed *template.ParseResult, rs *resolver.ResolvedSet) {
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

	// Count resolved required
	resolvedRequired := 0
	for _, v := range rs.Vars {
		if (v.Status == resolver.StatusOK || v.Status == resolver.StatusDefaulted) &&
			classification[v.Name] == template.RequiredSecret {
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

	// Print diagnostics grouped by severity
	hasErrors := false
	for _, d := range rs.Diagnostics {
		if d.Severity == resolver.SeverityError {
			if !hasErrors {
				fmt.Fprintf(w, "\n  Missing secrets:\n")
				hasErrors = true
			}
			fmt.Fprintf(w, "    %s %s\n", red("x"), bold(d.Variable))
		}
	}

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

	errors := rs.ErrorCount()
	warnings := rs.WarningCount()
	if errors == 0 && warnings == 0 {
		fmt.Fprintf(w, "%s\n", green("All checks passed."))
	} else {
		parts := []string{}
		if errors > 0 {
			parts = append(parts, fmt.Sprintf("%d error(s)", errors))
		}
		if warnings > 0 {
			parts = append(parts, fmt.Sprintf("%d warning(s)", warnings))
		}
		for i, p := range parts {
			if i > 0 {
				fmt.Fprintf(w, ", ")
			}
			fmt.Fprintf(w, "%s", p)
		}
		fmt.Fprintln(w)
	}
}
