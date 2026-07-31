package search

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/koopa0/yomihon/internal/ui/pages"
)

const maxQueryBytes = 4096

// RequestSnapshot is the search index and shell state bound to one request
// capture of an atomic vault generation and its artifact authority.
type RequestSnapshot struct {
	Index *Index
	Shell pages.Shell
}

// Handler serves the search face: the full GET /search page and its read-only
// results fragment. It reads one request snapshot through the provider closure
// (main projects it from one Store read),
// so an edited note is reflected within one scan cycle and the index, sidebar,
// Journal, and pending chip cannot come from different generations. All
// business logic stays in this package (Parse + Index.Search); the handler only
// parses the query, calls, and renders.
type Handler struct {
	snapshot func() RequestSnapshot
	log      *slog.Logger
}

// NewHandler wires the search HTTP surface. snapshot must return both values
// from one store read so a request cannot combine two scanner generations.
func NewHandler(snapshotProvider func() RequestSnapshot, log *slog.Logger) *Handler {
	if snapshotProvider == nil {
		panic("search: NewHandler requires a non-nil snapshot provider")
	}
	if log == nil {
		panic("search: NewHandler requires a non-nil logger")
	}
	return &Handler{snapshot: snapshotProvider, log: log}
}

// Register mounts the search routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /search", h.search)
	mux.HandleFunc("GET /search/results", h.results)
}

// search parses q, queries the current index, and renders results or a named
// metadata-capability diagnostic. An empty or whitespace-only q parses to an
// empty Query, which Index.Search answers with no results.
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	q, ok := requestQuery(w, r)
	if !ok {
		return
	}
	snap := h.snapshot()
	results, diagnostic, tokens := h.query(snap.Index, q)

	view := pages.SearchView{
		Query:      q,
		Results:    viewResults(results, snap.Shell.Governed, tokens),
		Diagnostic: diagnostic,
		Governed:   snap.Shell.Governed,
		Nav:        snap.Shell.Nav,
	}
	if err := pages.Search(view, snap.Shell.Chrome(r, "搜尋")).Render(r.Context(), w); err != nil {
		h.logQueryError("write search page", q, err)
	}
}

// results renders only the lexical-results region used by progressive search.
// The ordinary /search form remains the complete no-JavaScript navigation path.
func (h *Handler) results(w http.ResponseWriter, r *http.Request) {
	q, ok := requestQuery(w, r)
	if !ok {
		return
	}
	snap := h.snapshot()
	results, diagnostic, tokens := h.query(snap.Index, q)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if err := pages.SearchResults(q, viewResults(results, snap.Shell.Governed, tokens), diagnostic, snap.Shell.Governed).Render(r.Context(), w); err != nil {
		h.logQueryError("write search results", q, err)
	}
}

// query parses once and hands back both the hits and the terms that produced
// them, so the page can mark those terms in a snippet without parsing again.
func (h *Handler) query(idx *Index, q string) (results []Result, diagnostic string, tokens []string) {
	parsed := Parse(q)
	results, err := idx.Search(parsed)
	if errors.Is(err, ErrMetadataUnavailable) {
		return nil, err.Error(), nil
	}
	if err != nil {
		h.logQueryError("search query", q, err)
		return nil, "搜尋目前暫時無法使用。", nil
	}
	return results, "", parsed.Tokens()
}

func (h *Handler) logQueryError(message, rawQuery string, err error) {
	query := Parse(rawQuery)
	filters := query.Filters()
	filterKeys := make([]string, 0, len(filters))
	for _, filter := range filters {
		filterKeys = append(filterKeys, filter.Key)
	}
	h.log.Error(message,
		"error_type", fmt.Sprintf("%T", err),
		"query_bytes", len(rawQuery),
		"filter_keys", filterKeys,
	)
}

func requestQuery(w http.ResponseWriter, r *http.Request) (string, bool) {
	q := r.URL.Query().Get("q")
	if len(q) > maxQueryBytes {
		http.Error(w, "搜尋字串過長", http.StatusBadRequest)
		return "", false
	}
	if strings.IndexFunc(q, func(r rune) bool {
		return r <= 0x1f || r == 0x7f || (r >= 0x80 && r <= 0x9f)
	}) >= 0 {
		http.Error(w, "搜尋字串含有控制字元", http.StatusBadRequest)
		return "", false
	}
	return q, true
}

// viewResults maps the engine's results onto the view's plain field type,
// keeping internal/ui/pages free of any dependency on this package (the same
// one-directional import shape the reading page uses).
//
// A hit's status is carried only for a vault something governs. The index will
// happily match and return raw frontmatter for an ungoverned folder — that is a
// text field like any other — but a status chip presents it as a value drawn
// from a declared vocabulary, which is a claim no contract backs there.
func viewResults(results []Result, governed bool, tokens []string) []pages.SearchResult {
	out := make([]pages.SearchResult, len(results))
	for i, r := range results {
		out[i] = pages.SearchResult{
			RelPath:     r.RelPath,
			Title:       r.Title,
			Snippet:     r.Snippet,
			SnippetRuns: markHits(r.Snippet, tokens),
			File:        r.File,
		}
		if governed {
			out[i].Status = r.Status
		}
	}
	return out
}

// markHits cuts a snippet into the stretches that matched and the stretches
// that did not, so the page can show the reader why this result is here.
//
// Matching is done on the same folded form the index matched on, and the runs
// carry slices of the original text, so what the reader sees is their own note
// and not a re-cased copy of it. Overlapping matches are merged: two tokens
// that cover the same words produce one mark rather than nested ones.
func markHits(snippet string, tokens []string) []pages.SnippetRun {
	if snippet == "" || len(tokens) == 0 {
		return nil
	}
	fold := strings.ToLower(snippet)
	covered := make([]bool, len(snippet))
	found := false
	for _, t := range tokens {
		if t == "" {
			continue
		}
		for at := 0; ; {
			i := strings.Index(fold[at:], t)
			if i < 0 {
				break
			}
			i += at
			for j := i; j < i+len(t); j++ {
				covered[j] = true
			}
			found = true
			at = i + len(t)
		}
	}
	if !found {
		return nil
	}
	var runs []pages.SnippetRun
	start := 0
	for i := 1; i <= len(snippet); i++ {
		if i < len(snippet) && covered[i] == covered[start] {
			continue
		}
		runs = append(runs, pages.SnippetRun{Text: snippet[start:i], Hit: covered[start]})
		start = i
	}
	return runs
}
