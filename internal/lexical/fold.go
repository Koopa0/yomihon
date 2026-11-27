package lexical

import (
	"iter"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/width"

	"github.com/koopa0/yomihon/internal/vault"
)

// Fold is the single definer of "what counts as a match": NFC, then the walk
// below, applied identically to stored text and to a query token. Case and
// width folding live only here; vault.NormalizeNFC supplies the shared NFC step.
func Fold(s string) string {
	var out strings.Builder
	out.Grow(len(s))
	for _, r := range foldRunes(vault.NormalizeNFC(s)) {
		out.WriteRune(r)
	}
	return out.String()
}

// foldRunes walks already-NFC text and yields each rune the fold keeps, folded,
// with the byte offset in s it came from. It is the whole of what folding does, so
// the folded string and the folded string mapped back to its source cannot
// disagree about what a match is. A line break is dropped only when the characters
// on both sides belong to scripts that do not divide their words with spaces.
// Walking from any offset it yielded gives the rest of the same answer.
func foldRunes(s string) iter.Seq2[int, rune] {
	return func(yield func(int, rune) bool) {
		var prev rune
		for i, r := range s {
			if r == '\n' && writesWithoutSpaces(prev) && writesWithoutSpaces(nextRune(s, i+1)) {
				continue
			}
			if !yield(i, foldRune(r)) {
				return
			}
			prev = r
		}
	}
}

// fullwidthASCIIMin and fullwidthASCIIMax bound the block foldRune narrows.
// Each of those runes has a one-rune halfwidth counterpart, so the fold's
// source-offset walk stays a walk of one emitted rune per source rune.
// Halfwidth katakana is outside it: a voiced mark beside one folds two runes
// to one and is a separate decision.
const (
	fullwidthASCIIMin = 0xFF01
	fullwidthASCIIMax = 0xFF5E
)

// foldRune is the per-character half of fold: the fullwidth ASCII block
// narrows to its halfwidth counterpart, then a fixed simple-case representative.
// Width first so a fullwidth letter and its ASCII counterpart meet before
// either is case-folded.
func foldRune(r rune) rune {
	if r >= fullwidthASCIIMin && r <= fullwidthASCIIMax {
		if n := width.LookupRune(r).Narrow(); n != 0 {
			r = n
		}
	}
	if r <= unicode.MaxASCII {
		return unicode.ToLower(r)
	}
	return simpleCaseRune(r)
}

// simpleCaseRune prefers the smallest letter in the orbit, lowered only if
// that lowercase remains a member. Greek iota's smallest member is a combining
// mark: emitting it after a vowel could create a new NFC contraction. Dotted
// capital I instead has no simple lowercase peer and stays in its own orbit.
func simpleCaseRune(r rune) rune {
	smallest := r
	for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
		letter, chosenLetter := unicode.IsLetter(next), unicode.IsLetter(smallest)
		if (letter && !chosenLetter) || (letter == chosenLetter && next < smallest) {
			smallest = next
		}
	}
	lower := unicode.ToLower(smallest)
	if strings.EqualFold(string(smallest), string(lower)) {
		return lower
	}
	return smallest
}

// nextRune returns the first rune at or after i, or zero at the end of s.
func nextRune(s string, i int) rune {
	for _, r := range s[i:] {
		return r
	}
	return 0
}

// writesWithoutSpaces reports whether a character belongs to a script that does
// not part its words with spaces, the distinction Unicode text segmentation
// (UAX #29) draws. Han, hiragana and katakana carry it. Hangul does not, and
// its exclusion is that script property rather than a case left for later:
// modern Korean divides its words with spaces, so closing a wrapped seam
// would fuse two words.
func writesWithoutSpaces(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r)
}

// foldWithSourceOffsets folds s and maps every byte position of the folded
// copy, one past its end included, back to the byte offset in s of the character
// it came from. It applies the walk half of the index's fold alone, which
// reproduces that fold provided s is already NFC, as every indexed text is.
func foldWithSourceOffsets(s string) (fold string, src []int) {
	var folded strings.Builder
	folded.Grow(len(s))
	src = make([]int, 0, len(s)+1)
	for at, r := range foldRunes(s) {
		n := folded.Len()
		folded.WriteRune(r)
		for ; n < folded.Len(); n++ {
			src = append(src, at)
		}
	}
	return folded.String(), append(src, len(s))
}

// sourceOffsetOfFold maps one byte offset in the folded copy of s back to the
// byte offset in s of the character that produced it, and len(s) past the end.
// It answers exactly what foldWithSourceOffsets tabulates, walked to one
// position rather than materialized because its caller measures a whole note.
// A test holds the two to one answer.
func sourceOffsetOfFold(s string, foldOff int) int {
	cursor := foldCursor{s: s}
	return cursor.source(foldOff)
}

// sourceEndOfFold maps the exclusive end of a folded match back to the exclusive
// source offset of its last rune. sourceOffsetOfFold at that same fold end would
// name the next kept rune, which can sit past a dropped break and is not the
// match.
func sourceEndOfFold(s string, foldEnd int) int {
	if foldEnd <= 0 {
		return 0
	}
	at := sourceOffsetOfFold(s, foldEnd-1)
	_, size := utf8.DecodeRuneInString(s[at:])
	return at + size
}

// foldCursor maps offsets in the folded copy of s back to s for offsets that
// never decrease: each walk resumes on the character the last one stopped at,
// so mapping every match of a term in a note walks the note once, and only as
// far as the last match asked about.
type foldCursor struct {
	s string
	// src is a character the fold keeps and fold the offset its folded form
	// begins at; both are zero before the first walk.
	src, fold int
}

// source answers sourceOffsetOfFold(c.s, foldOff) for a foldOff no smaller
// than the last one asked.
func (c *foldCursor) source(foldOff int) int {
	folded := c.fold
	for at, r := range foldRunes(c.s[c.src:]) {
		next := folded + utf8.RuneLen(r)
		if next > foldOff {
			c.src, c.fold = c.src+at, folded
			return c.src
		}
		folded = next
	}
	c.src, c.fold = len(c.s), folded
	return c.src
}
