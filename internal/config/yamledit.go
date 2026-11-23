package config

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

const ageRecipientsKey = "age_recipients"

var errFlowRecipients = errors.New("age_recipients in flow style is not supported; rewrite it as a block mapping")

// AddRecipient adds name: key to the age_recipients of envName ("" for sops) and leaves every other line of src untouched.
func AddRecipient(src []byte, envName, name, key string) ([]byte, error) {
	d, err := parseLines(src)
	if err != nil {
		return nil, err
	}
	if n := len(d.lines); n > 0 && !strings.HasSuffix(d.lines[n-1], "\n") {
		d.lines[n-1] += d.eol
	}
	step := d.indentStep()
	entry := func(indent int) string { return spaces(indent) + name + ": " + key + d.eol }

	if envName == "" {
		if sk, sv := mapValue(d.root, "sops"); sk != nil && isEmptyValue(sv) {
			if err := d.dropValue(sk, sv); err != nil {
				return nil, err
			}
			indent := sk.Column - 1
			d.insert(sk.Line, spaces(indent+step)+ageRecipientsKey+":"+d.eol, entry(indent+2*step))
			return d.bytes(), nil
		}
	}
	parent, err := d.scope(envName)
	if err != nil {
		return nil, err
	}

	if parent == nil {
		indent := d.root.Content[0].Column - 1
		d.insert(d.end(d.root, indent), spaces(indent)+"sops:"+d.eol, spaces(indent+step)+ageRecipientsKey+":"+d.eol, entry(indent+2*step))
		return d.bytes(), nil
	}

	ak, av := mapValue(parent, ageRecipientsKey)
	switch {
	case ak == nil:
		indent := parent.Content[0].Column - 1
		d.insert(d.end(parent, indent), spaces(indent)+ageRecipientsKey+":"+d.eol, entry(indent+step))
	case isEmptyValue(av):
		if err := d.dropValue(ak, av); err != nil {
			return nil, err
		}
		d.insert(ak.Line, entry(ak.Column-1+step))
	case av.Kind != yaml.MappingNode:
		return nil, errors.New("age_recipients is not a mapping")
	case av.Style&yaml.FlowStyle != 0:
		return nil, errFlowRecipients
	default:
		indent := av.Content[0].Column - 1
		d.insert(d.end(av, indent), entry(indent))
	}
	return d.bytes(), nil
}

// RemoveRecipient removes name from the age_recipients of envName ("" for sops) and leaves every other line of src untouched.
func RemoveRecipient(src []byte, envName, name string) ([]byte, error) {
	d, err := parseLines(src)
	if err != nil {
		return nil, err
	}
	parent, err := d.scope(envName)
	if err != nil {
		return nil, err
	}
	scope := envName
	if scope == "" {
		scope = ScopeGlobal
	}
	notFound := fmt.Errorf("recipient %q not found in scope %q", name, scope)
	if parent == nil {
		return nil, notFound
	}
	ak, av := mapValue(parent, ageRecipientsKey)
	switch {
	case ak == nil || isEmptyValue(av):
		return nil, notFound
	case av.Kind != yaml.MappingNode:
		return nil, errors.New("age_recipients is not a mapping")
	case av.Style&yaml.FlowStyle != 0:
		return nil, errFlowRecipients
	}
	k, v := mapValue(av, name)
	if k == nil {
		return nil, notFound
	}

	d.remove(k.Line-1, d.end(v, k.Column-1))
	if len(av.Content) == 2 {
		if envName != "" {
			d.remove(ak.Line-1, ak.Line)
		} else {
			// Global scope keeps the `age_recipients: {}` form written by init.
			body, eol := splitEOL(d.lines[ak.Line-1])
			colon := colonAfter(body, ak)
			d.lines[ak.Line-1] = body[:colon+1] + " {}" + body[colon+1:] + eol
		}
	}
	return d.bytes(), nil
}

type lineDoc struct {
	root  *yaml.Node
	lines []string
	eol   string
}

func parseLines(src []byte) (*lineDoc, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, fmt.Errorf("cannot parse config: %w", err)
	}
	if len(doc.Content) == 0 || !isBlockMapping(doc.Content[0]) || len(doc.Content[0].Content) == 0 {
		return nil, errors.New("config root is not a block mapping")
	}
	s := string(src)
	eol := "\n"
	if strings.Contains(s, "\r\n") {
		eol = "\r\n"
	}
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return &lineDoc{root: doc.Content[0], lines: lines, eol: eol}, nil
}

// scope returns the mapping holding age_recipients for envName, or nil when the global sops key is absent.
func (d *lineDoc) scope(envName string) (*yaml.Node, error) {
	if envName == "" {
		k, v := mapValue(d.root, "sops")
		if k == nil {
			return nil, nil
		}
		if !isBlockMapping(v) {
			return nil, errors.New("sops is not a block mapping")
		}
		return v, nil
	}
	_, envs := mapValue(d.root, "environments")
	var available []string
	if envs != nil && envs.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(envs.Content); i += 2 {
			available = append(available, envs.Content[i].Value)
		}
		if _, v := mapValue(envs, envName); v != nil {
			if !isBlockMapping(v) {
				return nil, fmt.Errorf("environment %q is not a block mapping", envName)
			}
			return v, nil
		}
	}
	sort.Strings(available)
	return nil, fmt.Errorf("environment %q not found (available: %v)", envName, available)
}

// indentStep is the column gap between `environments` and its children, so 2- and 4-space files both stay consistent.
func (d *lineDoc) indentStep() int {
	k, v := mapValue(d.root, "environments")
	if k != nil && v.Kind == yaml.MappingNode && len(v.Content) > 0 && v.Content[0].Column > k.Column {
		return v.Content[0].Column - k.Column
	}
	return 2
}

// end returns the index of the first line after n, skipping continuation lines indented deeper than indent.
func (d *lineDoc) end(n *yaml.Node, indent int) int {
	if last := lastNode(n); last.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return d.blockScalarEnd(last)
	}
	i := maxLine(n)
	if last := lastNode(n); last.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle) != 0 {
		i = max(i, d.quotedScalarEnd(last))
	}
	for j := i; j < len(d.lines); j++ {
		trimmed := strings.TrimSpace(d.lines[j])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// A flow collection may close at or left of its key's indent.
		if leadingSpaces(d.lines[j]) <= indent && trimmed[0] != ']' && trimmed[0] != '}' {
			break
		}
		i = j + 1
	}
	return i
}

// quotedScalarEnd returns the 1-based line of the closing quote of s, whose continuation lines may sit at any indent.
func (d *lineDoc) quotedScalarEnd(s *yaml.Node) int {
	quote := '"'
	if s.Style&yaml.SingleQuotedStyle != 0 {
		quote = '\''
	}
	opened := false
	for j := s.Line - 1; j < len(d.lines); j++ {
		line := []rune(d.lines[j])
		k := 0
		if j == s.Line-1 {
			k = s.Column - 1
		}
		for ; k < len(line); k++ {
			switch {
			case !opened:
				opened = line[k] == quote
			case quote == '"' && line[k] == '\\':
				k++
			case line[k] != quote:
			case quote == '\'' && k+1 < len(line) && line[k+1] == '\'':
				k++
			default:
				return j + 1
			}
		}
	}
	return s.Line
}

// blockScalarEnd returns the index of the line after block scalar s, whose content may hold blank and #-prefixed lines.
func (d *lineDoc) blockScalarEnd(s *yaml.Node) int {
	header := d.lines[s.Line-1]
	parent := leadingSpaces(header)
	keep := false
	// The node column can point at an anchor or tag that precedes the | or > indicator.
	for _, field := range strings.Fields(string([]rune(header)[s.Column-1:])) {
		if field[0] == '|' || field[0] == '>' {
			keep = strings.Contains(field, "+")
			break
		}
	}
	end, content := s.Line, -1
	for j := s.Line; j < len(d.lines); j++ {
		if strings.TrimSpace(d.lines[j]) == "" {
			if keep {
				end = j + 1
			}
			continue
		}
		ind := leadingSpaces(d.lines[j])
		if content < 0 && ind > parent {
			content = ind
		}
		if content < 0 || ind < content {
			break
		}
		end = j + 1
	}
	return end
}

// dropValue removes an empty value token ({} or a null), wherever it sits, so entries can follow the key.
func (d *lineDoc) dropValue(k, v *yaml.Node) error {
	if v.Kind == yaml.ScalarNode && v.Value == "" {
		return nil
	}
	body, eol := splitEOL(d.lines[v.Line-1])
	start := v.Column - 1
	end := start + len(v.Value)
	if v.Kind == yaml.MappingNode {
		closing := strings.IndexByte(body[start:], '}')
		if closing < 0 {
			return errFlowRecipients
		}
		end = start + closing + 1
	}
	switch {
	case v.Line == k.Line:
		d.lines[v.Line-1] = body[:colonAfter(body, k)+1] + body[end:] + eol
	case strings.TrimSpace(body[end:]) == "":
		d.remove(v.Line-1, v.Line)
	default:
		d.lines[v.Line-1] = body[:start] + body[end:] + eol
	}
	return nil
}

func (d *lineDoc) insert(at int, lines ...string) {
	d.lines = append(d.lines[:at], append(lines, d.lines[at:]...)...)
}

func (d *lineDoc) remove(from, to int) {
	d.lines = append(d.lines[:from], d.lines[to:]...)
}

func (d *lineDoc) bytes() []byte {
	return []byte(strings.Join(d.lines, ""))
}

func mapValue(m *yaml.Node, key string) (k, v *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i], m.Content[i+1]
		}
	}
	return nil, nil
}

func isBlockMapping(n *yaml.Node) bool {
	return n.Kind == yaml.MappingNode && n.Style&yaml.FlowStyle == 0
}

func isEmptyValue(n *yaml.Node) bool {
	return (n.Kind == yaml.MappingNode && len(n.Content) == 0) || (n.Kind == yaml.ScalarNode && n.Tag == "!!null")
}

func lastNode(n *yaml.Node) *yaml.Node {
	for len(n.Content) > 0 {
		n = n.Content[len(n.Content)-1]
	}
	return n
}

func maxLine(n *yaml.Node) int {
	line := n.Line
	for _, c := range n.Content {
		line = max(line, maxLine(c))
	}
	return line
}

func colonAfter(body string, key *yaml.Node) int {
	start := key.Column - 1
	return start + strings.IndexByte(body[start:], ':')
}

func splitEOL(line string) (body, eol string) {
	body = strings.TrimRight(line, "\r\n")
	return body, line[len(body):]
}

func leadingSpaces(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

func spaces(n int) string {
	return strings.Repeat(" ", n)
}
