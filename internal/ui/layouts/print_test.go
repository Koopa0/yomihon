package layouts

import (
	"maps"
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestPrintGivesThePaperItsMarginsAndKeepsBlocksWhole holds the half of the
// print stylesheet that is about paper rather than about ink. A printed lesson
// is a document with edges the screen does not have, and a block running
// across one comes back cut with no way to rejoin it: a code fence split down
// the middle, a table whose heading stayed on the sheet before, a heading
// stranded on a last line above a section that starts overleaf, a paragraph
// leaving one word behind. The colour reset beside these rules was already
// thorough — it names the whole palette so an evening in dark mode cannot
// print a black page — and it said nothing at all about geometry.
//
// The authored stylesheet is what this reads, not the built one: what is under
// review is what a person wrote, and that the build still agrees with it is a
// separate question the gate's own stylesheet comparison settles.
func TestPrintGivesThePaperItsMarginsAndKeepsBlocksWhole(t *testing.T) {
	t.Parallel()
	const path = "../../../assets/css/components.css"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	css := cssComments.ReplaceAllString(string(source), "")
	rules := rulesInside(t, atRuleBody(t, css, "@media print {"))

	// The colour reset was here before any of this and is not what is under
	// test; it is read as proof that the parse above found the print block
	// rather than three lines of some other file, because every check below
	// would otherwise report a missing rule for the wrong reason.
	if reset := rules[":root"]; reset["--bg"] != "#fff" {
		t.Fatalf("the parsed print block does not carry the palette reset (--bg = %q, want %q); this parse is reading the wrong text and nothing below it means anything", reset["--bg"], "#fff")
	}

	// An empty want means any value will do: what matters for the sheet's own
	// margins is that somebody chose them, not which ones.
	for _, want := range []struct {
		selector string
		property string
		value    string
		why      string
	}{
		{"@page", "margin", "", "text printed to the sheet's edge has no gutter to hold and nothing for a staple to take"},
		{".y-prose pre", "break-inside", "avoid", "a listing split across a page break loses the indentation the reader was following"},
		{".y-prose blockquote", "break-inside", "avoid", "a quotation split across sheets reads as two quotations"},
		{".y-prose .callout", "break-inside", "avoid", "a callout's coloured rule stops at the page edge and its body carries on unmarked"},
		{".y-prose .embed", "break-inside", "avoid", "an excerpt split across sheets loses the line that says which note it came from"},
		{".y-prose .mermaid-diagram", "break-inside", "avoid", "half a diagram is not a diagram"},
		{".y-prose table", "break-inside", "avoid", "a table split from its heading row is a grid of unlabelled numbers"},
		{".y-prose tr", "break-inside", "avoid", "a row split down the middle puts one cell of it on each sheet"},
		{".y-slotcard", "break-inside", "avoid", "a pattern card carries a sentence and its parts together or it carries nothing"},
		{".y-healthlist li", "break-inside", "avoid", "a finding and the note it is about belong on one sheet"},
		{".y-title", "break-after", "avoid", "a title alone at the foot of a sheet announces a page that is not there"},
		{".y-prose h1", "break-after", "avoid", "a heading alone at the foot of a sheet announces a section that is not there"},
		{".y-prose h2", "break-after", "avoid", "a heading alone at the foot of a sheet announces a section that is not there"},
		{".y-prose h3", "break-after", "avoid", "a heading alone at the foot of a sheet announces a section that is not there"},
		{".y-prose h4", "break-after", "avoid", "a heading alone at the foot of a sheet announces a section that is not there"},
		{".y-prose h1:not(:first-child)", "break-before", "page", "a note's own top-level heading is where one part of it ends and the next begins, and the reader can see that on paper only if the sheet ends there too"},
		{".y-prose p", "orphans", "2", "one line of a paragraph left at the foot of a sheet reads as a caption for whatever sits above it"},
		{".y-prose p", "widows", "2", "one line of a paragraph carried over reads as the opening of a new one"},
		{".y-prose li", "orphans", "2", "a list item broken after one line reads as two items"},
		{".y-prose li", "widows", "2", "a list item broken before its last line reads as two items"},
	} {
		declarations, written := rules[want.selector]
		if !written {
			t.Errorf("the print block says nothing about %s, so %s", want.selector, want.why)
			continue
		}
		got := declarations[want.property]
		switch {
		case want.value == "" && got == "":
			t.Errorf("%s declares no %s for print, so %s", want.selector, want.property, want.why)
		case want.value != "" && got != want.value:
			t.Errorf("%s declares %s: %q for print, want %q, because %s", want.selector, want.property, got, want.value, want.why)
		}
	}
}

// cssComments matches one CSS comment, including the newlines inside it. They
// are removed before any brace is counted, because a brace written in prose is
// not a rule boundary and this file's print block is heavily commented.
var cssComments = regexp.MustCompile(`(?s)/\*.*?\*/`)

// atRuleBody returns what the named at-rule encloses: the text between its
// opening brace and the brace that closes it. An at-rule nests where an
// ordinary rule body does not, so the end is found by counting rather than by
// taking the first close brace — a print block read only as far as its first
// nested rule would let every check over it hold against a handful of lines.
func atRuleBody(t *testing.T, css, opener string) string {
	t.Helper()
	at := strings.Index(css, opener)
	if at < 0 {
		t.Fatalf("the stylesheet has no %q", opener)
	}
	// Start on the opening brace itself, so the first character counted opens
	// depth 1 and the brace that returns to depth 0 is its partner.
	body := css[at+len(opener)-1:]
	depth := 0
	for i, r := range body {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return body[1:i]
			}
		}
	}
	t.Fatalf("%q is never closed", opener)
	return ""
}

// rulesInside splits an at-rule's body into the rules written in it, keyed by
// each selector separately: a rule naming several selectors is recorded under
// every one, so a check can name the single element it is about instead of
// restating whichever list that element currently shares. Where two rules name
// the same selector the later declarations join the earlier, which is what the
// cascade does with them.
func rulesInside(t *testing.T, body string) map[string]map[string]string {
	t.Helper()
	out := map[string]map[string]string{}
	rest := body
	for {
		open := strings.IndexByte(rest, '{')
		if open < 0 {
			break
		}
		end := strings.IndexByte(rest[open:], '}')
		if end < 0 {
			t.Fatalf("a rule inside the block is never closed, starting at %q", strings.TrimSpace(rest[:min(len(rest), 80)]))
		}
		declarations := cssDeclarations(t, rest[open+1:open+end])
		for selector := range strings.SplitSeq(strings.TrimSpace(rest[:open]), ",") {
			key := strings.Join(strings.Fields(selector), " ")
			if key == "" {
				continue
			}
			if out[key] == nil {
				out[key] = map[string]string{}
			}
			maps.Copy(out[key], declarations)
		}
		rest = rest[open+end+1:]
	}
	if len(out) == 0 {
		t.Fatal("the block holds no rules at all, so reading it proves nothing")
	}
	return out
}
