package resolver

import "fmt"

// Source describes where a variable's value came from.
type Source int

const (
	SourceOverride Source = iota
	SourceSecret
	SourceDefault
	SourceEmpty
)

func (s Source) String() string {
	switch s {
	case SourceOverride:
		return "override"
	case SourceSecret:
		return "secret"
	case SourceDefault:
		return "default"
	case SourceEmpty:
		return "empty"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// Status describes the resolution outcome for a variable.
type Status int

const (
	StatusOK        Status = iota
	StatusMissing          // required secret not found
	StatusDefaulted        // optional secret fell back to default
	StatusStripped         // variable removed from output
)

func (s Status) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusMissing:
		return "missing"
	case StatusDefaulted:
		return "defaulted"
	case StatusStripped:
		return "stripped"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// ResolvedVar represents a variable after resolution.
type ResolvedVar struct {
	Name   string
	Value  string
	Source Source
	Status Status
}

// Severity classifies a diagnostic message.
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityError
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// Diagnostic is a message about the resolution process.
type Diagnostic struct {
	Severity Severity
	Variable string
	Message  string
}

// ResolvedSet is the complete result of variable resolution.
type ResolvedSet struct {
	Vars        []ResolvedVar
	Diagnostics []Diagnostic
	Unused      []string // secrets in store but not in template
}

// Values returns a map of variable name to resolved value, excluding stripped vars.
func (rs *ResolvedSet) Values() map[string]string {
	m := make(map[string]string)
	for _, v := range rs.Vars {
		if v.Status != StatusStripped {
			m[v.Name] = v.Value
		}
	}
	return m
}

// HasErrors returns true if any diagnostic is an error.
func (rs *ResolvedSet) HasErrors() bool {
	for _, d := range rs.Diagnostics {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of error-level diagnostics.
func (rs *ResolvedSet) ErrorCount() int {
	count := 0
	for _, d := range rs.Diagnostics {
		if d.Severity == SeverityError {
			count++
		}
	}
	return count
}

// WarningCount returns the number of warning-level diagnostics.
func (rs *ResolvedSet) WarningCount() int {
	count := 0
	for _, d := range rs.Diagnostics {
		if d.Severity == SeverityWarning {
			count++
		}
	}
	return count
}

// FailureSummary describes why the resolution fails, or returns "" when it does not.
func (rs *ResolvedSet) FailureSummary() string {
	missing := 0
	for _, v := range rs.Vars {
		if v.Status == StatusMissing {
			missing++
		}
	}
	unused := make(map[string]bool, len(rs.Unused))
	for _, name := range rs.Unused {
		unused[name] = true
	}
	strictUnused := 0
	for _, d := range rs.Diagnostics {
		if d.Severity == SeverityError && unused[d.Variable] {
			strictUnused++
		}
	}
	switch {
	case missing == 0 && strictUnused == 0:
		return ""
	case strictUnused == 0:
		return fmt.Sprintf("%d missing required secret(s)", missing)
	case missing == 0:
		return fmt.Sprintf("%d unused secret(s) in strict mode", strictUnused)
	default:
		return fmt.Sprintf("%d missing required secret(s), %d unused secret(s) in strict mode", missing, strictUnused)
	}
}
