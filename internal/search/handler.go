// Package search is the reading surface for a query: the route, the language
// the answer is written in, and the view a result becomes. It puts the query
// to the lexical index and owns nothing about how the index answers.
//
// The split is the point. The index is built once per reading generation by
// the store every face reads from, so it must stay free of templates, the page
// shell and the security headers a served page carries; those live here, on
// the near side of the boundary, and this package imports the engine rather
// than the other way round.
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

// maxRenderedResults bounds how many hits one search response materializes
// and renders. A broad term over a vault of ten thousand notes matches most
// of it, and the full answer is a multi-megabyte page rebuilt on every pause
// in typing; the tally stays exact while the list holds only this opening
// stretch.
const maxRenderedResults = 200

// RequestSnapshot is the search index and shell state bound to one request
// capture of an atomic vault generation and its artifact authority.
type RequestSnapshot struct {
	Index *lexical.Index
	Shell nav.Shell

	// Status is the read-only status vocabulary the row rules against. A
	// result row states a note's status, and a status is a value drawn from a
	// list; without the contract that declared it the row can print the word
	// but cannot say whether the vault allows it, which is the one thing a
	// reader scanning the list needs. It may be absent, and a search face
	// then names statuses without ruling on them.
	Status StatusVocabulary
}

// StatusVocabulary is the minimal capability the search face needs from the
// vault contract, declared here in the consumer so the read-only status
// projection satisfies it without this package importing the write face.
// That direction is load-bearing rather than stylistic: search only reads, and
// naming the package that edits notes here would put the one component able to
// change the vault inside the import closure of the one a reader reaches on
// every query. What the search face needs is the read-only half of the status
// projection, so that is what it asks for.
type StatusVocabulary interface {
	// Closed reports whether this view can classify a governed instance at
	// all. A folder that declared no contract and one whose contract cannot be
	// honoured are both closed: neither holds a vocabulary to measure against.
	Closed() bool
	// KnownStatus reports whether the contract declares status for noteType.
	KnownStatus(noteType, status string) bool
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
	lang := origin.Language(r)
	results, total, diagnostic, tokens := h.query(snap.Index, q, lang)

	view := answerView(snap, q, results, total, diagnostic, tokens)
	view.Sidebar = pages.NewSidebar(snap.Shell.Nav, "")
	if err := pages.Search(view, layouts.ChromeFromRequest(r, wording.SearchTitle.In(lang))).Render(r.Context(), w); err != nil {
		h.logQueryWriteFailure(r, "write search page", q, err)
	}
}

// answerView is everything both faces say about one query. The full page and
// the live region are the same answer rendered in two places, so what they say
// is built once: a sentence added to one and forgotten in the other is a
// difference a reader meets by typing rather than submitting.
func answerView(
	snap RequestSnapshot,
	q string,
	results []lexical.Result,
	total int,
	diagnostic string,
	tokens []string,
) pages.SearchView {
	parsed := lexical.Parse(q)
	return pages.SearchView{
		Query:             q,
		Results:           viewResults(results, snap.Shell.Governed, snap.Status, tokens),
		Total:             total,
		Diagnostic:        diagnostic,
		Governed:          snap.Shell.Governed,
		StepBacks:         stepBackViews(snap.Index, q, results, diagnostic),
		UnknownFilterKeys: parsed.UnknownFilterKeys(),
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
	results, total, diagnostic, tokens := h.query(snap.Index, q, origin.Language(r))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	view := answerView(snap, q, results, total, diagnostic, tokens)
	if err := pages.SearchResults(view, origin.Language(r)).Render(r.Context(), w); err != nil {
		h.logQueryWriteFailure(r, "write search results", q, err)
	}
}

// query parses once and hands back the hits, the true match tally, and the
// terms that produced them, so the page can mark those terms in a snippet
// without parsing again. The hits are bounded by maxRenderedResults; total is
// not, so the page can stay honest about what the bounded list leaves out.
func (h *Handler) query(idx *lexical.Index, q string, lang wording.Lang) (results []lexical.Result, total int, diagnostic string, tokens []string) {
	parsed := lexical.Parse(q)
	results, total, err := idx.SearchN(parsed, maxRenderedResults)
	if errors.Is(err, lexical.ErrMetadataUnavailable) {
		return nil, 0, unavailableSentence(err, lang), nil
	}
	if err != nil {
		h.logQueryError("search query", q, err)
		return nil, 0, wording.SearchUnavailable.In(lang), nil
	}
	return results, total, "", parsed.Tokens()
}

// unavailableSentence says why a metadata query could not be answered, in the
// language this reader is reading in. The vault-level fault is the one the
// contract cannot write for itself: it is settled at startup, before anyone
// has asked for a page, so the contract hands over the reason and the loader's
// error and the sentence is built here. Any other rejection carries an
// operator's line already written, and that is what is shown.
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
// for any other page state: a capability diagnostic already explains itself,
// and a page with results needs no loosening.
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

// logQueryWriteFailure reports a page that could not be finished. It carries
// the same facts as a query fault and differs only in loudness: a reader who
// closed the tab mid-response is not a fault an operator can act on.
func (h *Handler) logQueryWriteFailure(r *http.Request, message, rawQuery string, err error) {
	h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), message, queryFacts(rawQuery, err)...)
}

// queryFacts is what a search fault may say about the query: its shape and its
// size, never its text.
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

// viewResults maps the engine's results onto the view's plain field type,
// keeping internal/ui/pages free of any dependency on this package (the same
// one-directional import shape the reading page uses).
//
// A hit's status is carried only for a vault something governs. The index will
// happily match and return raw frontmatter for an ungoverned folder — that is a
// text field like any other — but a status chip presents it as a value drawn
// from a declared vocabulary, which is a claim no contract backs there.
func viewResults(results []lexical.Result, governed bool, vocabulary StatusVocabulary, tokens []string) []pages.SearchResult {
	// One question decides it, asked of the vocabulary itself rather than
	// inferred from the shell beside it: can this view classify a governed
	// instance at all. A folder that declared no contract and one whose
	// contract cannot be honoured both answer no, and both would otherwise
	// answer "not declared" to every value — marking every governed row in the
	// vault as a fault. Knowing no vocabulary, the row accuses nothing.
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
		// The warning is a property of the status the row shows, so it is
		// decided where that status is set: a row that names no status can
		// never carry a verdict about one.
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
// fell is the index's answer, because it is the index that folded the text and
// matched on the folded form; what a matched stretch looks like is this page's.
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
