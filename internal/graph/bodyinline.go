package graph

import (
	"iter"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// InlineFootnoteLabel opens labels owned by the single-pass inline expansion.
const InlineFootnoteLabel = "yomihon-inline-footnote-"

// AuthoredInlineNotes yields the original grammar's single-pass admission for a
// presentation source. It does not recursively expand newly selected notes.
func AuthoredInlineNotes(body string) iter.Seq[InlineFootnoteFact] {
	return bodyValues(admittedInlineNotes(observeBody(body, bodyAuthoredMarkdown.Parser())))
}

// ExpandInlineFootnotes performs the existing single inline expansion on a
// presentation source. It does not strip that source's comments again.
func ExpandInlineFootnotes(body string) string {
	projection := originalBody(body)
	observation := observeBody(body, bodyAuthoredMarkdown.Parser())
	return expandBodyInline(projection, observation).text
}

func admittedInlineNotes(observation *bodyObservation) []InlineFootnoteFact {
	var notes []InlineFootnoteFact
	for _, note := range observation.inlineNotes {
		admitted := true
		for _, definition := range observation.footnotes {
			if definition.Span.Contains(note.Span.Start) && !definition.Emitted {
				admitted = false
				break
			}
		}
		if admitted {
			notes = append(notes, note)
		}
	}
	return notes
}

func expandBodyInline(body bodyProjection, observation *bodyObservation) bodyProjection {
	notes := admittedInlineNotes(observation)
	if len(notes) == 0 {
		return body
	}
	var definitions, prose bodyProjectionWriter
	start, serial := 0, 1
	for _, note := range notes {
		label := "[^" + InlineFootnoteLabel + strconv.Itoa(serial) + "]"
		for strings.Contains(body.text, label) {
			serial++
			label = "[^" + InlineFootnoteLabel + strconv.Itoa(serial) + "]"
		}
		serial++
		definitions.synthetic(label + ": ")
		writeBodyInlineContent(&definitions, body, observation.inlineSegments[note.Span.Start])
		definitions.synthetic("\n\n")
		prose.copied(body, start, note.Span.Start)
		prose.synthetic(label)
		start = note.Span.Stop
	}
	prose.copied(body, start, len(body.text))
	return placeBodyDefinitions(prose.projection(), definitions.projection(), len(notes))
}

func writeBodyInlineContent(output *bodyProjectionWriter, body bodyProjection, segments []text.Segment) {
	for _, segment := range segments {
		output.synthetic(strings.Repeat(" ", segment.Padding))
		at := segment.Start
		for i := segment.Start; i < segment.Stop; i++ {
			if body.text[i] == '\n' {
				output.copied(body, at, i+1)
				output.synthetic("    ")
				at = i + 1
			}
		}
		output.copied(body, at, segment.Stop)
	}
}

// Original comment protection needs code inside opaque notes too. Each note
// receives its own definition-form fragment, never its host's delimiter stack.
// These exceptional parses protect bytes only; final authority is not their
// union and ordinary references are resolved by the later whole-source role.
func protectBodyInline(body bodyProjection, observation *bodyObservation) {
	for _, note := range observation.inlineNotes {
		var fragment bodyProjectionWriter
		fragment.synthetic("[^yomihon-inline-protection]: ")
		writeBodyInlineContent(&fragment, body, observation.inlineSegments[note.Span.Start])
		fragment.synthetic("\n")
		projection := fragment.projection()
		protected := observeBody(projection.text, bodyExpandedMarkdown.Parser())
		for _, code := range protected.codes {
			if span, authored := projection.originalSpan(code.Span); authored {
				code.Span = span
				code.Opener = projection.sourceSpan(code.Opener)
				code.Info = projection.sourceSpan(code.Info)
				code.Closer = projection.sourceSpan(code.Closer)
				observation.codes = append(observation.codes, code)
			}
		}
	}
}

// Placement probes are the existing exceptional presentation parses. They are
// separate from ReadBody's original, admission and expanded authority roles.
func placeBodyDefinitions(prose, definitions bodyProjection, count int) bodyProjection {
	var after bodyProjectionWriter
	after.copied(prose, 0, len(prose.text))
	if !strings.HasSuffix(prose.text, "\n") {
		after.synthetic("\n")
	}
	after.copied(definitions, 0, len(definitions.text))
	appended := after.projection()
	if bodyGeneratedDefinitions(appended.text) == count {
		return appended
	}
	doc := bodyExpandedMarkdown.Parser().Parse(text.NewReader([]byte(prose.text)))
	var open ast.Node
	for node := doc.LastChild(); node != nil; node = node.PreviousSibling() {
		if _, list := node.(*east.FootnoteList); !list {
			open = node
			break
		}
	}
	if open == nil || open.Pos() < 0 {
		return appended
	}
	lineStart := strings.LastIndexByte(prose.text[:open.Pos()], '\n') + 1
	var before bodyProjectionWriter
	before.copied(prose, 0, lineStart)
	before.copied(definitions, 0, len(definitions.text))
	before.copied(prose, lineStart, len(prose.text))
	return before.projection()
}

func bodyGeneratedDefinitions(body string) int {
	doc := bodyExpandedMarkdown.Parser().Parse(text.NewReader([]byte(body)))
	found := 0
	for node := doc.LastChild(); node != nil; node = node.PreviousSibling() {
		list, ok := node.(*east.FootnoteList)
		if !ok {
			continue
		}
		for child := list.FirstChild(); child != nil; child = child.NextSibling() {
			if note, ok := child.(*east.Footnote); ok && strings.HasPrefix(string(note.Ref), InlineFootnoteLabel) {
				found++
			}
		}
	}
	return found
}
