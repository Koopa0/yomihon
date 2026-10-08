package graph

import (
	"regexp"
	"strings"
)

var calloutOpening = regexp.MustCompile(`^\s*>\s*\[!([A-Za-z0-9_-]+)\]([+-]?)\s?(.*)$`)

// CalloutStart reads the opening line that a callout's literal title consumes.
// The surrounding body grammar decides whether the line is quotation or prose.
func CalloutStart(line string) (typ, fold, title string, ok bool) {
	m := calloutOpening.FindStringSubmatch(line)
	if m == nil {
		return "", "", "", false
	}
	return strings.ToLower(m[1]), m[2], m[3], true
}

// calloutTitleAt reads only the physical source line owning this token. Nested
// quotes peel to the same opening grammar; ordinary quoted prose keeps refs.
func calloutTitleAt(source []byte, offset int) bool {
	start, stop := offset, offset
	for start > 0 && source[start-1] != '\n' {
		start--
	}
	for stop < len(source) && source[stop] != '\n' {
		stop++
	}
	line := string(source[start:stop])
	for {
		if _, _, _, ok := CalloutStart(line); ok {
			return true
		}
		prefix := QuotePrefix.FindString(line)
		if prefix == "" {
			return false
		}
		line = line[len(prefix):]
	}
}
