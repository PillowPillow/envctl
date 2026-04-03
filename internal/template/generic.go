package template

import (
	"io"
	"regexp"
	"strings"
)

// GenericParser handles any text file containing ${VAR} or ${VAR:-default} patterns.
// It performs in-place substitution while preserving all formatting.
type GenericParser struct{}

// genericVarPattern matches ${VAR} and ${VAR:-default} anywhere in text.
var genericVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::-((?:[^}\\]|\\.)*))?}`)

func (p *GenericParser) Parse(r io.Reader) (*ParseResult, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	content := string(data)
	result := &ParseResult{}
	seen := make(map[string]bool)

	lines := strings.Split(content, "\n")
	for lineNum, raw := range lines {
		line := Line{Type: LineBlank, Raw: raw}

		matches := genericVarPattern.FindAllStringSubmatch(raw, -1)
		if len(matches) > 0 {
			line.Type = LineVariable
			for _, m := range matches {
				name := m[1]
				if seen[name] {
					continue
				}
				seen[name] = true

				v := Variable{
					Name:       name,
					RawLine:    raw,
					LineNumber: lineNum + 1,
				}
				if m[2] != "" {
					v.Classification = OptionalSecret
					v.Default = m[2]
				} else {
					v.Classification = RequiredSecret
				}
				result.Variables = append(result.Variables, v)
			}
		}

		result.Lines = append(result.Lines, line)
	}

	return result, nil
}

func (p *GenericParser) Render(result *ParseResult, values map[string]string) string {
	var sb strings.Builder
	for i, line := range result.Lines {
		if i > 0 {
			sb.WriteByte('\n')
		}

		rendered := genericVarPattern.ReplaceAllStringFunc(line.Raw, func(match string) string {
			m := genericVarPattern.FindStringSubmatch(match)
			if m == nil {
				return match
			}
			name := m[1]
			if val, ok := values[name]; ok {
				return val
			}
			if m[2] != "" {
				return m[2]
			}
			return match // leave unresolved
		})
		sb.WriteString(rendered)
	}
	return sb.String()
}
