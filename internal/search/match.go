package search

import (
	"slices"
	"strings"
	"unicode/utf8"
)

// Result is one search hit: the note's path, display title, status badge, and
// a snippet centered on the earliest matched-token offset.
type Result struct {
	RelPath string
	Title   string
	Status  string
	Snippet string
}

const (
	// snippetBefore/snippetAfter bound the snippet window (in bytes) around the
	// earliest matched-token offset; both ends are clamped to rune boundaries.
	snippetBefore = 40
	snippetAfter  = 160
)

// Search runs a parsed query against the index and returns results in the final
// deterministic order (docs/search-plan.md §5/§6): title hits (every token in
// TitleFold) first, then body hits (every token in PlainFold, not already a
// title hit). Because entries are kept sorted by RelPath, each bucket is
// already rel_path-ordered, so concatenation is the whole order — no sort call.
//
// An empty query (no tokens and no filters) returns nothing. A pure-filter
// query is legal: with no tokens the title-bucket token test is vacuously true,
// so every filter match lands in the (rel_path-ordered) title bucket.
func (idx *Index) Search(q Query) []Result {
	if len(q.Tokens) == 0 && len(q.Filters) == 0 {
		return nil
	}
	var titleHits, bodyHits []Result
	for _, e := range idx.entries {
		if !e.matchesFilters(q.Filters) {
			continue
		}
		switch {
		case allContain(e.TitleFold, q.Tokens):
			titleHits = append(titleHits, e.result(q.Tokens))
		case allContain(e.PlainFold, q.Tokens):
			bodyHits = append(bodyHits, e.result(q.Tokens))
		}
	}
	return append(titleHits, bodyHits...)
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
func (e *entry) result(tokens []string) Result {
	return Result{
		RelPath: e.RelPath,
		Title:   e.Title,
		Status:  e.Status,
		Snippet: snippet(e.PlainText, e.PlainFold, tokens),
	}
}

// snippet returns a one-line window of plain around the earliest matched-token
// offset. The offset is located on plainFold (the folded copy) and reused as a
// byte index into plain: fold (NFC then lowercase) is length-preserving for
// this vault's zh/ja/en corpus, but a rare non-length-preserving fold (Turkish
// İ, etc.) could shift a boundary, so every bound below is clamped to a valid
// rune boundary rather than trusted.
func snippet(plain, plainFold string, tokens []string) string {
	off := min(earliestOffset(plainFold, tokens), len(plain))
	start := clampRuneStart(plain, off-snippetBefore)
	end := clampRuneEnd(plain, off+snippetAfter)

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

// clampRuneStart clamps i into [0,len(s)] and advances it to the next rune
// start so the snippet never begins mid-rune.
func clampRuneStart(s string, i int) int {
	if i <= 0 {
		return 0
	}
	if i >= len(s) {
		return len(s)
	}
	for i < len(s) && !utf8.RuneStart(s[i]) {
		i++
	}
	return i
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
