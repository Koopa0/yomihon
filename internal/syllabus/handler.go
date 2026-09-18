// Package syllabus serves the study-path page: one study-path tree with a
// switcher across every study-path in the vault. It reads paths the navigation
// model already parsed, and parses no note itself.
package syllabus

import (
	"log/slog"
	"net/http"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// RequestSnapshot is everything one study-path request reads, captured
// together. The shell and the generation it was projected from arrive as one
// answer because a page that took them from two readings of the published
// pointer can state a course's names from one version of the vault and its
// declared languages from another. Deriving the shell here instead is not open
// to this package: the projection also needs the write authority, which the
// study-path face has no other reason to hold.
type RequestSnapshot struct {
	Shell      nav.Shell
	Generation *snapshot.Generation
	// Kept is the place the reader deliberately left off at and Marked whether
	// they left one. A course compares the note it names against the rows it
	// already lists and follows nothing: a place kept anywhere else changes
	// nothing about the page.
	Kept   mark.Continuation
	Marked bool
}

// Handler serves the study-path page.
type Handler struct {
	current func() RequestSnapshot
	log     *slog.Logger
}

// New wires the syllabus feature. Every dependency must be non-nil: a nil
// is a wiring bug that must fail here, not on the first request.
func New(current func() RequestSnapshot, log *slog.Logger) *Handler {
	if current == nil {
		panic("syllabus: New requires a non-nil current-generation provider")
	}
	if log == nil {
		panic("syllabus: New requires a non-nil Log")
	}
	return &Handler{current: current, log: log}
}

// Register mounts the study-path index and one study path's own page.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /paths", h.index)
	mux.HandleFunc("GET /syllabus/{path...}", h.show)
}

// show renders the study-path whose vault path matches the request. An unknown
// path, and an empty model before the first scan, both answer the not-found
// page rather than a line of text.
func (h *Handler) show(w http.ResponseWriter, r *http.Request) {
	// A vault holds its names composed and a request can carry either spelling
	// of the same letter, so the name is composed before it is looked up.
	rel := vault.NormalizeNFC(r.PathValue("path"))

	lang := origin.Language(r)
	request := h.current()
	shell := request.Shell
	current := shell.Nav.Path(rel)
	if current == nil {
		view := pages.NotFoundView{Asked: r.URL.Path, Sidebar: pages.NewSidebar(shell.Nav, "")}
		// The title names which route refused; the page below it is shared.
		chrome := layouts.ChromeFromRequest(r, wording.PathNotFound.In(lang))
		if err := pages.WriteNotFound(r.Context(), w, view, chrome); err != nil {
			h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write study-path not-found page", "path", rel, "error", err)
		}
		return
	}

	// Where the reader is in this course, as the note they opened it from. It
	// is composed like the path above, because it names the same kind of thing
	// and a query can carry either spelling too. Nothing is followed: the value
	// is compared against the rows this course already lists, so a note the
	// course does not teach — and a reader who arrived from the desk carrying
	// nothing — marks no row.
	cover := pages.CourseCover{Here: vault.NormalizeNFC(r.URL.Query().Get(pages.SyllabusFromParam))}
	cover.OpeningHTML, cover.OpeningLanguage = h.opening(request.Generation, rel, lang)
	if request.Marked {
		cover.KeptNote = request.Kept.RelPath
		cover.KeptHref = pages.ResumeHref(request.Kept.RelPath, request.Kept.Anchor, request.Kept.Offset)
	}

	view := pages.BuildPathView(current, shell.Nav.Paths(), cover)
	if err := pages.Syllabus(view, layouts.ChromeFromRequest(r, current.Title)).Render(r.Context(), w); err != nil {
		h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write syllabus page", "path", rel, "error", err)
	}
}

// opening is the course note's own words above its first heading, rendered, and
// the language that note declared them in.
//
// It is rendered through the generation the rest of the page was built from, so
// a link the author wrote in those words resolves against the vault the page is
// describing rather than against whatever the folder holds a moment later. The
// places inside it come off afterwards: the opening shares a document with the
// course below it, and every name it brought would be a second element
// answering to one the page already has.
//
// A note this generation has no reading of — the file went missing between the
// scan and this request — has no opening to print, and the page draws none. The
// course itself is drawn from navigation and stands either way.
func (h *Handler) opening(snap *snapshot.Generation, rel string, lang wording.Lang) (html, language string) {
	note, ok := snap.Note(rel)
	if !ok {
		return "", ""
	}
	source := render.Opening(note.Body)
	if source == "" {
		return "", ""
	}
	return render.StripAnchors(snap.RenderIn(openingRegion, rel, source, lang).HTML), note.Language
}

// openingRegion names the cover's opening among the separately rendered bodies
// one page can carry, so a footnote the author wrote in it cannot answer to an
// id something else already claimed. It is fixed rather than counted: a course
// prints one opening and every reader of the page receives the same bytes.
const openingRegion = "c-"
