// Package note is the reading feature: it loads a note from the vault,
// renders it, and serves the reading page.
package note

import (
	"errors"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/koopa0/kurodo/internal/render"
	"github.com/koopa0/kurodo/internal/ui/pages"
	"github.com/koopa0/kurodo/internal/vault"
)

// Handler serves reading pages for a vault rooted at Root.
type Handler struct {
	root     string
	renderer *render.Renderer
	log      *slog.Logger
}

// NewHandler wires the reading feature.
func NewHandler(root string, renderer *render.Renderer, log *slog.Logger) *Handler {
	return &Handler{root: root, renderer: renderer, log: log}
}

// Register mounts the feature's routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /notes/{path...}", h.show)
	mux.HandleFunc("GET /{$}", h.home)
}

// home is a placeholder until the M2 navigation lands.
func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/notes/README.md", http.StatusFound)
}

func (h *Handler) show(w http.ResponseWriter, r *http.Request) {
	rel := r.PathValue("path")

	n, err := vault.ReadNote(h.root, rel)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		http.NotFound(w, r)
		return
	case err != nil:
		h.log.Error("read note", "path", rel, "error", err)
		http.Error(w, "cannot read note", http.StatusInternalServerError)
		return
	}

	body, err := h.renderer.HTML(n.Body)
	if err != nil {
		h.log.Error("render note", "path", rel, "error", err)
		http.Error(w, "cannot render note", http.StatusInternalServerError)
		return
	}

	view := pages.NoteView{
		Title:      n.Title(),
		RelPath:    n.RelPath,
		Type:       n.Type(),
		Status:     n.Status(),
		Diagnostic: n.FMDiagnostic,
		BodyHTML:   body,
	}
	if err := pages.Note(view).Render(r.Context(), w); err != nil {
		h.log.Error("write note page", "path", rel, "error", err)
	}
}
