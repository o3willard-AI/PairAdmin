package filter

import (
	"fmt"
	"regexp"
	"strings"
)

// customPattern holds a compiled user-defined filter pattern.
type customPattern struct {
	name   string
	re     *regexp.Regexp
	action string // "redact" | "remove"
}

// CustomFilter applies user-defined patterns to terminal content.
// Patterns are compiled at construction time; Apply is safe to call concurrently.
type CustomFilter struct {
	patterns []*customPattern
}

// CustomPatternInput is the public struct for constructing a CustomFilter.
// It avoids importing services/config in the filter package (filter must not import Viper).
type CustomPatternInput struct {
	Name   string
	Regex  string
	Action string // "redact" | "remove"
}

// NewCustomFilter compiles the given patterns and returns a CustomFilter ready to use.
// Returns an error if any pattern has an invalid regex.
func NewCustomFilter(patterns []CustomPatternInput) (*CustomFilter, error) {
	f := &CustomFilter{}
	for _, p := range patterns {
		re, err := regexp.Compile(p.Regex)
		if err != nil {
			return nil, fmt.Errorf("custom filter %q: invalid regex: %w", p.Name, err)
		}
		f.patterns = append(f.patterns, &customPattern{
			name:   p.Name,
			re:     re,
			action: p.Action,
		})
	}
	return f, nil
}

// Apply runs the content through each custom pattern in order.
//
//   - action "redact": replaces each regex match with [REDACTED:<name>]
//   - action "remove": strips entire lines that contain at least one match
//
// Returns content unchanged when no patterns are configured.
func (f *CustomFilter) Apply(content string) (string, error) {
	if len(f.patterns) == 0 {
		return content, nil
	}
	result := content
	for _, p := range f.patterns {
		switch p.action {
		case "redact":
			result = p.re.ReplaceAllString(result, fmt.Sprintf("[REDACTED:%s]", p.name))
		case "remove":
			lines := strings.Split(result, "\n")
			kept := make([]string, 0, len(lines))
			for _, line := range lines {
				if !p.re.MatchString(line) {
					kept = append(kept, line)
				}
			}
			result = strings.Join(kept, "\n")
		}
	}
	return result, nil
}
