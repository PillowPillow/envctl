package template

import (
	"io"
	"path/filepath"
	"strings"
)

// Parser is the interface for template file parsers.
type Parser interface {
	Parse(r io.Reader) (*ParseResult, error)
	Render(result *ParseResult, values map[string]string) string
}

// DetectParser returns the appropriate parser for the given filename.
// Files matching *.env* use DotenvParser; everything else uses GenericParser.
func DetectParser(filename string) Parser {
	base := filepath.Base(filename)
	if strings.Contains(base, ".env") {
		return &DotenvParser{}
	}
	return &GenericParser{}
}
