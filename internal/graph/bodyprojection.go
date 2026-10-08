package graph

import (
	"cmp"
	"slices"
	"strings"
)

// bodyPiece maps copied bytes with slope one. Padding and renderer-owned
// labels have negative origins and cannot manufacture an authored identity.
type bodyPiece struct {
	start  int
	stop   int
	origin int
}

type bodyProjection struct {
	text     string
	pieces   []bodyPiece
	roleGaps []string
}

func originalBody(body string) bodyProjection {
	return bodyProjection{text: body, pieces: []bodyPiece{{stop: len(body)}}}
}

type bodyProjectionWriter struct {
	text   strings.Builder
	pieces []bodyPiece
}

func (w *bodyProjectionWriter) copied(body bodyProjection, start, stop int) {
	if stop <= start {
		return
	}
	base := w.text.Len()
	w.text.WriteString(body.text[start:stop])
	for _, piece := range body.pieces {
		left, right := max(start, piece.start), min(stop, piece.stop)
		if right <= left {
			continue
		}
		origin := piece.origin
		if origin >= 0 {
			origin += left - piece.start
		}
		w.pieces = append(w.pieces, bodyPiece{start: base + left - start, stop: base + right - start, origin: origin})
	}
}

func (w *bodyProjectionWriter) synthetic(value string) {
	if value == "" {
		return
	}
	start := w.text.Len()
	w.text.WriteString(value)
	w.pieces = append(w.pieces, bodyPiece{start: start, stop: w.text.Len(), origin: -1})
}

// CommentRoleBlockPrefix identifies a private carrier for an authored raw block.
const CommentRoleBlockPrefix = "<!--yomihon-role-gap"

// rolePadding carries no authored identity or words. Empty inline formatting
// keeps a hidden prose prefix nonblank; a closed HTML comment keeps a raw block.
func (w *bodyProjectionWriter) rolePadding(value string) {
	start := w.text.Len()
	w.text.WriteString(value)
	w.pieces = append(w.pieces, bodyPiece{start: start, stop: w.text.Len(), origin: -2})
}

func (w *bodyProjectionWriter) projection() bodyProjection {
	return bodyProjection{text: w.text.String(), pieces: w.pieces}
}

func (p bodyProjection) withoutRolePadding() string {
	var out strings.Builder
	for _, piece := range p.pieces {
		if piece.origin != -2 {
			out.WriteString(p.text[piece.start:piece.stop])
		}
	}
	return out.String()
}

// roleProjection chooses a carrier absent from every surviving authored byte,
// so source-returning consumers can remove it without guessing among copies.
func (w *bodyProjectionWriter) roleProjection() bodyProjection {
	p := w.projection()
	plain := p.withoutRolePadding()
	gap := "<u></u>"
	for strings.Contains(plain, gap) {
		gap += gap
	}
	blockName := "yomihon-role-gap"
	blockGap := "<!--" + blockName + "-->"
	for strings.Contains(plain, blockGap) {
		blockName += blockName
		blockGap = "<!--" + blockName + "-->"
	}
	var out bodyProjectionWriter
	var gaps []string
	for _, piece := range p.pieces {
		value := p.text[piece.start:piece.stop]
		if piece.origin == -2 {
			if value == "<!---->" {
				value = blockGap
			} else {
				value = gap
			}
			if !slices.Contains(gaps, value) {
				gaps = append(gaps, value)
			}
		}
		start := out.text.Len()
		out.text.WriteString(value)
		out.pieces = append(out.pieces, bodyPiece{start: start, stop: out.text.Len(), origin: piece.origin})
	}
	result := out.projection()
	result.roleGaps = gaps
	return result
}

func (p bodyProjection) originalOffset(offset int) (int, bool) {
	for _, piece := range p.pieces {
		if offset >= piece.start && offset < piece.stop {
			if piece.origin < 0 {
				return 0, false
			}
			return piece.origin + offset - piece.start, true
		}
	}
	return 0, false
}

func (p bodyProjection) originalSpan(span Span) (Span, bool) {
	start, stop := -1, 0
	for _, piece := range p.pieces {
		left, right := max(span.Start, piece.start), min(span.Stop, piece.stop)
		if piece.origin < 0 || right <= left {
			continue
		}
		first := piece.origin + left - piece.start
		last := piece.origin + right - piece.start
		if start < 0 || first < start {
			start = first
		}
		stop = max(stop, last)
	}
	return Span{Start: start, Stop: stop}, start >= 0
}

func (p bodyProjection) sourceSpan(span Span) Span {
	if mapped, found := p.originalSpan(span); found {
		return mapped
	}
	return Span{}
}

type bodyReading struct {
	source        string
	admissionBody bodyProjection
	expanded      bodyProjection
	bootstrap     *bodyObservation
	admission     *bodyObservation
	authority     *bodyObservation
	comments      []Span
	unclosed      BodyComment
	containers    []BodyComment
}

func projectedBodyFacts(reading *bodyReading) *bodyFactsData {
	facts := &bodyFactsData{source: reading.source, comments: reading.comments, commentFree: reading.admissionBody.withoutRolePadding(), presentation: reading.admissionBody.text, roleGaps: slices.Clone(reading.admissionBody.roleGaps), comment: reading.unclosed, containerComments: slices.Clone(reading.containers)}
	facts.projectFootnotes(reading.bootstrap, reading.authority, reading.expanded)
	facts.projectCodes(reading.authority, reading.expanded)
	facts.projectProse(reading.authority, reading.expanded)
	facts.projectInlineNotes(reading.admission, reading.admissionBody)
	facts.htmlLimits = slices.Clone(reading.bootstrap.htmlLimits)
	facts.sortSourceFacts()
	return facts
}

func (f *bodyFactsData) projectFootnotes(bootstrap, authority *bodyObservation, expanded bodyProjection) {
	// Initial accepted definitions retain original provenance even when stripped.
	// Only the final grammar assigns emission.
	for _, definition := range bootstrap.footnotes {
		definition.Emitted = false
		f.footnotes = append(f.footnotes, definition)
	}
	for _, definition := range authority.footnotes {
		if _, authored := expanded.originalOffset(definition.Span.Start); !authored {
			continue // generated labels are not authored ordinary definitions
		}
		definition.Span = expanded.sourceSpan(definition.Span)
		f.recordFootnoteEmission(definition)
	}
}

func (f *bodyFactsData) recordFootnoteEmission(definition FootnoteFact) {
	for i := range f.footnotes {
		if f.footnotes[i].Span.Start == definition.Span.Start {
			f.footnotes[i].Emitted = definition.Emitted
			return
		}
	}
	f.footnotes = append(f.footnotes, definition)
}

func (f *bodyFactsData) projectCodes(authority *bodyObservation, expanded bodyProjection) {
	for _, code := range authority.codes {
		span, authored := expanded.originalSpan(code.Span)
		if !authored {
			continue
		}
		code.Span = span
		code.Opener = expanded.sourceSpan(code.Opener)
		code.Info = expanded.sourceSpan(code.Info)
		code.Closer = expanded.sourceSpan(code.Closer)
		f.codes = append(f.codes, code)
	}
}

func (f *bodyFactsData) projectProse(authority *bodyObservation, expanded bodyProjection) {
	for _, link := range authority.autolinks {
		if span, authored := expanded.originalSpan(link); authored {
			f.autolinks = append(f.autolinks, span)
		}
	}
	for _, heading := range authority.headings {
		span, authored := expanded.originalSpan(heading.Span)
		if !authored {
			continue
		}
		heading.Span = span
		f.headings = append(f.headings, heading)
	}
	for _, destination := range authority.destinations {
		offset, authored := expanded.originalOffset(destination.Offset)
		if !authored {
			continue
		}
		destination.Offset = offset
		f.destinations = append(f.destinations, destination)
	}
	for _, literal := range authority.literals {
		span, authored := expanded.originalSpan(literal.Span)
		if !authored {
			continue
		}
		literal.Span = span
		f.literals = append(f.literals, literal)
	}
}

func (f *bodyFactsData) projectInlineNotes(admission *bodyObservation, body bodyProjection) {
	for _, note := range admission.inlineNotes {
		span, authored := body.originalSpan(note.Span)
		if !authored {
			continue
		}
		opening, openingAuthored := body.originalOffset(note.Span.Start + 1)
		closing, closingAuthored := body.originalOffset(note.Span.Stop - 1)
		if !openingAuthored || !closingAuthored || opening < 0 || opening >= closing || closing >= len(f.source) {
			continue
		}
		if f.source[opening] != '[' || f.source[closing] != ']' {
			continue
		}
		note.Span = span
		note.Content = f.source[opening+1 : closing]
		f.inlineNotes = append(f.inlineNotes, note)
	}
}

func (f *bodyFactsData) sortSourceFacts() {
	slices.SortFunc(f.codes, func(a, b CodeFact) int { return cmp.Compare(a.Span.Start, b.Span.Start) })
	slices.SortFunc(f.footnotes, func(a, b FootnoteFact) int { return cmp.Compare(a.Span.Start, b.Span.Start) })
	slices.SortFunc(f.headings, func(a, b BodyHeading) int { return cmp.Compare(a.Span.Start, b.Span.Start) })
	slices.SortFunc(f.destinations, func(a, b BodyDestination) int { return cmp.Compare(a.Offset, b.Offset) })
	slices.SortFunc(f.literals, func(a, b CodeLiteral) int { return cmp.Compare(a.Span.Start, b.Span.Start) })
	slices.SortFunc(f.inlineNotes, func(a, b InlineFootnoteFact) int { return cmp.Compare(a.Span.Start, b.Span.Start) })
}
