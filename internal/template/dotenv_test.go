package template

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDotenvParser_Parse(t *testing.T) {
	input := `# Database configuration
DB_HOST=localhost
DB_PORT=5432
DB_PASSWORD=${DB_PASSWORD}
DB_NAME=${DB_NAME:-myapp}

# App settings
APP_KEY=${APP_KEY}
APP_DEBUG=true
APP_URL=${APP_URL:-http://localhost}
`
	parser := &DotenvParser{}
	result, err := parser.Parse(strings.NewReader(input))
	require.NoError(t, err)

	assert.Len(t, result.Variables, 7)

	// Static
	assert.Equal(t, "DB_HOST", result.Variables[0].Name)
	assert.Equal(t, Static, result.Variables[0].Classification)
	assert.Equal(t, "localhost", result.Variables[0].Default)

	assert.Equal(t, "DB_PORT", result.Variables[1].Name)
	assert.Equal(t, Static, result.Variables[1].Classification)

	// Required secret
	assert.Equal(t, "DB_PASSWORD", result.Variables[2].Name)
	assert.Equal(t, RequiredSecret, result.Variables[2].Classification)

	// Optional secret
	assert.Equal(t, "DB_NAME", result.Variables[3].Name)
	assert.Equal(t, OptionalSecret, result.Variables[3].Classification)
	assert.Equal(t, "myapp", result.Variables[3].Default)

	// Required secret
	assert.Equal(t, "APP_KEY", result.Variables[4].Name)
	assert.Equal(t, RequiredSecret, result.Variables[4].Classification)

	// Static
	assert.Equal(t, "APP_DEBUG", result.Variables[5].Name)
	assert.Equal(t, Static, result.Variables[5].Classification)

	// Optional secret
	assert.Equal(t, "APP_URL", result.Variables[6].Name)
	assert.Equal(t, OptionalSecret, result.Variables[6].Classification)
	assert.Equal(t, "http://localhost", result.Variables[6].Default)

	// Check stats
	required, optional, static := result.Stats()
	assert.Equal(t, 2, required)
	assert.Equal(t, 2, optional)
	assert.Equal(t, 3, static)

	// Check line preservation (comments + blank lines)
	assert.Equal(t, 10, len(result.Lines))
	assert.Equal(t, LineComment, result.Lines[0].Type)
	assert.Equal(t, LineBlank, result.Lines[5].Type)
}

func TestDotenvParser_Render(t *testing.T) {
	input := `# Config
DB_HOST=localhost
DB_PASSWORD=${DB_PASSWORD}
APP_KEY=${APP_KEY}
`
	parser := &DotenvParser{}
	result, err := parser.Parse(strings.NewReader(input))
	require.NoError(t, err)

	values := map[string]string{
		"DB_HOST":     "localhost",
		"DB_PASSWORD": "secret123",
		"APP_KEY":     "base64:abc",
	}

	output := parser.Render(result, values)
	assert.Contains(t, output, "# Config")
	assert.Contains(t, output, "DB_HOST=localhost")
	assert.Contains(t, output, "DB_PASSWORD=secret123")
	assert.Contains(t, output, "APP_KEY=base64:abc")
}

func TestDotenvParser_QuotedValues(t *testing.T) {
	input := `VAR1="hello world"
VAR2='single quoted'
VAR3=noquotes
`
	parser := &DotenvParser{}
	result, err := parser.Parse(strings.NewReader(input))
	require.NoError(t, err)

	assert.Len(t, result.Variables, 3)
	assert.Equal(t, "hello world", result.Variables[0].Default)
	assert.True(t, result.Variables[0].Quoted)
	assert.Equal(t, "single quoted", result.Variables[1].Default)
	assert.True(t, result.Variables[1].Quoted)
	assert.Equal(t, "noquotes", result.Variables[2].Default)
	assert.False(t, result.Variables[2].Quoted)
}

func TestDotenvParser_Render_QuotingPreserved(t *testing.T) {
	input := `PORT=${PORT:-"3000"}
HOST=${HOST:-localhost}
NAME=${NAME:-"my app"}
DEBUG=${DEBUG:-true}
`
	parser := &DotenvParser{}
	result, err := parser.Parse(strings.NewReader(input))
	require.NoError(t, err)

	// Use defaults (no overrides)
	values := map[string]string{
		"PORT":  "3000",
		"HOST":  "localhost",
		"NAME":  "my app",
		"DEBUG": "true",
	}

	output := parser.Render(result, values)
	assert.Contains(t, output, `PORT="3000"`)    // quoted in template → quoted in output
	assert.Contains(t, output, "HOST=localhost") // unquoted in template → unquoted
	assert.Contains(t, output, `NAME="my app"`)  // quoted in template → quoted (also has space)
	assert.Contains(t, output, "DEBUG=true")     // unquoted in template → unquoted
}

func TestDotenvParser_Render_QuotedStaticValues(t *testing.T) {
	input := `PORT="3000"
HOST=localhost
`
	parser := &DotenvParser{}
	result, err := parser.Parse(strings.NewReader(input))
	require.NoError(t, err)

	values := map[string]string{
		"PORT": "3000",
		"HOST": "localhost",
	}

	output := parser.Render(result, values)
	assert.Contains(t, output, `PORT="3000"`)
	assert.Contains(t, output, "HOST=localhost")
}

func TestDotenvQuote(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "simple", "simple"},
		{"with space", "has space", `"has space"`},
		{"with double quote", `has"quote`, `"has\"quote"`},
		{"with backslash", `back\slash`, `"back\\slash"`},
		{"empty", "", `""`},
		{"mixed special", `a"b\c`, `"a\"b\\c"`},
		{"hash char", "hello # comment", `"hello # comment"`},
		{"dollar sign", "price$5", `"price$5"`},
		{"single quote", "hello'world", `"hello'world"`},
		{"numeric", "3000", "3000"},
		{"boolean true", "true", "true"},
		{"boolean false", "false", "false"},
		{"hostname", "localhost", "localhost"},
		{"url no spaces", "http://localhost:3000", "http://localhost:3000"},
		{"with tab", "has\ttab", `"has	tab"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, dotenvQuote(tt.input))
		})
	}
}

func TestDotenvParser_Render_Interpolation(t *testing.T) {
	input := `APP_URL=${APP_URL:-http://localhost}
SLACK_URL="${APP_URL}/api/v1/integrations/slack/oauth/callback"
`
	parser := &DotenvParser{}
	result, err := parser.Parse(strings.NewReader(input))
	require.NoError(t, err)

	values := map[string]string{
		"APP_URL":   "https://myapp.com",
		"SLACK_URL": "${APP_URL}/api/v1/integrations/slack/oauth/callback",
	}

	output := parser.Render(result, values)
	assert.Contains(t, output, "APP_URL=https://myapp.com")
	assert.Contains(t, output, `SLACK_URL="https://myapp.com/api/v1/integrations/slack/oauth/callback"`)
}

func TestDotenvParser_Render_InterpolationMultipleRefs(t *testing.T) {
	input := `PROTO=${PROTO:-https}
HOST=${HOST}
PORT=${PORT:-443}
URL="${PROTO}://${HOST}:${PORT}"
`
	parser := &DotenvParser{}
	result, err := parser.Parse(strings.NewReader(input))
	require.NoError(t, err)

	values := map[string]string{
		"PROTO": "https",
		"HOST":  "example.com",
		"PORT":  "8080",
		"URL":   "${PROTO}://${HOST}:${PORT}",
	}

	output := parser.Render(result, values)
	assert.Contains(t, output, `URL="https://example.com:8080"`)
}

func TestDotenvParser_Render_InterpolationInlineDefault(t *testing.T) {
	input := `CALLBACK_URL="${APP_URL:-http://localhost}/callback"
`
	parser := &DotenvParser{}
	result, err := parser.Parse(strings.NewReader(input))
	require.NoError(t, err)

	// APP_URL not in values → inline default should be used
	values := map[string]string{
		"CALLBACK_URL": "${APP_URL:-http://localhost}/callback",
	}

	output := parser.Render(result, values)
	assert.Contains(t, output, `CALLBACK_URL="http://localhost/callback"`)
}

func TestDotenvParser_Render_InterpolationUnresolved(t *testing.T) {
	input := `URL="${MISSING_VAR}/path"
`
	parser := &DotenvParser{}
	result, err := parser.Parse(strings.NewReader(input))
	require.NoError(t, err)

	values := map[string]string{
		"URL": "${MISSING_VAR}/path",
	}

	output := parser.Render(result, values)
	// Unresolved ref left as-is
	assert.Contains(t, output, `URL="${MISSING_VAR}/path"`)
}

func TestDotenvParser_Render_SpecialValues(t *testing.T) {
	input := "APP_NAME=${APP_NAME}\nDB_PASS=${DB_PASS}\n"
	parser := &DotenvParser{}
	result, err := parser.Parse(strings.NewReader(input))
	require.NoError(t, err)

	values := map[string]string{
		"APP_NAME": "My App",
		"DB_PASS":  `p@ss"word`,
	}

	output := parser.Render(result, values)
	assert.Contains(t, output, `APP_NAME="My App"`)
	assert.Contains(t, output, `DB_PASS="p@ss\"word"`)
}
