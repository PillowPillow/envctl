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

		key, value, quoted, ok := parseKeyValue(trimmed)
		if !ok {
			result.Lines = append(result.Lines, Line{Type: LineComment, Raw: raw})
			continue
		}

		v := classifyVariable(key, value, quoted, raw, lineNum)
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
			val = dotenvInterpolate(val, values)
			if !first {
				sb.WriteByte('\n')
			}
			first = false
			sb.WriteString(line.Variable.Name)
			sb.WriteByte('=')
			if line.Variable.Quoted {
				sb.WriteString(dotenvForceQuote(val))
			} else {
				sb.WriteString(dotenvQuote(val))
			}
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

func parseKeyValue(line string) (key, value string, quoted, ok bool) {
	idx := strings.IndexByte(line, '=')
	if idx < 1 {
		return "", "", false, false
	}
	key = strings.TrimSpace(line[:idx])
	value = strings.TrimSpace(line[idx+1:])

	// Detect and strip surrounding quotes from value
	raw := value
	value = unquote(value)
	quoted = len(raw) != len(value)

	return key, value, quoted, true
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// dotenvInterpolate replaces ${VAR} and ${VAR:-default} references inside s
// with their resolved values from the values map.  If a referenced variable is
// not in the map, the inline default is used; if there is no default either,
// the reference is left as-is.
func dotenvInterpolate(s string, values map[string]string) string {
	return genericVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		m := genericVarPattern.FindStringSubmatch(match)
		if m == nil {
			return match
		}
		if v, ok := values[m[1]]; ok {
			return v
		}
		if m[2] != "" {
			return m[2]
		}
		return match
	})
}

// dotenvForceQuote always wraps s in double quotes with proper escaping.
func dotenvForceQuote(s string) string {
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

// dotenvNeedsQuoting reports whether s must be wrapped in double quotes to be
// safely consumed by .env parsers.  Values that are empty or contain spaces,
// shell-significant characters (#, $), quotes, or backslashes need quoting.
func dotenvNeedsQuoting(s string) bool {
	if s == "" {
		return true
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '#', '$', '"', '\'', '\\':
			return true
		}
	}
	return false
}

// dotenvQuote returns s ready to be placed after '=' in a .env line.
// Simple values (numbers, booleans, plain strings) are returned as-is.
// Values that need protection are wrapped in double quotes with proper escaping.
func dotenvQuote(s string) string {
	if !dotenvNeedsQuoting(s) {
		return s
	}
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

func classifyVariable(name, value string, quoted bool, rawLine string, lineNum int) Variable {
	v := Variable{
		Name:       name,
		RawLine:    rawLine,
		LineNumber: lineNum,
		Quoted:     quoted,
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
		raw := matches[2]
		v.Default = unquote(raw)
		if len(raw) != len(v.Default) {
			v.Quoted = true
		}
	} else {
		v.Classification = RequiredSecret
	}

	return v
}
