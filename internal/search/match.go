package search

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

	// File marks a hit that is not a note but a vault file read as characters.
	// It carries no lifecycle state and never will, so a surface that dresses a
	// hit in note furniture can tell the two apart.
	File bool
}

const (
	// snippetBefore/snippetAfter bound the snippet window around the earliest
	// matched-token offset, counted in characters the reader sees rather than
	// in bytes. As a byte budget it paid out by script: a Han character costs
	// three bytes, so the same numbers bought a reader of Chinese roughly a
	// third of the context they bought a reader of English — and the notes hurt
	// most were the long ones, which is where a reader searching their own
	// writing most needs to see the sentence around the hit.
	snippetBefore = 40
	snippetAfter  = 160
)

// Search runs a parsed query against the index and returns results in the final
// deterministic order: a note's title hits (every token in TitleFold) first,
// then a note's body hits (every token in PlainFold, not already a title hit),
// then the same two buckets over vault files that are not notes. Because entries
// are kept sorted by RelPath, each bucket is already rel_path-ordered, so
// concatenation is the whole order — no sort call.
//
// Notes come before files in both buckets, so the list a vault of notes alone
// produces is the opening of the list this returns and files are appended to it.
// Widening what can be found must never move what could already be found.
//
// An empty query (no tokens and no filters) returns nothing. A pure-filter
// query is legal: with no tokens the title-bucket token test is vacuously true,
// so every filter match lands in the (rel_path-ordered) title bucket.
// Metadata filters exclude non-instance artifacts. If the artifact policy was
// declared and could not be honoured, a query containing such a filter returns
// ErrMetadataUnavailable with the contract diagnostic; text and folder queries
// continue against the complete readable corpus. A vault that never declared
// one excludes nothing, so those filters run over raw frontmatter.
func (idx *Index) Search(q Query) ([]Result, error) {
	results, _, err := idx.search(q, -1)
	return results, err
}

// search is Search with a bound: at most limit results are materialized (a
// negative limit materializes them all), while total reports how many hits the
// query truly has. The tail beyond limit is counted but never built — a
// snippet is the per-hit cost, and skipping it is what keeps a broad query's
// work proportional to the page rather than to the vault. The kept results are
// exactly the opening stretch of the unbounded answer, in the same order.
func (idx *Index) search(q Query, limit int) (results []Result, total int, err error) {
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
		answers.bucket(e, q.tokens)
	}
	hits := answers.ordered()
	total = len(hits)
	if limit >= 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	results = make([]Result, len(hits))
	for i, h := range hits {
		results[i] = h.entry.result(q.tokens, h.bodyEvidence, metadataAvailable)
	}
	return results, total, nil
}

// hit is one matched entry before its Result is built. Buckets hold hits
// rather than Results so a bounded search can count every match while
// building — snippet included — only the results it will return.
type hit struct {
	entry        *entry
	bodyEvidence bool
}

// resultBuckets keeps the four answer groups apart while one pass over the
// entries fills them, so the final order is a concatenation rather than a sort:
// entries are already kept in rel-path order, and each group inherits it.
type resultBuckets struct {
	titleNotes []hit
	bodyNotes  []hit
	titleFiles []hit
	bodyFiles  []hit
	pathNotes  []hit
	pathFiles  []hit
}

// bucket files one filter-matching entry into its answer group by what the
// tokens matched: the title, the body, or only the path.
func (b *resultBuckets) bucket(e *entry, tokens []string) {
	switch {
	case allContain(e.TitleFold, tokens):
		bodyEvidence := len(tokens) != 0 && allContain(e.PlainFold, tokens)
		b.add(hit{entry: e, bodyEvidence: bodyEvidence}, true)
	case allContain(e.PlainFold, tokens):
		b.add(hit{entry: e, bodyEvidence: true}, false)
	case allContain(e.PathFold, tokens):
		// Last, and appended after every other group, so widening what can
		// be found still moves nothing that could be found already. A note
		// whose words match is always the better answer than one that
		// merely lives in a folder of that name.
		b.addPathHit(hit{entry: e})
	}
}

// addPathHit files a hit matched only by where the note lives.
func (b *resultBuckets) addPathHit(h hit) {
	if h.entry.isFile {
		b.pathFiles = append(b.pathFiles, h)
		return
	}
	b.pathNotes = append(b.pathNotes, h)
}

func (b *resultBuckets) add(h hit, titleMatch bool) {
	switch {
	case titleMatch && !h.entry.isFile:
		b.titleNotes = append(b.titleNotes, h)
	case titleMatch:
		b.titleFiles = append(b.titleFiles, h)
	case !h.entry.isFile:
		b.bodyNotes = append(b.bodyNotes, h)
	default:
		b.bodyFiles = append(b.bodyFiles, h)
	}
}

// ordered flattens the groups into the answer: a note's title hits, a note's
// body hits, then the same two over files.
func (b *resultBuckets) ordered() []hit {
	out := make([]hit, 0,
		len(b.titleNotes)+len(b.bodyNotes)+len(b.titleFiles)+len(b.bodyFiles)+len(b.pathNotes)+len(b.pathFiles))
	out = append(out, b.titleNotes...)
	out = append(out, b.bodyNotes...)
	out = append(out, b.titleFiles...)
	out = append(out, b.bodyFiles...)
	out = append(out, b.pathNotes...)
	out = append(out, b.pathFiles...)
	return out
}

// AllowedPaths returns the paths that satisfy q's structured filters. Bare
// text tokens are deliberately ignored: callers use this set to constrain a
// separate retrieval channel before that channel applies its own ranking.
// Metadata filters retain Search's instance-capability behavior, including a
// loud error when the artifact policy could not be honoured. With no filters,
// every indexed path is allowed.
func (idx *Index) AllowedPaths(q Query) (map[string]struct{}, error) {
	requiresMetadata := q.RequiresMetadata()
	if requiresMetadata && !idx.policy.Trustworthy() {
		return nil, idx.metadataUnavailableError()
	}

	paths := make(map[string]struct{}, len(idx.entries))
	for _, e := range idx.entries {
		if requiresMetadata && !e.metadataCapable {
			continue
		}
		if e.matchesFilters(q.filters) {
			paths[e.RelPath] = struct{}{}
		}
	}
	return paths, nil
}

// allContain reports whether hay contains every token (AND). Tokens are already
// folded and hay is a *Fold field, so this is a literal substring test — a
// query "%" matches a literal "%", there are no wildcards — except that a run
// of whitespace inside a token matches any run of whitespace (see
// phraseIndex). Zero tokens is vacuously true.
func allContain(hay string, tokens []string) bool {
	for _, t := range tokens {
		if start, _ := phraseIndex(hay, t, 0); start < 0 {
			return false
		}
	}
	return true
}

// phraseIndex reports the byte range of the first occurrence of token in hay at
// or after from, treating every run of whitespace inside token as a match for
// any run of whitespace in hay. start is -1 when token does not occur.
//
// The flexibility is what a quoted phrase needs to be answerable here. This
// vault's prose is hard-wrapped near 80 columns and the indexed text keeps
// those breaks, so the two words a reader quotes sit on two lines about as
// often as on one — and demanding the same bytes answered a phrase written
// verbatim in the vault with nothing, which is the one answer a reader has no
// way to argue with. Adjacency is still required: the words must be separated
// by whitespace and nothing else, which is what the quotes asked for.
//
// The indexed text separates one block from the next with a single break as
// well, so a phrase can join the last words of one block to the first words of
// the next. That is a known cost rather than an oversight: nothing in the
// stored text tells a wrapped sentence apart from a block boundary, and
// answering the wrapped sentence is worth the pair of blocks it also joins.
// A token with no whitespace in it takes the plain substring path it always
// took, so the ordinary query allocates nothing here; the phrase walk below
// allocates nothing either, reading both strings in place.
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
		// Unreachable: Parse only ever emits the six keys above.
		return false
	}
}

// result builds a Result for e, with a snippet centered on the earliest
// matched-token offset.
func (e *entry) result(tokens []string, bodyEvidence, metadataAvailable bool) Result {
	status := e.Status
	if !metadataAvailable || !e.metadataCapable {
		status = ""
	}
	var bodySnippet string
	if bodyEvidence {
		bodySnippet = snippet(e.PlainText, e.PlainFold, tokens)
	}
	return Result{
		RelPath: e.RelPath,
		Title:   e.Title,
		Status:  status,
		Snippet: bodySnippet,
		File:    e.isFile,
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
// offset. The offset is located on plainFold (the folded copy) and reused as a
// byte index into plain: fold (NFC then lowercase) is length-preserving for
// this vault's zh/ja/en corpus, but a rare non-length-preserving fold (Turkish
// İ, etc.) could shift a boundary, so every bound below is clamped to a valid
// rune boundary rather than trusted.
func snippet(plain, plainFold string, tokens []string) string {
	// The offset was located on the folded copy and is reused as an index into
	// the original. Folding is length-preserving for this vault's scripts, but
	// a rare fold that is not (Turkish İ, say) could land it inside a
	// character — so it is pulled back to a boundary before anything counts
	// characters outward from it.
	off := clampRuneEnd(plain, min(earliestOffset(plainFold, tokens), len(plain)))
	// Neither boundary may be moved past the match it was placed around. When a
	// boundary lands in a run longer than the budget below, it steps clear of
	// that run — the opening one forward off its tail, the closing one back off
	// its head — and a match buried deep enough in one run is stepped over by
	// both at once, which leaves the opening after the close and the slice
	// below reversed. A few hundred letters with no break in them is all it
	// takes: one long identifier, or a hash pasted into a note.
	start := min(wholeWordStart(plain, runesBefore(plain, off, snippetBefore)), off)
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
// the window cut into, so a date arrives as 2026-07-30 rather than 026-07-30
// and a term arrives as B-tree rather than ee. Only runs of ASCII letters and
// digits are treated this way: those are where a severed run reads as a typo or
// a wrong number, while CJK prose has no word boundary to respect and cutting
// between its characters is what a window is expected to do.
//
// Moving outward rather than inward is the point. Stepping forward past the
// fragment leaves the reader "…-07-30", which is honest but still a fragment;
// stepping back gives them the thing itself.
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
// Hyphen and underscore are in it because the runs that arrive mangled are
// dates and identifiers — 2026-07-30, B-tree, query-planner — and splitting
// them at the punctuation would keep only the tail.
func isWordByte(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b == '-' || b == '_'
}

// clampRuneEnd clamps i into [0,len(s)] and retreats it to a rune boundary so
// the snippet never ends mid-rune.
func clampRuneEnd(s string, i int) int {
	if i >= len(s) {
		return len(s)
	}
	if i <= 0 {
		return 0
	}
	for i > 0 && !utf8.RuneStart(s[i]) {
		i--
	}
	return i
}
