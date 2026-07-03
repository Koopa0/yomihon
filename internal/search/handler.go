package search

import (
	"log/slog"
	"net/http"

	"github.com/koopa0/kurodo/internal/ui/pages"
)

// Handler serves the search face: GET /search?q=... . It reads the CURRENT
// index each request through the provider closure (main wires it to the live
// Snapshot's Search, D25), so an edited note is reflected within one scan
// cycle. All business logic stays in this package (Parse + Index.Search); the
// handler only parses the query, calls, and renders.
type Handler struct {
	index func() *Index
	log   *slog.Logger
}

// NewHandler wires the search HTTP surface. index must not be nil — it is the
// live-Snapshot accessor, always a real closure (a wiring bug otherwise, caught
// here rather than three calls deep in the first request; mirrors the other
// handlers' nil-dependency guards).
func NewHandler(index func() *Index, log *slog.Logger) *Handler {
	if index == nil {
		panic("search: NewHandler requires a non-nil index provider")
	}
	return &Handler{index: index, log: log}
}

// Register mounts the search route.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /search", h.search)
}

// search parses q, queries the current index, and renders the plain results
// page. An empty or whitespace-only q parses to an empty Query, which
// Index.Search answers with no results (before any scanning).
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	results := h.index().Search(Parse(q))

	view := pages.SearchView{Query: q, Results: viewResults(results)}
	if err := pages.Search(view).Render(r.Context(), w); err != nil {
		h.log.Error("write search page", "query", q, "error", err)
	}
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
