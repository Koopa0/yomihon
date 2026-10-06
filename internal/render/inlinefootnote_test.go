package render_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestInlineFootnotesJoinTheExistingNumbering(t *testing.T) {
	t.Parallel()
	body := "Before^[行內註腳。] middle[^ordinary] after^[Second **inline** note.].\n\n[^ordinary]: Ordinary definition.\n"
	got := newRenderer(t, nil, nil, nil).HTMLIn("sheet-", "Notes/Footnotes.md", "", body, wording.ZhHant)
	for _, want := range []string{
		`Before<sup id="sheet-fnref:1"><a href="#sheet-fn:1"`,
		`middle<sup id="sheet-fnref:2"><a href="#sheet-fn:2"`,
		`after<sup id="sheet-fnref:3"><a href="#sheet-fn:3"`,
		`<li id="sheet-fn:1">`, `行內註腳。&#160;<a href="#sheet-fnref:1"`,
		`<li id="sheet-fn:2">`, `Ordinary definition.&#160;<a href="#sheet-fnref:2"`,
		`<li id="sheet-fn:3">`, `Second <strong>inline</strong> note.&#160;<a href="#sheet-fnref:3"`,
	} {
		if !strings.Contains(got.HTML, want) {
			t.Errorf("HTMLIn(inline footnotes) missing %q:\n%s", want, got.HTML)
		}
	}
	if strings.Contains(got.HTML, "^[") || strings.Count(got.HTML, `class="footnotes"`) != 1 || len(got.Diagnostics) != 0 {
		t.Errorf("HTMLIn(inline footnotes) = %+v, want one complete footnote list", got)
	}
	if plain := render.PlainText(body); plain != "Before middle after.\n行內註腳。\nOrdinary definition.\nSecond inline note." {
		t.Errorf("PlainText(inline footnotes) = %q, want prose then all three definitions", plain)
	}
}

func TestInlineFootnotesProtectLiteralAndMalformedSource(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, body, literal string }{
		{"code span", "`^[literal]`", "^[literal]"},
		{"fenced code", "```text\n^[literal]\n```", "^[literal]"},
		{"indented code", "    ^[literal]", "^[literal]"},
		{"escaped caret", `\^[literal]`, "^[literal]"},
		{"unclosed", "before^[not closed", "before^[not closed"},
		{"empty", "before^[]", "before^[]"},
		{"raw attribute", `<span title="^[literal]">body</span>`, "^[literal]"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := newRenderer(t, nil, nil, nil).HTML("Notes/F.md", "", tt.body, wording.En)
			if !strings.Contains(got.HTML, tt.literal) || strings.Contains(got.HTML, `class="footnote-ref"`) {
				t.Errorf("HTML(%q) = %q, want literal %q without a reference", tt.body, got.HTML, tt.literal)
			}
		})
	}
}

func TestInlineFootnoteKeepsAWrappedDefinitionTogether(t *testing.T) {
	t.Parallel()
	body := "A^[wrapped\nwords].\n"
	got := newRenderer(t, nil, nil, nil).HTML("Notes/F.md", "", body, wording.En).HTML
	if !strings.Contains(got, "<p>wrapped\nwords&#160;") || !strings.Contains(got, `<li id="fn:1">`) || strings.Contains(got, "^[") {
		t.Errorf("HTML(wrapped inline note) = %q, want one continued definition", got)
	}
}

func TestInlineFootnotesPreserveNestedFormattingAndCodeBrackets(t *testing.T) {
	t.Parallel()
	body := "A^[**bold** [label](#place), `]`, and escaped \\] text.] B^[second].\n\n## place\n"
	got := newRenderer(t, nil, nil, nil).HTML("Notes/F.md", "", body, wording.En)
	for _, want := range []string{`<li id="fn:1">`, `<strong>bold</strong>`, `<a href="#place">label</a>`, `<code>]</code>`, "and escaped ] text.", `<li id="fn:2">`, "second&#160;"} {
		if !strings.Contains(got.HTML, want) {
			t.Errorf("HTML(nested footnote) missing %q:\n%s", want, got.HTML)
		}
	}
}

func TestInlineFootnotesCannotBorrowAnAuthoredDefinition(t *testing.T) {
	t.Parallel()
	body := "A^[inline] B[^yomihon-inline-footnote-1].\n\n[^yomihon-inline-footnote-1]: authored\n"
	got := render.PlainText(body)
	if diff := cmp.Diff("A B.\ninline\nauthored", got); diff != "" {
		t.Errorf("PlainText(collision) mismatch (-want +got):\n%s", diff)
	}
}

func TestInlineFootnoteBeforeAnUnclosedFenceStillHasItsDefinition(t *testing.T) {
	t.Parallel()
	body := "A^[visible].\n\n```text\n^[literal]\n"
	got := newRenderer(t, nil, nil, nil).HTML("Notes/F.md", "", body, wording.En)
	if !strings.Contains(got.HTML, `<li id="fn:1">`) || !strings.Contains(got.HTML, "visible&#160;") || !strings.Contains(got.HTML, "^[literal]") {
		t.Errorf("HTML(unclosed fence) = %q, want a working preceding note and literal code", got.HTML)
	}
}

func TestInlineFootnotesShareCalloutScopeAndKeepEmbedRegionsDistinct(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Embedded.md"}}, nil, transclusions{
		"Embedded.md": "Embedded^[embedded words].\n",
	})
	body := "Host[^h].\n\n> [!note] Aside\n> Callout^[callout words].\n\n![[Embedded]]\n\n![[Embedded]]\n\n[^h]: host words\n"
	got := r.HTML("Notes/Host.md", "", body, wording.En).HTML
	if strings.Count(got, `class="footnote-ref"`) != 4 || strings.Count(got, `class="footnotes"`) != 3 || !strings.Contains(got, `Callout<sup id="fnref:2">`) {
		t.Fatalf("HTML(composed inline notes) lost reference, scope or numbering:\n%s", got)
	}
	ids := allMatches(elementID, got)
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Errorf("HTML(composed inline notes) repeated id %q", id)
		}
		seen[id] = true
	}
	for _, target := range allMatches(fragmentHref, got) {
		if !seen[target] {
			t.Errorf("HTML(composed inline notes) has no target %q", target)
		}
	}
}

func TestInlineFootnotesPreserveSearchBlockProvenance(t *testing.T) {
	t.Parallel()
	body := "Ordinary lead needle.\n\nWith^[註腳詞] citation.\n\n```text\n^[code]\n```\n"
	plain, blocks, fences := render.PlainBlocks(body)
	if diff := cmp.Diff("Ordinary lead needle.\nWith citation.\n^[code]\n註腳詞", plain); diff != "" {
		t.Errorf("PlainBlocks() text mismatch (-want +got):\n%s", diff)
	}
	want := []render.Block{{End: 21, Verbatim: true}, {End: 36}, {End: 44}, {End: 54}}
	if diff := cmp.Diff(want, blocks); diff != "" {
		t.Errorf("PlainBlocks() blocks mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([][2]int{{37, 44}}, fences); diff != "" {
		t.Errorf("PlainBlocks() fence ranges mismatch (-want +got):\n%s", diff)
	}
}

// The generated definitions never change how an authored line is read: with
// the note's number and its list taken off, the page is the page the author
// would get without writing the note at all.
func TestInlineFootnoteDefinitionsLeaveAuthoredBlocksAlone(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	reference := regexp.MustCompile(`<sup id="fnref:\d+"><a href="#fn:\d+" class="footnote-ref" role="doc-noteref">\d+</a></sup>`)
	list := regexp.MustCompile(`(?s)<div class="footnotes" role="doc-endnotes">.*</div>\n?\z`)
	for _, tt := range []struct{ name, body string }{
		{"indented code at the start", "    code\n\nText^[note].\n"},
		{"indented code at the start, no blank line after", "    code\nText^[note].\n"},
		{"unclosed fence in a list item", "- item^[note]\n  ```\n  open\n"},
		{"unclosed fence at the margin", "Text^[note].\n\n```text\nopen\n"},
		{"unclosed raw block", "Text^[note].\n\n<pre>\nraw\n"},
		{"raw block with no blank line to end it", "Text^[note].\n<div>\nraw"},
		{"quote", "> quoted^[note]\n> more"},
		{"list", "- a^[note]\n- b"},
		{"table", "| a |\n|---|\n| b^[note] |"},
		{"authored definition last", "Text^[note][^a].\n\n[^a]: authored\n    continued"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Notes/F.md", "", tt.body, wording.En).HTML
			if !strings.Contains(got, `<li id="fn:1">`) || !strings.Contains(got, "<p>note&#160;") {
				t.Fatalf("HTML(%q) lost the inline note's definition:\n%s", tt.body, got)
			}
			without := r.HTML("Notes/F.md", "", strings.ReplaceAll(tt.body, "^[note]", ""), wording.En).HTML
			strip := func(s string) string {
				s = list.ReplaceAllString(reference.ReplaceAllString(s, ""), "")
				return strings.TrimRight(s, "\n")
			}
			if diff := cmp.Diff(strip(without), strip(got)); diff != "" {
				t.Errorf("HTML(%q) moved authored content (-without note +with note):\n%s", tt.body, diff)
			}
		})
	}
}

// A heading's inline note is cited there, not part of the section's name: the
// id, the contents entry and the name a link or the check face reads all stay
// what they were before the note was written.
func TestInlineFootnoteInAHeadingKeepsTheSectionName(t *testing.T) {
	t.Parallel()
	body := "## Heading^[note]\n\ntext\n"
	r := newRenderer(t, []graph.NoteInput{{RelPath: "N.md"}}, nil, transclusions{"N.md": body})
	got := r.HTML("N.md", "", body, wording.En)
	if !strings.Contains(got.HTML, `<h3 id="heading" data-level="2">Heading<span class="y-heading-note"><sup id="fnref:1">`) {
		t.Errorf("heading with an inline note:\n%s", got.HTML)
	}
	if diff := cmp.Diff([]render.TOCEntry{{Level: 2, Text: "Heading", ID: "heading"}}, got.TOC); diff != "" {
		t.Errorf("contents entry (-want +got):\n%s", diff)
	}
	if words := render.HeadingWords("Heading^[note]"); words != "Heading" {
		t.Errorf("HeadingWords(inline note) = %q, want %q", words, "Heading")
	}
	link := r.HTML("C.md", "", "[[N#Heading]] [[N#Heading note]]\n", wording.En)
	if !strings.Contains(link.HTML, `<a href="/notes/N.md#heading" class="wikilink">N#Heading</a>`) {
		t.Errorf("link to the section's name:\n%s", link.HTML)
	}
	if len(link.Diagnostics) != 1 || link.Diagnostics[0].Kind != render.DiagLinkSectionMissing || link.Diagnostics[0].Section != "Heading note" {
		t.Errorf("link diagnostics = %+v, want the note's words to name no section", link.Diagnostics)
	}
}

// A caret followed by a bracket opens an inline note whatever its text holds;
// it is never a block address.
func TestInlineFootnoteAtTheEndOfALineIsNotABlockAddress(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	for _, body := range []string{"Para ^[note]\n", "Para ^[a b]\n", "^[note]\n"} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Notes/F.md", "", body, wording.En)
			if !strings.Contains(got.HTML, `<sup id="fnref:1">`) || strings.Contains(got.HTML, `<span id="^`) || strings.Contains(got.HTML, "^[") {
				t.Errorf("HTML(%q) took the note as an address:\n%s", body, got.HTML)
			}
		})
	}
	if got := r.HTML("Notes/F.md", "", "Para ^[note] ^real\n", wording.En); !strings.Contains(got.HTML, `<span id="^real">`) || !strings.Contains(got.HTML, `<sup id="fnref:1">`) {
		t.Errorf("a real address after an inline note:\n%s", got.HTML)
	}
}
