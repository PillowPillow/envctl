package resolver

import (
	"fmt"

	"github.com/PillowPillow/envctl/v2/internal/template"
)

// Options configures the resolution process.
type Options struct {
	Secrets   map[string]string // decrypted secrets
	Overrides map[string]string // overrides from config
	Strip     []string          // variables to remove from output
	Strict    bool              // treat warnings as errors
}

// Resolve applies the substitution priority chain to produce a ResolvedSet.
// Priority (highest wins): overrides > secrets > defaults > empty.
func Resolve(parsed *template.ParseResult, opts Options) *ResolvedSet {
	stripSet := make(map[string]bool, len(opts.Strip))
	for _, s := range opts.Strip {
		stripSet[s] = true
	}

	rs := &ResolvedSet{}

	for _, v := range parsed.Variables {
		rv := ResolvedVar{Name: v.Name}

		if stripSet[v.Name] {
			rv.Status = StatusStripped
			rs.Vars = append(rs.Vars, rv)
			continue
		}

		// Priority chain
		if val, ok := opts.Overrides[v.Name]; ok {
			rv.Value = val
			rv.Source = SourceOverride
			rv.Status = StatusOK
		} else if val, ok := opts.Secrets[v.Name]; ok {
			rv.Value = val
			rv.Source = SourceSecret
			rv.Status = StatusOK
		} else if v.Classification == template.OptionalSecret && v.Default != "" { //nolint:gocritic // can't use switch: first branches use map lookups
			rv.Value = v.Default
			rv.Source = SourceDefault
			rv.Status = StatusDefaulted
			rs.Diagnostics = append(rs.Diagnostics, Diagnostic{
				Severity: SeverityInfo,
				Variable: v.Name,
				Message:  fmt.Sprintf("using default value %q", v.Default),
			})
		} else if v.Classification == template.Static {
			rv.Value = v.Default
			rv.Source = SourceDefault
			rv.Status = StatusOK
		} else {
			// Missing required secret
			rv.Source = SourceEmpty
			if v.Classification == template.RequiredSecret {
				rv.Status = StatusMissing
				sev := SeverityError
				rs.Diagnostics = append(rs.Diagnostics, Diagnostic{
					Severity: sev,
					Variable: v.Name,
					Message:  "required secret is missing",
				})
			} else {
				rv.Status = StatusDefaulted
			}
		}

		rs.Vars = append(rs.Vars, rv)
	}

	// Find unused secrets (in store but not referenced in template)
	templateVars := make(map[string]bool, len(parsed.Variables))
	for _, v := range parsed.Variables {
		templateVars[v.Name] = true
	}
	for name := range opts.Secrets {
		if !templateVars[name] {
			rs.Unused = append(rs.Unused, name)
			rs.Diagnostics = append(rs.Diagnostics, Diagnostic{
				Severity: SeverityWarning,
				Variable: name,
				Message:  "secret exists in store but not referenced in template",
			})
		}
	}

	// In strict mode, promote warnings to errors
	if opts.Strict {
		for i := range rs.Diagnostics {
			if rs.Diagnostics[i].Severity == SeverityWarning {
				rs.Diagnostics[i].Severity = SeverityError
			}
		}
	}

	return rs
}
