// Package syllabus serves the study-path page: a full-page render of one
// parsed study-path tree, with a switcher across every study-path in the vault.
// It reads internal/nav's already-parsed Paths plus the shared shell projection,
// both captured from one atomic snapshot; it never parses notes itself.
package syllabus

import (
	"log/slog"
	"net/http"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/ui/pages"
)

// Deps is what the syllabus feature reads from: the shared shell projection
// captured once per request and a logger. A concrete value keeps this small
// wiring seam from becoming a cross-package interface.
type Deps struct {
	Shell func() pages.ShellData
	Log   *slog.Logger
}

// Handler serves the study-path page.
type Handler struct {
	deps Deps
}

// NewHandler wires the syllabus feature. Every dependency must be non-nil: a nil
// is a wiring bug that must fail here, not on the first request.
func NewHandler(d Deps) *Handler {
	if d.Shell == nil {
		panic("syllabus: NewHandler requires a non-nil Shell provider")
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

	shell := h.deps.Shell()
	current := findPath(shell.Nav, rel)
	if current == nil {
		http.NotFound(w, r)
		return
	}

	view := pages.BuildPathView(current, shell.Nav.Paths)
	if err := pages.Syllabus(view, shell.Chrome(r, current.Title)).Render(r.Context(), w); err != nil {
		h.deps.Log.Error("write syllabus page", "path", rel, "error", err)
	}
}

// findPath returns the study-path with the given vault-relative path, or nil
// when the model is nil or none matches.
func findPath(model *nav.Model, rel string) *nav.Map {
	if model == nil {
		return nil
	}
	for i := range model.Paths {
		if model.Paths[i].RelPath == rel {
			return &model.Paths[i]
		}
	}
	return nil
}
