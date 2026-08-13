package graph

import "strings"

// Wikilink is one wikilink's inner text — the characters between its enclosing
// "[[" and "]]" — split into the four separate things a caller can want from
// it. They are kept apart because they answer to different owners: Target is
// the only part resolution looks at, Display is what a reader sees, and the two
// fragments address a place inside the file rather than the file itself.
//
// Heading and Block are the raw authored text with no interpretation applied.
// Nothing here checks that either one exists in the target file; whether an
// anchor can be reached is a question about the destination's rendering, and
// this package only answers what a name refers to.
type Wikilink struct {
	// Target is the note or resource name to resolve, with both fragment
	// markers and any display text removed. Empty means the link addresses a
	// place in the current file and names no other file at all.
	Target string
	// Display is the text a reader sees. It is the words after the display
	// separator when the author wrote one, and otherwise the whole inner
	// text with any fragment still visible in it — at that point the
	// fragment is part of what the author chose to show, not an address.
	Display string
	// Heading is the section name written after "#", empty when absent.
	Heading string
	// Block is the block name written after "^", empty when absent. Obsidian
	// writes a block address as "#^name", so a fragment opening with "^" is
	// one of these and not a section called "^name"; the bare "^name" form,
	// stripped from the segment ahead of any "#", is read as one too.
	Block string
}

// ParseWikilink splits inner into its target, display text, and fragments.
//
// The markers are stripped in a fixed order: first '|' (display separator),
// then '#' (heading fragment), then '^' (block fragment). Both fragments are
// always removed from the target, because a link addresses a file by name and
// the part after the marker addresses a place inside it.
//
// A fragment that opens with '^' is a block address, not a section whose name
// begins with a caret: "#^name" is how Obsidian writes one. Reading it as a
// heading is not a cosmetic mistake — a caller that turns heading text into an
// anchor would then produce an address for a place that has no anchor at all.
//
// A markdown table cell escapes a literal '|' as '\|' so the GFM table syntax
// doesn't split the cell on it. This treats an escaped and an unescaped pipe
// identically: split on the first literal '|' regardless of a preceding
// backslash, then trim any trailing backslash off the left side. That yields
// the same target either way, so the escape never changes which file a link
// resolves to.
//
// ok is false when the target strips to empty (e.g. "#heading" alone): a
// same-file anchor jump, not a cross-file link.
func ParseWikilink(inner string) (Wikilink, bool) {
	link := Wikilink{Display: strings.TrimSpace(inner)}

	beforePipe := inner
	if before, after, found := strings.Cut(inner, "|"); found {
		beforePipe = strings.TrimRight(before, `\`)
		link.Display = strings.TrimSpace(after)
	}
	beforeFragment, fragment, hasFragment := strings.Cut(beforePipe, "#")
	if hasFragment {
		if block, isBlock := strings.CutPrefix(strings.TrimSpace(fragment), "^"); isBlock {
			link.Block = block
		} else {
			link.Heading = strings.TrimSpace(fragment)
		}
	}
	beforeBlock, block, hasBlock := strings.Cut(beforeFragment, "^")
	if hasBlock && link.Block == "" {
		link.Block = strings.TrimSpace(block)
	}
	link.Target = strings.TrimSpace(beforeBlock)

	return link, link.Target != ""
}

// SplitWikilink is ParseWikilink for the callers that only ever needed the
// resolution target and the words to show for it — a map row, a study path
// row, a note's searchable text. They resolve a name and print a label; where
// inside the file the link pointed is not part of either job.
func SplitWikilink(inner string) (target, display string, ok bool) {
	link, ok := ParseWikilink(inner)
	return link.Target, link.Display, ok
}
