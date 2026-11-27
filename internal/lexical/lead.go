package lexical

import (
	"strings"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/render"
)

// Lead is the excerpt a result carries under its title: one passage of prose
// that opens at the sentence holding the first hit, as a reader would start
// reading it. It is a different question from the snippet the landing is cut
// from, which is why it has its own fields.
type Lead struct {
	// Runs open at the sentence's start, with a leading ellipsis when that is
	// not the paragraph's start. The page clamps them to the lines it has.
	Runs []ExcerptRun
	// Near is the same passage cut for a single line: it opens at the nearest
	// word boundary before the hit when the hit would fall beyond the first
	// line. It is empty when it would read as Runs does.
	Near []ExcerptRun
	// Code says the hit lives only in a fenced block, so the passage is that
	// line of code and not a sentence.
	Code bool
}

const (
	// leadReach is how far a lead runs on from where it opens: two lines of
	// the wide result page, with room for a line that wraps short.
	leadReach = 240
	// leadFirstLine is how far into a sentence a hit may sit before a single
	// line would show none of it.
	leadFirstLine = 22
	// leadNearBefore is how much a narrow lead keeps before the hit when it
	// has to cut the sentence short.
	leadNearBefore = 8
	// leadNearReach is how far a narrow lead runs on from where it opens.
	leadNearReach = 90
	// leadSentenceBudget is the longest stretch searched back for a sentence
	// opening. A hit buried in a run with no stop shows from a word boundary
	// near it instead.
	leadSentenceBudget = 120
)

// sentenceStops end a sentence in the scripts a vault is written in.
const sentenceStops = "。！？；!?;"

// lead builds a result's lead from the first prose block that holds a hit.
// A heading's hit belongs to the title and the location, not to the excerpt,
// and a table's separator rows and the like never reach the plain text, so the
// blocks walked are the ones a reader reads as running text. Where the words
// are only in a fenced block the lead is that line.
func (e *entry) lead(tokens []string) Lead {
	if len(tokens) == 0 {
		return Lead{}
	}
	var code [2]int
	foundCode := false
	prev := 0
	blocks := e.blocks
	if len(blocks) == 0 {
		// A document built without block boundaries is one block.
		blocks = []render.Block{{End: len(e.plainText)}}
	}
	for _, b := range blocks {
		start, end := prev, b.End
		prev = b.End
		if b.Heading || start >= end {
			continue
		}
		text := e.plainText[start:end]
		if strings.EqualFold(collapseFields(text), collapseFields(e.title)) {
			// The heading the page shows as the title stays in the plain text,
			// and is the title's, not the excerpt's.
			continue
		}
		hitStart, hitEnd := e.firstShownHit(start, text, tokens)
		if hitStart < 0 {
			continue
		}
		if fenced, _ := fenceAt(start+hitStart, e.fenceRanges); fenced {
			if !foundCode {
				code, foundCode = [2]int{start, end}, true
			}
			continue
		}
		return e.leadFrom(start, text, hitStart, hitEnd, tokens)
	}
	if foundCode {
		// A block opens at the break that parted it from the one before.
		from := code[0] + len(e.plainText[code[0]:code[1]]) - len(strings.TrimLeft(e.plainText[code[0]:code[1]], "\n "))
		line := e.plainText[from:code[1]]
		if nl := strings.IndexByte(line, '\n'); nl >= 0 {
			line = line[:nl]
		}
		end := from + len(line)
		return Lead{Runs: e.leadRuns(from, end, tokens, false, end < code[1]), Code: true}
	}
	return Lead{}
}

// firstShownHit is the earliest hit in text that the reading page shows, as
// source offsets into text, or -1. A word the page does not show, such as a
// heading's role marker or the name a link's display stands in for, is
// findable but is not an excerpt, and a later hit in the same block can be.
func (e *entry) firstShownHit(blockStart int, text string, tokens []string) (start, end int) {
	for from := 0; from < len(text); {
		hitStart, hitEnd := firstHitIn(text[from:], tokens)
		if hitStart < 0 {
			return -1, 0
		}
		hitStart, hitEnd = hitStart+from, hitEnd+from
		if hidden, _ := e.excerptEffects(blockStart+hitStart, blockStart+hitEnd); !hidden {
			return hitStart, hitEnd
		}
		if hitEnd <= from {
			break
		}
		from = hitEnd
	}
	return -1, 0
}

// firstHitIn is the earliest place any token occurs in text, as source
// offsets, or -1 when none does.
func firstHitIn(text string, tokens []string) (start, end int) {
	folded := Fold(text)
	foldStart, foldEnd := -1, 0
	for _, t := range tokens {
		if t == "" {
			continue
		}
		if i, stop := phraseIndex(folded, t, 0); i >= 0 && (foldStart < 0 || i < foldStart) {
			foldStart, foldEnd = i, stop
		}
	}
	if foldStart < 0 {
		return -1, -1
	}
	return sourceOffsetOfFold(text, foldStart), sourceEndOfFold(text, foldEnd)
}

// leadFrom cuts the lead of one block, whose text begins at blockStart in the
// note's plain text and holds the hit at [hitStart, hitEnd).
func (e *entry) leadFrom(blockStart int, text string, hitStart, hitEnd int, tokens []string) Lead {
	open := sentenceOpening(text, hitStart)
	end := min(len(text), max(hitEnd, wholeWordEnd(text, runesAfter(text, open, leadReach))))
	lead := Lead{Runs: e.leadRuns(blockStart+open, blockStart+end, tokens, open > 0, end < len(text))}
	if utf8.RuneCountInString(text[open:hitStart]) <= leadFirstLine {
		return lead
	}
	nearOpen := min(hitStart, wholeWordStart(text, runesBefore(text, hitStart, leadNearBefore)))
	nearEnd := min(len(text), max(hitEnd, wholeWordEnd(text, runesAfter(text, nearOpen, leadNearReach))))
	lead.Near = e.leadRuns(blockStart+nearOpen, blockStart+nearEnd, tokens, nearOpen > 0, nearEnd < len(text))
	return lead
}

// sentenceOpening is where the sentence holding off begins, in text: after the
// last stop before it, with the space that follows the stop skipped. A hit
// further into a sentence than a reader can be asked to look back shows from a
// word boundary near it, and a hit in the first sentence shows from the block's
// start.
func sentenceOpening(text string, off int) int {
	from := runesBefore(text, off, leadSentenceBudget)
	before := text[from:off]
	at := strings.LastIndexAny(before, sentenceStops)
	stopSize := 0
	if at >= 0 {
		_, stopSize = utf8.DecodeRuneInString(before[at:])
	}
	if dot := strings.LastIndex(before, ". "); dot >= 0 && dot+1 > at+stopSize {
		at, stopSize = dot, 1
	}
	if at < 0 {
		if from == 0 {
			return 0
		}
		return min(off, wholeWordStart(text, from))
	}
	open := from + at + stopSize
	for open < off && (text[open] == ' ' || text[open] == '\t' || text[open] == '\n') {
		open++
	}
	return open
}

// leadRuns writes the plain text from start to end as runs, the stretches the
// query found marked and retractions kept, with an ellipsis where the passage
// is cut from a longer block.
func (e *entry) leadRuns(start, end int, tokens []string, cutBefore, cutAfter bool) []ExcerptRun {
	var runs []ExcerptRun
	if cutBefore {
		appendExcerptRun(&runs, "…", false, false)
	}
	for _, atom := range collapseExcerptAtoms(e.excerptAtoms(start, end, tokens)) {
		appendExcerptRun(&runs, atom.text, atom.hit, atom.deleted)
	}
	if cutAfter {
		appendExcerptRun(&runs, "…", false, false)
	}
	return runs
}
