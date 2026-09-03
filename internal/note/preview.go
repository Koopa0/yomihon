package note

import (
	"net/http"
	"strings"

	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// previewSourceCap bounds the markdown one card is cut from — the source, not
// the bytes the answer becomes: a cut taken after rendering would leave markup
// half-open and could stop inside a character, so the budget is spent where the
// words are still whole. A passage dense in fenced code therefore renders to
// more than this. A preview is a taste of a note, not a transfer of it: past a
// point the reader is scrolling inside a hover card rather than opening the
// note, which is what the link under their pointer is for.
const previewSourceCap = 24 << 10

// previewRegion names the card's body among the separately rendered bodies one
// reading page carries, so its footnotes cannot answer to the ids the article's
// own already claimed. It is fixed rather than counted, because only one card
// is ever filled and every reader of a page receives the same bytes.
const previewRegion = "p-"

// preview answers a reading page's hover card with an excerpt of the note the
// reader is pointing at, cut at whatever section or block that link addressed.
// It is a bare fragment with no page around it: the card is already open on the
// reader's own page, and everything a page carries — the shell, the navigation,
// the script tag — would be a second copy of what is already around them.
//
// The excerpt comes from the snapshot published now, not from the one the open
// page was rendered out of. Two requests cannot be pinned to one generation
// without carrying a token for it, and whether the page has fallen behind the
// file is already a question this feature has an answer to: the freshness watch
// polls it and says so in the reader's own words. A second, weaker answer here
// would be a second way to be right about one thing.
//
// Every refusal is answered with a card saying what it could not show rather
// than with nothing at all: a card that silently fails to appear is
// indistinguishable from a hover that did not register, and leaves the reader
// tapping at a link wondering which of the two happened. An address naming a
// place the note does not have is refused rather than widened — the reader
// named one place, and quietly handing them a different one reads as the place
// they named.
func (h *Handler) preview(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	rel := vault.NormalizeNFC(r.PathValue("path"))
	view, found := h.previewOf(rel, r.URL.Query().Get("section"), lang)

	// A live look at a note someone may be editing, so a held answer is the one
	// thing it must not give — the refusal the freshness poll makes, for the
	// same reason.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if !found {
		w.WriteHeader(http.StatusNotFound)
	}
	if err := pages.PreviewFragment(view, lang).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write preview card", "path", rel, "error", err)
	}
}

// previewOf cuts one card's excerpt out of the note at rel. section is the
// fragment the link's own address carries, already folded by the pass that
// wrote it, so nothing here re-reads a name: an empty one asks for the note
// itself.
//
// The false answer covers every way an address reaches no note — a path outside
// what this server hands over, a path that is not markdown, and a path this
// generation holds no note for. They are one answer because they are one fact
// to the reader hovering the link: there is nothing here to look at. A note
// that has no such place inside it is a different fact and stays a found one:
// the note is there, and the card says which of it is not.
func (h *Handler) previewOf(rel, section string, lang wording.Lang) (pages.PreviewView, bool) {
	if !servable(rel) || !vault.IsMarkdown(rel) {
		return pages.PreviewView{Notice: wording.PreviewNoNote.In(lang)}, false
	}
	snap := h.sources.Snapshot().Capture()
	n, ok := snap.Note(rel)
	if !ok {
		return pages.PreviewView{Notice: wording.PreviewNoNote.In(lang)}, false
	}
	slice, found := render.Excerpt(n.Body, section)
	if !found {
		return pages.PreviewView{RelPath: rel, Notice: missingPlace(section, lang)}, true
	}
	source, truncated := capPreviewSource(slice)
	view := pages.PreviewView{
		RelPath:  rel,
		Language: n.Language,
		// Rendered through the same generation the excerpt was cut from, so a
		// link inside the card resolves against the vault the card is showing.
		// The ids come off afterwards: the excerpt shares a document with the
		// page that opened it, and every name it brought would be a second
		// place answering to one the page already has.
		BodyHTML: render.StripAnchorIDs(snap.RenderIn(previewRegion, rel, source, lang).HTML),
	}
	if truncated {
		view.Notice = wording.PreviewMore.In(lang)
	}
	return view, true
}

// missingPlace is what the card says about an address naming a place its note
// does not have. The two sentences are the ones the diagnostics panel already
// gives a reader who follows such a link, so the card and the panel report one
// fact in one voice rather than in two.
func missingPlace(section string, lang wording.Lang) string {
	if strings.HasPrefix(section, "^") {
		return wording.DiagBlockNote.In(lang)
	}
	return wording.DiagSectionNote.In(lang)
}

// capPreviewSource shortens the markdown a card is cut from to the budget, and
// says whether anything was left behind. The cut lands on a line boundary and
// never inside a line: a rule that could stop mid-character would put a broken
// one on the card, and a rule that could stop mid-word would read as the note's
// own writing. A single line wider than the whole budget is kept whole — it is
// the budget that gives way there, not the words.
func capPreviewSource(source string) (capped string, truncated bool) {
	if len(source) <= previewSourceCap {
		return source, false
	}
	if cut := strings.LastIndexByte(source[:previewSourceCap], '\n'); cut >= 0 {
		return source[:cut], true
	}
	if first, _, ok := strings.Cut(source, "\n"); ok {
		return first, true
	}
	return source, false
}
