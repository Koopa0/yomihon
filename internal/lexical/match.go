package lexical

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Result is one search hit: the note's path, display title, optional status
// badge, and a snippet centered on the earliest matched-token offset. Status is
// empty when the hit is not metadata-capable.
type Result struct {
	RelPath string
	Title   string
	Status  string
	Snippet string

	// Alias is the name that answered the query when it was not the title, in
	// the note's own spelling, and empty otherwise. The row shows a title, so
	// without it a reader who typed another name sees no reason for the hit.
	Alias string

	// NoteType is the note's own declared type, carried beside Status because a
	// status value is declared per type. It is blanked with Status when the entry
	// may not answer metadata projections.
	NoteType string

	// File marks a hit that is not a note but a vault file read as characters.
	// It carries no lifecycle state and never will, so a surface that dresses a
	// hit in note furniture can tell the two apart.
	File bool
}

const (
	// snippetBefore/snippetAfter bound the snippet window around the earliest
	// matched-token offset, counted in characters the reader sees. A byte budget
	// paid out by script, buying a reader of Chinese a third of the context.
	snippetBefore = 40
	snippetAfter  = 160
)

// SearchN runs a parsed query against the index and returns results in the final
// deterministic order, six groups concatenated: a note's title hits, a note's
// body hits, the same two over vault files that are not notes, then the
// path-only hits, notes again before files. Entries are kept in the vault's
// reading order, so each group carries it and no sort is needed, and every text
// hit outranks every path-only hit.
//
// An empty query returns nothing and a pure-filter query lands every match in
// the title bucket. A metadata filter excludes non-instance artifacts, and
// returns ErrMetadataUnavailable when the artifact policy was declared and could
// not be honoured. At most limit results are materialized, a negative limit all
// of them; total counts every hit, and the tail beyond limit is never built.
func (idx *Index) SearchN(q *Query, limit int) (results []Result, total int, err error) {
	if len(q.tokens) == 0 && len(q.filters) == 0 {
		return nil, 0, nil
	}
	metadataAvailable := idx.policy.Trustworthy()
	requiresMetadata := q.RequiresMetadata()
	if requiresMetadata && !metadataAvailable {
		return nil, 0, idx.metadataUnavailableError()
	}
	var answers resultBuckets
	for _, e := range idx.entries {
		if requiresMetadata && !e.metadataCapable {
			continue
		}
		if !e.matchesFilters(q.filters) {
			continue
		}
		answers.place(e, q.tokens)
	}
	hits := answers.ordered()
	total = len(hits)
	if limit >= 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	results = make([]Result, len(hits))
	for i, h := range hits {
		results[i] = h.entry.result(q.tokens, h.bodyEvidence, metadataAvailable, h.alias)
	}
	return results, total, nil
}

// hit is one matched entry before its Result is built. Buckets hold hits
// rather than Results so a bounded search can count every match while
// building — snippet included — only the results it will return.
type hit struct {
	entry        *entry
	bodyEvidence bool
	// alias is the name that answered the query when it was not the title, in
	// the note's own spelling, since the row itself shows a title.
	alias string
}

// bucket names one answer group. This declaration order is the result order and
// the only statement of it, so a group moves by moving its constant: notes before
// files under each kind of evidence, and both kinds of text before an address.
type bucket uint8

const (
	titleNote bucket = iota
	bodyNote
	titleFile
	bodyFile
	pathNote
	pathFile
	bucketCount
)

// resultBuckets keeps the answer groups apart while one pass fills them, so the
// final order is a concatenation rather than a sort.
type resultBuckets struct {
	groups [bucketCount][]hit
}

// place files one filter-matching entry into its answer group by what the
// tokens matched: the title, the body, or only the path.
func (b *resultBuckets) place(e *entry, tokens []string) {
	switch {
	case allContain(e.TitleFold, tokens):
		bodyEvidence := len(tokens) != 0 && allContain(e.PlainFold, tokens)
		b.add(titleNote, titleFile, hit{entry: e, bodyEvidence: bodyEvidence})
	case aliasAnswering(e, tokens) != "":
		// An alias stands with the title: a link written to one resolves and a
		// link written to a title does not.
		bodyEvidence := len(tokens) != 0 && allContain(e.PlainFold, tokens)
		b.add(titleNote, titleFile, hit{entry: e, bodyEvidence: bodyEvidence, alias: aliasAnswering(e, tokens)})
	case allContain(e.PlainFold, tokens):
		b.add(bodyNote, bodyFile, hit{entry: e, bodyEvidence: true})
	case allContain(e.PathFold, tokens):
		b.add(pathNote, pathFile, hit{entry: e})
	}
}

// add files one hit under the group its evidence and its kind put it in. The hit
// carries which kind it is, so the caller names the pair and never chooses.
func (b *resultBuckets) add(note, file bucket, h hit) {
	g := note
	if h.entry.isFile {
		g = file
	}
	b.groups[g] = append(b.groups[g], h)
}

// ordered flattens the groups into the answer, in the order the constants are
// declared in.
func (b *resultBuckets) ordered() []hit {
	return slices.Concat(b.groups[:]...)
}

// aliasAnswering returns the note's own spelling of the first alias that holds
// every token, or empty when none does. The first is taken because an author
// listing several puts the one they think of first at the front.
func aliasAnswering(e *entry, tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	for i, folded := range e.AliasFolds {
		if allContain(folded, tokens) {
			return e.Aliases[i]
		}
	}
	return ""
}

// allContain reports whether hay contains every token (AND). Tokens are folded
// and hay is a *Fold field, so this is a literal substring test with no wildcards,
// except that whitespace in a token matches any whitespace. Zero tokens is true.
func allContain(hay string, tokens []string) bool {
	for _, t := range tokens {
		if start, _ := phraseIndex(hay, t, 0); start < 0 {
			return false
		}
	}
	return true
}

// phraseIndex reports the byte range of the first occurrence of token in hay at or
// after from, treating every run of whitespace inside token as a match for any run
// of whitespace in hay; start is -1 when token does not occur. That flexibility is
// what lets a quoted phrase be answered at all, since this vault's prose is
// hard-wrapped and the indexed text keeps the breaks. Adjacency is still required,
// and one block is parted from the next by a break too, so a phrase can join them.
func phraseIndex(hay, token string, from int) (start, end int) {
	if from < 0 || from > len(hay) {
		return -1, -1
	}
	head := token[:wordRun(token, 0)]
	if len(head) == len(token) {
		i := strings.Index(hay[from:], token)
		if i < 0 {
			return -1, -1
		}
		return from + i, from + i + len(token)
	}
	for at := from; at <= len(hay); {
		i := strings.Index(hay[at:], head)
		if i < 0 {
			return -1, -1
		}
		i += at
		if stop, ok := phraseAt(hay, token, i); ok {
			return i, stop
		}
		at = i + 1
	}
	return -1, -1
}

// phraseAt matches token against hay starting at pos, taking each run of
// whitespace in token as a demand for whitespace rather than for those exact
// characters, and reports where the match ends.
func phraseAt(hay, token string, pos int) (end int, ok bool) {
	for t := 0; t < len(token); {
		if run := whitespaceRun(token, t); run > 0 {
			t += run
			run = whitespaceRun(hay, pos)
			if run == 0 {
				return 0, false
			}
			pos += run
			continue
		}
		word := token[t : t+wordRun(token, t)]
		if !strings.HasPrefix(hay[pos:], word) {
			return 0, false
		}
		pos += len(word)
		t += len(word)
	}
	return pos, true
}

// whitespaceRun reports the byte length of the run of whitespace beginning at
// pos, zero when nothing there is whitespace.
func whitespaceRun(s string, pos int) int {
	n := 0
	for pos+n < len(s) {
		r, size := utf8.DecodeRuneInString(s[pos+n:])
		if !unicode.IsSpace(r) {
			break
		}
		n += size
	}
	return n
}

// wordRun reports the byte length of the run of non-whitespace beginning at
// pos, zero when whitespace begins there.
func wordRun(s string, pos int) int {
	n := 0
	for pos+n < len(s) {
		r, size := utf8.DecodeRuneInString(s[pos+n:])
		if unicode.IsSpace(r) {
			break
		}
		n += size
	}
	return n
}

// matchesFilters reports whether e satisfies every filter (a repeated key is
// therefore an AND: two "type:" filters both must hold, so they are jointly
// unsatisfiable rather than last-wins).
func (e *entry) matchesFilters(filters []Filter) bool {
	for _, f := range filters {
		if !e.matchesFilter(f) {
			return false
		}
	}
	return true
}

// matchesFilter reports whether e satisfies one filter. type/status/domain/slug
// are exact equality on the NFC field; topic is exact membership of Topics;
// folder is a rel_path prefix at a "/" boundary, so "folder:Writing" matches
// "Writing" and "Writing/x.md" but never "Writing-old/x.md".
func (e *entry) matchesFilter(f Filter) bool {
	switch f.Key {
	case "type":
		return e.NoteType == f.Value
	case "status":
		return e.Status == f.Value
	case "domain":
		return e.Domain == f.Value
	case "slug":
		return e.Slug == f.Value
	case "topic":
		return slices.Contains(e.Topics, f.Value)
	case "folder":
		return e.RelPath == f.Value || strings.HasPrefix(e.RelPath, f.Value+"/")
	default:
		// A filter reaches this only where Parse recognized its key, and Parse
		// recognizes exactly the keys the grammar table holds; a Query keeps
		// its filters unexported, so nothing else can introduce one. What this
		// arm cannot catch is a key added to that table with no arm here: it
		// would parse, be offered on the page, and match nothing without
		// saying so. The two sets are compared in a test instead.
		return false
	}
}

// result builds a Result for e, with a snippet centered on the earliest
// matched-token offset.
func (e *entry) result(tokens []string, bodyEvidence, metadataAvailable bool, alias string) Result {
	status, noteType := e.Status, e.NoteType
	if !metadataAvailable || !e.metadataCapable {
		status, noteType = "", ""
	}
	var bodySnippet string
	if bodyEvidence {
		bodySnippet = snippet(e.PlainText, e.PlainFold, tokens)
	}
	return Result{
		RelPath:  e.RelPath,
		Title:    e.Title,
		Status:   status,
		Snippet:  bodySnippet,
		Alias:    alias,
		NoteType: noteType,
		File:     e.isFile,
	}
}

// runesBefore returns the byte offset n characters back from off, and the start
// of the text when there are fewer than n. It walks characters rather than
// subtracting bytes so the window is the same size to every reader.
func runesBefore(s string, off, n int) int {
	if off > len(s) {
		off = len(s)
	}
	for range n {
		if off <= 0 {
			return 0
		}
		_, size := utf8.DecodeLastRuneInString(s[:off])
		off -= size
	}
	return off
}

// runesAfter returns the byte offset n characters forward from off, and the end
// of the text when there are fewer than n.
func runesAfter(s string, off, n int) int {
	if off < 0 {
		off = 0
	}
	for range n {
		if off >= len(s) {
			return len(s)
		}
		_, size := utf8.DecodeRuneInString(s[off:])
		off += size
	}
	return off
}

// snippet returns a one-line window of plain around the earliest matched-token
// offset. That offset is found on the folded copy, and lowercasing does not
// preserve length, so it comes back through the fold's own mapping: used directly
// it drifts until the window slides clear of the term it was placed around.
func snippet(plain, plainFold string, tokens []string) string {
	off := sourceOffsetOfFold(plain, earliestOffset(plainFold, tokens))
	// Neither boundary may move past the match it was placed around: a match buried
	// deep in one unbroken run can be stepped over by both at once, reversing the
	// slice. The sentence-start reach runs first and the whole-word adjustment
	// last, because the second has to hold whatever the first leaves.
	opening := sentenceStart(plain, runesBefore(plain, off, snippetBefore), off)
	start := min(wholeWordStart(plain, opening), off)
	end := max(wholeWordEnd(plain, runesAfter(plain, off, snippetAfter)), off)

	s := strings.Join(strings.Fields(plain[start:end]), " ")
	if start > 0 {
		s = "…" + s
	}
	if end < len(plain) {
		s += "…"
	}
	return s
}

// earliestOffset returns the smallest index at which any token occurs in hay,
// or 0 when no token occurs (a title-only or pure-filter hit shows the start).
func earliestOffset(hay string, tokens []string) int {
	off := -1
	for _, t := range tokens {
		if i, _ := phraseIndex(hay, t, 0); i >= 0 && (off < 0 || i < off) {
			off = i
		}
	}
	if off < 0 {
		return 0
	}
	return off
}

// wordEdgeBudget bounds how far a boundary may move to keep a word whole. A
// date or an identifier fits inside it; a run longer than this is not a word
// anyone is reading as one, and the window matters more than it does.
const wordEdgeBudget = 24

// wholeWordStart moves a snippet's opening boundary back to the start of a word
// the window cut into, so a date arrives as 2026-07-30 rather than 026-07-30. Only
// runs of ASCII letters and digits are treated this way, since CJK prose has no
// word boundary to respect. It moves outward, since forward leaves a fragment.
func wholeWordStart(s string, i int) int {
	if i <= 0 || i >= len(s) {
		return i
	}
	if !isWordByte(s[i-1]) || !isWordByte(s[i]) {
		return i
	}
	j := i
	for j > 0 && isWordByte(s[j-1]) && i-j < wordEdgeBudget {
		j--
	}
	if j > 0 && isWordByte(s[j-1]) {
		// The run outran the budget, so it is not a word worth keeping whole;
		// step off its tail instead of dragging it in.
		for i < len(s) && isWordByte(s[i]) {
			i++
		}
		return i
	}
	return j
}

// wholeWordEnd is the same adjustment at the closing boundary.
func wholeWordEnd(s string, i int) int {
	if i <= 0 || i >= len(s) {
		return i
	}
	if !isWordByte(s[i-1]) || !isWordByte(s[i]) {
		return i
	}
	j := i
	for j < len(s) && isWordByte(s[j]) && j-i < wordEdgeBudget {
		j++
	}
	if j < len(s) && isWordByte(s[j]) {
		for i > 0 && isWordByte(s[i-1]) {
			i--
		}
		return i
	}
	return j
}

// isWordByte reports whether b belongs to a run the reader reads as one thing.
// Hyphen and underscore are in it because the runs that arrive mangled are dates
// and identifiers, which splitting at the punctuation would cut to their tail.
func isWordByte(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b == '-' || b == '_'
}

// sentenceTerminators end a sentence in the scripts this corpus is written in; a
// comma of either width is not among them. The unambiguous ones are a subset:
// a full-width stop appears inside no token and ends a sentence wherever it
// stands, while an ASCII stop is the character inside vault-schema.toml and
// 3.14159, so it ends one only with white space or the end of the text after
// it. The two sets are all that is needed — every terminator outside the
// unambiguous set is by definition one of the ASCII ones, and lastSentenceEnd
// asks for the white space there.
const (
	sentenceTerminators    = "。！？；.!?;\n"
	unambiguousTerminators = "。！？；\n"
)

// snippetBeforeMax bounds the whole opening side of the window: the boundary may
// travel back this far to reach the start of its sentence and no further, so a
// note written without terminators cannot turn one result row into the note.
const snippetBeforeMax = 120

// sentenceStart moves a snippet's opening boundary back to the beginning of the
// sentence it landed inside, when that beginning is within reach. The words a
// sentence opens with decide what it means — 不得, 本段僅限, "this is not
// recommended" — and cutting them leaves the predicate reading as the instruction.
// Where no terminator is in reach the window is left where it was.
func sentenceStart(plain string, start, off int) int {
	if start <= 0 {
		return start
	}
	limit := runesBefore(plain, off, snippetBeforeMax)
	if limit >= start {
		return start
	}
	i, ok := lastSentenceEnd(plain, limit, start)
	if !ok {
		return start
	}
	for i < start {
		r, width := utf8.DecodeRuneInString(plain[i:])
		if !unicode.IsSpace(r) {
			break
		}
		i += width
	}
	return i
}

// lastSentenceEnd reports the byte just past the last sentence-ending punctuation
// in plain[limit:start], and whether there was one. An ASCII stop counts only when
// white space or the end of the text follows, which excludes a filename's dot.
func lastSentenceEnd(plain string, limit, start int) (int, bool) {
	for at := start; at > limit; {
		j := strings.LastIndexAny(plain[limit:at], sentenceTerminators)
		if j < 0 {
			return 0, false
		}
		i := limit + j
		r, size := utf8.DecodeRuneInString(plain[i:])
		after := i + size
		if strings.ContainsRune(unambiguousTerminators, r) {
			return after, true
		}
		if after >= len(plain) {
			return after, true
		}
		if next, _ := utf8.DecodeRuneInString(plain[after:]); unicode.IsSpace(next) {
			return after, true
		}
		at = i
	}
	return 0, false
}

// foldWithSourceOffsets lowercases s and maps every byte position of the folded
// copy, one past its end included, back to the byte offset in s of the character
// it came from. It applies the lowercase half of the index's fold alone, which
// reproduces that fold provided s is already NFC, as every snippet is.
func foldWithSourceOffsets(s string) (fold string, src []int) {
	var folded strings.Builder
	folded.Grow(len(s))
	src = make([]int, 0, len(s)+1)
	foldRunes(s, func(r rune, at int) {
		n := folded.Len()
		folded.WriteRune(r)
		for ; n < folded.Len(); n++ {
			src = append(src, at)
		}
	})
	return folded.String(), append(src, len(s))
}

// sourceOffsetOfFold maps one byte offset in the lowercased copy of s back to the
// byte offset in s of the character that produced it. It answers exactly what
// foldWithSourceOffsets tabulates, walked to one position rather than materialized
// because its caller measures a whole note. A test holds the two to one answer.
func sourceOffsetOfFold(s string, foldOff int) int {
	folded, at := 0, len(s)
	found := false
	foldRunes(s, func(r rune, i int) {
		if found {
			return
		}
		next := folded + utf8.RuneLen(r)
		if next > foldOff {
			at, found = i, true
			return
		}
		folded = next
	})
	return at
}

// HitRun is one stretch of a piece of text and whether the query matched it.
// The runs of one text cover it exactly once, in order, so a caller can render
// them straight through without consulting an offset.
type HitRun struct {
	Text string
	Hit  bool
}

// MarkHits cuts a piece of text into the stretches that matched and those that did
// not, so a page can show why a result is here. Nothing in it is particular to a
// snippet: the same cut serves the body excerpt, the path, and any other name a
// note can match by. Matching is done on the folded form while the runs carry
// slices of the original text, and overlapping matches merge into one mark.
func MarkHits(snippet string, tokens []string) []HitRun {
	if snippet == "" || len(tokens) == 0 {
		return nil
	}
	fold, src := foldWithSourceOffsets(snippet)
	covered := make([]bool, len(snippet))
	found := false
	for _, t := range tokens {
		if t == "" {
			continue
		}
		for at := 0; at <= len(fold); {
			i, stop := phraseIndex(fold, t, at)
			if i < 0 {
				break
			}
			for j := src[i]; j < src[stop]; j++ {
				covered[j] = true
			}
			found = true
			at = max(stop, i+1)
		}
	}
	if !found {
		return nil
	}
	var runs []HitRun
	start := 0
	for i := 1; i <= len(snippet); i++ {
		if i < len(snippet) && covered[i] == covered[start] {
			continue
		}
		runs = append(runs, HitRun{Text: snippet[start:i], Hit: covered[start]})
		start = i
	}
	return runs
}
