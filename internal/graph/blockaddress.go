package graph

import (
	"regexp"
	"strings"
)

// A block address is the "^name" an author writes at the end of a line to give
// that block a name a link can reach, and Obsidian scrolls such a link to it. The
// anchor goes on the address itself rather than on the element around it: the
// excerpt scan finds a marker by reading lines, so every line it matches is a line
// this pass sees. The id keeps the caret, which no heading anchor can collide with.

// blockMarkerTail is the token grammar read only by BlockAddress.
var blockMarkerTail = regexp.MustCompile(`(?:\A|[ \t])(\^[A-Za-z0-9-]+)\z`)

// headingOpening is what can stand before the text of an ATX heading: its
// indentation, any quote marks around it and its hashes.
var headingOpening = regexp.MustCompile(`\A[ \t>]*#{1,6}[ \t]*\z`)

// BlockAddress returns the authored caret and nonempty ASCII letter, digit,
// or hyphen token at the end of line, preserving its case. The caret starts
// the line or follows an ASCII space or tab; only spaces and tabs may follow
// the token. It returns an empty string when no supported address is present.
// A heading whose whole text is the token has nothing to address, so the token
// is its words and not an address. Code zones and other structural exclusions
// belong to the caller.
func BlockAddress(line string) string {
	if strings.IndexByte(line, '^') < 0 {
		return ""
	}
	trimmed := strings.TrimRight(line, " \t")
	m := blockMarkerTail.FindStringSubmatchIndex(trimmed)
	if m == nil {
		return ""
	}
	if headingOpening.MatchString(trimmed[:m[2]]) {
		return ""
	}
	return trimmed[m[2]:m[3]]
}

// WithoutBlockAddress returns line without the address that ends it and the
// space or tab that set it apart, or line itself when it ends in none.
func WithoutBlockAddress(line string) string {
	address := BlockAddress(line)
	if address == "" {
		return line
	}
	trimmed := strings.TrimRight(line, " \t")
	return strings.TrimRight(trimmed[:len(trimmed)-len(address)], " \t")
}

// WithoutBlockAddresses is WithoutBlockAddress for each line of the text of a
// heading, which is how a heading written over several lines carries one. A
// heading that would be left with no words keeps them: a heading is an address
// only when words come before it.
func WithoutBlockAddresses(text string) string {
	if !strings.Contains(text, "^") {
		return text
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = WithoutBlockAddress(line)
	}
	stripped := strings.Join(lines, "\n")
	if strings.TrimSpace(stripped) == "" {
		return text
	}
	return stripped
}
