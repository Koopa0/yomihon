// Package note is the reading feature: it loads a note from the vault,
// renders it, and serves the reading page.
package note

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/koopa0/kurodo/internal/nav"
	"github.com/koopa0/kurodo/internal/render"
	"github.com/koopa0/kurodo/internal/ui/pages"
	"github.com/koopa0/kurodo/internal/vault"
)

// sealStatus is the one primary status — the koopa-only seal. Only a ready note
// carries the seal display, so the handler gates its git-provenance read and the
// one-shot seal signal on this value; it duplicates no schema enum.
const sealStatus = "ready"

// StatusPolicy is the subset of the write face's state the reading page needs:
// whether the face is closed (Closed), which transition keys, if any, to offer
// (Transitions), and the stable note-status axis the Lifecycle rail lists
// (Order). It is a genuine slice of *status.Service — never its write path, Flip
// is wall 1's alone. *status.Service satisfies this.
type StatusPolicy interface {
	Closed() bool
	Transitions(noteType, current string) []string
	Order() []string
}

// Deps is everything the reading feature reads from. Grouping the providers in a
// struct keeps the constructor within the parameter budget as the page grew a
// per-status count source (Counts, from the snapshot's search index) and a
// read-only git-provenance source (Provenance, from the write face). Counts,
// Provenance, and Nav are plain closures because "give me the current value" is
// a closure, not a method set — the models live behind the atomic snapshot and
// must be read fresh per request.
type Deps struct {
	Root       string
	Renderer   *render.Renderer
	Status     StatusPolicy
	Nav        func() *nav.Model
	Counts     func() map[string]int
	Provenance func(ctx context.Context, rel string) (string, error)
	Log        *slog.Logger
}

// Handler serves reading pages for a vault rooted at Deps.Root.
type Handler struct {
	deps Deps
}

// NewHandler wires the reading feature. Every dependency must be non-nil: a nil
// is a wiring bug that must fail here, not on the first request three calls deep
// inside show(). A fail-closed write face is still a non-nil Status whose Closed
// reports true, not a missing one.
func NewHandler(d Deps) *Handler {
	if d.Renderer == nil {
		panic("note: NewHandler requires a non-nil Renderer")
	}
	if d.Status == nil {
		panic("note: NewHandler requires a non-nil Status")
	}
	if d.Nav == nil {
		panic("note: NewHandler requires a non-nil Nav provider")
	}
	if d.Counts == nil {
		panic("note: NewHandler requires a non-nil Counts provider")
	}
	if d.Provenance == nil {
		panic("note: NewHandler requires a non-nil Provenance provider")
	}
	if d.Log == nil {
		panic("note: NewHandler requires a non-nil Log")
	}
	return &Handler{deps: d}
}

// Register mounts the feature's routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /notes/{path...}", h.show)
	mux.HandleFunc("GET /{$}", h.home)
}

// home redirects to the vault README. The Vault-Index home page is deferred by
// design; until it is built, / lands on README.md.
func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/notes/README.md", http.StatusFound)
}

func (h *Handler) show(w http.ResponseWriter, r *http.Request) {
	rel := r.PathValue("path")

	n, err := vault.ReadNote(h.deps.Root, rel)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		http.NotFound(w, r)
		return
	case err != nil:
		h.deps.Log.Error("read note", "path", rel, "error", err)
		http.Error(w, "cannot read note", http.StatusInternalServerError)
		return
	}

	// render.Renderer.HTML never fails the whole render (wall 4): a content-level
	// problem becomes a Diagnostic, not an error — no error path left to handle.
	result := h.deps.Renderer.HTML(n.Body)

	view := pages.NoteView{
		Title:             n.Title(),
		RelPath:           n.RelPath,
		Type:              n.Type(),
		Status:            n.Status(),
		Diagnostic:        n.FMDiagnostic,
		RenderDiagnostics: result.Diagnostics,
		TOC:               result.TOC,
		BodyHTML:          result.HTML,
		Nav:               h.deps.Nav(),
		Lifecycle:         h.lifecycle(n.Status()),
		WriteClosed:       h.deps.Status.Closed(),
	}
	switch {
	case n.FMDiagnostic != "":
		// Bad YAML: diagnostic only, no keys — read isn't reliable enough to
		// write (wall 4).
	case n.Frontmatter == nil:
		// Legally no frontmatter (e.g. drills): no keys either.
		view.NoFrontmatter = true
	default:
		view.Transitions = h.deps.Status.Transitions(n.Type(), n.Status())
	}

	// A ready note shows the seal, so only then spend a git read for the commit
	// short-hash and honor the one-shot ?sealed=1 ceremony signal.
	if n.Status() == sealStatus {
		view.JustSealed = r.URL.Query().Get("sealed") == "1"
		if hash, herr := h.deps.Provenance(r.Context(), rel); herr == nil {
			view.SealedHash = hash
		} else {
			h.deps.Log.Warn("seal provenance", "path", rel, "error", herr)
		}
	}

	if err := pages.Note(view, pages.ChromeFromRequest(r, n.Title())).Render(r.Context(), w); err != nil {
		h.deps.Log.Error("write note page", "path", rel, "error", err)
	}
}

// lifecycle assembles the status-first Lifecycle rail: the note group's statuses
// in the contract's toml order (Status.Order — never hardcoded), each with its
// live snapshot count and whether it is the current note's status. Empty when
// the write face is closed.
func (h *Handler) lifecycle(current string) []pages.LifecycleItem {
	order := h.deps.Status.Order()
	if len(order) == 0 {
		return nil
	}
	counts := h.deps.Counts()
	items := make([]pages.LifecycleItem, 0, len(order))
	for _, s := range order {
		items = append(items, pages.LifecycleItem{
			Name:   s,
			Count:  counts[s],
			Active: s == current,
		})
	}
	return items
}
