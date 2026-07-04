package graph

import "strings"

// SplitWikilink splits inner — a wikilink's raw text between its
// enclosing "[[" and "]]" — into the resolution target and the display
// text a renderer shows for it.
//
// Target extraction strips delimiters in a fixed order: first '|'
// (display separator), then '#' (heading fragment), then '^' (block
// fragment); '#' and '^' are always discarded from the
// target — anchors are never verified against the target file's actual
// contents, only the file's existence matters.
//
// A markdown table cell escapes a literal '|' as '\|' so the GFM table
// syntax doesn't split the cell on it. This function treats an escaped
// and an unescaped pipe identically: split on the first literal '|'
// regardless of a preceding backslash, then trim any trailing backslash
// off the left side. That yields the same target either way, so the
// escape never changes which file a link resolves to. Display text
// serves rendering rather than resolution: it is the text after the same
// split point, or — when there is no
// pipe at all — the original trimmed inner text, kept intact with any
// '#'/'^' suffix still visible (it is literal display text, not a
// resolution target).
//
// ok is false when the target strips to empty (e.g. "#heading" alone): a
// same-file anchor jump, not a cross-file link.
func SplitWikilink(inner string) (target, display string, ok bool) {
	trimmed := strings.TrimSpace(inner)
	beforePipe := inner
	display = trimmed
	if before, after, found := strings.Cut(inner, "|"); found {
		beforePipe = strings.TrimRight(before, `\`)
		display = strings.TrimSpace(after)
	}
	beforeHeading, _, _ := strings.Cut(beforePipe, "#")
	beforeBlock, _, _ := strings.Cut(beforeHeading, "^")
	target = strings.TrimSpace(beforeBlock)
	return target, display, target != ""
}
