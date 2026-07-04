package graph

import "strings"

// SplitWikilink splits inner — a wikilink's raw text between its
// enclosing "[[" and "]]" — into the resolution target and the display
// text a renderer shows for it.
//
// The target-extraction order mirrors kura's wikilink.rs::strip_target
// exactly: first '|' (display separator), then '#' (heading fragment),
// then '^' (block fragment); '#' and '^' are always discarded from the
// target — anchors are never verified against the target file's actual
// contents, only the file's existence matters (docs/vault-model.md's
// wikilink table).
//
// A markdown table cell escapes a literal '|' as '\|' so the GFM table
// syntax doesn't split the cell on it. kura's approach treats an escaped
// and unescaped pipe identically: split on the first literal '|'
// regardless of a preceding backslash, then trim any trailing backslash
// off the left side. That yields the same target either way, so kurodo
// mirrors it exactly here to keep target extraction byte-for-byte
// agreement with kura's resolver. Display text is a kurodo-specific
// extension beyond kura (which has no rendering concept, only a target to
// resolve): the text after the same split point, or — when there is no
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
