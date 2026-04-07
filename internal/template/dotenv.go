package template

import (
	"bufio"
	"io"
	"regexp"
	"strings"
)

// DotenvParser parses .env.example style files.
// It classifies variables as static, required-secret, or optional-secret
// based on the value syntax.
type DotenvParser struct{}

// Pattern matches: VAR=${VAR} or VAR=${VAR:-default}
var varRefPattern = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)(?::-(.*))?\}$`)

func (p *DotenvParser) Parse(r io.Reader) (*ParseResult, error) {
	result := &ParseResult{}
	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)

		if trimmed == "" {
			result.Lines = append(result.Lines, Line{Type: LineBlank, Raw: raw})
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			result.Lines = append(result.Lines, Line{Type: LineComment, Raw: raw})
			continue
		}

		key, value, ok := parseKeyValue(trimmed)
		if !ok {
			result.Lines = append(result.Lines, Line{Type: LineComment, Raw: raw})
			continue
		}

		v := classifyVariable(key, value, raw, lineNum)
		result.Variables = append(result.Variables, v)
		result.Lines = append(result.Lines, Line{Type: LineVariable, Raw: raw, Variable: &result.Variables[len(result.Variables)-1]})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (p *DotenvParser) Render(result *ParseResult, values map[string]string) string {
	var sb strings.Builder
	first := true
	for _, line := range result.Lines {
		if line.Type == LineVariable && line.Variable != nil {
			val, ok := values[line.Variable.Name]
			if !ok {
				continue // omit stripped variables without emitting a blank line
			}
			if !first {
				sb.WriteByte('\n')
			}
			first = false
			sb.WriteString(line.Variable.Name)
			sb.WriteByte('=')
			sb.WriteString(dotenvQuote(val))
		} else {
			if !first {
				sb.WriteByte('\n')
			}
			first = false
			sb.WriteString(line.Raw)
		}
	}
	sb.WriteByte('\n')
	return sb.String()
}

func parseKeyValue(line string) (key, value string, ok bool) {
	idx := strings.IndexByte(line, '=')
	if idx < 1 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	value = strings.TrimSpace(line[idx+1:])

	// Strip surrounding quotes from value
	value = unquote(value)

	return key, value, true
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// dotenvQuote wraps a value in double quotes, escaping inner backslashes and
// double-quote characters.
func dotenvQuote(s string) string {
	var sb strings.Builder
	sb.Grow(len(s) + 2)
	sb.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			sb.WriteString(`\\`)
		case '"':
			sb.WriteString(`\"`)
		default:
			sb.WriteByte(s[i])
		}
	}
	sb.WriteByte('"')
	return sb.String()
}

func classifyVariable(name, value, rawLine string, lineNum int) Variable {
	v := Variable{
		Name:       name,
		RawLine:    rawLine,
		LineNumber: lineNum,
	}

	matches := varRefPattern.FindStringSubmatch(value)
	if matches == nil {
		// No ${...} pattern → static value
		v.Classification = Static
		v.Default = value
		return v
	}

	// matches[1] = variable name inside ${}, matches[2] = default (if present)
	if matches[2] != "" {
		v.Classification = OptionalSecret
		v.Default = matches[2]
	} else {
		v.Classification = RequiredSecret
	}

	return v
}
