package render

import (
	"cmp"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// inlineFootnote records a source span without rendering it. Goldmark decides
// where inline syntax is legal, so code, escapes and HTML attributes do not
// become notes. The second parse uses ordinary footnotes for numbering and ids.
type inlineFootnote struct {
	ast.BaseInline

	span    text.Segment
	content *text.Segments
}

var kindInlineFootnote = ast.NewNodeKind("InlineFootnote")

func (*inlineFootnote) Kind() ast.NodeKind { return kindInlineFootnote }

func (n *inlineFootnote) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

type inlineFootnoteParser struct{}

func (inlineFootnoteParser) Trigger() []byte { return []byte{'^'} }

func (inlineFootnoteParser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, start := block.PeekLine()
	if !strings.HasPrefix(string(line), "^[") {
		return nil
	}
	block.Advance(2)
	content, found := block.FindClosure('[', ']', text.FindClosureOptions{
		CodeSpan: true, Nesting: true, Newline: true, Advance: true,
	})
	if !found || util.IsBlank(content.Value(block.Source())) {
		return nil
	}
	_, end := block.Position()
	return &inlineFootnote{span: text.NewSegment(start.Start, end.Start), content: content}
}

var inlineFootnoteRecognizer = goldmark.New(
	goldmark.WithExtensions(extension.GFM, extension.Footnote),
	goldmark.WithParserOptions(parser.WithInlineParsers(util.Prioritized(inlineFootnoteParser{}, 100))),
).Parser()

// inlineFootnoteLabel opens every label the expansion writes. A reference's
// label is what tells a later pass which footnotes an author wrote inline.
const inlineFootnoteLabel = "yomihon-inline-footnote-"

// expandInlineFootnotes translates Obsidian's inline form into ordinary
// references and definitions. The definitions follow the note, where they
// cannot change how any line the author wrote is read: written in front, an
// indented block at the start of the note became part of the last definition.
// Their position does not number them: goldmark numbers references in the
// document's reading order.
func expandInlineFootnotes(body string) string {
	notes := inlineFootnotesIn(body)
	if len(notes) == 0 {
		return body
	}
	source := []byte(body)
	// The internal labels must not borrow an authored definition, including
	// one the recognizer discarded because it had no reference yet.
	var definitions, prose strings.Builder
	start := 0
	serial := 1
	for _, note := range notes {
		label := "[^" + inlineFootnoteLabel + strconv.Itoa(serial) + "]"
		for strings.Contains(body, label) {
			serial++
			label = "[^" + inlineFootnoteLabel + strconv.Itoa(serial) + "]"
		}
		serial++
		definitions.WriteString(label + ": ")
		definitions.WriteString(strings.ReplaceAll(string(note.content.Value(source)), "\n", "\n    "))
		definitions.WriteString("\n\n")
		prose.WriteString(body[start:note.span.Start])
		prose.WriteString(label)
		start = note.span.Stop
	}
	prose.WriteString(body[start:])
	return placeDefinitions(prose.String(), definitions.String(), len(notes))
}

// inlineFootnotesIn is every inline footnote goldmark finds in body, in source
// order. Goldmark decides where inline syntax is legal, so one written in code,
// escaped, or inside a raw attribute is not among them.
func inlineFootnotesIn(body string) []*inlineFootnote {
	if !strings.Contains(body, "^[") {
		return nil
	}
	doc := inlineFootnoteRecognizer.Parse(text.NewReader([]byte(body)))
	var notes []*inlineFootnote
	if err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if note, ok := n.(*inlineFootnote); ok && entering {
			notes = append(notes, note)
		}
		return ast.WalkContinue, nil
	}); err != nil {
		// The walk has no error-producing branch; preserving the source is
		// the safe fallback if the dependency ever introduces one.
		return nil
	}
	slices.SortFunc(notes, func(a, b *inlineFootnote) int { return cmp.Compare(a.span.Start, b.span.Start) })
	return notes
}

// placeDefinitions writes the generated definitions on the line after the
// note. No blank line comes between: a code block still open inside a list item
// would keep it as an empty line of code, while a definition at the margin
// already ends every container and paragraph above it. A note can end inside a
// block that only a blank line or a closing marker ends — a fenced code block or
// a raw HTML block — which holds everything after it as its own text; the
// definitions then stand just before that block, between two blocks the author
// wrote, where they end at its first line and leave it to be read as before.
func placeDefinitions(prose, definitions string, count int) string {
	after := prose
	if !strings.HasSuffix(after, "\n") {
		after += "\n"
	}
	after += definitions
	if generatedDefinitions(after) == count {
		return after
	}
	doc := inlineFootnoteRecognizer.Parse(text.NewReader([]byte(prose)))
	var open ast.Node
	for n := doc.LastChild(); n != nil; n = n.PreviousSibling() {
		if _, list := n.(*east.FootnoteList); !list {
			open = n
			break
		}
	}
	if open == nil || open.Pos() < 0 {
		return after
	}
	lineStart := strings.LastIndexByte(prose[:open.Pos()], '\n') + 1
	return prose[:lineStart] + definitions + prose[lineStart:]
}

// generatedDefinitions counts the expansion's own definitions that body's parse
// keeps as footnotes.
func generatedDefinitions(body string) int {
	doc := inlineFootnoteRecognizer.Parse(text.NewReader([]byte(body)))
	found := 0
	for n := doc.LastChild(); n != nil; n = n.PreviousSibling() {
		list, ok := n.(*east.FootnoteList)
		if !ok {
			continue
		}
		for f := list.FirstChild(); f != nil; f = f.NextSibling() {
			if note, ok := f.(*east.Footnote); ok && strings.HasPrefix(string(note.Ref), inlineFootnoteLabel) {
				found++
			}
		}
	}
	return found
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
