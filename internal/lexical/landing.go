package lexical

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// landingPrefixWords and landingPrefixRunes bound the run of words named
	// ahead of a match, in words and in characters the reader sees. The
	// budget is small deliberately: the run is written for a browser to find
	// again in rendered text, and every further word is another chance for
	// the two spellings of the same sentence to part.
	landingPrefixWords = 3
	landingPrefixRunes = 30
)

// landing places a result's link on its earliest body match: the earliest the
// page shows, where the note carries text the page hides, and otherwise the
// earliest prose match, or a fence's where the words live only there. A match
// within its first block is taken together with every match of any term that
// overlaps or touches it, which is the stretch the page marks as one.
func (e *entry) landing(tokens []string) (terms landingTerms, fromFence bool) {
	if len(e.displaySpans) > 0 {
		centre := e.shownExcerptCentre(tokens)
		if !centre.found {
			return landingTerms{}, false
		}
		return e.landingAtSource(centre.start, centre.end), centre.fromFence
	}
	foldStart, foldEnd, fromFence := earliestOffset(e.plainFold, tokens, e.fenceFoldRanges)
	if foldStart < 0 {
		return landingTerms{}, false
	}
	terms = e.landingAt(foldStart, foldEnd)
	if terms.crossing {
		return terms, fromFence
	}
	start, end := sourceOffsetOfFold(e.plainText, foldStart), sourceEndOfFold(e.plainText, foldEnd)
	if markStart, markEnd, ok := e.markedRunFrom(start, end, tokens); ok {
		terms = e.landingAtSource(markStart, markEnd)
	}
	return terms, fromFence
}

// landingMarkReach is how many characters past the start of the earliest match
// the run of marks it opens is followed, which is further than any run of
// marks a reader reads as one.
const landingMarkReach = 160

// markedRunFrom is the first stretch MarkHits marks as one in the text that
// opens at the earliest match [start, end): the match and every match of any
// token that overlaps or touches it, white space at its ends left out. Line
// breaks are read as spaces, so a run is never joined across one, and the text
// stops short of a fence the match is not in, or at the end of the one it is
// in. ok is false where nothing in that text is marked.
func (e *entry) markedRunFrom(start, end int, tokens []string) (markStart, markEnd int, ok bool) {
	stop := max(end, runesAfter(e.plainText, start, landingMarkReach))
	for _, fence := range e.fenceRanges {
		switch {
		case fence[0] <= start && start < fence[1]:
			stop = min(stop, fence[1])
		case start < fence[0]:
			stop = min(stop, max(end, fence[0]))
		}
	}
	at := start
	for _, run := range MarkHits(strings.ReplaceAll(e.plainText[start:stop], "\n", " "), tokens) {
		if text := strings.TrimSpace(run.Text); run.Hit && text != "" {
			markStart = at + strings.Index(run.Text, text)
			return markStart, markStart + len(text), true
		}
		at += len(run.Text)
	}
	return 0, 0, false
}

// landingTerms is everything one body match gives a browser text directive:
// the stretch to arrive at, that same stretch grown to whole words for a
// directive that names nothing ahead of it, the run of words the match
// follows, the stretch its far end sits in grown the same way, the run after
// the last of those stretches, and whether it left its block to reach that
// end. They travel together because a directive is assembled from all of them
// at once, and five loose strings are five a caller can pair up the wrong way
// round.
type landingTerms struct {
	prefix   string
	suffix   string
	first    string
	bare     string
	last     string
	crossing bool
}

// landingAt is the terms one folded body match offers. The exclusive end is
// the last matched rune plus its length: the next kept rune can sit in a later
// block after a fold-dropped break, which is not itself a crossing. Without
// recorded block ends there is nothing to tell a wrap from a paragraph, so it
// offers nothing and does not claim a crossing.
//
// The run of words ahead of the match is cut at the start of its own block, and
// is not named at all in a block the page does not reproduce as written (a ruby
// reading, a footnote mark), where it would match nothing. Every stretch is
// also grown to whole words, inside its own block, for the directives that
// have no run to name.
func (e *entry) landingAt(foldStart, foldEnd int) landingTerms {
	if foldStart < 0 || foldEnd <= foldStart || len(e.blocks) == 0 {
		return landingTerms{}
	}
	start := sourceOffsetOfFold(e.plainText, foldStart)
	end := sourceEndOfFold(e.plainText, foldEnd)
	if start >= end || end > len(e.plainText) {
		return landingTerms{}
	}
	return e.landingAtSource(start, end)
}

// landingAtSource places a selected source span on the directive's word edges.
//
// The run after the match is measured from the end of the last stretch a
// directive can name, wherever growth left that end, and asked of the block
// that stretch sits in: for a crossing match that is the last block, whose own
// reproduction decides whether a run beside the term can be read as one.
func (e *entry) landingAtSource(start, end int) landingTerms {
	var terms landingTerms
	if e.landingBlocked(start, end) {
		return terms
	}
	blockStart, verbatim := e.blockAt(start)
	blockStart, firstLimit := e.insertionBounds(start, blockStart, e.blockEndAfter(start))
	if verbatim {
		terms.prefix = landingPrefix(e.plainText[blockStart:start])
	}
	firstEnd := e.blockEndAfter(start)
	terms.crossing = end > firstEnd
	firstStop := end
	if terms.crossing {
		firstStop = firstEnd
	}
	bareStart, bareEnd := wordEdges(e.plainText, start, firstStop, blockStart, firstLimit)
	terms.first = collapseFields(e.plainText[start:bareEnd])
	terms.bare = collapseFields(e.plainText[bareStart:bareEnd])
	if !terms.crossing {
		if headingStart, before, after := e.sectionContext(firstEnd); firstLimit == firstEnd && terms.first != "" && after != "" {
			// The stretch runs on to the heading's end so the run after it
			// is the section's own opening, which the contents list's copy
			// of the heading is not followed by.
			terms.first = collapseFields(e.plainText[start:firstEnd])
			terms.bare = collapseFields(e.plainText[bareStart:firstEnd])
			terms.suffix = after
			return terms
		} else if terms.first != "" && before != "" && blockStart <= headingStart {
			// A run named ahead of a term is read only where it ends right
			// at the term, so the stretch is the whole heading, from the
			// edge the previous block's words stand against to its end.
			terms.prefix = before
			terms.first = collapseFields(e.plainText[headingStart:firstEnd])
			terms.bare = terms.first
			return terms
		}
		if terms.first != "" && (verbatim || e.contextSuffixAt(start, bareEnd, firstLimit)) {
			terms.suffix = landingSuffix(e.plainText, bareEnd, firstLimit)
		}
		return terms
	}
	return e.finishCrossingLanding(end, firstEnd, &terms)
}

// finishCrossingLanding measures the last stretch and its following run inside
// the block that holds the match's far end.
func (e *entry) finishCrossingLanding(end, firstEnd int, terms *landingTerms) landingTerms {
	lastStart, lastVerbatim := e.blockAt(end - 1)
	from := max(lastStart, firstEnd)
	lastEnd := e.blockEndAfter(end - 1)
	from, lastEnd = e.insertionBounds(end-1, from, lastEnd)
	// The far end is grown for the reason the first stretch's standalone form
	// is, and inside its own block for the reason that one is: a word cannot
	// be assembled out of two blocks the page draws apart. This stretch opens
	// where that block opens, which is a word's edge already, so that same
	// offset serves as the floor and the only growth is outward.
	_, lastStop := wordEdges(e.plainText, from, end, from, lastEnd)
	terms.last = collapseFields(e.plainText[from:lastStop])
	if lastVerbatim && terms.last != "" {
		terms.suffix = landingSuffix(e.plainText, lastStop, lastEnd)
	}
	return *terms
}

// sectionContext is the run a directive names beside a match inside a
// heading the page shows as written, so that the body's copy of the heading
// answers it and the contents list's copy does not. The list repeats the
// heading's own words, and on a narrow page it stands above the prose, so any
// run taken from inside the heading is answered by the list's copy first.
//
// The first words of the next block are preferred: the list's copy is
// followed by the next entry's name, or its last entry is followed by reading
// aids and then the body's first block. The last words of the previous block
// provide context when a known preceding contents entry cannot answer them.
// Each run comes from adjacent page text, and only where the list cannot
// answer it too. Both empty means the hit keeps the terms any other block
// gives it. headingStart is where the heading's text begins.
func (e *entry) sectionContext(headingEnd int) (headingStart int, before, after string) {
	for i, b := range e.blocks {
		if b.End != headingEnd {
			headingStart = b.End
			continue
		}
		if !b.Heading || e.hasInsertion(headingStart, headingEnd) {
			return headingStart, "", ""
		}
		if i+1 < len(e.blocks) && e.shownAsWritten(i+1) {
			_, openingEnd := e.insertionBounds(headingEnd, headingEnd, e.blocks[i+1].End)
			opening := landingOpening(e.plainText[headingEnd:openingEnd])
			if opening != "" && !e.opensAnotherBlock(opening, i+1) {
				return headingStart, "", opening
			}
		}
		if i > 0 && e.shownAsWritten(i-1) {
			closingStart, _ := e.insertionBounds(headingStart, e.blockStart(i-1), headingStart)
			closing := landingPrefix(e.plainText[closingStart:headingStart])
			if closing != "" && e.closingFollowsAnEntry(closing, i-1) {
				return headingStart, closing, ""
			}
		}
		return headingStart, "", ""
	}
	return 0, "", ""
}

// shownAsWritten reports that block i's corpus words reach the page in this
// order. Insertions are clipped separately before the words name context.
func (e *entry) shownAsWritten(i int) bool {
	return e.blocks[i].Verbatim
}

// blockStart is where block i's text begins.
func (e *entry) blockStart(i int) int {
	if i == 0 {
		return 0
	}
	return e.blocks[i-1].End
}

// opensAnotherBlock reports that the text from the start of some block other
// than block own reads, the way a browser compares (white space as one space,
// letters in either case), as opening. The next entry's name starts a block,
// and so does the body's first block, which the last entry's copy is
// followed by once the page's other reading aids have been passed; an
// opening that also starts a block could be answered beside the list's copy.
// What those aids say is the page's, not the note's, so it is not asked.
func (e *entry) opensAnotherBlock(opening string, own int) bool {
	words := strings.Fields(opening)
	for i := range e.blocks {
		if i != own && readsAs(e.plainText[e.blockStart(i):], words) {
			return true
		}
	}
	return false
}

// readsAs reports that text opens with words, white space between them read
// as one space and case ignored. The last word need only open a word of
// text, which only ever counts more texts as a match.
func readsAs(text string, words []string) bool {
	for i, w := range words {
		text = strings.TrimLeftFunc(text, unicode.IsSpace)
		if len(text) < len(w) || !strings.EqualFold(text[:len(w)], w) {
			return false
		}
		text = text[len(w):]
		if i < len(words)-1 {
			if r, _ := utf8.DecodeRuneInString(text); text == "" || !unicode.IsSpace(r) {
				return false
			}
		}
	}
	return true
}

// closingFollowsAnEntry reports that a contents entry the closing words of
// block prev cannot end is known to stand before the heading's own entry.
// The walk back from prev passes only over blocks the page shows as written,
// none of which adds an entry, to the heading that names the section prev
// sits in; that heading's words are the ones the list writes ahead of the
// heading's copy. Reaching the note's start, or a block that could hide an
// entry — a heading the page changes, an embed — leaves the entry before it
// unknown, as is the first entry's, which follows the list's own label.
func (e *entry) closingFollowsAnEntry(closing string, prev int) bool {
	for i := prev - 1; i >= 0; i-- {
		b := e.blocks[i]
		if b.Heading {
			name := strings.ToLower(collapseFields(e.plainText[e.blockStart(i):b.End]))
			lower := strings.ToLower(closing)
			return !strings.HasSuffix(lower, name) && !strings.HasSuffix(name, lower)
		}
		if !b.Verbatim {
			return false
		}
	}
	return false
}

// landingOpening is the first few words of a block, stopping where a run
// ends at a certain boundary: before a hard line break, and within the word
// and character budget a preceding run keeps. A first word over that budget
// is cut at its first certain boundary instead, the way a following run is.
func landingOpening(block string) string {
	block = strings.TrimLeftFunc(block, unicode.IsSpace)
	if i := strings.IndexByte(block, '\n'); i >= 0 {
		block = block[:i]
	}
	words := strings.Fields(block)
	if len(words) > landingPrefixWords {
		words = words[:landingPrefixWords]
	}
	for len(words) > 0 && utf8.RuneCountInString(strings.Join(words, " ")) > landingPrefixRunes {
		words = words[:len(words)-1]
	}
	if len(words) > 0 {
		return strings.Join(words, " ")
	}
	return landingSuffix(block, 0, len(block))
}

// contextSuffixAt vouches for the match and its following run in one local
// range. A removed delimiter is not a word boundary, so the boundary after
// the run must also be reproduced, unless it is the actual block end.
func (e *entry) contextSuffixAt(start, from, blockEnd int) bool {
	suffix := landingSuffix(e.plainText, from, blockEnd)
	if suffix == "" {
		return false
	}
	stop := from + len(suffix)
	if stop < blockEnd {
		_, size := utf8.DecodeRuneInString(e.plainText[stop:blockEnd])
		stop += size
	}
	for _, b := range e.blocks {
		if b.End < blockEnd {
			continue
		}
		for _, span := range b.ContextRanges {
			if span[0] <= start && stop <= span[1] {
				return true
			}
		}
		return false
	}
	return false
}

// wordEdges grows [start, end) out to the edges of the words it lies inside,
// never leaving [low, high). Only an end whose own character is one a word
// joins can be inside a word at all; every other end already stands on a
// boundary and is left alone. Each end is asked separately. That one condition
// is what keeps a stretch of white space from drawing in the word beside it,
// and what leaves an end resting inside a script that parts no words with
// spaces exactly where it was — the edges there are a segmentation this vault
// has no way to find, and the sentence around the match is not them.
func wordEdges(s string, start, end, low, high int) (grownStart, grownEnd int) {
	grownStart, grownEnd = start, end
	if opening, _ := utf8.DecodeRuneInString(s[start:end]); joinsAWord(opening) {
		for grownStart > low {
			r, size := utf8.DecodeLastRuneInString(s[low:grownStart])
			if !joinsAWord(r) {
				break
			}
			grownStart -= size
		}
	}
	if closing, _ := utf8.DecodeLastRuneInString(s[:end]); joinsAWord(closing) {
		for grownEnd < high {
			r, size := utf8.DecodeRuneInString(s[grownEnd:high])
			if !joinsAWord(r) {
				break
			}
			grownEnd += size
		}
	}
	return grownStart, grownEnd
}

// joinsAWord reports whether r is joined to the character beside it into one
// word, the way Unicode text segmentation (UAX #29) joins them: a letter or a
// digit, in a script that parts its words with spaces. Neither wordRun, which
// only white space ends, nor isWordByte, which keeps a hyphen, answers that.
func joinsAWord(r rune) bool {
	return (unicode.IsLetter(r) || unicode.IsDigit(r)) && !writesWithoutSpaces(r)
}

// landingPrefix cuts a block's run of words before a match down to what one
// term of a directive can carry. Only whole words come off the front, and a
// last word still over the budget comes off with them.
//
// A browser finds a leading run only where it begins at a word boundary, and
// the boundaries this index can be certain of are the ones every segmenter
// agrees on: white space, punctuation, and the start of the block. A cut made
// anywhere else in a script that parts no words with spaces divides them by
// dictionary, which nothing here has a model of, so it falls inside a word
// almost every time — and a run that begins inside a word is not merely
// weaker: it is found nowhere, and the browser abandons the whole directive.
//
// Where the last word is over the budget and is written in such a script, the
// run therefore begins after the earliest certain boundary that leaves it
// inside the budget, and where there is none, after the nearest one before
// it. A last word in any other script is dropped instead: the term that
// follows it is grown to the edges of its own word, so it stands alone
// without one.
func landingPrefix(run string) string {
	words := strings.Fields(run)
	if len(words) > landingPrefixWords {
		words = words[len(words)-landingPrefixWords:]
	}
	for len(words) > 0 && utf8.RuneCountInString(strings.Join(words, " ")) > landingPrefixRunes {
		words = words[1:]
	}
	if len(words) > 0 {
		return strings.Join(words, " ")
	}
	return unspacedPrefix(run)
}

// unspacedPrefix is the run a directive names ahead of a match whose last word
// is over the budget, or empty where that word is not written without spaces.
// It never reaches back over a hard line break: the page draws a break as a
// break, not as the white space it becomes once the run is collapsed, so a
// run spanning one is text the page does not carry in one piece.
func unspacedPrefix(run string) string {
	if i := strings.LastIndex(run, "\n"); i >= 0 {
		run = run[i+1:]
	}
	runes := []rune(collapseFields(run))
	if len(runes) == 0 {
		return ""
	}
	lastWord := runes
	for i, r := range slices.Backward(runes) {
		if r == ' ' {
			lastWord = runes[i+1:]
			break
		}
	}
	if !slices.ContainsFunc(lastWord, writesWithoutSpaces) {
		return ""
	}
	last := len(runes) - 1
	if isCertainBoundary(runes, last) {
		return ""
	}
	for i := max(1, len(runes)-landingPrefixRunes); i <= last; i++ {
		if isCertainBoundary(runes, i-1) {
			return string(runes[i:])
		}
	}
	for i := last - 1; i >= 0; i-- {
		if isCertainBoundary(runes, i) {
			return string(runes[i+1:])
		}
	}
	return string(runes)
}

// landingSuffix cuts the block after a term, text[from:to], down to the
// nearest certain boundary, which is the run itself ending where the browser
// agrees a word ends. A hard line break is one, so the run never reaches over
// it, and with no other the rest of the block is the run, whose end is one.
// Empty where the term already stands against a boundary. The character
// before from is read too, because whether a mark joins a word depends on
// what stands on both sides of it.
func landingSuffix(text string, from, to int) string {
	runes := []rune(text[from:to])
	offset := 0
	if from > 0 {
		before, _ := utf8.DecodeLastRuneInString(text[:from])
		runes = append([]rune{before}, runes...)
		offset = 1
	}
	for i := offset; i < len(runes); i++ {
		if isCertainBoundary(runes, i) {
			return collapseFields(string(runes[offset:i]))
		}
	}
	return collapseFields(string(runes[offset:]))
}

// isCertainBoundary reports whether the character at i, being white space or
// punctuation, is one every text segmenter breaks a word at. A few marks only
// join: an apostrophe or a full stop inside a word, a comma inside a number.
// They are skipped where a letter or digit stands on both sides, and are
// boundaries otherwise, since beside characters of a script without spaces
// they join nothing. Two more join whatever stands beside them, katakana
// included, and are never a boundary.
func isCertainBoundary(runes []rune, i int) bool {
	r := runes[i]
	switch {
	case unicode.IsSpace(r):
		return true
	case !unicode.IsPunct(r), strings.ContainsRune(alwaysJoiningMarks, r):
		return false
	case !strings.ContainsRune(joiningMarks, r):
		return true
	}
	return i == 0 || i == len(runes)-1 || !joinsAWord(runes[i-1]) || !joinsAWord(runes[i+1])
}

// joiningMarks are the punctuation characters that, between two letters or two
// digits, are part of the word around them rather than a break in it.
const joiningMarks = "'\u2018\u2019.\u2024:,;\u00b7\u2027\uff07\uff0e\uff1a\uff0c\uff1b"

// alwaysJoiningMarks are the punctuation characters a word continues through
// on both sides: the katakana middle dot, which katakana words are written
// with, and the low line, which identifiers are.
const alwaysJoiningMarks = "\u30fb_\uff3f"

func collapseFields(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func (e *entry) blockEndAfter(off int) int {
	for _, b := range e.blocks {
		if b.End > off {
			return b.End
		}
	}
	return len(e.plainText)
}

// blockAt is where the block holding off begins, and whether the reading page
// reproduces that block as written. An offset past every block this entry
// knows about is reproduced by nothing anyone here has looked at.
func (e *entry) blockAt(off int) (start int, verbatim bool) {
	prev := 0
	for _, b := range e.blocks {
		if b.End > off {
			return prev, b.Verbatim
		}
		prev = b.End
	}
	return prev, false
}

// landingBlocked reports that inserted page text interrupts a source span.
// Normalization barriers also withhold any overlapping span.
func (e *entry) landingBlocked(start, end int) bool {
	for _, at := range e.insertions {
		if start < at && at < end {
			return true
		}
	}
	for _, span := range e.insertionBarriers {
		if start < span[1] && end > span[0] {
			return true
		}
	}
	return false
}

// insertionBounds clips a run to the adjacent text the page keeps on the
// same side of every inserted label. An edge at the match belongs to its side.
func (e *entry) insertionBounds(at, low, high int) (floor, ceiling int) {
	for _, edge := range e.insertions {
		if edge <= at {
			low = max(low, edge)
		} else {
			high = min(high, edge)
		}
	}
	return low, high
}

func (e *entry) hasInsertion(start, end int) bool {
	for _, edge := range e.insertions {
		if start < edge && edge <= end {
			return true
		}
	}
	return false
}
