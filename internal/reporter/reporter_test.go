package reporter

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/PillowPillow/envctl/v2/internal/resolver"
	"github.com/PillowPillow/envctl/v2/internal/template"
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

	assert.Contains(t, output, "FAIL")
	assert.Contains(t, output, "1/3 required secrets found")
	// Each var should be listed with its status
	assert.Contains(t, output, "DB_HOST")
	assert.Contains(t, output, "DB_PASS")
	assert.Contains(t, output, "API_KEY")
	assert.NotContains(t, output, "All checks passed")
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
	assert.Contains(t, output, "DB_HOST")
	assert.Contains(t, output, "DB_PASS")
	assert.Contains(t, output, "secret")
	assert.Contains(t, output, "All checks passed")
	assert.Equal(t, 0, result.MissingCount)
}

func TestPrintVerifyResult_StrippedRequiredSecrets(t *testing.T) {
	// Reproduces example-frontend: RequiredSecret vars in strip list
	// get StatusStripped. They should count as resolved and show as "(stripped)".
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

	assert.Contains(t, output, "OK")
	assert.Contains(t, output, "4/4 required secrets found")
	assert.Contains(t, output, "E2E_USER_EMAIL")
	assert.Contains(t, output, "stripped")
	assert.Contains(t, output, "All checks passed")
	assert.NotContains(t, output, "FAIL")
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
	// All three should be visible with their status
	assert.Contains(t, output, "API_URL")
	assert.Contains(t, output, "API_KEY")
	assert.Contains(t, output, "E2E_VAR")
	assert.Contains(t, output, "stripped")
	assert.NotContains(t, output, "All checks passed")
	assert.Equal(t, 1, result.MissingCount)
}

func TestPrintVerifyResult_UnusedSecrets(t *testing.T) {
	parsed := &template.ParseResult{
		Variables: []template.Variable{
			{Name: "DB_HOST", Classification: template.RequiredSecret},
		},
	}

	rs := &resolver.ResolvedSet{
		Vars: []resolver.ResolvedVar{
			{Name: "DB_HOST", Value: "localhost", Source: resolver.SourceSecret, Status: resolver.StatusOK},
		},
		Diagnostics: []resolver.Diagnostic{
			{Severity: resolver.SeverityWarning, Variable: "OLD_SECRET", Message: "unused"},
		},
	}

	var buf bytes.Buffer
	result := PrintVerifyResult(&buf, parsed, rs)
	output := buf.String()

	assert.Contains(t, output, "OK")
	assert.Contains(t, output, "OLD_SECRET")
	assert.Contains(t, output, "unused")
	assert.Equal(t, 0, result.MissingCount)
}
