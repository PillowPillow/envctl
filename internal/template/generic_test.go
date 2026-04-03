package template

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenericParser_Parse(t *testing.T) {
	input := `server {
    listen 80;
    server_name ${DOMAIN};

    location / {
        proxy_pass http://${BACKEND_HOST:-localhost}:${BACKEND_PORT:-8080};
    }
}`
	parser := &GenericParser{}
	result, err := parser.Parse(strings.NewReader(input))
	require.NoError(t, err)

	assert.Len(t, result.Variables, 3)

	assert.Equal(t, "DOMAIN", result.Variables[0].Name)
	assert.Equal(t, RequiredSecret, result.Variables[0].Classification)

	assert.Equal(t, "BACKEND_HOST", result.Variables[1].Name)
	assert.Equal(t, OptionalSecret, result.Variables[1].Classification)
	assert.Equal(t, "localhost", result.Variables[1].Default)

	assert.Equal(t, "BACKEND_PORT", result.Variables[2].Name)
	assert.Equal(t, OptionalSecret, result.Variables[2].Classification)
	assert.Equal(t, "8080", result.Variables[2].Default)
}

func TestGenericParser_Render(t *testing.T) {
	input := `server_name ${DOMAIN};
proxy_pass http://${BACKEND_HOST:-localhost}:${BACKEND_PORT:-8080};`

	parser := &GenericParser{}
	result, err := parser.Parse(strings.NewReader(input))
	require.NoError(t, err)

	values := map[string]string{
		"DOMAIN":       "example.com",
		"BACKEND_HOST": "api-server",
	}

	output := parser.Render(result, values)
	assert.Contains(t, output, "server_name example.com;")
	assert.Contains(t, output, "proxy_pass http://api-server:8080;")
}

func TestGenericParser_DeduplicatesVariables(t *testing.T) {
	input := `${VAR} and ${VAR} again
${VAR} on another line`

	parser := &GenericParser{}
	result, err := parser.Parse(strings.NewReader(input))
	require.NoError(t, err)

	assert.Len(t, result.Variables, 1)
	assert.Equal(t, "VAR", result.Variables[0].Name)
}
