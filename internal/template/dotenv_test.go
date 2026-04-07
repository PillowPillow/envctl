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
	assert.Contains(t, output, `DB_HOST="localhost"`)
	assert.Contains(t, output, `DB_PASSWORD="secret123"`)
	assert.Contains(t, output, `APP_KEY="base64:abc"`)
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
	assert.Equal(t, "single quoted", result.Variables[1].Default)
	assert.Equal(t, "noquotes", result.Variables[2].Default)
}

func TestDotenvQuote(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "simple", `"simple"`},
		{"with space", "has space", `"has space"`},
		{"with double quote", `has"quote`, `"has\"quote"`},
		{"with backslash", `back\slash`, `"back\\slash"`},
		{"empty", "", `""`},
		{"mixed special", `a"b\c`, `"a\"b\\c"`},
		{"hash char", "hello # comment", `"hello # comment"`},
		{"dollar sign", "price$5", `"price$5"`},
		{"single quote", "hello'world", `"hello'world"`},
		{"numeric", "3000", `"3000"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, dotenvQuote(tt.input))
		})
	}
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
