package reporter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/PillowPillow/envctl/internal/resolver"
	"github.com/PillowPillow/envctl/internal/template"
)

func TestPrintVerifyResult_MissingSecrets(t *testing.T) {
	parsed := &template.ParseResult{
		Variables: []template.Variable{
			{Name: "DB_HOST", Classification: template.RequiredSecret},
			{Name: "DB_PASS", Classification: template.RequiredSecret},
			{Name: "API_KEY", Classification: template.RequiredSecret},
			{Name: "STATIC_VAR", Classification: template.Static, Default: "hello"},
		},
	}

	// Only DB_HOST is resolved, DB_PASS and API_KEY are missing
	rs := &resolver.ResolvedSet{
		Vars: []resolver.ResolvedVar{
			{Name: "DB_HOST", Value: "localhost", Source: resolver.SourceSecret, Status: resolver.StatusOK},
			{Name: "DB_PASS", Source: resolver.SourceEmpty, Status: resolver.StatusMissing},
			{Name: "API_KEY", Source: resolver.SourceEmpty, Status: resolver.StatusMissing},
			{Name: "STATIC_VAR", Value: "hello", Source: resolver.SourceDefault, Status: resolver.StatusOK},
		},
		Diagnostics: []resolver.Diagnostic{
			{Severity: resolver.SeverityError, Variable: "DB_PASS", Message: "required secret is missing"},
			{Severity: resolver.SeverityError, Variable: "API_KEY", Message: "required secret is missing"},
		},
	}

	var buf bytes.Buffer
	result := PrintVerifyResult(&buf, parsed, rs)
	output := buf.String()

	// Must show FAIL with correct count
	assert.Contains(t, output, "FAIL")
	assert.Contains(t, output, "1/3 required secrets found")

	// Must list which secrets are missing
	assert.Contains(t, output, "Missing secrets:")
	assert.Contains(t, output, "DB_PASS")
	assert.Contains(t, output, "API_KEY")

	// Must NOT say "All checks passed"
	assert.NotContains(t, output, "All checks passed")

	// Result must report correct missing count
	assert.Equal(t, 2, result.MissingCount)
}

func TestPrintVerifyResult_AllPresent(t *testing.T) {
	parsed := &template.ParseResult{
		Variables: []template.Variable{
			{Name: "DB_HOST", Classification: template.RequiredSecret},
			{Name: "DB_PASS", Classification: template.RequiredSecret},
		},
	}

	rs := &resolver.ResolvedSet{
		Vars: []resolver.ResolvedVar{
			{Name: "DB_HOST", Value: "localhost", Source: resolver.SourceSecret, Status: resolver.StatusOK},
			{Name: "DB_PASS", Value: "secret", Source: resolver.SourceSecret, Status: resolver.StatusOK},
		},
	}

	var buf bytes.Buffer
	result := PrintVerifyResult(&buf, parsed, rs)
	output := buf.String()

	assert.Contains(t, output, "OK")
	assert.Contains(t, output, "2/2 required secrets found")
	assert.Contains(t, output, "All checks passed")
	assert.Equal(t, 0, result.MissingCount)
}

func TestPrintVerifyResult_MissingWithoutDiagnostics(t *testing.T) {
	// Reproduce the bug: resolvedRequired < required but no diagnostics
	parsed := &template.ParseResult{
		Variables: []template.Variable{
			{Name: "VAR_A", Classification: template.RequiredSecret},
			{Name: "VAR_B", Classification: template.RequiredSecret},
			{Name: "VAR_C", Classification: template.RequiredSecret},
		},
	}

	rs := &resolver.ResolvedSet{
		Vars: []resolver.ResolvedVar{
			{Name: "VAR_A", Value: "ok", Source: resolver.SourceSecret, Status: resolver.StatusOK},
			{Name: "VAR_B", Source: resolver.SourceEmpty, Status: resolver.StatusMissing},
			{Name: "VAR_C", Source: resolver.SourceEmpty, Status: resolver.StatusMissing},
		},
		// No diagnostics — this was the bug scenario
		Diagnostics: nil,
	}

	var buf bytes.Buffer
	result := PrintVerifyResult(&buf, parsed, rs)
	output := buf.String()

	// Even without diagnostics, reporter should detect missing vars
	assert.Contains(t, output, "FAIL")
	assert.Contains(t, output, "Missing secrets:")
	assert.Contains(t, output, "VAR_B")
	assert.Contains(t, output, "VAR_C")
	assert.NotContains(t, output, "All checks passed")
	assert.Equal(t, 2, result.MissingCount)

	_ = strings.Contains(output, "")
}

func TestPrintVerifyResult_StrippedRequiredSecrets(t *testing.T) {
	// Reproduces the example-frontend scenario:
	// RequiredSecret vars (E2E_TEST_USER_EMAIL, E2E_TEST_USER_PASSWORD) are
	// in the strip list, so they get StatusStripped. They should NOT count
	// as missing — stripping is intentional exclusion.
	parsed := &template.ParseResult{
		Variables: []template.Variable{
			{Name: "VITE_API_URL", Classification: template.RequiredSecret},
			{Name: "VITE_AUTH_DOMAIN", Classification: template.RequiredSecret},
			{Name: "E2E_USER_EMAIL", Classification: template.RequiredSecret},
			{Name: "E2E_USER_PASS", Classification: template.RequiredSecret},
			{Name: "APP_NAME", Classification: template.Static, Default: "MyApp"},
		},
	}

	rs := &resolver.ResolvedSet{
		Vars: []resolver.ResolvedVar{
			{Name: "VITE_API_URL", Value: "https://api.example.com", Source: resolver.SourceSecret, Status: resolver.StatusOK},
			{Name: "VITE_AUTH_DOMAIN", Value: "auth.example.com", Source: resolver.SourceSecret, Status: resolver.StatusOK},
			{Name: "E2E_USER_EMAIL", Status: resolver.StatusStripped},
			{Name: "E2E_USER_PASS", Status: resolver.StatusStripped},
			{Name: "APP_NAME", Value: "MyApp", Source: resolver.SourceDefault, Status: resolver.StatusOK},
		},
	}

	var buf bytes.Buffer
	result := PrintVerifyResult(&buf, parsed, rs)
	output := buf.String()

	// Stripped required secrets should count as resolved
	assert.Contains(t, output, "OK")
	assert.Contains(t, output, "4/4 required secrets found")
	assert.Contains(t, output, "All checks passed")
	assert.NotContains(t, output, "FAIL")
	assert.NotContains(t, output, "Missing secrets:")
	assert.Equal(t, 0, result.MissingCount)
}

func TestPrintVerifyResult_StrippedAndMissing(t *testing.T) {
	// Mix: some stripped (ok), some genuinely missing (error)
	parsed := &template.ParseResult{
		Variables: []template.Variable{
			{Name: "API_URL", Classification: template.RequiredSecret},
			{Name: "API_KEY", Classification: template.RequiredSecret},
			{Name: "E2E_VAR", Classification: template.RequiredSecret},
		},
	}

	rs := &resolver.ResolvedSet{
		Vars: []resolver.ResolvedVar{
			{Name: "API_URL", Value: "https://api.example.com", Source: resolver.SourceSecret, Status: resolver.StatusOK},
			{Name: "API_KEY", Source: resolver.SourceEmpty, Status: resolver.StatusMissing},
			{Name: "E2E_VAR", Status: resolver.StatusStripped},
		},
	}

	var buf bytes.Buffer
	result := PrintVerifyResult(&buf, parsed, rs)
	output := buf.String()

	assert.Contains(t, output, "FAIL")
	assert.Contains(t, output, "2/3 required secrets found")
	assert.Contains(t, output, "Missing secrets:")
	assert.Contains(t, output, "API_KEY")
	assert.NotContains(t, output, "E2E_VAR")
	assert.NotContains(t, output, "All checks passed")
	assert.Equal(t, 1, result.MissingCount)
}
