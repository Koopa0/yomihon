package graph

import (
	"cmp"
	"iter"
	"slices"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// CodeKind distinguishes inline quotation from fenced and indented blocks.
type CodeKind uint8

const (
	// CodeInline is a paired backtick span, including its delimiters.
	CodeInline CodeKind = iota
	// CodeFence is a recognized fenced block, including its opening line.
	CodeFence
	// CodeIndent is an indented block recognized in its Markdown container.
	CodeIndent
)

// String names a code region for a diagnostic or a log line.
func (k CodeKind) String() string {
	switch k {
	case CodeInline:
		return "inline"
	case CodeFence:
		return "fence"
	case CodeIndent:
		return "indent"
	default:
		panic("graph: unknown CodeKind: " + strconv.Itoa(int(k)))
	}
}

// CodeFact identifies authored code in original byte coordinates. Fence spans
// include the opener, info and closer, even when there are no content lines.
// A fence ending with its Markdown container has no closer and is not at EOF.
type CodeFact struct {
	Kind      CodeKind
	Span      Span
	Opener    Span
	Info      Span
	Closer    Span
	EndOfBody bool
}

// FootnoteFact retains a definition before goldmark relocates or removes it.
// Emitted distinguishes a referenced definition from source the page omits.
type FootnoteFact struct {
	Span    Span
	Label   string
	Emitted bool
}

// InlineFootnoteFact is one accepted authored token before expansion.
// Content retains the original bytes between its delimiters, including authored
// indentation and comments. Private admission segments drive presentation.
type InlineFootnoteFact struct {
	Span    Span
	Content string
}

// BodyHeading is a parsed heading's original source and its unquoted prose.
type BodyHeading struct {
	Span  Span
	Level int
	Text  string
}

// BodyDestination is a Markdown link or image in its authored source position.
type BodyDestination struct {
	Offset int
	Target string
	Image  bool
}

// CodeLiteral is the displayed text of an inline code span, with its complete
// authored delimiters retained beside it for original-source attribution.
type CodeLiteral struct {
	Span Span
	Text string
}

// CommentLimit is the end of an HTML comment's parsed Markdown container.
type CommentLimit struct{ Open, Stop int }

// BodyFacts owns one immutable original body and its recognition results.
// Iterators yield values; no tree, parser context, map or slice escapes.
type BodyFacts struct {
	data *bodyFactsData
}

type bodyFactsData struct {
	source       string
	codes        []CodeFact
	comments     []Span
	footnotes    []FootnoteFact
	headings     []BodyHeading
	destinations []BodyDestination
	literals     []CodeLiteral
	htmlLimits   []CommentLimit
	inlineNotes  []InlineFootnoteFact
	commentFree  string
	comment      BodyComment
	structure    bodyStructure
	richHeadings []RichBodyHeading
	richOrigins  [][]BodyOrigin
}

var bodyExpandedMarkdown = NewBodyMarkdown(nil)
var bodyAuthoredMarkdown = newBodyMarkdown(nil, bodyAuthored)

// ReadBody retains original bytes while recognizing authored protection,
// comment-surviving admission when needed, and final expanded authority when
// needed. Inline protection fragments and exceptional definition-placement
// probes use the same grammar separately. Observation state stays private.
func ReadBody(body string) BodyFacts {
	original := originalBody(body)
	bootstrap := observeBodyRequested(body, bodyAuthoredMarkdown.Parser(), bodyCollection{rows: true, rich: true})
	protectBodyInline(original, bootstrap)
	admissionBody, comments, unclosed := stripBodyProjection(original, bootstrap, nil)
	admission := bootstrap
	if admissionBody.text != body {
		admission = observeBodyRequested(admissionBody.text, bodyAuthoredMarkdown.Parser(), bodyCollection{rich: true})
	}
	expanded := expandBodyInline(admissionBody, admission)
	authority := admission
	expandedAuthority := false
	if expanded.text != admissionBody.text || len(admission.inlineNotes) > 0 {
		// Even an unexpanded opaque token in an unused definition is literal
		// under the page's final grammar, and can contain an ordinary reference.
		authority = observeBodyRequested(expanded.text, bodyExpandedMarkdown.Parser(), bodyCollection{
			rows: expanded.text == body, rich: expanded.text == admissionBody.text,
		})
		expandedAuthority = true
	}
	facts := projectedBodyFacts(&bodyReading{
		source: body, admissionBody: admissionBody, expanded: expanded,
		bootstrap: bootstrap, admission: admission, authority: authority,
		comments: comments, unclosed: unclosed,
	})
	// Authored recognition is transferable only without opaque inline tokens.
	// Matching bytes alone do not make the two grammars interchangeable.
	rows := bootstrap
	if len(bootstrap.inlineNotes) > 0 {
		if expandedAuthority && expanded.text == body {
			rows = authority
		} else {
			rows = observeBodyRequested(body, bodyExpandedMarkdown.Parser(), bodyCollection{rows: true, rich: body == admissionBody.text})
		}
	}
	rich := admission
	if len(admission.inlineNotes) > 0 {
		switch {
		case expandedAuthority && expanded.text == admissionBody.text:
			rich = authority
		case body == admissionBody.text && rows.collection.rich:
			rich = rows
		default:
			rich = observeBodyRequested(admissionBody.text, bodyExpandedMarkdown.Parser(), bodyCollection{rich: true})
		}
	}
	facts.structure = rows.structure
	facts.attachRichHeadings(admissionBody, rich.rich)
	return BodyFacts{data: facts}
}

func observeBody(body string, grammar parser.Parser) *bodyObservation {
	return observeBodyRequested(body, grammar, bodyCollection{})
}

func observeBodyRequested(body string, grammar parser.Parser, collection bodyCollection) *bodyObservation {
	observation := &bodyObservation{
		collection: collection,
		blocks:     make(map[ast.Node]int), definitions: make(map[*east.Footnote]int),
		inlineSegments: make(map[int][]text.Segment),
	}
	context := parser.NewContext()
	context.Set(bodyObservationKey, observation)
	grammar.Parse(text.NewReader([]byte(body)), parser.WithContext(context))
	for node, index := range observation.definitions {
		observation.footnotes[index].Emitted = node.Index >= 0
	}
	slices.SortFunc(observation.codes, func(a, b CodeFact) int { return cmp.Compare(a.Span.Start, b.Span.Start) })
	slices.SortFunc(observation.footnotes, func(a, b FootnoteFact) int { return cmp.Compare(a.Span.Start, b.Span.Start) })
	slices.SortFunc(observation.headings, func(a, b BodyHeading) int { return cmp.Compare(a.Span.Start, b.Span.Start) })
	slices.SortFunc(observation.destinations, func(a, b BodyDestination) int { return cmp.Compare(a.Offset, b.Offset) })
	slices.SortFunc(observation.literals, func(a, b CodeLiteral) int { return cmp.Compare(a.Span.Start, b.Span.Start) })
	slices.SortFunc(observation.htmlLimits, func(a, b CommentLimit) int { return cmp.Compare(a.Open, b.Open) })
	slices.SortFunc(observation.inlineNotes, func(a, b InlineFootnoteFact) int { return cmp.Compare(a.Span.Start, b.Span.Start) })
	return observation
}

// Source returns the unchanged original bytes as an immutable string.
func (f BodyFacts) Source() string {
	if f.data == nil {
		return ""
	}
	return f.data.source
}

// CodeProtection recognizes a bounded presentation fragment under the authored
// grammar. It does not strip or expand that fragment before protecting markup.
func CodeProtection(body string) iter.Seq[CodeFact] {
	observation := observeBody(body, bodyAuthoredMarkdown.Parser())
	protectBodyInline(originalBody(body), observation)
	return bodyValues(observation.codes)
}

// PresentationCodes recognizes an already transformed presentation source.
// It neither strips comments again nor admits another inline expansion.
func PresentationCodes(body string) iter.Seq[CodeFact] {
	return bodyValues(observeBody(body, bodyExpandedMarkdown.Parser()).codes)
}

// CommentFree returns the one comment strip's immutable presentation source.
func (f BodyFacts) CommentFree() string {
	if f.data == nil {
		return ""
	}
	return f.data.commentFree
}

// UnclosedComment returns the original line of the strip's unpaired delimiter.
func (f BodyFacts) UnclosedComment() BodyComment {
	if f.data == nil {
		return BodyComment{}
	}
	return f.data.comment
}

// CodeAt reports whether an original byte belongs to recognized code.
func (f BodyFacts) CodeAt(offset int) bool {
	if f.data == nil {
		return false
	}

	for _, code := range f.data.codes {
		if code.Span.Contains(offset) {
			return true
		}
	}
	return false
}

// CommentAt reports whether an original byte belongs to a hidden comment.
func (f BodyFacts) CommentAt(offset int) bool {
	if f.data == nil {
		return false
	}
	return In(f.data.comments, offset)
}

// EmittedAt distinguishes hidden comments and unused footnote definitions
// from original text that participates in the page's recognized body.
func (f BodyFacts) EmittedAt(offset int) bool {
	if f.data == nil {
		return false
	}

	if offset < 0 || offset >= len(f.data.source) || f.CommentAt(offset) {
		return false
	}
	for _, definition := range f.data.footnotes {
		if !definition.Emitted && definition.Span.Contains(offset) {
			return false
		}
	}
	return true
}

func bodyValues[T any](values []T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, value := range values {
			if !yield(value) {
				return
			}
		}
	}
}

// Codes yields code regions in original source order.
func (f BodyFacts) Codes() iter.Seq[CodeFact] {
	if f.data == nil {
		return bodyValues[CodeFact](nil)
	}
	return bodyValues(f.data.codes)
}

// Comments yields hidden comment regions in original source order.
func (f BodyFacts) Comments() iter.Seq[Span] {
	if f.data == nil {
		return bodyValues[Span](nil)
	}
	return bodyValues(f.data.comments)
}

// Footnotes yields every accepted definition, including unreferenced ones.
func (f BodyFacts) Footnotes() iter.Seq[FootnoteFact] {
	if f.data == nil {
		return bodyValues[FootnoteFact](nil)
	}
	return bodyValues(f.data.footnotes)
}

// InlineFootnotes yields original inline notes in source order.
func (f BodyFacts) InlineFootnotes() iter.Seq[InlineFootnoteFact] {
	if f.data == nil {
		return bodyValues[InlineFootnoteFact](nil)
	}
	return bodyValues(f.data.inlineNotes)
}

// Headings yields recognized original headings, before visibility filtering.
func (f BodyFacts) Headings() iter.Seq[BodyHeading] {
	if f.data == nil {
		return bodyValues[BodyHeading](nil)
	}
	return bodyValues(f.data.headings)
}

// Destinations yields recognized original Markdown link and image targets.
func (f BodyFacts) Destinations() iter.Seq[BodyDestination] {
	if f.data == nil {
		return bodyValues[BodyDestination](nil)
	}
	return bodyValues(f.data.destinations)
}

// CodeLiterals yields displayed inline code words with source provenance.
func (f BodyFacts) CodeLiterals() iter.Seq[CodeLiteral] {
	if f.data == nil {
		return bodyValues[CodeLiteral](nil)
	}
	return bodyValues(f.data.literals)
}

// HTMLCommentLimits yields container ends without another recognition parse.
func (f BodyFacts) HTMLCommentLimits() iter.Seq[CommentLimit] {
	if f.data == nil {
		return bodyValues[CommentLimit](nil)
	}
	return bodyValues(f.data.htmlLimits)
}

type bodySourceCollector struct{}

func (bodySourceCollector) Transform(doc *ast.Document, reader text.Reader, context parser.Context) {
	observation := bodyObservationIn(context)
	if observation == nil {
		return
	}
	slices.SortFunc(observation.inlineNotes, func(a, b InlineFootnoteFact) int { return cmp.Compare(a.Span.Start, b.Span.Start) })
	source := reader.Source()
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:errcheck // the visitor never returns an error
		if !entering {
			return ast.WalkContinue, nil
		}
		observation.collectSourceNode(node, source)
		return ast.WalkContinue, nil
	})
}

func bodyProse(node ast.Node, source []byte, notes []InlineFootnoteFact) string {
	var out strings.Builder
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		switch n := child.(type) {
		case *ast.Text:
			start := n.Segment.Start
			for _, note := range notes {
				if note.Span.Stop <= start || note.Span.Start >= n.Segment.Stop {
					continue
				}
				if note.Span.Start > start {
					out.Write(source[start:note.Span.Start])
				}
				start = min(note.Span.Stop, n.Segment.Stop)
			}
			out.Write(source[start:n.Segment.Stop])
		case *ast.CodeSpan:
		default:
			out.WriteString(bodyProse(child, source, notes))
		}
	}
	return out.String()
}

func (o *bodyObservation) collectSourceNode(node ast.Node, source []byte) {
	switch n := node.(type) {
	case *ast.Heading:
		if span, present := bodyLinesRange(n); present {
			o.headings = append(o.headings, BodyHeading{Span: span, Level: n.Level, Text: bodyProse(n, source, o.inlineNotes)})
		}
	case *ast.Link:
		o.destinations = append(o.destinations, BodyDestination{Offset: n.Pos(), Target: string(n.Destination)})
	case *ast.Image:
		o.destinations = append(o.destinations, BodyDestination{Offset: n.Pos(), Target: string(n.Destination), Image: true})
	case *ast.CodeSpan:
		o.collectCodeLiteral(n, source)
	case *ast.HTMLBlock:
		o.collectHTMLLimit(n, source)
	}
}

func (o *bodyObservation) collectCodeLiteral(n *ast.CodeSpan, source []byte) {
	var words strings.Builder
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if part, ok := child.(*ast.Text); ok {
			words.Write(part.Segment.Value(source))
		}
	}
	for _, code := range o.codes {
		if code.Kind == CodeInline && code.Span.Start == n.Pos() {
			o.literals = append(o.literals, CodeLiteral{Span: code.Span, Text: strings.ReplaceAll(words.String(), "\n", " ")})
			return
		}
	}
}

func (o *bodyObservation) collectHTMLLimit(n *ast.HTMLBlock, source []byte) {
	if n.Lines().Len() == 0 {
		return
	}
	first := n.Lines().At(0)
	line := string(source[first.Start:first.Stop])
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(trimmed, "<!--") {
		return
	}
	stop := n.Lines().At(n.Lines().Len() - 1).Stop
	if n.HasClosure() {
		stop = n.ClosureLine.Stop
	}
	o.htmlLimits = append(o.htmlLimits, CommentLimit{Open: first.Start + len(line) - len(trimmed), Stop: stop})
}
