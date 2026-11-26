package resolver

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/PillowPillow/envctl/v2/internal/template"
)

func TestResolve_PriorityChain(t *testing.T) {
	parsed := &template.ParseResult{
		Variables: []template.Variable{
			{Name: "FROM_OVERRIDE", Classification: template.RequiredSecret},
			{Name: "FROM_SECRET", Classification: template.RequiredSecret},
			{Name: "FROM_DEFAULT", Classification: template.OptionalSecret, Default: "fallback"},
			{Name: "STATIC_VAR", Classification: template.Static, Default: "hardcoded"},
			{Name: "MISSING_VAR", Classification: template.RequiredSecret},
		},
	}

	rs := Resolve(parsed, Options{
		Secrets:   map[string]string{"FROM_SECRET": "secret-val", "FROM_OVERRIDE": "secret-val"},
		Overrides: map[string]string{"FROM_OVERRIDE": "override-val"},
	})

	// Override wins over secret
	assert.Equal(t, "override-val", rs.Vars[0].Value)
	assert.Equal(t, SourceOverride, rs.Vars[0].Source)

	// Secret when no override
	assert.Equal(t, "secret-val", rs.Vars[1].Value)
	assert.Equal(t, SourceSecret, rs.Vars[1].Source)

	// Default when no secret
	assert.Equal(t, "fallback", rs.Vars[2].Value)
	assert.Equal(t, SourceDefault, rs.Vars[2].Source)
	assert.Equal(t, StatusDefaulted, rs.Vars[2].Status)

	// Static value
	assert.Equal(t, "hardcoded", rs.Vars[3].Value)
	assert.Equal(t, SourceDefault, rs.Vars[3].Source)
	assert.Equal(t, StatusOK, rs.Vars[3].Status)

	// Missing required
	assert.Equal(t, StatusMissing, rs.Vars[4].Status)
	assert.True(t, rs.HasErrors())
}

func TestResolve_Strip(t *testing.T) {
	parsed := &template.ParseResult{
		Variables: []template.Variable{
			{Name: "KEEP_ME", Classification: template.Static, Default: "val"},
			{Name: "STRIP_ME", Classification: template.Static, Default: "val"},
		},
	}

	rs := Resolve(parsed, Options{
		Strip: []string{"STRIP_ME"},
	})

	assert.Equal(t, StatusOK, rs.Vars[0].Status)
	assert.Equal(t, StatusStripped, rs.Vars[1].Status)

	values := rs.Values()
	assert.Contains(t, values, "KEEP_ME")
	assert.NotContains(t, values, "STRIP_ME")
}

func TestResolve_UnusedSecrets(t *testing.T) {
	parsed := &template.ParseResult{
		Variables: []template.Variable{
			{Name: "USED", Classification: template.RequiredSecret},
		},
	}

	rs := Resolve(parsed, Options{
		Secrets: map[string]string{"USED": "val", "OLD_KEY": "val"},
	})

	assert.Contains(t, rs.Unused, "OLD_KEY")
	assert.Equal(t, 1, rs.WarningCount())
}

func TestResolve_StrictMode(t *testing.T) {
	parsed := &template.ParseResult{
		Variables: []template.Variable{
			{Name: "USED", Classification: template.RequiredSecret},
		},
	}

	rs := Resolve(parsed, Options{
		Secrets: map[string]string{"USED": "val", "OLD_KEY": "val"},
		Strict:  true,
	})

	// Warning should be promoted to error in strict mode
	assert.Equal(t, 1, rs.ErrorCount())
	assert.Equal(t, 0, rs.WarningCount())
}

func TestFailureSummary(t *testing.T) {
	missing := ResolvedVar{Name: "TOKEN", Status: StatusMissing}
	ok := ResolvedVar{Name: "API_URL", Status: StatusOK}
	unusedAs := func(sev Severity) Diagnostic {
		return Diagnostic{Severity: sev, Variable: "OLD", Message: "secret exists in store but not referenced in template"}
	}
	missingDiag := Diagnostic{Severity: SeverityError, Variable: "TOKEN", Message: "required secret is missing"}
	cases := []struct {
		name string
		rs   ResolvedSet
		want string
	}{
		{"all good", ResolvedSet{Vars: []ResolvedVar{ok}}, ""},
		{"unused outside strict mode", ResolvedSet{Vars: []ResolvedVar{ok}, Unused: []string{"OLD"}, Diagnostics: []Diagnostic{unusedAs(SeverityWarning)}}, ""},
		{"missing only", ResolvedSet{Vars: []ResolvedVar{missing}, Diagnostics: []Diagnostic{missingDiag}}, "1 missing required secret(s)"},
		{"unused in strict mode", ResolvedSet{Vars: []ResolvedVar{ok}, Unused: []string{"OLD"}, Diagnostics: []Diagnostic{unusedAs(SeverityError)}}, "1 unused secret(s) in strict mode"},
		{"missing and unused in strict mode", ResolvedSet{Vars: []ResolvedVar{missing}, Unused: []string{"OLD"}, Diagnostics: []Diagnostic{missingDiag, unusedAs(SeverityError)}}, "1 missing required secret(s), 1 unused secret(s) in strict mode"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.rs.FailureSummary())
		})
	}
}
