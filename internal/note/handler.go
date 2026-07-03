// Package note is the reading feature: it loads a note from the vault,
// renders it, and serves the reading page.
package note

import (
	"errors"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/koopa0/kurodo/internal/nav"
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
//
// nav is a provider closure returning the CURRENT whole-vault sidebar model,
// not a fixed *nav.Model: the model is one of the three live-Snapshot models
// (internal/snapshot, D25), rebuilt on any vault change, so the reading page
// must read it fresh per request rather than capture a startup value. It is a
// plain function, not a consumer interface — the reading page and its templ
// (pages.NoteView.Nav) both depend on the exact concrete *nav.Model, so an
// interface would hide no behavioral subset (unlike StatusPolicy/render.Resolver,
// which are genuine slices of a larger type). Held as a func because "give me
// the current value" is a closure, not a method set.
type Handler struct {
	root      string
	renderer  *render.Renderer
	statusSvc StatusPolicy
	nav       func() *nav.Model
	log       *slog.Logger
}

// NewHandler wires the reading feature. statusSvc supplies the write
// face's state for the page (which transition keys, if any, to show); a
// fail-closed Service is fine here, reading never depends on the contract —
// but statusSvc itself must not be nil (fail-closed is still a *Service
// with Closed() == true, not a missing one; mirrors status.NewHandler's
// same check on its sibling dependency). navigation returns the current
// sidebar model on each request; it must not be nil (a wiring bug), though the
// model it returns may be an empty/partial one when navigation was unavailable
// (§0.1).
func NewHandler(root string, renderer *render.Renderer, statusSvc StatusPolicy, navigation func() *nav.Model, log *slog.Logger) *Handler {
	if statusSvc == nil {
		panic("note: NewHandler requires a non-nil StatusPolicy")
	}
	if navigation == nil {
		panic("note: NewHandler requires a non-nil navigation provider")
	}
	return &Handler{root: root, renderer: renderer, statusSvc: statusSvc, nav: navigation, log: log}
}

// Register mounts the feature's routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /notes/{path...}", h.show)
	mux.HandleFunc("GET /{$}", h.home)
}

// home redirects to the vault README. The navigation sidebar (spec §2) has
// already landed — every /notes page renders the folder tree, syllabi, and
// reports — so this is not deferred navigation work. What is still deferred,
// deliberately and design-led, is the Vault-Index home page (spec §2's four
// stable blocks: the domain-MOC entry points, the cross-domain boards, the
// mechanical-gate list, and the governing-document pointers). That deferral
// is by design, not gated behind any milestone (D15: there is no M1/M2).
// Until the home page is built, / lands on README.md.
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
		Nav:               h.nav(),
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
