package render

import (
	"strings"

	"github.com/koopa0/yomihon/internal/sequence"
)

// stripListRowRoles walks assembled HTML and takes a declared role off each
// list row the way assignHeadingIDs takes one off a heading. The declaration
// is grammar the course parser consumes; the reader should meet the row's
// words. Source bytes are not touched.
//
// "The line" is the <li>'s own text up to its nested list. Descendant rows
// are other lines, and a marker that is the whole line stays: HeadingName
// already answers that, because stripping it would leave no words at all. A
// marker quoted in code keeps the closing tag after it, which readMarker
// already refuses.
func stripListRowRoles(htmlOut string) string {
	var out strings.Builder
	rest := 0
	for {
		start, openEnd := nextListItemOpen(htmlOut, rest)
		if start < 0 {
			out.WriteString(htmlOut[rest:])
			return out.String()
		}
		out.WriteString(htmlOut[rest:openEnd])
		ownEnd := listRowOwnEnd(htmlOut, openEnd)
		out.WriteString(stripListRowRole(htmlOut[openEnd:ownEnd]))
		rest = ownEnd
	}
}

// listRowLevel is the level HeadingName needs to take a role off. A list row
// is not a heading, but it is a place a course opens a branch, which is the
// same test a level-2 heading faces. Level 1 would leave every marker on.
const listRowLevel = 2

func stripListRowRole(own string) string {
	if got := sequence.HeadingName(own, listRowLevel); got != own {
		return got
	}
	// A loose item wraps its own line in <p>, so the marker is no longer the
	// last thing on the string HeadingName sees. The inner of that one
	// paragraph is the line; unwrapping anything else — a code span especially
	// — would hide the closing tag that keeps a quoted marker in place.
	inner, prefix, suffix, ok := unwrapOwnParagraph(own)
	if !ok {
		return own
	}
	stripped := sequence.HeadingName(inner, listRowLevel)
	if stripped == inner {
		return own
	}
	return prefix + stripped + suffix
}

func unwrapOwnParagraph(own string) (inner, prefix, suffix string, ok bool) {
	start := 0
	for start < len(own) && isHTMLSpace(own[start]) {
		start++
	}
	const pOpen = "<p>"
	if !strings.HasPrefix(own[start:], pOpen) {
		return "", "", "", false
	}
	end := len(own)
	for end > start && isHTMLSpace(own[end-1]) {
		end--
	}
	const pClose = "</p>"
	if !strings.HasSuffix(own[start:end], pClose) {
		return "", "", "", false
	}
	innerStart := start + len(pOpen)
	innerEnd := end - len(pClose)
	if innerEnd < innerStart {
		return "", "", "", false
	}
	return own[innerStart:innerEnd], own[:innerStart], own[innerEnd:], true
}

func isHTMLSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

func nextListItemOpen(htmlOut string, from int) (start, openEnd int) {
	for from < len(htmlOut) {
		rel := strings.Index(htmlOut[from:], "<li")
		if rel < 0 {
			return -1, -1
		}
		start = from + rel
		if !tagNamed(htmlOut[start:], "<li") {
			from = start + len("<li")
			continue
		}
		openEnd = skipTag(htmlOut, start)
		if openEnd == start || htmlOut[openEnd-1] != '>' {
			return -1, -1
		}
		return start, openEnd
	}
	return -1, -1
}

// listRowOwnEnd is just past the <li>'s own text: the first nested list that
// is still this row's child, or the close of this item when it has none.
// Nested <li> tags sit inside that list, so they are not this row's line.
func listRowOwnEnd(htmlOut string, innerStart int) int {
	liDepth := 0
	for i := innerStart; i < len(htmlOut); {
		rel := strings.IndexByte(htmlOut[i:], '<')
		if rel < 0 {
			return len(htmlOut)
		}
		i += rel
		rest := htmlOut[i:]
		switch {
		case tagNamed(rest, "</li"):
			if liDepth == 0 {
				return i
			}
			liDepth--
			i = skipTag(htmlOut, i)
		case tagNamed(rest, "<li"):
			liDepth++
			i = skipTag(htmlOut, i)
		case liDepth == 0 && (tagNamed(rest, "<ul") || tagNamed(rest, "<ol")):
			return i
		default:
			i = skipTag(htmlOut, i)
		}
	}
	return len(htmlOut)
}

func tagNamed(s, name string) bool {
	if !strings.HasPrefix(s, name) {
		return false
	}
	if len(s) == len(name) {
		return false
	}
	switch s[len(name)] {
	case '>', ' ', '\t', '\n', '\r', '/':
		return true
	default:
		return false
	}
}

func skipTag(htmlOut string, at int) int {
	rel := strings.IndexByte(htmlOut[at:], '>')
	if rel < 0 {
		return len(htmlOut)
	}
	return at + rel + 1
}
