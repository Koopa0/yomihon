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

// StatusPolicy is the subset of the write face's state the reading page
// needs to decide which transition keys, if any, to offer. Defined by the
// consumer (rules/interfaces.md's cross-feature discovery case: note needs
// only a subset of *status.Service's behavior, never its write path —
// Flip is wall 1's business alone). *status.Service satisfies this.
type StatusPolicy interface {
	Closed() bool
	Transitions(noteType, current string) []string
}

// Handler serves reading pages for a vault rooted at Root.
type Handler struct {
	root      string
	renderer  *render.Renderer
	statusSvc StatusPolicy
	log       *slog.Logger
}

// NewHandler wires the reading feature. statusSvc supplies the write
// face's state for the page (which transition keys, if any, to show); a
// fail-closed Service is fine here, reading never depends on the contract —
// but statusSvc itself must not be nil (fail-closed is still a *Service
// with Closed() == true, not a missing one; mirrors status.NewHandler's
// same check on its sibling dependency).
func NewHandler(root string, renderer *render.Renderer, statusSvc StatusPolicy, log *slog.Logger) *Handler {
	if statusSvc == nil {
		panic("note: NewHandler requires a non-nil StatusPolicy")
	}
	return &Handler{root: root, renderer: renderer, statusSvc: statusSvc, log: log}
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

	// render.Renderer.HTML never fails the whole render (wall 4): a
	// content-level problem becomes a Diagnostic, not an error — there is
	// no error path left to handle here.
	result := h.renderer.HTML(n.Body)

	view := pages.NoteView{
		Title:             n.Title(),
		RelPath:           n.RelPath,
		Type:              n.Type(),
		Status:            n.Status(),
		Diagnostic:        n.FMDiagnostic,
		RenderDiagnostics: result.Diagnostics,
		TOC:               result.TOC,
		BodyHTML:          result.HTML,
		WriteClosed:       h.statusSvc.Closed(),
	}
	switch {
	case n.FMDiagnostic != "":
		// Bad YAML: diagnostic only, no keys — read isn't reliable enough
		// to write (wall 4).
	case n.Frontmatter == nil:
		// Legally no frontmatter (e.g. drills, per the contract's
		// no_frontmatter_is_legal): no keys either.
		view.NoFrontmatter = true
	default:
		view.Transitions = h.statusSvc.Transitions(n.Type(), n.Status())
	}
	if err := pages.Note(view).Render(r.Context(), w); err != nil {
		h.log.Error("write note page", "path", rel, "error", err)
	}
}
