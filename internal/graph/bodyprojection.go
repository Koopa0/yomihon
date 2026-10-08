package graph

import (
	"cmp"
	"slices"
	"strings"
)

// bodyPiece maps copied bytes with slope one. Padding and renderer-owned
// labels have origin -1 and cannot manufacture an authored source identity.
type bodyPiece struct {
	start  int
	stop   int
	origin int
}

type bodyProjection struct {
	text   string
	pieces []bodyPiece
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

func (w *bodyProjectionWriter) projection() bodyProjection {
	return bodyProjection{text: w.text.String(), pieces: w.pieces}
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
}

func projectedBodyFacts(reading bodyReading) BodyFacts {
	admissionBody, expanded := reading.admissionBody, reading.expanded
	bootstrap, admission, authority := reading.bootstrap, reading.admission, reading.authority
	facts := BodyFacts{source: reading.source, comments: reading.comments, commentFree: admissionBody.text, comment: reading.unclosed}
	// Initial accepted definitions retain their original provenance even if the
	// comment strip removes them. Only the final grammar assigns emission.
	for _, definition := range bootstrap.footnotes {
		definition.Emitted = false
		facts.footnotes = append(facts.footnotes, definition)
	}
	for _, definition := range authority.footnotes {
		if _, authored := expanded.originalOffset(definition.Span.Start); !authored {
			continue // generated labels are not authored ordinary definitions
		}
		definition.Span = expanded.sourceSpan(definition.Span)
		matched := false
		for i := range facts.footnotes {
			if facts.footnotes[i].Span.Start == definition.Span.Start {
				facts.footnotes[i].Emitted = definition.Emitted
				matched = true
				break
			}
		}
		if !matched {
			facts.footnotes = append(facts.footnotes, definition)
		}
	}
	for _, code := range authority.codes {
		span, authored := expanded.originalSpan(code.Span)
		if !authored {
			continue
		}
		code.Span = span
		code.Opener = expanded.sourceSpan(code.Opener)
		code.Info = expanded.sourceSpan(code.Info)
		code.Closer = expanded.sourceSpan(code.Closer)
		facts.codes = append(facts.codes, code)
	}
	for _, heading := range authority.headings {
		if span, authored := expanded.originalSpan(heading.Span); authored {
			heading.Span = span
			facts.headings = append(facts.headings, heading)
		}
	}
	for _, destination := range authority.destinations {
		if offset, authored := expanded.originalOffset(destination.Offset); authored {
			destination.Offset = offset
			facts.destinations = append(facts.destinations, destination)
		}
	}
	for _, literal := range authority.literals {
		if span, authored := expanded.originalSpan(literal.Span); authored {
			literal.Span = span
			facts.literals = append(facts.literals, literal)
		}
	}
	for _, note := range admission.inlineNotes {
		if span, authored := admissionBody.originalSpan(note.Span); authored {
			opening, openingAuthored := admissionBody.originalOffset(note.Span.Start + 1)
			closing, closingAuthored := admissionBody.originalOffset(note.Span.Stop - 1)
			if !openingAuthored || !closingAuthored || opening < 0 || opening >= closing || closing >= len(reading.source) {
				continue
			}
			if reading.source[opening] != '[' || reading.source[closing] != ']' {
				continue
			}
			note.Span = span
			note.Content = reading.source[opening+1 : closing]
			facts.inlineNotes = append(facts.inlineNotes, note)
		}
	}
	facts.htmlLimits = slices.Clone(bootstrap.htmlLimits)
	slices.SortFunc(facts.codes, func(a, b CodeFact) int { return cmp.Compare(a.Span.Start, b.Span.Start) })
	slices.SortFunc(facts.footnotes, func(a, b FootnoteFact) int { return cmp.Compare(a.Span.Start, b.Span.Start) })
	slices.SortFunc(facts.headings, func(a, b BodyHeading) int { return cmp.Compare(a.Span.Start, b.Span.Start) })
	slices.SortFunc(facts.destinations, func(a, b BodyDestination) int { return cmp.Compare(a.Offset, b.Offset) })
	slices.SortFunc(facts.literals, func(a, b CodeLiteral) int { return cmp.Compare(a.Span.Start, b.Span.Start) })
	slices.SortFunc(facts.inlineNotes, func(a, b InlineFootnoteFact) int { return cmp.Compare(a.Span.Start, b.Span.Start) })
	return facts
}
