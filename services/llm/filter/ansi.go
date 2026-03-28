package filter

import (
	"regexp"
)

// ansiSequenceRe matches all ANSI/VT100/CSI/OSC/DCS escape sequences:
//   - CSI sequences: ESC [ ... final-byte (cursor movement, color, erase, etc.)
//   - OSC sequences: ESC ] ... (BEL or ST terminated) — window title, etc.
//   - DCS sequences: ESC P ... ST — device control strings
//   - APC sequences: ESC _ ... ST
//   - PM sequences:  ESC ^ ... ST
//   - SOS sequences: ESC X ... ST
//   - Simple ESC sequences: ESC followed by a single character (e.g. ESC M, ESC c)
//   - C1 control codes: 0x80–0x9F
//
// The regex is applied first; then go-ansi-parser Cleanse handles any residual
// SGR styling that the regex may have missed due to multi-chunk content.
var ansiSequenceRe = regexp.MustCompile(
	`(?:\x1b[@-Z\\-_][\x80-\x9f]*)` + // C1 introduced by ESC
		`|(?:[\x80-\x9f])` + // C1 control codes (8-bit shorthand)
		`|(?:\x1b\[[\x30-\x3f]*[\x20-\x2f]*[@-~])` + // CSI sequences
		`|(?:\x1b[\]P_^X][^\x07\x1b]*(?:\x07|\x1b\\))` + // OSC/DCS/APC/PM/SOS (BEL or ST)
		`|(?:\x1b[\]P_^X][^\x1b]*)` + // OSC/DCS/APC/PM/SOS unterminated (safety fallback)
		`|(?:\x1b[^[\]P_^X@-Z\\-_])`, // Simple 2-byte ESC sequences
)

// ANSIFilter strips ANSI/VT100/OSC/CSI escape sequences from content.
// It must be the first filter in any pipeline to prevent ANSI injection attacks
// and to ensure credential patterns are not obscured by escape sequences.
type ANSIFilter struct{}

// NewANSIFilter creates a new ANSIFilter.
func NewANSIFilter() *ANSIFilter {
	return &ANSIFilter{}
}

// Apply strips all ANSI/VT100 escape sequences from content and returns the clean text.
// Uses a comprehensive regex to strip CSI, OSC, DCS, and other escape sequences
// that go-ansi-parser does not handle (cursor movement, erase sequences, etc.).
func (f *ANSIFilter) Apply(content string) (string, error) {
	return ansiSequenceRe.ReplaceAllString(content, ""), nil
}
