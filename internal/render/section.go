package render

import (
	"fmt"
	"path"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/sequence"
	"github.com/koopa0/yomihon/internal/wording"
)

// Where one section of a note ends, so an embed can cut it out. Which lines
// are headings is answered by the one grammar every face reads; this file puts
// that answer to the note's own source rather than to rendered HTML, because an
// embed slices the file's own bytes.

// indentedCodeLines reports which lines of body this pipeline will show the
// reader as written because an indented code block holds them, so a bracket pair
// on one of them is syntax on display rather than a citation. Only this kind of
// code has to be asked about: the dialect pass tracks a fenced block itself, and
// a code span never spans a line.
//
// The question is put to the pipeline's own parser, and it is a method for that
// reason: the answer has to come from the reading that will render the body,
// never from a second parser configured beside it. A plain CommonMark reading
// looks close enough and is not — it calls a footnote definition's later
// paragraphs an indented code block, so a citation an author wrote in one would
// reach the reader as brackets in the middle of a sentence, with neither face
// reporting anything.
//
// It also has to be a parse rather than an indent test, because the indent that
// opens a block is measured from the content column of whatever list encloses
// the line. Read over the 535 notes of the vault this serves — its notes, not
// the agent files kept under its dot directories — an indent test disagrees with
// the parse on four, every disagreement a list whose own indented prose the test
// called code.
func (r *Pipeline) indentedCodeLines(body string) map[int]bool {
	var spans [][2]int
	//nolint:errcheck // the visitor never returns an error, so the walk cannot fail
	_ = ast.Walk(r.md.Parser().Parse(text.NewReader([]byte(body))), func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if _, ok := n.(*ast.CodeBlock); ok && entering {
			if ls := n.Lines(); ls != nil && ls.Len() > 0 {
				spans = append(spans, [2]int{ls.At(0).Start, ls.At(ls.Len() - 1).Stop})
			}
		}
		return ast.WalkContinue, nil
	})
	if len(spans) == 0 {
		return nil
	}
	quoted := make(map[int]bool)
	at := 0
	for i, line := range strings.Split(body, "\n") {
		end := at + len(line)
		for _, s := range spans {
			if at < s[1] && end > s[0] {
				quoted[i] = true
				break
			}
		}
		at = end + 1
	}
	return quoted
}

// headingSlice returns the section of body that heading names: the first heading
// whose text folds to the same slug, through to the line before the next heading
// of the same or a higher level, deeper ones included. A repeated name takes the
// first, as Obsidian's reading view does. The name folds through the section id
// over heading text reduced the way the anchor pass reduces it, so the destination's
// own table of contents lists the spellings an embed accepts.
func headingSlice(body, heading string) (slice string, matches int) {
	want := graph.SectionID(heading)
	lines := strings.Split(body, "\n")
	headings := graph.Headings(body, graph.LineSkipZones(body))
	for i, h := range headings {
		if graph.SectionID(headingSourceText(h.Text, h.Level)) != want {
			continue
		}
		matches++
		if matches > 1 {
			// The later ones are counted, not cut: the excerpt is the first,
			// and what the rest are for is to say how many there were.
			continue
		}
		slice = strings.Join(lines[h.Line:], "\n")
		for _, next := range headings[i+1:] {
			if next.Level <= h.Level {
				slice = strings.Join(lines[h.Line:next.Line], "\n")
				break
			}
		}
	}
	return slice, matches
}

// ledeSlice returns the opening of body a hover card shows when no fragment
// named a place: the lines before the first heading when they hold any
// non-blank content, or that heading plus the lines up to the next heading of
// any level when the note opens on one — leading blanks ignored, so a file
// that starts `\n## …` is an opening heading, not an empty lede. The next
// heading of any level is the edge, not the next same-or-higher one: an H1
// opener would otherwise run to the end of the note. narrowed is true when
// anything is left behind; a cut that is the whole body is not a narrowing.
func ledeSlice(body string) (slice string, narrowed bool) {
	lines := strings.Split(body, "\n")
	headings := graph.Headings(body, graph.LineSkipZones(body))
	if len(headings) == 0 {
		return body, false
	}
	first := headings[0]
	if opening := openingBefore(lines, first.Line); opening != "" {
		return opening, true
	}
	end := len(lines)
	if len(headings) > 1 {
		end = headings[1].Line
	}
	if end >= len(lines) {
		return body, false
	}
	return strings.Join(lines[first.Line:end], "\n"), true
}

// Opening is what a note's author wrote before its first heading: the words
// that say what the note is, ahead of whatever it goes on to list. A note that
// opens on a heading has none and the answer is empty, which a surface renders
// as nothing rather than as an empty box; a note carrying no heading at all is
// all opening.
//
// Obsidian's %% comments come off first, as they do before any other cut, so a
// marker cannot arrive visible in the words a course prints under its title.
//
// It is the same cut the hover card makes over the same lines, and both ask
// openingBefore for it. The card goes on to a second answer where this one is
// empty — it has to show something of the note under the pointer — and that
// fallback is the card's, not this.
func Opening(body string) string {
	stripped, _ := stripBody(body)
	lines := strings.Split(stripped.text, "\n")
	headings := graph.Headings(stripped.text, graph.LineSkipZones(stripped.text))
	if len(headings) == 0 {
		return openingBefore(lines, len(lines))
	}
	return openingBefore(lines, headings[0].Line)
}

// openingBefore is the lines up to first, or empty where they hold nothing but
// blanks. A run of blanks is not an opening: the note opens on the heading
// below them, and a surface asking for words it has none of must be told so.
func openingBefore(lines []string, first int) string {
	if !ledeBeforeHeading(lines, first) {
		return ""
	}
	return strings.Join(lines[:first], "\n")
}

// ledeBeforeHeading reports whether the lines before the first heading hold
// any non-blank content. A run of blanks is not a lede: the note opens on
// that heading, and the card has to show its words rather than an empty body.
func ledeBeforeHeading(lines []string, first int) bool {
	for _, line := range lines[:first] {
		if !graph.BlankLine(line) {
			return true
		}
	}
	return false
}

// Excerpt is the part of body that a link's own fragment addresses: a caret
// opens a block address, anything else names a section, and an empty fragment
// asks for the note itself. Obsidian's %% comments come off before any edge is
// chosen, so a marker cannot span the cut and arrive visible in the excerpt.
//
// An address the note does not answer to comes back not found, and the caller
// says so. Widening to the whole note would answer a question nobody asked —
// the reader named one place, and being shown a different one without being
// told reads as the place they named.
//
// The fragment is the one an anchor already carries, already folded by the pass
// that wrote it; nothing here folds a name a second time. An embed of a whole
// note still comes through here as the note: a hover card that wants only the
// lede asks ExcerptPreview, so the two surfaces stay one cut for a named
// fragment and two answers for an empty one.
func Excerpt(body, fragment string) (slice string, found bool) {
	stripped, _ := stripBody(body)
	slice, matches := excerptOf(stripped, fragment)
	return slice, matches > 0
}

// ExcerptPreview is the hover card's reading of the same address Excerpt
// takes. A named fragment is the same cut. An empty one is the lede — the
// lines before the first heading when they hold content, or that heading plus
// the lines up to the next heading of any level when the note opens on one —
// and narrowed reports that the rest of the note was left behind, so the card
// can say so with the sentence a byte-capped preview already uses. An embed
// still asks Excerpt (or excerptOf) for the whole note; this cut is the card's.
func ExcerptPreview(body, fragment string) (slice string, found, narrowed bool) {
	stripped, _ := stripBody(body)
	if fragment == "" {
		slice, narrowed = ledeSlice(stripped.text)
		return slice, true, narrowed
	}
	slice, matches := excerptOf(stripped, fragment)
	return slice, matches > 0, false
}

// excerptOf is the one cut every excerpt is made with, over a body whose
// comments are already off: it returns the part of it that fragment addresses,
// spelled as Excerpt reads it, and counts the places that answered — one for a
// block or for the note itself, and for a section every heading folding to the
// name, of which the first is cut. Zero is an address the note does not answer
// to, and then nothing is cut: there is no narrower answer than the one asked
// for, and a wider one would be this renderer's rather than the author's.
func excerptOf(stripped strippedBody, fragment string) (slice string, matches int) {
	switch {
	case strings.HasPrefix(fragment, "^"):
		cut, ok := blockSlice(stripped, strings.TrimPrefix(fragment, "^"))
		if !ok {
			return "", 0
		}
		return cut, 1
	case fragment != "":
		return headingSlice(stripped.text, fragment)
	}
	return stripped.text, 1
}

// fragmentOf is the address an embed carries, in the spelling Excerpt reads. A
// block wins when the author wrote both a block and a section, which is the
// order a link's address resolves that conflict in too.
func fragmentOf(link graph.Wikilink) string {
	if link.Block != "" {
		return "^" + link.Block
	}
	return link.Heading
}

// ExcerptWithheld is what a surface says where an excerpt it could not cut
// would have stood: the address the note does not answer to, spelled the way
// the author's own link spells it, and the note it was asked of. The reading
// page says it inside the block an embed leaves behind and the hover card says
// it under the note's name, from this one sentence, so the two report one fact
// in one voice. fragment is the one Excerpt reads.
func ExcerptWithheld(relPath, fragment string, lang wording.Lang) string {
	return fmt.Sprintf(wording.ExcerptWithheldFmt.In(lang), "#"+fragment, noteName(relPath))
}

// noteName is the name a note is cited by: its file's own, without the
// extension. A provenance line and a withheld notice both call a note this,
// because it is the name a citation resolves by.
func noteName(relPath string) string {
	return strings.TrimSuffix(path.Base(relPath), ".md")
}

// ExcerptHeading is the words a reader sees in the heading an excerpt opens on,
// or empty when it does not open on one — a block excerpt, or a note read from
// its first line. It reduces the heading the way the anchor pass does, so a
// reading that names a section names it as its own table of contents does.
func ExcerptHeading(slice string) string {
	lines := strings.SplitN(slice, "\n", 3)
	if len(lines) == 0 {
		return ""
	}
	if m := graph.ATXHeading.FindStringSubmatch(lines[0]); m != nil {
		return headingSourceText(m[2], len(m[1]))
	}
	// A heading written under its own underline opens on the line of text, so
	// the line below it is what says the text was a heading at all.
	if len(lines) > 1 && graph.SetextUnderline.MatchString(lines[1]) && !graph.BlankLine(lines[0]) {
		return headingSourceText(lines[0], graph.SetextLevel(lines[1]))
	}
	return ""
}

// headingSourceText reduces a heading's markdown source to the text the page
// stamps its anchor from. A course branch declares its part in the order at
// the end of the heading that opens it, and that declaration is grammar
// rather than words, so the level decides what the heading is called for the
// same reason it does on the page. The words themselves fold through
// HeadingWords, the one reduction the check face reads too.
func headingSourceText(raw string, level int) string {
	return HeadingWords(sequence.HeadingName(raw, level))
}

// blockSlice returns the block carrying the "^name" marker: the run of non-blank
// lines around the first line outside fenced code ending with the marker, stopping
// at a list item's own line, and reaching back to the line its block opens on when
// the marked line is a continuation. The address matches through the fold both
// fragment kinds share, so "^quote-1" and "^quote1" stay two names. Nothing rules
// how wide a block reference reaches, so the narrow reading is taken.
func blockSlice(body strippedBody, block string) (string, bool) {
	lines := strings.Split(body.text, "\n")
	at := blockMarkerLine(lines, body.address, block)
	if at < 0 {
		return "", false
	}
	start := at
	for start > 0 && !graph.ListItemLine.MatchString(lines[start]) && strings.TrimSpace(lines[start-1]) != "" {
		start--
	}
	end := at + 1
	for end < len(lines) && strings.TrimSpace(lines[end]) != "" && !graph.ListItemLine.MatchString(lines[end]) {
		end++
	}
	return strings.Join(lines[start:end], "\n"), true
}

// blockMarkerLine reports which line carries the marker naming block, or -1
// when the note has no such marker. A marker written inside a fenced block is
// code rather than an address, so the scan tracks fences as it walks. A caret
// a code span owns is the same kind of quoted text, asked of the one predicate
// the page and the check share, over the line geometry its author wrote rather
// than over whatever the comment strip left.
func blockMarkerLine(lines, address []string, block string) int {
	want := graph.FoldFragment("^" + block)
	inFence, fenceByte, fenceLen := false, byte(0), 0
	for i, line := range lines {
		// A fence is looked for with any quote marker taken off it, because a
		// fence written inside a callout opens one: that body is read on its
		// own with the markers stripped, and a line of code in it is code.
		unquoted := quotePrefix.ReplaceAllString(line, "")
		if inFence {
			if fenceCloses(unquoted, fenceByte, fenceLen) {
				inFence = false
			}
			continue
		}
		if open, n, _, ok := fenceOpen(unquoted); ok {
			inFence, fenceByte, fenceLen = true, open, n
			continue
		}
		if UnanchorableLine(line) || CodeSpanOwnsBlockAddress(address, i) {
			continue
		}
		trimmed := graph.FoldFragment(strings.TrimRight(line, " \t"))
		if trimmed == want || strings.HasSuffix(trimmed, " "+want) || strings.HasSuffix(trimmed, "\t"+want) {
			return i
		}
	}
	return -1
}
