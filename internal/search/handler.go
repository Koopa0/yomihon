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

// searchPageSize is how many hits one page of the answer holds. A result row
// is three lines — the title, the address, the excerpt — so twenty is about
// two screen-heights at a desk and the strip under them is reached in one
// scroll; higher and the strip is never seen, lower and an ordinary one-word
// query becomes a stack of pages.
//
// The index takes a bound and no offset, so a later page is asked for as
// everything up to its end and the pages walked past are materialized on the
// way. That is the price of leaving one way to ask the index rather than two;
// the divisions beside the rows are unaffected, being tallied over every hit
// before the bound cuts the list.
const searchPageSize = 20

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
	asked := pages.ParsePageNumber(r.URL.Query().Get("page"))
	// The page itself always has the column and the strip, so the first render
	// never depends on the marker the live refreshes carry.
	view := answerView(snap, q, h.query(snap.Index, q, lang, asked), lang, asked, true)
	view.Sidebar = pages.NewSidebar(snap.Shell.Nav, "")
	if err := pages.Search(view, layouts.ChromeFromRequest(r, wording.SearchTitle.In(lang))).Render(r.Context(), w); err != nil {
		h.logQueryWriteFailure(r, "write search page", q, err)
	}
}

// answer is what one query came to: how it was read, what the index found,
// the sentence to show in its place when the index could not answer, and the
// terms the hits matched. It belongs to one request — query builds it,
// answerView reads it, and nothing keeps it. It travels by pointer for its
// size, not so anyone can change it.
type answer struct {
	parsed     *lexical.Query
	found      lexical.Answer
	diagnostic string
	tokens     []string
}

// answerView is everything both faces say about one query, built once so the
// full page and the live region cannot drift apart. onPage says whether this
// face is the search page itself rather than the palette floating over another
// one: only the page has a column to put the answer's divisions in, and only
// the page has room under the rows for the way to the rest of them.
//
// The hits the index built run from the answer's start to the end of the page
// asked for, so the stretch this page shows is the tail of them. A face with
// no strip keeps what it was given and says how far the list was cut.
func answerView(snap RequestSnapshot, q string, a *answer, lang wording.Lang, asked pages.PageNumber, onPage bool) pages.SearchView {
	found := a.found.Results
	var strip pages.Pager
	if onPage {
		strip = pages.NewPager(asked, searchPageSize, a.found.Total, func(n pages.PageNumber) string {
			return pages.SearchPageHref(q, n)
		})
		found = found[strip.First:strip.Last]
	}
	view := pages.SearchView{
		Query:             q,
		Results:           viewResults(found, snap.Shell.Governed, snap.Status, a.tokens),
		Pager:             strip,
		Total:             a.found.Total,
		Diagnostic:        a.diagnostic,
		Governed:          snap.Shell.Governed,
		StepBacks:         stepBackViews(snap.Index, q, a.found.Results, a.diagnostic),
		UnknownFilterKeys: a.parsed.UnknownFilterKeys(),
		FilterKeys:        lexical.FilterKeys(),
	}
	if onPage && a.diagnostic == "" && snap.Shell.Governed {
		view.Facets = facetViews(q, a.found.Facets, snap.Status, lang)
	}
	return view
}

// facetViews maps the index's divisions onto the column the page draws, and
// builds each row's search by asking the grammar to write it. Removal and
// addition both go through the parser's own rewriting, so a row can only offer
// a query this grammar reads back as the one the row's count was taken from —
// and the reader's own spelling, spacing and quoting survive untouched on
// every part of the query the row is not about.
//
// A division no hit stated a value for draws nothing: a column reading "not
// stated: 12" and offering nothing else narrows no search.
func facetViews(q string, divisions []lexical.Facet, vocabulary StatusVocabulary, lang wording.Lang) []pages.SearchFacet {
	// Asked of the vocabulary rather than inferred from the shell, for the same
	// reason the result rows ask: a view that knows none would otherwise mark
	// every value a fault.
	rules := vocabulary != nil && !vocabulary.Closed()
	out := make([]pages.SearchFacet, 0, len(divisions))
	for _, division := range divisions {
		if len(division.Values) == 0 {
			continue
		}
		facet := pages.SearchFacet{
			Key:     division.Key,
			Heading: wording.FacetHeading(division.Key, lang),
			Rows:    make([]pages.SearchFacetRow, 0, len(division.Values)+1),
		}
		for _, value := range division.Values {
			facet.Rows = append(facet.Rows, facetRow(q, division.Key, value, rules, vocabulary))
		}
		if division.Unstated > 0 {
			facet.Rows = append(facet.Rows, pages.SearchFacetRow{
				Label:    wording.FacetUnstated.In(lang),
				Count:    division.Unstated,
				Unstated: true,
			})
		}
		out = append(out, facet)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// facetRow writes one value's row. Whether the query already names the value is
// decided by removing it and seeing whether anything changed, rather than by a
// second comparison of folded values kept here: the grammar decides what names
// what, and the answer and the link the row offers then cannot disagree.
func facetRow(q, key string, value lexical.FacetValue, rules bool, vocabulary StatusVocabulary) pages.SearchFacetRow {
	row := pages.SearchFacetRow{Label: value.Value, Count: value.Count}
	if rules && key == lexical.StatusFilterKey {
		row.OutsideEnum = !declaredByACarrier(vocabulary, value)
	}
	constraint := lexical.Filter{Key: key, Value: value.Value}
	if without := lexical.WithoutFilter(q, constraint); without != q {
		row.Query, row.Active = without, true
		return row
	}
	// A value this grammar cannot spell back — one carrying a quote character —
	// leaves the row a cell with its count and no offer, the way a lifecycle
	// square with nothing to query stops being a link.
	if narrowed, ok := lexical.WithFilter(q, constraint); ok {
		row.Query = narrowed
	}
	return row
}

// declaredByACarrier reports whether any note type holding this status declares
// it. A status is declared per type, so the value alone cannot be ruled on: the
// same value is vocabulary under one type and outside every enum under another,
// and a verdict reached without the types would contradict the rows beneath it.
func declaredByACarrier(vocabulary StatusVocabulary, value lexical.FacetValue) bool {
	for _, noteType := range value.Carriers {
		if vocabulary.KnownStatus(noteType, value.Value) {
			return true
		}
	}
	return false
}

// results renders only the lexical-results region used by progressive search.
// The ordinary /search form remains the complete no-JavaScript navigation path.
func (h *Handler) results(w http.ResponseWriter, r *http.Request) {
	q, ok := requestQuery(w, r)
	if !ok {
		return
	}
	snap := h.snapshot()
	lang := origin.Language(r)
	asked := pages.ParsePageNumber(r.URL.Query().Get("page"))
	answered := h.query(snap.Index, q, lang, asked)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	view := answerView(snap, q, answered, lang, asked, wantsFacets(r))
	if err := pages.SearchResults(view, lang).Render(r.Context(), w); err != nil {
		h.logQueryWriteFailure(r, "write search results", q, err)
	}
}

// query reads one query and answers it. The hits are built as far as the end
// of the page asked for, and the whole listing where that is what was asked;
// the tally and the divisions are taken over every hit either way, so a page
// can state what the whole answer holds while showing one stretch of it. The
// parse travels in the answer because the view needs it too, and reading the
// same text twice per request is the cost this saves.
func (h *Handler) query(idx *lexical.Index, q string, lang wording.Lang, asked pages.PageNumber) *answer {
	parsed := lexical.Parse(q)
	found, err := idx.Search(parsed, searchLimit(asked))
	if errors.Is(err, lexical.ErrMetadataUnavailable) {
		return &answer{parsed: parsed, diagnostic: unavailableSentence(err, lang)}
	}
	if err != nil {
		h.logQueryError("search query", q, err)
		return &answer{parsed: parsed, diagnostic: wording.SearchUnavailable.In(lang)}
	}
	return &answer{parsed: parsed, found: found, tokens: parsed.Tokens()}
}

// searchLimit is how far into the answer the index is asked to build. A page
// is everything up to its end, because the index takes a bound and no offset;
// the undivided listing is asked for whole.
func searchLimit(asked pages.PageNumber) int {
	if asked == pages.AllPages {
		return -1
	}
	return int(asked) * searchPageSize
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

// facetsParam is the request's name for the one difference between the two
// faces that ask this endpoint for the same rows. The search page's live region
// carries it on the endpoint it names; the command palette's does not, because
// it floats over a page and has no column to put a division in.
//
// It is what keeps the palette's answer exactly what it was: without it the
// only way to serve one fragment to two faces would be to send the column to
// both and paint it out of one, which is markup a reader's browser is handed
// and told to ignore. A handler test holds the difference, because two routes
// that look alike are what a later tidying merges.
const facetsParam = "facets"

// wantsFacets reads that marker off the request. It is a query parameter and
// not an environment read: the process still has exactly one variable.
func wantsFacets(r *http.Request) bool {
	return r.URL.Query().Get(facetsParam) == "1"
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
	for i := range results {
		r := &results[i]
		out[i] = pages.SearchResult{
			RelPath:       r.RelPath,
			Title:         r.Title,
			Language:      r.Language,
			Snippet:       r.Snippet,
			SnippetRuns:   snippetRuns(r.Snippet, tokens),
			PathRuns:      snippetRuns(r.RelPath, tokens),
			AliasRuns:     snippetRuns(r.Alias, tokens),
			TopicRuns:     snippetRuns(r.Topic, tokens),
			File:          r.File,
			Landing:       r.Landing,
			LandingBare:   r.LandingBare,
			LandingEnd:    r.LandingEnd,
			LandingPrefix: r.LandingPrefix,
			BlockCrossing: r.BlockCrossing,
			FromFence:     r.FromFence,
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
