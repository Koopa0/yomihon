// Package search is the reading surface for a query: the route, the language
// the answer is written in, and the view a result becomes. It puts the query
// to the lexical index and owns nothing about how the index answers.
package search

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

const maxQueryBytes = 4096

// maxRenderedResults bounds how many hits one response renders. The tally
// stays exact; only the list is cut.
const maxRenderedResults = 200

// RequestSnapshot is the search index and shell state bound to one request
// capture of an atomic vault generation and its artifact authority.
type RequestSnapshot struct {
	Index *lexical.Index
	Shell nav.Shell

	// Status is the read-only status vocabulary a result row rules against.
	// It may be absent, and rows then name statuses without ruling on them.
	Status StatusVocabulary
}

// StatusVocabulary is the read-only half of the status projection, declared
// here in the consumer so a face that only reads does not put the component
// able to change the vault inside its import closure.
type StatusVocabulary interface {
	// Closed reports whether this view can classify a governed instance at
	// all: a folder with no contract, or one that cannot be honoured.
	Closed() bool
	// KnownStatus reports whether the contract declares status for noteType.
	KnownStatus(noteType, status string) bool
}

// Handler serves the search page and its read-only results fragment, reading
// one request snapshot so index and sidebar cannot come from two generations.
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
	lang := origin.Language(r)
	view := answerView(snap, q, h.query(snap.Index, q, lang))
	view.Sidebar = pages.NewSidebar(snap.Shell.Nav, "")
	if err := pages.Search(view, layouts.ChromeFromRequest(r, wording.SearchTitle.In(lang))).Render(r.Context(), w); err != nil {
		h.logQueryWriteFailure(r, "write search page", q, err)
	}
}

// answer is what one query came to: how it was read, the hits the page shows,
// the true tally behind them, the sentence to show in their place when the
// index could not answer, and the terms the hits matched. It belongs to one
// request — query builds it, answerView reads it, and nothing keeps it. It
// travels by pointer for its size, not so anyone can change it.
type answer struct {
	parsed     *lexical.Query
	results    []lexical.Result
	total      int
	diagnostic string
	tokens     []string
}

// answerView is everything both faces say about one query, built once so the
// full page and the live region cannot drift apart.
func answerView(snap RequestSnapshot, q string, a *answer) pages.SearchView {
	return pages.SearchView{
		Query:             q,
		Results:           viewResults(a.results, snap.Shell.Governed, snap.Status, a.tokens),
		Total:             a.total,
		Diagnostic:        a.diagnostic,
		Governed:          snap.Shell.Governed,
		StepBacks:         stepBackViews(snap.Index, q, a.results, a.diagnostic),
		UnknownFilterKeys: a.parsed.UnknownFilterKeys(),
		FilterKeys:        lexical.FilterKeys(),
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
	answered := h.query(snap.Index, q, origin.Language(r))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	view := answerView(snap, q, answered)
	if err := pages.SearchResults(view, origin.Language(r)).Render(r.Context(), w); err != nil {
		h.logQueryWriteFailure(r, "write search results", q, err)
	}
}

// query reads one query and answers it. The hits are bounded by
// maxRenderedResults; the tally is not, so the page can stay honest about what
// the bounded list leaves out. The parse travels in the answer because the view
// needs it too, and reading the same text twice per request is the cost this
// saves.
func (h *Handler) query(idx *lexical.Index, q string, lang wording.Lang) *answer {
	parsed := lexical.Parse(q)
	results, total, err := idx.SearchN(parsed, maxRenderedResults)
	if errors.Is(err, lexical.ErrMetadataUnavailable) {
		return &answer{parsed: parsed, diagnostic: unavailableSentence(err, lang)}
	}
	if err != nil {
		h.logQueryError("search query", q, err)
		return &answer{parsed: parsed, diagnostic: wording.SearchUnavailable.In(lang)}
	}
	return &answer{parsed: parsed, results: results, total: total, tokens: parsed.Tokens()}
}

// unavailableSentence says why a metadata query could not be answered, in this
// reader's language. Every other rejection already carries an operator's line.
func unavailableSentence(err error, lang wording.Lang) string {
	claim, ok := lexical.MetadataClaim(err)
	if !ok || claim.Reason() != schema.ReasonContractUnreadable {
		return err.Error()
	}
	if cause := claim.Cause(); cause != nil {
		return wording.ContractUnreadablePrefix.In(lang) + cause.Error()
	}
	return wording.ContractUnreadable.In(lang)
}

// stepBackViews computes the loosened offers for an empty answer, and nothing
// for any other page state.
func stepBackViews(idx *lexical.Index, q string, results []lexical.Result, diagnostic string) []pages.SearchStepBack {
	if len(results) > 0 || diagnostic != "" || strings.TrimSpace(q) == "" {
		return nil
	}
	steps := idx.StepBacks(q)
	out := make([]pages.SearchStepBack, 0, len(steps))
	for _, s := range steps {
		out = append(out, pages.SearchStepBack{Query: s.Query, Count: s.Count})
	}
	return out
}

func (h *Handler) logQueryError(message, rawQuery string, err error) {
	h.log.Error(message, queryFacts(rawQuery, err)...)
}

// logQueryWriteFailure reports a page that could not be finished: the same
// facts as a query fault, at the loudness a reader who left deserves.
func (h *Handler) logQueryWriteFailure(r *http.Request, message, rawQuery string, err error) {
	h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), message, queryFacts(rawQuery, err)...)
}

// queryFacts is what a search fault may say about a query: its shape and its
// size, never its text — which is why the error arrives as a type rather than
// under the "error" key the rest of this repository logs errors under.
func queryFacts(rawQuery string, err error) []any {
	query := lexical.Parse(rawQuery)
	filters := query.Filters()
	filterKeys := make([]string, 0, len(filters))
	for _, filter := range filters {
		filterKeys = append(filterKeys, filter.Key)
	}
	return []any{
		"error_type", fmt.Sprintf("%T", err),
		"query_bytes", len(rawQuery),
		"filter_keys", filterKeys,
	}
}

func requestQuery(w http.ResponseWriter, r *http.Request) (string, bool) {
	q := r.URL.Query().Get("q")
	if len(q) > maxQueryBytes {
		http.Error(w, wording.QueryTooLong.In(origin.Language(r)), http.StatusBadRequest)
		return "", false
	}
	if strings.IndexFunc(q, func(r rune) bool {
		return r <= 0x1f || r == 0x7f || (r >= 0x80 && r <= 0x9f)
	}) >= 0 {
		http.Error(w, wording.QueryHasControlByte.In(origin.Language(r)), http.StatusBadRequest)
		return "", false
	}
	return q, true
}

// viewResults maps the engine's results onto the view's plain field type. A
// hit's status is carried only for a governed vault: elsewhere a status chip
// would present raw frontmatter as a value drawn from a declared vocabulary.
func viewResults(results []lexical.Result, governed bool, vocabulary StatusVocabulary, tokens []string) []pages.SearchResult {
	// Asked of the vocabulary rather than inferred from the shell beside it: a
	// view that knows none would otherwise mark every governed row a fault.
	rules := vocabulary != nil && !vocabulary.Closed()
	out := make([]pages.SearchResult, len(results))
	for i, r := range results {
		out[i] = pages.SearchResult{
			RelPath:     r.RelPath,
			Title:       r.Title,
			Snippet:     r.Snippet,
			SnippetRuns: snippetRuns(r.Snippet, tokens),
			PathRuns:    snippetRuns(r.RelPath, tokens),
			AliasRuns:   snippetRuns(r.Alias, tokens),
			File:        r.File,
		}
		// A row that names no status can carry no verdict about one.
		if governed {
			out[i].Status = r.Status
			if rules && r.Status != "" {
				out[i].StatusOutsideEnum = !vocabulary.KnownStatus(r.NoteType, r.Status)
			}
		}
	}
	return out
}

// snippetRuns dresses one piece of matched text for the page. Where the query
// fell is the index's answer; what a matched stretch looks like is this page's.
func snippetRuns(text string, tokens []string) []pages.SnippetRun {
	marked := lexical.MarkHits(text, tokens)
	if len(marked) == 0 {
		return nil
	}
	runs := make([]pages.SnippetRun, len(marked))
	for i, run := range marked {
		runs[i] = pages.SnippetRun{Text: run.Text, Hit: run.Hit}
	}
	return runs
}
