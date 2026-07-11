package search

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/koopa0/yomihon/internal/ui/pages"
)

// RequestSnapshot is the search index and shell state captured from one atomic
// vault snapshot read at request entry.
type RequestSnapshot struct {
	Index *Index
	Shell pages.ShellData
}

// Handler serves the search face: GET /search?q=... . It reads one request
// snapshot through the provider closure (main projects it from one Store read),
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

// Register mounts the search route.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /search", h.search)
}

// search parses q, queries the current index, and renders results or a named
// metadata-capability diagnostic. An empty or whitespace-only q parses to an
// empty Query, which Index.Search answers with no results.
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	snap := h.snapshot()
	q := r.URL.Query().Get("q")
	results, err := snap.Index.Search(Parse(q))
	diagnostic := ""
	if errors.Is(err, ErrMetadataUnavailable) {
		diagnostic = err.Error()
	} else if err != nil {
		h.log.Error("search query", "query", q, "error", err)
		results = nil
		diagnostic = "Search is temporarily unavailable."
	}

	view := pages.SearchView{Query: q, Results: viewResults(results), Diagnostic: diagnostic, Nav: snap.Shell.Nav}
	if err := pages.Search(view, snap.Shell.Chrome(r, "Search")).Render(r.Context(), w); err != nil {
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
