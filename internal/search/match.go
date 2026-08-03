package search

import (
	"slices"
	"strings"
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
	if len(q.tokens) == 0 && len(q.filters) == 0 {
		return nil, nil
	}
	metadataAvailable := idx.policy.Trustworthy()
	requiresMetadata := q.RequiresMetadata()
	if requiresMetadata && !metadataAvailable {
		return nil, idx.metadataUnavailableError()
	}
	var answers resultBuckets
	for _, e := range idx.entries {
		if requiresMetadata && !e.metadataCapable {
			continue
		}
		if !e.matchesFilters(q.filters) {
			continue
		}
		switch {
		case allContain(e.TitleFold, q.tokens):
			bodyEvidence := len(q.tokens) != 0 && allContain(e.PlainFold, q.tokens)
			answers.add(e.result(q.tokens, bodyEvidence, metadataAvailable), true)
		case allContain(e.PlainFold, q.tokens):
			answers.add(e.result(q.tokens, true, metadataAvailable), false)
		case allContain(e.PathFold, q.tokens):
			// Last, and appended after every other group, so widening what can
			// be found still moves nothing that could be found already. A note
			// whose words match is always the better answer than one that
			// merely lives in a folder of that name.
			answers.addPathHit(e.result(q.tokens, false, metadataAvailable))
		}
	}
	return answers.ordered(), nil
}

// resultBuckets keeps the four answer groups apart while one pass over the
// entries fills them, so the final order is a concatenation rather than a sort:
// entries are already kept in rel-path order, and each group inherits it.
type resultBuckets struct {
	titleNotes []Result
	bodyNotes  []Result
	titleFiles []Result
	bodyFiles  []Result
	pathNotes  []Result
	pathFiles  []Result
}

// addPathHit files a result matched only by where the note lives.
func (b *resultBuckets) addPathHit(hit Result) {
	if hit.File {
		b.pathFiles = append(b.pathFiles, hit)
		return
	}
	b.pathNotes = append(b.pathNotes, hit)
}

func (b *resultBuckets) add(hit Result, titleMatch bool) {
	switch {
	case titleMatch && !hit.File:
		b.titleNotes = append(b.titleNotes, hit)
	case titleMatch:
		b.titleFiles = append(b.titleFiles, hit)
	case !hit.File:
		b.bodyNotes = append(b.bodyNotes, hit)
	default:
		b.bodyFiles = append(b.bodyFiles, hit)
	}
}

// ordered flattens the groups into the answer: a note's title hits, a note's
// body hits, then the same two over files.
func (b *resultBuckets) ordered() []Result {
	out := make([]Result, 0,
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
// query "%" matches a literal "%", there are no wildcards. Zero tokens is
// vacuously true.
func allContain(hay string, tokens []string) bool {
	for _, t := range tokens {
		if !strings.Contains(hay, t) {
			return false
		}
	}
	return true
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
	start := wholeWordStart(plain, runesBefore(plain, off, snippetBefore))
	end := wholeWordEnd(plain, runesAfter(plain, off, snippetAfter))

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
		if i := strings.Index(hay, t); i >= 0 && (off < 0 || i < off) {
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
