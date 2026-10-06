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
		span.Replacement = norm.NFC.String(span.Replacement)
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
	replaced   bool
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
		hidden, deleted, replacement := e.excerptEffects(at, stop)
		if replacement != nil {
			stop = min(end, replacement.End)
			for _, span := range e.displaySpans {
				if span.Start < replacement.End && span.End > replacement.Start {
					hidden = hidden || span.Hidden
					deleted = deleted || span.Deleted
				}
			}
			// An atomic entity/tag crossing a cut contributes no fragment of
			// its source or decoded value. The original window never expands.
			// An earlier replacement owns any overlap. A later atomic value
			// cannot repeat source whose opening was already consumed.
			if !hidden && replacement.Start == at && replacement.Start >= start && replacement.End <= end {
				atoms = append(atoms, e.replacementAtoms(replacement, tokens, deleted)...)
			}
		} else if !hidden {
			atoms = append(atoms, excerptAtom{text: e.PlainText[at:stop], start: at, end: stop, hit: anyExcerptHit(covered, at-start, stop-start), deleted: deleted})
		}
		at = stop
	}
	return atoms
}

func (e *entry) excerptEffects(at, stop int) (hidden, deleted bool, replacement *render.DisplaySpan) {
	for i := range e.displaySpans {
		span := &e.displaySpans[i]
		if span.Start >= stop || span.End <= at {
			continue
		}
		hidden = hidden || span.Hidden
		deleted = deleted || span.Deleted
		if span.Replacement != "" && (replacement == nil || span.Start < replacement.Start) {
			replacement = span
		}
	}
	return hidden, deleted, replacement
}

func firstExcerptHit(atoms []excerptAtom) (first, last int) {
	first, last = -1, -1
	for _, atom := range atoms {
		if !atom.hit || atom.replaced {
			if first >= 0 {
				// Unmarked text or a replacement ends the first shown occurrence.
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

func (e *entry) replacementAtoms(span *render.DisplaySpan, tokens []string, deleted bool) []excerptAtom {
	covered := make([]bool, len(span.Replacement))
	for _, token := range tokens {
		// Only a token with evidence in this very source range may mark
		// its decoded value. Another token matching elsewhere is no proof.
		if len(MarkHits(e.PlainText[span.Start:span.End], []string{token})) == 0 {
			continue
		}
		at := 0
		for _, run := range MarkHits(span.Replacement, []string{token}) {
			if run.Hit {
				for i := range len(run.Text) {
					covered[at+i] = true
				}
			}
			at += len(run.Text)
		}
	}
	var atoms []excerptAtom
	for at := 0; at < len(span.Replacement); {
		_, size := utf8.DecodeRuneInString(span.Replacement[at:])
		stop := at + size
		atoms = append(atoms, excerptAtom{text: span.Replacement[at:stop], start: span.Start, end: span.End, hit: anyExcerptHit(covered, at, stop), deleted: deleted, replaced: true})
		at = stop
	}
	return atoms
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
	space.replaced = space.replaced || atom.replaced
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

// shownOccurrence keeps a display centre and its landing eligibility together.
// Decoded atoms may choose a window, but never name original page coordinates.
type shownOccurrence struct {
	start, end int
	replaced   bool
	found      bool
	fromFence  bool
}

type excerptDisplayIndex struct {
	hidden       [][2]int
	replacements []render.DisplaySpan
}

// displayIndex merges only visibility intervals. Replacement order retains the
// first owner's atomic semantics; an overlapping tail cannot emit a new atom.
func (e *entry) displayIndex() excerptDisplayIndex {
	var index excerptDisplayIndex
	for _, span := range e.displaySpans {
		if span.Hidden {
			index.hidden = append(index.hidden, [2]int{span.Start, span.End})
		}
		if span.Replacement != "" {
			index.replacements = append(index.replacements, span)
		}
	}
	slices.SortStableFunc(index.replacements, func(a, b render.DisplaySpan) int { return cmp.Compare(a.Start, b.Start) })
	atomic := index.replacements[:0]
	consumed := 0
	for _, span := range index.replacements {
		if span.Start < consumed {
			if span.End > consumed {
				index.hidden = append(index.hidden, [2]int{consumed, span.End})
			}
		} else {
			atomic = append(atomic, span)
		}
		consumed = max(consumed, span.End)
	}
	index.replacements = atomic
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

func (index *excerptDisplayIndex) replacementAt(start, end int) *render.DisplaySpan {
	i := sort.Search(len(index.replacements), func(i int) bool { return index.replacements[i].End > start })
	if i < len(index.replacements) && index.replacements[i].Start < end {
		return &index.replacements[i]
	}
	return nil
}

func (e *entry) shownReplacement(span *render.DisplaySpan, token string, index *excerptDisplayIndex) bool {
	_, hidden := index.hiddenAt(span.Start, span.End)
	if hidden || strings.TrimSpace(span.Replacement) == "" {
		return false
	}
	return token == "" || len(MarkHits(e.PlainText[span.Start:span.End], []string{token})) > 0 && len(MarkHits(span.Replacement, []string{token})) > 0
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
		if span := index.replacementAt(at, at+size); span != nil {
			if e.shownReplacement(span, token, index) {
				return shownOccurrence{start: span.Start, end: span.End, replaced: true, found: true}
			}
			at = min(end, span.End)
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
		i = sort.Search(len(index.replacements), func(i int) bool { return index.replacements[i].End > at })
		if i < len(index.replacements) {
			stop = min(stop, index.replacements[i].Start)
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
