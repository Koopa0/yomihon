package lexical

import (
	"cmp"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/koopa0/yomihon/internal/render"
)

// ExcerptRun is visible excerpt text carrying query-hit and retraction meaning.
// Query hits come from the original corpus; display cleanup creates no matches.
type ExcerptRun struct {
	Text    string
	Hit     bool
	Deleted bool
}

func remapDisplaySpans(raw string, spans []render.DisplaySpan) []render.DisplaySpan {
	if len(spans) == 0 {
		return nil
	}
	// Normalization can compose or reorder a whole combining segment. Only
	// segment edges and unchanged prefixes/suffixes have provable positions.
	positions := nfcDisplayPositions(raw)
	var mapped []render.DisplaySpan
	for _, span := range spans {
		if span.Start < 0 || span.End > len(raw) || span.Start >= span.End {
			continue
		}
		start, end := positions[span.Start], positions[span.End]
		if start < 0 || end <= start {
			continue
		}
		span.Start, span.End = start, end
		mapped = append(mapped, span)
	}
	return mapped
}

func nfcDisplayPositions(raw string) []int {
	positions := make([]int, len(raw)+1)
	for i := range positions {
		positions[i] = -1
	}
	positions[0] = 0
	var iterator norm.Iter
	iterator.InitString(norm.NFC, raw)
	stored := 0
	for !iterator.Done() {
		start := iterator.Pos()
		normalized := string(iterator.Next())
		end := iterator.Pos()
		original := raw[start:end]
		positions[start], positions[end] = stored, stored+len(normalized)
		prefix := 0
		for prefix < len(original) && prefix < len(normalized) && original[prefix] == normalized[prefix] {
			prefix++
		}
		for i := 1; i <= prefix; i++ {
			if excerptBoundary(original, i) && excerptBoundary(normalized, i) {
				positions[start+i] = stored + i
			}
		}
		suffix := 0
		for suffix < len(original)-prefix && suffix < len(normalized)-prefix && original[len(original)-suffix-1] == normalized[len(normalized)-suffix-1] {
			suffix++
		}
		for i := 1; i <= suffix; i++ {
			rawAt, normalizedAt := len(original)-i, len(normalized)-i
			if excerptBoundary(original, rawAt) && excerptBoundary(normalized, normalizedAt) {
				positions[start+rawAt] = stored + normalizedAt
			}
		}
		stored += len(normalized)
	}
	return positions
}

func excerptBoundary(text string, at int) bool {
	return at == 0 || at == len(text) || at > 0 && at < len(text) && utf8.RuneStart(text[at])
}

type excerptAtom struct {
	text       string
	start, end int
	hit        bool
	deleted    bool
}

// excerptSourceHits marks the original window before whitespace is collapsed.
// Folding can join a CJK soft wrap; inserting a space first loses that evidence.
func excerptSourceHits(plain string, tokens []string) []bool {
	covered := make([]bool, len(plain))
	at := 0
	for _, run := range MarkHits(plain, tokens) {
		if run.Hit {
			for i := range len(run.Text) {
				covered[at+i] = true
			}
		}
		at += len(run.Text)
	}
	return covered
}

func (e *entry) displayedExcerpt(start, end int, tokens []string) (runs []ExcerptRun, first, last int) {
	atoms := collapseExcerptAtoms(e.excerptAtoms(start, end, tokens))
	first, last = firstExcerptHit(atoms)
	// Run assembly is independent of selecting the first landing occurrence.
	if start > 0 {
		appendExcerptRun(&runs, "…", false, false)
	}
	for _, atom := range atoms {
		appendExcerptRun(&runs, atom.text, atom.hit, atom.deleted)
	}
	if end < len(e.PlainText) {
		appendExcerptRun(&runs, "…", false, false)
	}
	return runs, first, last
}

func (e *entry) excerptAtoms(start, end int, tokens []string) []excerptAtom {
	covered := excerptSourceHits(e.PlainText[start:end], tokens)
	var atoms []excerptAtom
	for at := start; at < end; {
		_, size := utf8.DecodeRuneInString(e.PlainText[at:end])
		stop := at + size
		hidden, deleted := e.excerptEffects(at, stop)
		if !hidden {
			atoms = append(atoms, excerptAtom{text: e.PlainText[at:stop], start: at, end: stop, hit: anyExcerptHit(covered, at-start, stop-start), deleted: deleted})
		}
		at = stop
	}
	return atoms
}

func (e *entry) excerptEffects(at, stop int) (hidden, deleted bool) {
	for i := range e.displaySpans {
		span := &e.displaySpans[i]
		if span.Start >= stop || span.End <= at {
			continue
		}
		hidden = hidden || span.Hidden
		deleted = deleted || span.Deleted
	}
	return hidden, deleted
}

func firstExcerptHit(atoms []excerptAtom) (first, last int) {
	first, last = -1, -1
	for _, atom := range atoms {
		if !atom.hit {
			if first >= 0 {
				// Unmarked text ends the first shown occurrence.
				break
			}
			continue
		}
		if first < 0 && strings.TrimSpace(atom.text) == "" {
			continue
		}
		switch {
		case first < 0:
			first, last = atom.start, atom.end
		case atom.start == last:
			last = atom.end
		default:
			return first, last
		}
	}
	return first, last
}

func anyExcerptHit(covered []bool, start, end int) bool {
	for _, hit := range covered[start:end] {
		if hit {
			return true
		}
	}
	return false
}

func collapseExcerptAtoms(atoms []excerptAtom) []excerptAtom {
	var out []excerptAtom
	var space *excerptAtom
	for _, atom := range atoms {
		r, _ := utf8.DecodeRuneInString(atom.text)
		if unicode.IsSpace(r) {
			if len(out) == 0 {
				continue
			}
			space = mergeExcerptSpace(space, atom)
			continue
		}
		if space != nil {
			out = append(out, *space)
			space = nil
		}
		out = append(out, atom)
	}
	return out
}

func mergeExcerptSpace(space *excerptAtom, atom excerptAtom) *excerptAtom {
	if space == nil {
		atom.text = " "
		return &atom
	}
	space.end, space.hit = atom.end, space.hit || atom.hit
	space.deleted = space.deleted && atom.deleted
	return space
}

func appendExcerptRun(runs *[]ExcerptRun, text string, hit, deleted bool) {
	if text == "" {
		return
	}
	if n := len(*runs); n > 0 && (*runs)[n-1].Hit == hit && (*runs)[n-1].Deleted == deleted {
		(*runs)[n-1].Text += text
		return
	}
	*runs = append(*runs, ExcerptRun{Text: text, Hit: hit, Deleted: deleted})
}

// shownOccurrence keeps a display centre and where it was found together.
type shownOccurrence struct {
	start, end int
	found      bool
	fromFence  bool
}

type excerptDisplayIndex struct {
	hidden [][2]int
}

// displayIndex merges the hidden intervals, so a scan can jump each one whole.
func (e *entry) displayIndex() excerptDisplayIndex {
	var index excerptDisplayIndex
	for _, span := range e.displaySpans {
		if span.Hidden {
			index.hidden = append(index.hidden, [2]int{span.Start, span.End})
		}
	}
	slices.SortFunc(index.hidden, func(a, b [2]int) int { return cmp.Compare(a[0], b[0]) })
	merged := index.hidden[:0]
	for _, span := range index.hidden {
		if len(merged) > 0 && span[0] <= merged[len(merged)-1][1] {
			merged[len(merged)-1][1] = max(merged[len(merged)-1][1], span[1])
		} else {
			merged = append(merged, span)
		}
	}
	index.hidden = merged
	return index
}

func (index *excerptDisplayIndex) hiddenAt(start, end int) (stop int, hidden bool) {
	i := sort.Search(len(index.hidden), func(i int) bool { return index.hidden[i][1] > start })
	if i < len(index.hidden) && index.hidden[i][0] < end {
		return index.hidden[i][1], true
	}
	return 0, false
}

// firstShown inspects original evidence, jumping hidden ranges instead of
// projecting every byte. Only the bounded excerpt later allocates text atoms.
func (e *entry) firstShown(start, end int, token string, index *excerptDisplayIndex) shownOccurrence {
	for at := start; at < end; {
		_, size := utf8.DecodeRuneInString(e.PlainText[at:end])
		if stop, hidden := index.hiddenAt(at, at+size); hidden {
			at = min(end, stop)
			continue
		}
		if strings.TrimSpace(e.PlainText[at:at+size]) == "" {
			at += size
			continue
		}
		stop := end
		i := sort.Search(len(index.hidden), func(i int) bool { return index.hidden[i][1] > at })
		if i < len(index.hidden) {
			stop = min(stop, index.hidden[i][0])
		}
		if token == "" {
			stop = at + size
		}
		return shownOccurrence{start: at, end: stop, found: true}
	}
	return shownOccurrence{}
}

func (e *entry) shownToken(token string, source []int, index *excerptDisplayIndex) (prose, fence shownOccurrence) {
	for at := 0; at <= len(e.PlainFold); {
		start, end := phraseIndex(e.PlainFold, token, at)
		if start < 0 {
			return prose, fence
		}
		at = max(end, start+1)
		last := source[end-1]
		_, size := utf8.DecodeRuneInString(e.PlainText[last:])
		shown := e.firstShown(source[start], last+size, token, index)
		if !shown.found {
			continue
		}
		shown.fromFence = inFenceRange(start, e.fenceFoldRanges)
		if !shown.fromFence {
			return shown, fence
		}
		if !fence.found || shown.start < fence.start {
			fence = shown
		}
	}
	return prose, fence
}

func (e *entry) shownExcerptCentre(tokens []string) shownOccurrence {
	index := e.displayIndex()
	_, source := foldWithSourceOffsets(e.PlainText)
	var prose, fence shownOccurrence
	for _, token := range tokens {
		shownProse, shownFence := e.shownToken(token, source, &index)
		if shownProse.found && (!prose.found || shownProse.start < prose.start) {
			prose = shownProse
		}
		if shownFence.found && (!fence.found || shownFence.start < fence.start) {
			fence = shownFence
		}
	}

	if prose.found {
		return prose
	}
	if fence.found {
		return fence
	}
	opening := e.firstShown(0, len(e.PlainText), "", &index)
	// Preserve the whole opening when it already fits the initial window;
	// a long hidden prefix alone must not leave that window empty.
	if opening.start < runesAfter(e.PlainText, 0, snippetAfter) {
		opening.start = 0
	}
	opening.found = false
	return opening
}
