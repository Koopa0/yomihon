package render

import (
	"bytes"
	"html"
	"slices"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/sequence"
)

// Projection carries the searchable reading text and its display metadata.
// All offsets name bytes in Text, before excerpt selection or normalization.
type Projection struct {
	Text         string
	Blocks       []Block
	FenceRanges  [][2]int
	DisplaySpans []DisplaySpan

	// Insertions name conservative cuts where the page may add words absent
	// from Text. A missing local link can add an out-of-sight explanation; the
	// corpus cannot tell which local destinations exist.
	Insertions []int
}

// DisplaySpan describes a display effect on a half-open range of corpus bytes.
// Hidden wins over decoration. An empty Replacement keeps the original bytes.
type DisplaySpan struct {
	Start, End  int
	Hidden      bool
	Deleted     bool
	Replacement string
}

// PlainProjection returns the searchable corpus with note display annotations.
func PlainProjection(body string) Projection {
	var emissions []sourceEmission
	var insertions []int
	source, rewritten := plainPreprocess(body)
	plain, blocks, fences := plainSourceBlocks(body, source, &rewritten, &emissions, &insertions)
	result := Projection{Text: plain, Blocks: blocks, FenceRanges: fences, Insertions: insertions}
	if plain == "" {
		return result
	}
	effects := noteDisplayEffects([]byte(source), &rewritten)
	for _, hidden := range []bool{true, false} {
		intervals := displayIntervals(effects, hidden)
		result.DisplaySpans = append(result.DisplaySpans, emittedDisplaySpans(intervals, emissions, len(plain))...)
	}
	// Stripping markup can join separate source emissions into one output rune.
	// Coalesce the emitted flags too, keeping ranges whole across such joins.
	var merged []DisplaySpan
	for _, hidden := range []bool{true, false} {
		merged = append(merged, displayIntervals(result.DisplaySpans, hidden)...)
	}
	result.DisplaySpans = merged
	slices.SortFunc(result.DisplaySpans, func(a, b DisplaySpan) int {
		if a.Start != b.Start {
			return a.Start - b.Start
		}
		return a.End - b.End
	})
	return result
}

func emittedDisplaySpans(intervals []DisplaySpan, emissions []sourceEmission, size int) []DisplaySpan {
	var spans []DisplaySpan
	for _, e := range emissions {
		// Merged intervals are disjoint, so both their starts and ends are
		// ordered. Ruby readings can emit out of source order; each copied
		// segment therefore seeks independently to its first intersection.
		first, _ := slices.BinarySearchFunc(intervals, e.start, func(span DisplaySpan, start int) int {
			if span.End <= start {
				return -1
			}
			return 1
		})
		for _, interval := range intervals[first:] {
			if interval.Start >= e.end {
				break
			}
			span := interval
			// One decoded source unit is indivisible in the output coordinate
			// space: an escape or entity can occupy fewer or more bytes.
			span.Start = max(0, e.out)
			span.End = min(size, e.outEnd)
			if !e.atomic {
				span.Start = max(0, e.out+max(e.start, interval.Start)-e.start)
				span.End = min(size, e.out+min(e.end, interval.End)-e.start)
			}
			if span.Start < span.End {
				spans = append(spans, span)
			}
		}
	}
	return spans
}

// displayIntervals merges each independently cumulative display flag.
// These private effects carry only Hidden or Deleted, never replacements.
func displayIntervals(effects []DisplaySpan, hidden bool) []DisplaySpan {
	var intervals []DisplaySpan
	for _, effect := range effects {
		if hidden && effect.Hidden || !hidden && effect.Deleted {
			intervals = append(intervals, DisplaySpan{Start: effect.Start, End: effect.End, Hidden: hidden, Deleted: !hidden})
		}
	}
	slices.SortFunc(intervals, func(a, b DisplaySpan) int { return a.Start - b.Start })
	merged := intervals[:0]
	for _, interval := range intervals {
		if len(merged) > 0 && interval.Start <= merged[len(merged)-1].End {
			merged[len(merged)-1].End = max(merged[len(merged)-1].End, interval.End)
		} else {
			merged = append(merged, interval)
		}
	}
	return merged
}

// sourceEmission records a copied source segment at its actual write site.
// Held ruby readings use reading-builder offsets until flushReadings moves them.
type sourceEmission struct {
	start, end, out, outEnd int
	atomic                  bool
}

// proseText uses the same escape/entity writer as the reading page. The
// resulting HTML escaping is removed once to recover the browser's text.
func proseText(source []byte) []byte {
	var markup roleMarkup
	goldmarkhtml.DefaultWriter.Write(&markup, source)
	return []byte(html.UnescapeString(markup.String()))
}

func (w *plainWalk) writeProse(seg text.Segment, source []byte) {
	if seg.Padding > 0 {
		w.writeVisible(bytes.Repeat([]byte{' '}, seg.Padding))
	}
	for at := seg.Start; at < seg.Stop; {
		if source[at] != '&' && source[at] != '\\' && source[at] != 0 {
			end := seg.Stop
			if next := bytes.IndexAny(source[at:seg.Stop], "&\\\x00"); next >= 0 {
				end = at + next
			}
			w.emitSource(sourceEmission{start: at, end: end}, source[at:end])
			at = end
			continue
		}
		end := at + roleSourceUnitWidth(source[at:seg.Stop])
		value := source[at:end]
		if !w.rewritten.literalRoleAt(at) && !withinAny(w.rewritten.wikilinks, at, end) {
			value = proseText(value)
		}
		w.emitSource(sourceEmission{start: at, end: end, atomic: true}, value)
		at = end
	}
	if seg.ForceNewline && (seg.Stop == seg.Start || source[seg.Stop-1] != '\n') {
		w.writeBreak()
	}
}

func (w *plainWalk) writeSource(seg text.Segment, source []byte) {
	value := seg.Value(source)
	if seg.Padding > 0 {
		w.writeVisible(value[:seg.Padding])
		value = value[seg.Padding:]
	}
	w.emitSource(sourceEmission{start: seg.Start, end: seg.Stop}, value)
}

func (w *plainWalk) emitSource(e sourceEmission, value []byte) {
	if w.emissions != nil && rubyRoute(w.ruby) != rubyParen {
		e.out = w.b.Len()
		if rubyRoute(w.ruby) == rubyAnnotation {
			e.out = w.readings.Len()
			e.outEnd = e.out + len(value)
			w.readingEmissions = append(w.readingEmissions, e)
		} else {
			e.outEnd = e.out + len(value)
			*w.emissions = append(*w.emissions, e)
		}
	}
	w.writeVisible(value)
}

func (w *plainWalk) recordInsertion() {
	if w.insertions == nil {
		return
	}
	switch rubyRoute(w.ruby) {
	case rubyParen:
	case rubyAnnotation:
		w.readingInsertions = append(w.readingInsertions, w.readings.Len())
	default:
		*w.insertions = append(*w.insertions, w.b.Len())
	}
}

func (w *plainWalk) writeSourceBreak(after int, source []byte) {
	if w.emissions != nil {
		// Goldmark has already recognized the break and attached it to this text.
		// Its segment trims the trailing spaces/backslash; the source newline is
		// the one byte that the existing writeBreak emits in their place.
		if relative := bytes.IndexByte(source[after:], '\n'); relative >= 0 {
			at := after + relative
			e := sourceEmission{start: at, end: at + 1, out: w.b.Len(), outEnd: w.b.Len() + 1}
			switch rubyRoute(w.ruby) {
			case rubyParen:
			case rubyAnnotation:
				e.out = w.readings.Len()
				e.outEnd = e.out + 1
				w.readingEmissions = append(w.readingEmissions, e)
			default:
				*w.emissions = append(*w.emissions, e)
			}
		}
	}
	w.writeBreak()
}

func (w *plainWalk) writeCodeLines(n ast.Node, source []byte) {
	for i := range n.Lines().Len() {
		seg := n.Lines().At(i)
		if w.emissions != nil {
			*w.emissions = append(*w.emissions, sourceEmission{start: seg.Start, end: seg.Stop, out: w.b.Len() + seg.Padding, outEnd: w.b.Len() + len(seg.Value(source))})
		}
		w.b.Write(seg.Value(source))
	}
}

func (w *plainWalk) writeAutoLink(a *ast.AutoLink, source []byte) {
	label := a.Label(source)

	written := false
	for _, start := range []int{a.Pos(), a.Pos() + 1} {
		if start >= 0 && start+len(label) <= len(source) && bytes.Equal(source[start:start+len(label)], label) {
			w.writeSource(text.NewSegment(start, start+len(label)), source)
			written = true
			break
		}
	}
	if !written {
		w.writeVisible(label)
	}
	if a.AutoLinkType == ast.AutoLinkURL && leavesTheLibrary(a.URL(source)) {
		w.recordInsertion()
	}

}

// delimiterObservation delegates grammar to the existing inline parsers. The
// parser exposes consumed lengths only while OnMatch runs: afterwards Pos keeps
// just the opener anchor, and removed closing delimiters have left the tree.
type delimiterObservation struct {
	effects   []DisplaySpan
	lengths   map[*parser.Delimiter]int
	left      map[*parser.Delimiter]int
	rewritten *rewrittenLines
	corpus    map[ast.Node][2]text.Segment
}

// delimiterRemnant maps the parser's surviving prefix-shaped segment back to
// the actual remaining bytes after a closer consumed characters from the left.
type delimiterRemnant struct {
	span   text.Segment
	offset int
}

func (o *delimiterObservation) remnants() []delimiterRemnant {
	var remnants []delimiterRemnant
	for delimiter, offset := range o.left {
		if delimiter.Length > 0 {
			remnants = append(remnants, delimiterRemnant{span: delimiter.Segment, offset: offset})
		}
	}
	slices.SortFunc(remnants, func(a, b delimiterRemnant) int { return a.span.Start - b.span.Start })
	return remnants
}

type observedInlineParser struct {
	delegate    parser.InlineParser
	observation *delimiterObservation
}

func (p observedInlineParser) Trigger() []byte { return p.delegate.Trigger() }

func (p observedInlineParser) Parse(parent ast.Node, reader text.Reader, pc parser.Context) ast.Node {
	node := p.delegate.Parse(parent, reader, pc)
	if delimiter, ok := node.(*parser.Delimiter); ok {
		p.observation.lengths[delimiter] = delimiter.Length
		delimiter.Processor = observedDelimiterProcessor{delegate: delimiter.Processor, opener: delimiter, observation: p.observation}
	}
	return node
}

type observedDelimiterProcessor struct {
	delegate    parser.DelimiterProcessor
	opener      *parser.Delimiter
	observation *delimiterObservation
}

func (p observedDelimiterProcessor) IsDelimiter(char byte) bool { return p.delegate.IsDelimiter(char) }

func (p observedDelimiterProcessor) CanOpenCloser(opener, closer *parser.Delimiter) bool {
	return p.delegate.CanOpenCloser(opener, closer)
}

func (p observedDelimiterProcessor) OnMatch(consumes int) ast.Node {
	node := p.delegate.OnMatch(consumes)
	opener := p.opener
	previous, ok := p.observation.lengths[opener]
	if ok && previous-opener.Length == consumes {
		// Pinned ProcessDelimiters consumes only opener and closer immediately
		// before OnMatch. Every previous match updated those same two records,
		// so the first known changed downstream run is this match's closer.
		for closer := opener.NextDelimiter; closer != nil; closer = closer.NextDelimiter {
			before, known := p.observation.lengths[closer]
			if !known || before-closer.Length != consumes {
				continue
			}
			if p.observation.corpus != nil {
				openEnd := opener.Segment.Start + p.observation.left[opener] + previous
				closeStart := closer.Segment.Start + p.observation.left[closer]
				p.observation.corpus[node] = [2]text.Segment{
					text.NewSegment(openEnd-consumes, openEnd),
					text.NewSegment(closeStart, closeStart+consumes),
				}
			}
			p.observation.recordMatch(opener, closer, consumes)
			p.observation.left[closer] += consumes
			p.observation.lengths[opener] = opener.Length
			p.observation.lengths[closer] = closer.Length
			break
		}
	}
	return node
}

func (o *delimiterObservation) recordMatch(opener, closer *parser.Delimiter, consumes int) {
	openEnd := opener.Segment.Start + o.left[opener] + o.lengths[opener]
	openStart := openEnd - consumes
	closeStart := closer.Segment.Start + o.left[closer]
	closeEnd := closeStart + consumes
	if o.literalMatch([2]int{openStart, openEnd}, [2]int{closeStart, closeEnd}) {
		return
	}
	o.effects = append(o.effects, DisplaySpan{Start: openStart, End: openEnd, Hidden: true}, DisplaySpan{Start: closeStart, End: closeEnd, Hidden: true})
	if opener.Char == '~' {
		o.effects = append(o.effects, DisplaySpan{Start: openEnd, End: closeStart, Deleted: true})
	}
}

func (o *delimiterObservation) literalMatch(open, closeSpan [2]int) bool {
	if o.rewritten.literalRoleAt(open[0]) || o.rewritten.literalRoleAt(closeSpan[0]) {
		return true
	}
	// A delimiter manufactured inside a rewritten link label is literal on
	// the page. A real outer pair has both ends outside every replacement.
	for _, span := range o.rewritten.wikilinks {
		if open[0] < span[1] && open[1] > span[0] || closeSpan[0] < span[1] && closeSpan[1] > span[0] {
			return true
		}
	}
	return false
}

// SetParserOption wraps the already registered delimiters in place. Keeping
// their priorities avoids a competing unobserved parser consuming the pair.
func (o *delimiterObservation) SetParserOption(config *parser.Config) {
	for i, item := range config.InlineParsers {
		delegate, ok := item.Value.(parser.InlineParser)
		if ok && (delegate == extension.NewStrikethroughParser() || delegate == graph.NewHighlightParser()) {
			config.InlineParsers[i].Value = observedInlineParser{delegate: delegate, observation: o}
		}
	}
}

func noteDisplayEffects(source []byte, rewritten *rewrittenLines) []DisplaySpan {
	observation := delimiterObservation{lengths: make(map[*parser.Delimiter]int), left: make(map[*parser.Delimiter]int), rewritten: rewritten}
	md := pageMarkdown()
	md.Parser().AddOptions(&observation)
	roleText := &roleTextRenderer{}
	goldmarkhtml.NewRenderer().RegisterFuncs(roleText)
	md.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(roleText, 500)))
	doc := md.Parser().Parse(text.NewReader(source))
	// The walk's callback cannot fail. Rendering is limited to each heading's
	// inline children or one list row's own children, solely for the page's role
	// predicate; it never builds a page or resolves a link/transclusion.
	if err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if n.Kind() != ast.KindHeading && n.Kind() != ast.KindListItem {
			return ast.WalkContinue, nil
		}
		if rewritten.literalRoleAt(n.Pos()) {
			return ast.WalkContinue, nil
		}
		observation.effects = append(observation.effects, roleDisplayEffects(md, n, source, rewritten)...)

		return ast.WalkContinue, nil
	}); err != nil {
		return nil
	}
	return observation.effects
}

// roleMarkup is an unbuffered writer: each source unit records the position at
// which the existing text renderer actually writes it into this own markup.
type roleMarkup struct {
	bytes.Buffer

	units []roleUnit
}

func (w *roleMarkup) Available() int { return 0 }
func (w *roleMarkup) Buffered() int  { return 0 }
func (w *roleMarkup) Flush() error   { return nil }

type roleUnit struct{ sourceStart, sourceEnd, markupStart, markupEnd int }

type roleTextRenderer struct{ renderText renderer.NodeRendererFunc }

func (r *roleTextRenderer) Register(kind ast.NodeKind, render renderer.NodeRendererFunc) {
	if kind == ast.KindText {
		r.renderText = render
	}
}
func (r *roleTextRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindText, r.observeText)
}
func (r *roleTextRenderer) observeText(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	output, ok := w.(*roleMarkup)
	leaf, hasText := n.(*ast.Text)
	if !ok || !hasText || !entering || leaf.IsRaw() || leaf.Segment.Padding != 0 || leaf.Segment.ForceNewline {
		return r.renderText(w, source, n, entering)
	}
	var rendered roleMarkup
	for start := leaf.Segment.Start; start < leaf.Segment.Stop; {
		end := start + roleSourceUnitWidth(source[start:leaf.Segment.Stop])
		unit := ast.NewTextSegment(text.NewSegment(start, end))
		if end == leaf.Segment.Stop {
			unit.SetSoftLineBreak(leaf.SoftLineBreak())
			unit.SetHardLineBreak(leaf.HardLineBreak())
		}
		before := rendered.Len()
		if _, err := r.renderText(&rendered, source, unit, true); err != nil {
			return ast.WalkStop, err
		}
		rendered.units = append(rendered.units, roleUnit{sourceStart: start, sourceEnd: end, markupStart: before, markupEnd: rendered.Len()})
		start = end
	}
	// Grouping source into units must reproduce the actual renderer's bytes.
	// If a future renderer needs wider context, keep its output without guessing.
	var actual roleMarkup
	if _, err := r.renderText(&actual, source, n, true); err != nil {
		return ast.WalkStop, err
	}
	if !bytes.Equal(actual.Bytes(), rendered.Bytes()) {
		return r.renderText(w, source, n, true)
	}
	offset := output.Len()
	for _, unit := range rendered.units {
		unit.markupStart += offset
		unit.markupEnd += offset
		output.units = append(output.units, unit)
	}
	_, err := w.Write(actual.Bytes())
	return ast.WalkContinue, err
}

func roleSourceUnitWidth(source []byte) int {
	if len(source) > 1 && source[0] == '\\' && util.IsPunct(source[1]) {
		return 2
	}
	if source[0] == '&' {
		end := 1
		for end < len(source) && (util.IsAlphaNumeric(source[end]) || source[end] == '#') {
			end++
		}
		if end < len(source) && source[end] == ';' {
			reference := source[:end+1]
			if !bytes.ContainsRune(reference[1:], '&') && (!bytes.Equal(util.ResolveEntityNames(reference), reference) || !bytes.Equal(util.ResolveNumericReferences(reference), reference)) {
				return end + 1
			}
		}
	}
	_, width := utf8.DecodeRune(source)
	return width
}

func roleDisplayEffects(md goldmark.Markdown, n ast.Node, source []byte, rewritten *rewrittenLines) []DisplaySpan {
	var own roleMarkup
	for child := n.FirstChild(); child != nil && child.Kind() != ast.KindList; child = child.NextSibling() {
		if err := md.Renderer().Render(&own, source, child); err != nil {
			return nil
		}
	}
	inner := own.String()
	var start, end int
	if heading, ok := n.(*ast.Heading); ok {
		stripped := sequence.HeadingName(inner, heading.Level)
		if stripped == inner {
			return nil
		}
		start, end = len(stripped), len(inner)
	} else {
		var ok bool
		start, end, ok = listRowRoleRange(inner)
		if !ok {
			return nil
		}
	}
	var effects []DisplaySpan
	for _, unit := range own.units {
		if unit.markupStart >= start && unit.markupEnd <= end && unit.markupStart < unit.markupEnd && !withinAny(rewritten.wikilinks, unit.sourceStart, unit.sourceEnd) {
			effects = append(effects, DisplaySpan{Start: unit.sourceStart, End: unit.sourceEnd, Hidden: true})
		}
	}
	return effects
}
