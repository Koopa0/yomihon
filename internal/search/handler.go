// Package search is the reading surface for a query: the route, the language
// the answer is written in, and the view a result becomes. It puts the query
// to the lexical index and owns nothing about how the index answers.
package search

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

const maxQueryBytes = 4096

// RequestSnapshot is the search index and shell state bound to one request
// capture of an atomic vault generation and its artifact authority.
type RequestSnapshot struct {
	Index *lexical.Index
	Shell nav.Shell
	// Roles says which note types the rules give a navigation role, so a filter
	// row names a type in the vocabulary's word rather than the vault's own.
	Roles schema.NavigationRoles
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
	// A search answers in notes: the vault's other files stay in the index but
	// are neither found nor counted here.
	snap.Index = snap.Index.Notes()
	lang := origin.Language(r)
	st := readState(r, q)
	view := answerView(&snap, h.query(snap.Index, q, lang, wholeAnswer), st, lang)
	view.Sidebar = pages.NewSidebar(snap.Shell, "")
	chrome := layouts.ChromeFromRequest(r, wording.SearchTitleFor(lang, q))
	chrome.SearchQuery = view.Query
	if err := pages.Search(view, chrome).Render(r.Context(), w); err != nil {
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

// answerView is everything the page says about one query: the notes laid out
// so far in the order asked for, the rows that narrow them, and the way on.
// The fragment a live refresh asks for is built from the same view, so the two
// cannot drift apart.
func answerView(snap *RequestSnapshot, a *answer, st pages.SearchState, lang wording.Lang) pages.SearchView {
	q := st.Query
	nar := narrow(a.found.Results, st, snap.Roles, lang)
	laid := orderedBy(nar.results, st.Sort, snap.Shell.Nav)
	shown := min(max(st.Shown, pages.SearchStep), len(laid))
	view := pages.SearchView{
		Query:             q,
		Results:           viewResults(laid[:shown], a.tokens, snap.Roles, lang),
		Total:             len(nar.results),
		Diagnostic:        a.diagnostic,
		Sort:              sortChoices(st, lang),
		Kinds:             nar.kinds,
		Locations:         nar.locations,
		Shown:             shown,
		StepBacks:         stepBackViews(snap.Index, q, a.found.Results, a.diagnostic),
		UnknownFilterKeys: a.parsed.UnknownFilterKeys(),
		FilterKeys:        lexical.FilterKeys(),
	}
	if st.Sort == pages.SearchSortRecent && snap.Shell.Nav != nil {
		view.ByDate, view.Now = true, time.Now()
		for i := range view.Results {
			view.Results[i].Changed = snap.Shell.Nav.Listed(view.Results[i].RelPath).Modified
		}
	}
	if shown < len(laid) {
		more := st
		more.Shown = shown + pages.SearchStep
		view.More = pages.SearchMore{Count: min(pages.SearchStep, len(laid)-shown), Href: more.Address()}
	}
	if a.found.Total == 0 && a.diagnostic == "" {
		view.LiteralOperators = a.parsed.LiteralOperatorTerms()
		for _, s := range snap.Index.FolderSuggestions(q) {
			view.FolderSuggestions = append(view.FolderSuggestions, pages.SearchStepBack{Query: s.Query, Count: s.Count})
		}
		if word := snap.Index.NearWord(q); word != "" {
			view.Near = pages.SearchNear{Word: word, Href: pages.SearchState{Query: word}.Address()}
		}
	}
	return view
}

// orderedBy lays hits out in the order the reader asked for. The index's own
// order is by relevance; by last change puts the newest edit first and keeps
// relevance among notes edited at the same moment.
func orderedBy(hits []lexical.Result, sort string, m *nav.Model) []lexical.Result {
	if sort != pages.SearchSortRecent || m == nil {
		return hits
	}
	hits = slices.Clone(hits)
	slices.SortStableFunc(hits, func(a, b lexical.Result) int {
		return m.Listed(b.RelPath).Modified.Compare(m.Listed(a.RelPath).Modified)
	})
	return hits
}

// results renders only the lexical-results region used by progressive search.
// The ordinary /search form remains the complete no-JavaScript navigation path.
func (h *Handler) results(w http.ResponseWriter, r *http.Request) {
	q, ok := requestQuery(w, r)
	if !ok {
		return
	}
	snap := h.snapshot()
	// A search answers in notes: the vault's other files stay in the index but
	// are neither found nor counted here.
	snap.Index = snap.Index.Notes()
	lang := origin.Language(r)
	panel := r.URL.Query().Get(shapeParam) == shapePanel

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if panel {
		view := panelView(&snap, h.query(snap.Index, q, lang, panelNotes), q)
		if err := pages.SearchPanel(view, lang).Render(r.Context(), w); err != nil {
			h.logQueryWriteFailure(r, "write search panel", q, err)
		}
		return
	}
	view := answerView(&snap, h.query(snap.Index, q, lang, wholeAnswer), readState(r, q), lang)
	if err := pages.SearchResults(view, lang).Render(r.Context(), w); err != nil {
		h.logQueryWriteFailure(r, "write search results", q, err)
	}
}

// The two shapes this route answers in. The page asks for the whole answer
// laid out as a list; the field's dropdown and the phone surface ask for the
// few rows that fit under a field.
const (
	shapeParam = "shape"
	shapePanel = "panel"
)

// wholeAnswer asks the index for every hit, and panelNotes for the few a
// dropdown shows, the tally being taken over all of them either way.
const (
	wholeAnswer = -1
	panelNotes  = 6
)

// query reads one query and answers it, building as many hits as limit allows;
// the tally is over every hit either way. The parse travels in the answer
// because the view needs it too, and reading the same text twice per request
// is the cost this saves.
func (h *Handler) query(idx *lexical.Index, q string, lang wording.Lang, limit int) *answer {
	parsed := lexical.Parse(q)
	found, err := idx.Search(parsed, limit)
	if errors.Is(err, lexical.ErrMetadataUnavailable) {
		return &answer{parsed: parsed, diagnostic: unavailableSentence(err, lang)}
	}
	if err != nil {
		h.logQueryError("search query", q, err)
		return &answer{parsed: parsed, diagnostic: wording.SearchUnavailable.In(lang)}
	}
	return &answer{parsed: parsed, found: found, tokens: parsed.Tokens()}
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

// viewResults maps the engine's results onto the view's plain field type.
func viewResults(results []lexical.Result, tokens []string, roles schema.NavigationRoles, lang wording.Lang) []pages.SearchResult {
	out := make([]pages.SearchResult, len(results))
	for i := range results {
		r := &results[i]
		out[i] = pages.SearchResult{
			RelPath:       r.RelPath,
			Title:         r.Title,
			Draft:         r.Draft,
			Language:      r.Language,
			TitleRuns:     snippetRuns(r.Title, tokens),
			Lead:          excerptRuns(r.Lead.Runs),
			LeadNear:      excerptRuns(r.Lead.Near),
			LeadCode:      r.Lead.Code,
			WhereRuns:     whereRuns(r, tokens, lang),
			AliasRuns:     snippetRuns(r.Alias, tokens),
			TagRuns:       viewRuns(lexical.MarkTagHits(r.Tag, tokens)),
			Landing:       r.Landing,
			LandingBare:   r.LandingBare,
			LandingEnd:    r.LandingEnd,
			LandingPrefix: r.LandingPrefix,
			LandingSuffix: r.LandingSuffix,
			BlockCrossing: r.BlockCrossing,
			FromFence:     r.FromFence,
		}
		if kind := kindOf(r, roles); kind != kindNote {
			out[i].Kind = kindWord(kind, lang)
		}
	}
	return out
}

// whereRuns is the location line of a result. Hits are marked in the title and
// the excerpt; the line carries one only for a note the query met by its folder
// alone, where nothing else on the row would say why it is an answer.
func whereRuns(r *lexical.Result, tokens []string, lang wording.Lang) []pages.SnippetRun {
	where := whereWords(r.RelPath, lang)
	if len(r.Lead.Runs) > 0 || r.Alias != "" || r.Tag != "" || slices.ContainsFunc(snippetRuns(r.Title, tokens), func(run pages.SnippetRun) bool { return run.Hit }) {
		return []pages.SnippetRun{{Text: where}}
	}
	return plainRuns(where, tokens)
}

func excerptRuns(runs []lexical.ExcerptRun) []pages.SnippetRun {
	if len(runs) == 0 {
		return nil
	}
	out := make([]pages.SnippetRun, len(runs))
	for i, run := range runs {
		out[i] = pages.SnippetRun{Text: run.Text, Hit: run.Hit, Deleted: run.Deleted}
	}
	return out
}

// snippetRuns dresses one piece of matched text for the page. Where the query
// fell is the index's answer; what a matched stretch looks like is this page's.
func snippetRuns(text string, tokens []string) []pages.SnippetRun {
	return viewRuns(lexical.MarkHits(text, tokens))
}

// plainRuns is snippetRuns for a text that is shown whether or not the query
// touched it: with no hit it is the one unmarked run.
func plainRuns(text string, tokens []string) []pages.SnippetRun {
	if text == "" {
		return nil
	}
	if runs := snippetRuns(text, tokens); runs != nil {
		return runs
	}
	return []pages.SnippetRun{{Text: text}}
}

func viewRuns(marked []lexical.HitRun) []pages.SnippetRun {
	if len(marked) == 0 {
		return nil
	}
	runs := make([]pages.SnippetRun, len(marked))
	for i, run := range marked {
		runs[i] = pages.SnippetRun{Text: run.Text, Hit: run.Hit}
	}
	return runs
}
