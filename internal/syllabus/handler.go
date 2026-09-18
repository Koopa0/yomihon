// Package syllabus serves a study path as a course: the study-path tree with a
// switcher across every study path in the vault, and the same course read
// aloud. The tree reads paths the navigation model already parsed; the
// listening page renders each lesson the course teaches, because a note's
// read-aloud markers survive only into rendered HTML and the snapshot keeps
// none.
package syllabus

import (
	"log/slog"
	"net/http"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// RequestSnapshot is everything a study-path page reads from one capture of an
// atomic vault generation. They arrive as one answer because a page that took
// them from two readings of the published pointer can state a course's names
// from one version of the vault and its lessons' words from another. Deriving
// the shell here instead is not open to this package: the projection also needs
// the write authority, which the study-path face has no other reason to hold.
type RequestSnapshot struct {
	Shell      nav.Shell
	Generation *snapshot.Generation

	// Status answers which note type this vault files its course members as.
	// It may be absent, and the listening page then teaches nothing rather
	// than guessing at the name.
	Status LessonTypes
}

// LessonTypes is the read-only sliver of the status projection this face
// needs, declared at the consumer so a page that only reads does not put the
// component able to change the vault inside its import closure.
type LessonTypes interface {
	// IsLessonType reports whether noteType is the type the vault files its
	// course members as.
	IsLessonType(noteType string) bool
}

// Handler serves the study-path page and the same course read aloud.
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

// Register mounts the study-path index, one study path's own page, and that
// path read aloud.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /paths", h.index)
	mux.HandleFunc("GET /syllabus/{path...}", h.show)
	mux.HandleFunc("GET /listen/{path...}", h.listen)
}

// show renders the study-path whose vault path matches the request. An unknown
// path, and an empty model before the first scan, both answer the not-found
// page rather than a line of text.
func (h *Handler) show(w http.ResponseWriter, r *http.Request) {
	// A vault holds its names composed and a request can carry either spelling
	// of the same letter, so the name is composed before it is looked up.
	rel := vault.NormalizeNFC(r.PathValue("path"))

	shell := h.current().Shell
	current := shell.Nav.Path(rel)
	if current == nil {
		lang := origin.Language(r)
		view := pages.NotFoundView{Asked: r.URL.Path, Sidebar: pages.NewSidebar(shell, "")}
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
	here := vault.NormalizeNFC(r.URL.Query().Get(pages.SyllabusFromParam))

	view := pages.BuildPathView(current, shell.Nav.Paths(), here)
	view.Vault = shell.Vault
	if err := pages.Syllabus(view, layouts.ChromeFromRequest(r, current.Title)).Render(r.Context(), w); err != nil {
		h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write syllabus page", "path", rel, "error", err)
	}
}
