package template

import "fmt"

// Classification describes how a variable should be resolved.
type Classification int

const (
	Static         Classification = iota // VAR=hardcoded
	RequiredSecret                       // VAR=${VAR}
	OptionalSecret                       // VAR=${VAR:-default}
)

func (c Classification) String() string {
	switch c {
	case Static:
		return "static"
	case RequiredSecret:
		return "required"
	case OptionalSecret:
		return "optional"
	default:
		return fmt.Sprintf("unknown(%d)", int(c))
	}
}

// Variable represents a single environment variable extracted from a template.
type Variable struct {
	Name           string
	Classification Classification
	Default        string // only meaningful for OptionalSecret
	RawLine        string // original line for output preservation
	LineNumber     int
}

// LineType describes the kind of line in a template file.
type LineType int

const (
	LineBlank LineType = iota
	LineComment
	LineVariable
)

// Line represents a single line in the template, preserving structure.
type Line struct {
	Type     LineType
	Raw      string    // original text
	Variable *Variable // non-nil only for LineVariable
}

// ParseResult holds the complete result of parsing a template file.
type ParseResult struct {
	Variables []Variable
	Lines     []Line
}

// Stats returns counts by classification.
func (pr *ParseResult) Stats() (required, optional, static int) {
	for _, v := range pr.Variables {
		switch v.Classification {
		case RequiredSecret:
			required++
		case OptionalSecret:
			optional++
		case Static:
			static++
		}
	}
	return
}
