package render

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"

	"github.com/koopa0/yomihon/internal/graph"
)

// inlineFootnote records a source span without rendering it. Goldmark decides
// where inline syntax is legal, so code, escapes and HTML attributes do not
// become notes. The second parse uses ordinary footnotes for numbering and ids.
type inlineFootnote struct {
	span graph.Span
}

// inlineFootnoteLabel opens every label the expansion writes. A reference's
// label is what tells a later pass which footnotes an author wrote inline.
const inlineFootnoteLabel = graph.InlineFootnoteLabel

// expandInlineFootnotes translates Obsidian's inline form into ordinary
// references and definitions. The definitions follow the note, where they
// cannot change how any line the author wrote is read: written in front, an
// indented block at the start of the note became part of the last definition.
// Their position does not number them: goldmark numbers references in the
// document's reading order.
func expandInlineFootnotes(body string) string {
	return graph.ExpandInlineFootnotes(body)
}

// inlineFootnotesIn is every inline footnote goldmark finds in body, in source
// order. Goldmark decides where inline syntax is legal, so one written in code,
// escaped, or inside a raw attribute is not among them.
func inlineFootnotesIn(body string) []*inlineFootnote {
	if !strings.Contains(body, "^[") {
		return nil
	}
	var notes []*inlineFootnote
	for note := range graph.AuthoredInlineNotes(body) {
		notes = append(notes, &inlineFootnote{span: note.Span})
	}
	return notes
}

// headingNoteClass marks an inline footnote's reference written in a heading.
// The number is where the note was cited, not part of what the section is
// called, so the heading pass leaves it out of the id and the contents entry.
const headingNoteClass = "y-heading-note"

// headingNote matches one marked reference with its contents. The element is
// renderer-owned and never nests another, so the shortest close is its own.
var headingNote = regexp.MustCompile(`(?s)<span class="` + headingNoteClass + `">.*?</span>`)

// markHeadingNotes wraps every inline footnote reference inside a heading in
// the element the heading pass recognises. Only the inline form is marked: its
// source words are what HeadingWords takes off, so the page and the check face
// both call `## Heading^[note]` "Heading".
func markHeadingNotes(doc ast.Node) {
	inline := map[int]bool{}
	for n := doc.LastChild(); n != nil; n = n.PreviousSibling() {
		list, ok := n.(*east.FootnoteList)
		if !ok {
			continue
		}
		for f := list.FirstChild(); f != nil; f = f.NextSibling() {
			if note, ok := f.(*east.Footnote); ok && strings.HasPrefix(string(note.Ref), inlineFootnoteLabel) {
				inline[note.Index] = true
			}
		}
	}
	if len(inline) == 0 {
		return
	}
	var refs []*east.FootnoteLink
	//nolint:errcheck // the visitor never returns an error, so the walk cannot fail
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if _, ok := n.(*ast.Heading); ok {
			_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
				if ref, ok := c.(*east.FootnoteLink); ok && entering && inline[ref.Index] {
					refs = append(refs, ref)
				}
				return ast.WalkContinue, nil
			})
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	for _, ref := range refs {
		parent := ref.Parent()
		parent.InsertBefore(parent, ref, rawInline(`<span class="`+headingNoteClass+`">`))
		parent.InsertAfter(parent, ref, rawInline(`</span>`))
	}
}

// rawInline is renderer-owned markup written as it stands.
func rawInline(markup string) *ast.String {
	s := ast.NewString([]byte(markup))
	s.SetCode(true)
	return s
}

// withoutInlineFootnotes is source with every inline footnote taken out, as the
// heading pass leaves a heading's marked references out of its name.
func withoutInlineFootnotes(source string) string {
	notes := inlineFootnotesIn(source)
	if len(notes) == 0 {
		return source
	}
	var out strings.Builder
	start := 0
	for _, note := range notes {
		out.WriteString(source[start:note.span.Start])
		start = note.span.Stop
	}
	out.WriteString(source[start:])
	return out.String()
}
