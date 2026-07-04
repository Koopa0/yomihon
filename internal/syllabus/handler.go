// Package syllabus serves the study-path page: a full-page render of one
// parsed study-path tree, with a switcher across every study-path in the vault.
// It reads only the navigation model (internal/nav's already-parsed Syllabi,
// held behind the atomic snapshot); it never parses notes or touches the schema
// contract, so it renders whether or not the write face is available.
package syllabus

import (
	"log/slog"
	"net/http"

	"github.com/koopa0/kurodo/internal/nav"
	"github.com/koopa0/kurodo/internal/ui/pages"
)

// Deps is what the syllabus feature reads from: the current navigation model
// (read fresh per request, since it lives behind the snapshot) and a logger.
// Nav is a plain closure because "give me the current model" is a closure, not
// a method set — mirroring the reading feature's Nav provider.
type Deps struct {
	Nav func() *nav.Model
	Log *slog.Logger
}

// Handler serves the study-path page.
type Handler struct {
	deps Deps
}

// NewHandler wires the syllabus feature. Every dependency must be non-nil: a nil
// is a wiring bug that must fail here, not on the first request.
func NewHandler(d Deps) *Handler {
	if d.Nav == nil {
		panic("syllabus: NewHandler requires a non-nil Nav provider")
	}
	if d.Log == nil {
		panic("syllabus: NewHandler requires a non-nil Log")
	}
	return &Handler{deps: d}
}

// Register mounts the feature's route.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /syllabus/{path...}", h.show)
}

// show renders the study-path whose vault path matches the request. An unknown
// path (or an empty model, before the first scan) is a 404 — the same
// fail-quiet stance the reading page takes for a missing note.
func (h *Handler) show(w http.ResponseWriter, r *http.Request) {
	rel := r.PathValue("path")

	model := h.deps.Nav()
	current := findSyllabus(model, rel)
	if current == nil {
		http.NotFound(w, r)
		return
	}

	view := pages.BuildSyllabusView(*current, model.Syllabi)
	if err := pages.Syllabus(view, pages.ChromeFromRequest(r, current.Title)).Render(r.Context(), w); err != nil {
		h.deps.Log.Error("write syllabus page", "path", rel, "error", err)
	}
}

// findSyllabus returns the study-path with the given vault-relative path, or nil
// when the model is nil or none matches.
func findSyllabus(model *nav.Model, rel string) *nav.Syllabus {
	if model == nil {
		return nil
	}
	for i := range model.Syllabi {
		if model.Syllabi[i].RelPath == rel {
			return &model.Syllabi[i]
		}
	}
	return nil
}
