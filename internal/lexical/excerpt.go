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

func (e *entry) excerptAtoms(start, end int, tokens []string) []excerptAtom {
	covered := excerptSourceHits(e.plainText[start:end], tokens)
	var atoms []excerptAtom
	for at := start; at < end; {
		_, size := utf8.DecodeRuneInString(e.plainText[at:end])
		stop := at + size
		hidden, deleted := e.excerptEffects(at, stop)
		if !hidden {
			atoms = append(atoms, excerptAtom{text: e.plainText[at:stop], start: at, end: stop, hit: anyExcerptHit(covered, at-start, stop-start), deleted: deleted})
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

// hiddenSpans are the merged, ordered source spans a page hides, so a scan can
// jump each one whole.
type hiddenSpans [][2]int

func (e *entry) hiddenSpans() hiddenSpans {
	var spans hiddenSpans
	for _, span := range e.displaySpans {
		if span.Hidden {
			spans = append(spans, [2]int{span.Start, span.End})
		}
	}
	slices.SortFunc(spans, func(a, b [2]int) int { return cmp.Compare(a[0], b[0]) })
	merged := spans[:0]
	for _, span := range spans {
		if len(merged) > 0 && span[0] <= merged[len(merged)-1][1] {
			merged[len(merged)-1][1] = max(merged[len(merged)-1][1], span[1])
		} else {
			merged = append(merged, span)
		}
	}
	return merged
}

// next is the first span that ends after at, or false when none does.
func (h hiddenSpans) next(at int) ([2]int, bool) {
	i := sort.Search(len(h), func(i int) bool { return h[i][1] > at })
	if i == len(h) {
		return [2]int{}, false
	}
	return h[i], true
}

// firstShown inspects original evidence, jumping hidden ranges instead of
// projecting every byte. Only the bounded excerpt later allocates text atoms.
func (e *entry) firstShown(start, end int, token string, hidden hiddenSpans) shownOccurrence {
	for at := start; at < end; {
		_, size := utf8.DecodeRuneInString(e.plainText[at:end])
		next, found := hidden.next(at)
		if found && next[0] < at+size {
			at = min(end, next[1])
			continue
		}
		if strings.TrimSpace(e.plainText[at:at+size]) == "" {
			at += size
			continue
		}
		stop := end
		if found {
			stop = min(stop, next[0])
		}
		if token == "" {
			stop = at + size
		}
		return shownOccurrence{start: at, end: stop, found: true}
	}
	return shownOccurrence{}
}

// shownToken is token's earliest match the page shows in prose, and its
// earliest in a fence.
func (e *entry) shownToken(token string, hidden hiddenSpans) (prose, fence shownOccurrence) {
	cursor := foldCursor{s: e.plainText}
	for at := 0; at <= len(e.plainFold); {
		start, end := phraseIndex(e.plainFold, token, at)
		if start < 0 {
			return prose, fence
		}
		at = max(end, start+1)
		from := cursor.source(start)
		last := cursor.source(end - 1)
		_, size := utf8.DecodeRuneInString(e.plainText[last:])
		shown := e.firstShown(from, last+size, token, hidden)
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

// shownExcerptCentre is the earliest match of any token the page shows, in
// prose when there is one and otherwise in a fence; found is false when the
// page shows none.
func (e *entry) shownExcerptCentre(tokens []string) shownOccurrence {
	hidden := e.hiddenSpans()
	var prose, fence shownOccurrence
	for _, token := range tokens {
		if token == "" {
			continue
		}
		shownProse, shownFence := e.shownToken(token, hidden)
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
	return fence
}
