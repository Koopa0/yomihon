package graph

import (
	"regexp"
	"strings"

	"github.com/koopa0/yomihon/internal/vault"
)

// FoldFragment folds the half of a link written after its "#" the way both
// kinds of fragment fold: Unicode form and letter case, and nothing else. Those
// are the two ways a name differs for reasons its author never chose; every
// other difference they did choose, and it is kept. A block address folds
// through here too, which is what keeps "^quote-1" and "^quote1" two names.
func FoldFragment(s string) string {
	return strings.ToLower(vault.NormalizeNFC(s))
}

// sectionIDDrop matches every run of characters a section id drops: anything
// that is neither a Unicode letter nor a digit collapses to a single hyphen.
var sectionIDDrop = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// SectionID is the id a page stamps for a heading of this name, and therefore
// the fragment a link has to carry to reach it: fold, keep letters and digits,
// collapse every other run to one hyphen, trim the ends, and fall back to
// "section" when nothing is left. Keeping every Unicode letter is what lets a
// CJK heading produce a usable id, and folding first is what keeps か+◌゙ん and
// がん one id rather than two, since a combining mark left alone is not a letter
// and would become a hyphen.
//
// Every face that stamps an id, follows one, or asks whether a note answers one
// reads it from here, so a link and the heading it names cannot drift apart.
func SectionID(name string) string {
	id := strings.Trim(sectionIDDrop.ReplaceAllString(FoldFragment(name), "-"), "-")
	if id == "" {
		return "section"
	}
	return id
}

// Wikilink is one wikilink's inner text — the characters between its enclosing
// "[[" and "]]" — split into what resolution reads, what a reader sees, and
// the two fragments. The fragments are the raw authored text; nothing here
// checks that either addresses a place the target file actually has.
type Wikilink struct {
	// Target is the name to resolve, fragments and display text removed.
	// Empty means the link addresses a place in the current file.
	Target string
	// Display is the text a reader sees: the words after the display
	// separator, or the whole inner text when the author wrote none.
	Display string
	// Heading is the section name written after "#", empty when absent.
	Heading string
	// Block is the block name written after "^", empty when absent. Obsidian
	// writes a block address as "#^name", so a fragment opening with "^" is
	// one of these and not a section named "^name".
	Block string
}

// EscapedWikilinkAt reports whether the wikilink whose "[[" begins at open is
// written to be shown rather than followed: the CommonMark backslash escape,
// an odd-length run of '\' in front of it, counted from in front of an embed's
// '!' when the author wrote one. A shown name is not a cited name, so every
// reader of this vault answers this question the same way.
func EscapedWikilinkAt(text string, open int) bool {
	if open > 0 && text[open-1] == '!' {
		open--
	}
	n := 0
	for i := open - 1; i >= 0 && text[i] == '\\'; i-- {
		n++
	}
	return n%2 == 1
}

// ParseWikilink splits inner into its target, display text, and fragments,
// stripping the markers in a fixed order: '|', then '#', then '^'. An escaped
// pipe, which a GFM table cell writes as "\|", splits the same as a bare one
// and yields the same target, so the escape never changes what resolves.
//
// ok is false when the target strips to empty ("#heading" alone): a same-file
// anchor jump, not a cross-file link.
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

// SplitWikilink is ParseWikilink for a caller that resolves a name and prints
// a label, and has no use for where inside the file the link pointed.
func SplitWikilink(inner string) (target, display string, ok bool) {
	link, ok := ParseWikilink(inner)
	return link.Target, link.Display, ok
}
