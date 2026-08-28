package search

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"unicode"

	"github.com/koopa0/yomihon/internal/ui/pages"
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
	Index *Index
	Shell pages.Shell

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
// That direction is load-bearing rather than stylistic: the offline
// retrieval commands are built from this package and are the only part of
// yomihon allowed to reach a network, and their dependency closure is held
// to a reviewed list. Naming the write face here
// would put the one component that edits the vault inside the closure of the
// one component that can leave the machine.
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
	results, total, diagnostic, tokens := h.query(snap.Index, q)

	view := pages.SearchView{
		Query:      q,
		Results:    viewResults(results, snap.Shell.Governed, snap.Status, tokens),
		Total:      total,
		Diagnostic: diagnostic,
		Governed:   snap.Shell.Governed,
		StepBacks:  stepBackViews(snap.Index, q, results, diagnostic),
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
	results, total, diagnostic, tokens := h.query(snap.Index, q)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	stepBacks := stepBackViews(snap.Index, q, results, diagnostic)
	if err := pages.SearchResults(q, viewResults(results, snap.Shell.Governed, snap.Status, tokens), total, diagnostic, snap.Shell.Governed, stepBacks).Render(r.Context(), w); err != nil {
		h.logQueryError("write search results", q, err)
	}
}

// query parses once and hands back the hits, the true match tally, and the
// terms that produced them, so the page can mark those terms in a snippet
// without parsing again. The hits are bounded by maxRenderedResults; total is
// not, so the page can stay honest about what the bounded list leaves out.
func (h *Handler) query(idx *Index, q string) (results []Result, total int, diagnostic string, tokens []string) {
	parsed := Parse(q)
	results, total, err := idx.search(parsed, maxRenderedResults)
	if errors.Is(err, ErrMetadataUnavailable) {
		return nil, 0, err.Error(), nil
	}
	if err != nil {
		h.logQueryError("search query", q, err)
		return nil, 0, "搜尋目前暫時無法使用。", nil
	}
	return results, total, "", parsed.Tokens()
}

// stepBackViews computes the loosened offers for an empty answer, and nothing
// for any other page state: a capability diagnostic already explains itself,
// and a page with results needs no loosening.
func stepBackViews(idx *Index, q string, results []Result, diagnostic string) []pages.SearchStepBack {
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
func viewResults(results []Result, governed bool, vocabulary StatusVocabulary, tokens []string) []pages.SearchResult {
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
			SnippetRuns: markHits(r.Snippet, tokens),
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

// markHits cuts a snippet into the stretches that matched and the stretches
// that did not, so the page can show the reader why this result is here.
//
// Matching is done on the same folded form the index matched on, and the runs
// carry slices of the original text, so what the reader sees is their own note
// and not a re-cased copy of it. Overlapping matches are merged: two tokens
// that cover the same words produce one mark rather than nested ones.
//
// A match offset lives in the folded copy, and lowercasing does not preserve
// length, so every offset is carried back through the fold's source mapping
// before it touches the snippet: covered is marked and the runs are sliced in
// the snippet's own bytes, never the fold's.
func markHits(snippet string, tokens []string) []pages.SnippetRun {
	if snippet == "" || len(tokens) == 0 {
		return nil
	}
	fold, src := foldWithSourceOffsets(snippet)
	covered := make([]bool, len(snippet))
	found := false
	for _, t := range tokens {
		if t == "" {
			continue
		}
		for at := 0; at <= len(fold); {
			i, stop := phraseIndex(fold, t, at)
			if i < 0 {
				break
			}
			for j := src[i]; j < src[stop]; j++ {
				covered[j] = true
			}
			found = true
			at = max(stop, i+1)
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

// foldWithSourceOffsets lowercases s, and maps every byte position of the
// folded copy — including one past its end — back to the byte offset in s of
// the character it came from. The index's fold is NFC then lowercase; this
// applies the lowercase half alone, which reproduces the index's fold under
// one precondition: s is already NFC. Every snippet satisfies it, because a
// snippet is cut from the entry's stored text and the entry stored that text
// normalized. Lowercasing does not preserve length: Ⱥ grows from two bytes to
// three, a byte that is not valid UTF-8 becomes the three-byte replacement
// character, and Turkish İ shrinks from two bytes to one. An offset found in
// the folded copy therefore cannot index s directly; it has to come back
// through this mapping.
func foldWithSourceOffsets(s string) (fold string, src []int) {
	var folded strings.Builder
	folded.Grow(len(s))
	src = make([]int, 0, len(s)+1)
	for i, r := range s {
		n := folded.Len()
		folded.WriteRune(unicode.ToLower(r))
		for ; n < folded.Len(); n++ {
			src = append(src, i)
		}
	}
	return folded.String(), append(src, len(s))
}
