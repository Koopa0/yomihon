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

// RequestSnapshot is the search index and shell state captured from one atomic
// vault snapshot read at request entry.
type RequestSnapshot struct {
	Index *Index
	Shell pages.ShellData
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
	runQuery func(*Index, Query) ([]Result, error)
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
	return &Handler{snapshot: snapshotProvider, log: log, runQuery: (*Index).Search}
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
	results, diagnostic := h.query(snap.Index, q)

	view := pages.SearchView{Query: q, Results: viewResults(results), Diagnostic: diagnostic, Nav: snap.Shell.Nav}
	if err := pages.Search(view, snap.Shell.Chrome(r, "Search")).Render(r.Context(), w); err != nil {
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
	results, diagnostic := h.query(snap.Index, q)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if err := pages.SearchResults(q, viewResults(results), diagnostic).Render(r.Context(), w); err != nil {
		h.logQueryError("write search results", q, err)
	}
}

func (h *Handler) query(idx *Index, q string) (results []Result, diagnostic string) {
	results, err := h.runQuery(idx, Parse(q))
	if errors.Is(err, ErrMetadataUnavailable) {
		return nil, err.Error()
	}
	if err != nil {
		h.logQueryError("search query", q, err)
		return nil, "Search is temporarily unavailable."
	}
	return results, ""
}

func (h *Handler) logQueryError(message, rawQuery string, err error) {
	query := Parse(rawQuery)
	filterKeys := make([]string, 0, len(query.Filters))
	for _, filter := range query.Filters {
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
		http.Error(w, "search query is too long", http.StatusBadRequest)
		return "", false
	}
	if strings.IndexFunc(q, func(r rune) bool {
		return r <= 0x1f || r == 0x7f || (r >= 0x80 && r <= 0x9f)
	}) >= 0 {
		http.Error(w, "search query contains control characters", http.StatusBadRequest)
		return "", false
	}
	return q, true
}

// viewResults maps the engine's results onto the view's plain field type,
// keeping internal/ui/pages free of any dependency on this package (the same
// one-directional import shape the reading page uses).
func viewResults(results []Result) []pages.SearchResult {
	out := make([]pages.SearchResult, len(results))
	for i, r := range results {
		out[i] = pages.SearchResult{
			RelPath: r.RelPath,
			Title:   r.Title,
			Status:  r.Status,
			Snippet: r.Snippet,
		}
	}
	return out
}
